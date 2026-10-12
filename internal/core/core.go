// Package core is rommate's frontend-independent logic. The CLI, the TUI and
// any future integration (e.g. a Decky plugin via `--json`) sit on top of it.
package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/arunoruto/rommate/internal/folders"
	"github.com/arunoruto/rommate/internal/romm"
	"github.com/arunoruto/rommate/internal/state"
)

// Engine ties the RomM client, folder mapping and local state together.
type Engine struct {
	Client *romm.Client
	Mapper folders.Mapper
	Store  *state.Store
	// Covers downloads cover art into the frontend's media folder.
	Covers bool
	// Warn, if set, receives problems that don't fail an operation, such as a
	// cover that could not be fetched.
	Warn func(msg string)
}

// EventType identifies an Event.
type EventType string

const (
	EventStart    EventType = "start"
	EventProgress EventType = "progress"
	EventDone     EventType = "done"
	EventSkipped  EventType = "skipped"
	EventError    EventType = "error"
)

// Event reports download progress. It is also the JSON-lines wire format.
type Event struct {
	Type  EventType `json:"event"`
	RomID int       `json:"rom_id"`
	Name  string    `json:"name"`
	File  string    `json:"file,omitempty"`
	Path  string    `json:"path,omitempty"`
	Done  int64     `json:"done,omitempty"`
	Total int64     `json:"total,omitempty"`
	Error string    `json:"error,omitempty"`
}

// Emit receives events. It must not block for long.
type Emit func(Event)

// PlatformInfo is a server platform plus where it lands locally.
type PlatformInfo struct {
	romm.Platform
	Folder string `json:"folder"`
	Dir    string `json:"dir"`
}

// Platforms lists server platforms that have ROMs, sorted by name.
func (e *Engine) Platforms(ctx context.Context) ([]PlatformInfo, error) {
	ps, err := e.Client.Platforms(ctx)
	if err != nil {
		return nil, err
	}
	var out []PlatformInfo
	for _, p := range ps {
		if p.RomCount == 0 {
			continue
		}
		out = append(out, PlatformInfo{Platform: p, Folder: e.Mapper.Folder(p.Slug), Dir: e.Mapper.Dir(p.Slug)})
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].DisplayName()) < strings.ToLower(out[j].DisplayName())
	})
	return out, nil
}

// RomInfo is a server ROM plus its local install state.
type RomInfo struct {
	romm.Rom
	Installed bool   `json:"installed"`
	Path      string `json:"path"`
}

// Roms lists a platform's ROMs with their local install state.
func (e *Engine) Roms(ctx context.Context, platformID int) ([]RomInfo, error) {
	roms, err := e.Client.Roms(ctx, platformID)
	if err != nil {
		return nil, err
	}
	out := make([]RomInfo, 0, len(roms))
	for _, r := range roms {
		path := e.LocalPath(r)
		out = append(out, RomInfo{Rom: r, Path: path, Installed: e.isComplete(r, path)})
	}
	return out, nil
}

// Search finds games on every platform by name, with their local install
// state. At most limit results are returned.
func (e *Engine) Search(ctx context.Context, term string, limit int) ([]RomInfo, error) {
	roms, err := e.Client.Search(ctx, term, limit)
	if err != nil {
		return nil, err
	}
	out := make([]RomInfo, 0, len(roms))
	for _, r := range roms {
		path := e.LocalPath(r)
		out = append(out, RomInfo{Rom: r, Path: path, Installed: e.isComplete(r, path)})
	}
	return out, nil
}

// LocalPath is where a ROM is stored: the file itself for single-file ROMs,
// or a folder named after the ROM for multi-file ROMs.
func (e *Engine) LocalPath(r romm.Rom) string {
	dir := e.Mapper.Dir(r.PlatformSlug)
	if isSingleFile(r) {
		return filepath.Join(dir, safeName(r.Files[0].FileName))
	}
	return filepath.Join(dir, safeName(r.FSName))
}

func isSingleFile(r romm.Rom) bool {
	return !r.HasMultipleFiles && len(r.Files) == 1
}

// isComplete reports whether every file of r exists locally at full size.
func (e *Engine) isComplete(r romm.Rom, path string) bool {
	if len(r.Files) == 0 {
		return false
	}
	for _, f := range r.Files {
		dest, err := filePath(r, path, f)
		if err != nil {
			return false
		}
		st, err := os.Stat(dest)
		if err != nil || st.Size() != f.FileSizeBytes {
			return false
		}
	}
	return true
}

