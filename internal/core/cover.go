package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/arunoruto/rommate/internal/romm"
)

// coverExts are the image types ES-DE reads, in the order rommate looks for
// existing covers.
var coverExts = []string{".png", ".jpg"}

// maxCoverSize guards against a misbehaving server filling the disk.
const maxCoverSize = 32 << 20

// CoverPath returns where ES-DE looks for the cover of a game stored at
// romPath, without extension: <media>/<system>/covers/<name>. ES-DE names
// media after the ROM file without its extension, or after the folder of a
// multi-file game. Empty when covers are off.
func (e *Engine) CoverPath(r romm.Rom, romPath string) string {
	dir := e.Mapper.MediaDir(r.PlatformSlug)
	if dir == "" || !e.Covers {
		return ""
	}
	name := filepath.Base(romPath)
	if isSingleFile(r) {
		name = strings.TrimSuffix(name, filepath.Ext(name))
	}
	return filepath.Join(dir, "covers", name)
}

// existingCover returns the cover file at base with any supported extension.
func existingCover(base string) string {
	for _, ext := range coverExts {
		if _, err := os.Stat(base + ext); err == nil {
			return base + ext
		}
	}
	return ""
}

// FetchCover downloads a game's cover from RomM into ES-DE's media folder,
// unless covers are off, the game has none or one is already there. It
// returns the cover's path ("" when nothing was written).
func (e *Engine) FetchCover(ctx context.Context, r romm.Rom, romPath string) (string, error) {
	base := e.CoverPath(r, romPath)
	if base == "" || r.PathCoverLarge == "" || existingCover(base) != "" {
		return "", nil
	}
	if !strings.HasPrefix(r.PathCoverLarge, "/") {
		return "", fmt.Errorf("unexpected cover path %q", r.PathCoverLarge)
	}
	body, err := e.Client.Asset(ctx, r.PathCoverLarge)
	if err != nil {
		return "", err
	}
	defer body.Close()
	data, err := io.ReadAll(io.LimitReader(body, maxCoverSize+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxCoverSize {
		return "", errors.New("cover image is too large")
	}
	// RomM's file names don't say the format (big.png is often a JPEG), and
	// ES-DE picks the decoder by extension, so look at the data.
	var ext string
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG")):
		ext = ".png"
	case bytes.HasPrefix(data, []byte("\xff\xd8\xff")):
		ext = ".jpg"
	default:
		return "", errors.New("cover is not a PNG or JPEG image")
	}
	if err := os.MkdirAll(filepath.Dir(base), 0o755); err != nil {
		return "", err
	}
	tmp := base + ext + ".part"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, base+ext); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return base + ext, nil
}

// removeCover deletes a game's cover, if rommate's media folder has one.
func (e *Engine) removeCover(r romm.Rom, romPath string) {
	base := e.CoverPath(r, romPath)
	if base == "" {
		return
	}
	for _, ext := range coverExts {
		_ = os.Remove(base + ext)
	}
}

// CoverResult reports one game of FetchCovers.
type CoverResult struct {
	RomID int    `json:"rom_id"`
	Name  string `json:"name"`
	Path  string `json:"path,omitempty"`  // written cover; empty if none was needed
	Error string `json:"error,omitempty"` // why it failed
}

// FetchCovers adds missing covers for every downloaded game.
func (e *Engine) FetchCovers(ctx context.Context, report func(CoverResult)) error {
	if !e.Covers || e.Mapper.Media == "" {
		return errors.New("covers are turned off (covers = false in config.toml)")
	}
	entries, err := e.Store.All()
	if err != nil {
		return err
	}
	for _, en := range entries {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		res := CoverResult{RomID: en.RomID, Name: en.Name}
		r, err := e.Client.Rom(ctx, en.RomID)
		if err == nil {
			res.Path, err = e.FetchCover(ctx, r, en.Path)
		}
		if err != nil {
			res.Error = err.Error()
		}
		report(res)
	}
	return nil
}