// Pull downloads a ROM. Files already present at the right size are kept, so
// pulling is idempotent and adopts files downloaded by other tools.
func (e *Engine) Pull(ctx context.Context, r romm.Rom, emit Emit) error {
	if len(r.Files) == 0 {
		// List responses may omit files; fetch the full record.
		full, err := e.Client.Rom(ctx, r.ID)
		if err != nil {
			return e.fail(emit, r, err)
		}
		r = full
	}
	path := e.LocalPath(r)
	if e.isComplete(r, path) {
		if err := e.record(r, path); err != nil {
			return e.fail(emit, r, err)
		}
		e.cover(ctx, r, path)
		emit(Event{Type: EventSkipped, RomID: r.ID, Name: r.Title(), Path: path})
		return nil
	}

	emit(Event{Type: EventStart, RomID: r.ID, Name: r.Title(), Path: path, Total: totalSize(r)})
	var done int64
	for _, f := range r.Files {
		dest, err := filePath(r, path, f)
		if err != nil {
			return e.fail(emit, r, err)
		}
		base := done
		err = downloadFile(ctx, e.Client, r.ID, f, dest, func(n int64) {
			emit(Event{Type: EventProgress, RomID: r.ID, Name: r.Title(), File: f.FileName,
				Done: base + n, Total: totalSize(r)})
		})
		if err != nil {
			return e.fail(emit, r, err)
		}
		done += f.FileSizeBytes
	}
	if err := e.record(r, path); err != nil {
		return e.fail(emit, r, err)
	}
	e.cover(ctx, r, path)
	emit(Event{Type: EventDone, RomID: r.ID, Name: r.Title(), Path: path, Done: done, Total: done})
	return nil
}

// Remove deletes a downloaded ROM from disk and from the state database.
func (e *Engine) Remove(r romm.Rom) error {
	path := e.LocalPath(r)
	if !strings.HasPrefix(path, filepath.Clean(e.Mapper.Root)+string(filepath.Separator)) {
		return fmt.Errorf("refusing to delete %s: outside ROM root", path)
	}
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	e.removeCover(r, path)
	return e.Store.Delete(r.ID)
}

// StatusEntry is a recorded download plus whether it is still on disk.
type StatusEntry struct {
	state.Entry
	Present bool `json:"present"`
}

// Status lists recorded downloads and checks they still exist.
func (e *Engine) Status() ([]StatusEntry, error) {
	entries, err := e.Store.All()
	if err != nil {
		return nil, err
	}
	out := make([]StatusEntry, 0, len(entries))
	for _, en := range entries {
		_, err := os.Stat(en.Path)
		out = append(out, StatusEntry{Entry: en, Present: err == nil})
	}
	return out, nil
}

// DownloadedCounts returns how many games are downloaded per platform slug.
func (e *Engine) DownloadedCounts() (map[string]int, error) {
	return e.Store.CountByPlatform()
}

// cover fetches the cover of a pulled game. A missing cover never fails the
// download; `rommate covers` retries later.
func (e *Engine) cover(ctx context.Context, r romm.Rom, path string) {
	if _, err := e.FetchCover(ctx, r, path); err != nil && e.Warn != nil {
		e.Warn(fmt.Sprintf("cover of %s: %v", r.Title(), err))
	}
}

func (e *Engine) record(r romm.Rom, path string) error {
	return e.Store.Put(state.Entry{
		RomID: r.ID, PlatformSlug: r.PlatformSlug, Name: r.Title(),
		Path: path, SizeBytes: totalSize(r), DownloadedAt: time.Now(),
	})
}

func (e *Engine) fail(emit Emit, r romm.Rom, err error) error {
	emit(Event{Type: EventError, RomID: r.ID, Name: r.Title(), Error: err.Error()})
	return err
}

func totalSize(r romm.Rom) int64 {
	var n int64
	for _, f := range r.Files {
		n += f.FileSizeBytes
	}
	return n
}

// filePath returns where file f of rom r is written, keeping any sub-folder
// structure inside multi-file ROMs.
func filePath(r romm.Rom, romPath string, f romm.RomFile) (string, error) {
	if isSingleFile(r) {
		return romPath, nil
	}
	rel := f.FileName
	// file_path is the server-side directory of the file; keep whatever comes
	// after the ROM's own folder name.
	if i := strings.Index(f.FilePath, r.FSName); i >= 0 {
		sub := strings.Trim(f.FilePath[i+len(r.FSName):], "/")
		if sub != "" {
			rel = sub + "/" + f.FileName
		}
	}
	rel = filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe file path from server: %q", f.FileName)
	}
	return filepath.Join(romPath, rel), nil
}

// safeName strips path separators so a server-provided name stays one element.
func safeName(name string) string {
	name = strings.NewReplacer("/", "_", "\\", "_").Replace(name)
	if name == "" || name == "." || name == ".." {
		return "_"
	}
	return name
}

// ErrBusy means another rommate process is downloading the same file.
var ErrBusy = errors.New("this file is already being downloaded by another rommate process")
