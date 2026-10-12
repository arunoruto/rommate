package saves

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/arunoruto/rommate/internal/romm"
	"github.com/arunoruto/rommate/internal/state"
)

// Slot is the RomM save slot rommate syncs. Other RomM clients (the web
// player, Argosy) use the same name, so saves are shared with them.
const Slot = "autosave"

// Game is a downloaded game whose saves may be synced.
type Game struct {
	RomID    int
	Platform string // RomM platform slug
	Name     string
	Path     string // the ROM file, or its folder for multi-file games
}

// LocalSave is a save found on disk for a game.
type LocalSave struct {
	Emulator string // handler id, stored on the server
	Path     string // file, or folder for folder saves
	IsDir    bool
	// Prefix limits a folder save to the top-level entries starting with it
	// (PPSSPP keeps several SAVEDATA folders per game, named by disc ID).
	Prefix   string
	FileName string // name to upload under (folders get .zip)
}

// Handler knows where one emulator keeps its saves.
type Handler interface {
	// ID is the emulator name stored with the save on the server.
	ID() string
	// Find returns the game's local save, if there is one.
	Find(g Game) (LocalSave, bool, error)
	// Target returns where a downloaded save for the game goes when there is
	// no local save yet.
	Target(g Game) (LocalSave, error)
}

// Registry picks the handler for a platform.
type Registry interface {
	For(platform string) Handler
}

// Engine syncs saves of downloaded games with RomM.
type Engine struct {
	Client   *romm.Client
	Store    *state.Store
	Handlers Registry
	DataDir  string // backups and temporary uploads live here
	Version  string // rommate version, sent when registering the device
	// Now returns the current time (tests replace it).
	Now func() time.Time
}

// Event reports progress of a sync, for the CLI, TUI and JSON output.
type Event struct {
	Action string `json:"action"` // upload, download, conflict, in_sync, skipped, error
	RomID  int    `json:"rom_id"`
	Name   string `json:"name"`
	Detail string `json:"detail,omitempty"`
}

// Result summarises a sync.
type Result struct {
	Uploaded   int `json:"uploaded"`
	Downloaded int `json:"downloaded"`
	Conflicts  int `json:"conflicts"`
	InSync     int `json:"in_sync"`
	Failed     int `json:"failed"`
}

type entry struct {
	game  Game
	save  LocalSave
	file  string // what gets uploaded: the save file or a zip of the folder
	hash  string
	mtime time.Time
	size  int64
}

func (e *Engine) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

// DeviceID returns this machine's RomM device id, registering it once.
func (e *Engine) DeviceID(ctx context.Context) (string, error) {
	if id, err := e.Store.GetValue("device_id"); err != nil || id != "" {
		return id, err
	}
	host, _ := os.Hostname()
	if host == "" {
		host = "rommate"
	}
	id, err := e.Client.RegisterDevice(ctx, romm.DeviceRequest{
		Name: host, Platform: runtime.GOOS + "/" + runtime.GOARCH, Client: "rommate",
		ClientVersion: e.Version, Hostname: host, SyncMode: "api", AllowExisting: true,
	})
	if err != nil {
		return "", fmt.Errorf("registering this device with RomM: %w", err)
	}
	return id, e.Store.SetValue("device_id", id)
}

// Sync syncs the saves of games with RomM.
func (e *Engine) Sync(ctx context.Context, games []Game, emit func(Event)) (Result, error) {
	var res Result
	deviceID, err := e.DeviceID(ctx)
	if err != nil {
		return res, err
	}
	tmp, err := os.MkdirTemp(e.DataDir, "sync-")
	if err != nil {
		return res, err
	}
	defer os.RemoveAll(tmp)

	local := map[int]*entry{}
	byRom := map[int]Game{}
	var req romm.NegotiateRequest
	req.DeviceID = deviceID
	for _, g := range games {
		byRom[g.RomID] = g
		h := e.Handlers.For(g.Platform)
		if h == nil {
			continue
		}
		req.RomIDs = append(req.RomIDs, g.RomID)
		s, ok, err := h.Find(g)
		if err != nil {
			emit(Event{Action: "error", RomID: g.RomID, Name: g.Name, Detail: err.Error()})
			continue
		}
		if !ok {
			continue
		}
		en, err := prepare(g, s, tmp)
		if err != nil {
			emit(Event{Action: "error", RomID: g.RomID, Name: g.Name, Detail: err.Error()})
			continue
		}
		local[g.RomID] = en
		req.Saves = append(req.Saves, romm.ClientSave{
			RomID: g.RomID, FileName: s.FileName, Slot: Slot, Emulator: s.Emulator,
			ContentHash: en.hash, UpdatedAt: en.mtime.UTC(), FileSizeBytes: en.size,
		})
	}

	plan, err := e.Client.Negotiate(ctx, req)
	if err != nil {
		return res, fmt.Errorf("negotiating with RomM: %w", err)
	}

	done, failed := 0, 0
	for _, op := range plan.Operations {
		if op.Slot != Slot {
			continue
		}
		g, isLocal := byRom[op.RomID]
		en := local[op.RomID]
		var err error
		switch op.Action {
		case "upload":
			if en == nil {
				continue
			}
			err = e.upload(ctx, en, deviceID, plan.SessionID, true)
			if err == nil {
				res.Uploaded++
				emit(Event{Action: "upload", RomID: g.RomID, Name: g.Name})
			}
		case "download":
			// Older servers offer saves of every game; only take ours.
			if !isLocal || e.Handlers.For(g.Platform) == nil || op.SaveID == nil {
				continue
			}
			// Saves from a different emulator may use another format.
			if h := e.Handlers.For(g.Platform); op.Emulator != "" && op.Emulator != h.ID() {
				emit(Event{Action: "skipped", RomID: g.RomID, Name: g.Name,
					Detail: "the server copy was made with " + op.Emulator})
				continue
			}
			err = e.download(ctx, g, en, op, deviceID, plan.SessionID)
			if err == nil {
				res.Downloaded++
				emit(Event{Action: "download", RomID: g.RomID, Name: g.Name})
			}
		case "conflict":
			if en == nil || op.SaveID == nil {
				continue
			}
			err = e.recordConflict(en, *op.SaveID, op.ServerUpdatedAt)
			if err == nil {
				res.Conflicts++
				emit(Event{Action: "conflict", RomID: g.RomID, Name: g.Name, Detail: op.Reason})
			}
		case "no_op":
			if en == nil {
				continue
			}
			var uploaded, conflicted bool
			uploaded, conflicted, err = e.reconcileNoOp(ctx, en, op, deviceID, plan.SessionID)
			switch {
			case err != nil:
			case uploaded:
				res.Uploaded++
				emit(Event{Action: "upload", RomID: g.RomID, Name: g.Name})
			case conflicted:
				res.Conflicts++
				emit(Event{Action: "conflict", RomID: g.RomID, Name: g.Name, Detail: "changed here and on the server"})
			default:
				res.InSync++
			}
		default: // "delete" (RomM 5.4): never delete local saves automatically
			continue
		}
		if err != nil {
			failed++
			res.Failed++
			emit(Event{Action: "error", RomID: op.RomID, Name: g.Name, Detail: err.Error()})
			continue
		}
		done++
	}
	if err := e.Client.CompleteSession(ctx, plan.SessionID, done, failed); err != nil {
		return res, fmt.Errorf("completing the sync session: %w", err)
	}
	return res, nil
}

// prepare hashes a save (zipping folders) so it can be negotiated and uploaded.
func prepare(g Game, s LocalSave, tmp string) (*entry, error) {
	en := &entry{game: g, save: s, file: s.Path}
	if s.IsDir {
		en.file = filepath.Join(tmp, fmt.Sprintf("%d-%s", g.RomID, s.FileName))
		f, err := os.Create(en.file)
		if err != nil {
			return nil, err
		}
		if err := ZipPrefix(s.Path, s.Prefix, f); err != nil {
			f.Close()
			return nil, err
		}
		if err := f.Close(); err != nil {
			return nil, err
		}
	}
	var err error
	if en.hash, err = Hash(en.file); err != nil {
		return nil, err
	}
	if en.mtime, err = latestMTime(s.Path, s.Prefix); err != nil {
		return nil, err
	}
	st, err := os.Stat(en.file)
	if err != nil {
		return nil, err
	}
	en.size = st.Size()
	return en, nil
}

func latestMTime(path, prefix string) (time.Time, error) {
	st, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	if !st.IsDir() {
		return st.ModTime(), nil
	}
	var latest time.Time
	err = filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(path, p)
		if rel != "." && !hasPrefix(filepath.ToSlash(rel), prefix) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if rel != "." && info.ModTime().After(latest) {
			latest = info.ModTime()
		}
		return nil
	})
	return latest, err
}

func (e *Engine) key(en *entry) state.SaveKey {
	return state.SaveKey{RomID: en.game.RomID, Slot: Slot, Emulator: en.save.Emulator}
}

func (e *Engine) upload(ctx context.Context, en *entry, deviceID string, session int, overwrite bool) error {
	f, err := os.Open(en.file)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = e.Client.UploadSave(ctx, romm.UploadParams{
		RomID: en.game.RomID, Slot: Slot, Emulator: en.save.Emulator, DeviceID: deviceID,
		ContentHash: en.hash, SessionID: session, Overwrite: overwrite,
	}, en.save.FileName, f)
	if err != nil {
		return err
	}
	if err := e.Store.DeleteConflict(e.key(en)); err != nil {
		return err
	}
	return e.Store.PutBaseline(state.Baseline{SaveKey: e.key(en), Hash: en.hash, SyncedAt: e.now()})
}

// reconcileNoOp handles a server "nothing to do". Before RomM 5.4 the server
// only compares timestamps, so a local edit whose file time is not newer than
// the last sync (clock skew, restored files) is missed. If the save differs
// from both the last synced state and the server, push it without
// overwriting: a 409 means the server changed too, which is a conflict.
func (e *Engine) reconcileNoOp(ctx context.Context, en *entry, op romm.SyncOp, deviceID string, session int) (uploaded, conflicted bool, err error) {
	base, ok, err := e.Store.GetBaseline(e.key(en))
	if err != nil {
		return false, false, err
	}
	if en.hash == op.ServerContentHash || !ok || en.hash == base.Hash {
		return false, false, e.Store.PutBaseline(state.Baseline{SaveKey: e.key(en), Hash: en.hash, SyncedAt: e.now()})
	}
	err = e.upload(ctx, en, deviceID, session, false)
	var apiErr *romm.APIError
	if errors.As(err, &apiErr) && apiErr.Status == 409 && op.SaveID != nil {
		return false, true, e.recordConflict(en, *op.SaveID, op.ServerUpdatedAt)
	}
	return err == nil, false, err
}

func (e *Engine) recordConflict(en *entry, serverSaveID int, serverUpdated *time.Time) error {
	c := state.Conflict{SaveKey: e.key(en), ServerSaveID: serverSaveID, LocalHash: en.hash, DetectedAt: e.now()}
	if serverUpdated != nil {
		c.ServerUpdatedAt = *serverUpdated
	}
	return e.Store.PutConflict(c)
}

func (e *Engine) download(ctx context.Context, g Game, en *entry, op romm.SyncOp, deviceID string, session int) error {
	h := e.Handlers.For(g.Platform)
	var target LocalSave
	if en != nil {
		target = en.save
	} else {
		var err error
		if target, err = h.Target(g); err != nil {
			return err
		}
	}
	body, err := e.Client.DownloadSave(ctx, *op.SaveID, deviceID, session)
	if err != nil {
		return err
	}
	defer body.Close()
	hash, err := e.restore(g, target, body, op.ServerUpdatedAt)
	if err != nil {
		return err
	}
	if err := e.Client.ConfirmDownloaded(ctx, *op.SaveID, deviceID, hash); err != nil {
		return err
	}
	k := state.SaveKey{RomID: g.RomID, Slot: Slot, Emulator: target.Emulator}
	if err := e.Store.DeleteConflict(k); err != nil {
		return err
	}
	return e.Store.PutBaseline(state.Baseline{SaveKey: k, Hash: hash, SyncedAt: e.now()})
}

// restore backs up the current save, writes the downloaded one in its place
// and returns the RomM hash of what was written.
func (e *Engine) restore(g Game, target LocalSave, body io.Reader, serverTime *time.Time) (string, error) {
	if err := os.MkdirAll(e.DataDir, 0o755); err != nil {
		return "", err
	}
	dl, err := os.CreateTemp(e.DataDir, "download-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(dl.Name())
	if _, err := io.Copy(dl, body); err != nil {
		dl.Close()
		return "", err
	}
	if err := dl.Close(); err != nil {
		return "", err
	}
	hash, err := Hash(dl.Name())
	if err != nil {
		return "", err
	}
	if err := e.backup(g, target); err != nil {
		return "", fmt.Errorf("backing up the current save: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(target.Path), 0o755); err != nil {
		return "", err
	}
	switch {
	case target.IsDir && target.Prefix != "":
		err = UnzipPrefix(dl.Name(), target.Path, target.Prefix)
	case target.IsDir:
		err = UnzipDir(dl.Name(), target.Path)
	default:
		err = copyFileAtomic(dl.Name(), target.Path)
	}
	if err != nil {
		return "", err
	}
	if serverTime != nil && target.Prefix == "" {
		// Matching the server's time keeps timestamp-only servers from
		// seeing a fresh local change.
		_ = os.Chtimes(target.Path, *serverTime, *serverTime)
	}
	return hash, nil
}

func copyFileAtomic(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dest + ".rommate-tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dest)
}

// maxBackups is how many old versions of each save are kept.
const maxBackups = 5

// BackupDir returns where backups of a game's saves are kept.
func (e *Engine) BackupDir(romID int) string {
	return filepath.Join(e.DataDir, "save-backups", fmt.Sprint(romID))
}

// backup copies the current save (if any) into the backup folder and prunes
// old backups.
func (e *Engine) backup(g Game, s LocalSave) error {
	st, err := os.Stat(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if s.Prefix != "" {
		if m, _ := filepath.Glob(filepath.Join(s.Path, s.Prefix+"*")); len(m) == 0 {
			return nil // nothing of this game to back up yet
		}
	}
	dir := e.BackupDir(g.RomID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	name := fmt.Sprintf("%s_%s", e.now().UTC().Format("20060102-150405"), s.FileName)
	dest := filepath.Join(dir, name)
	if st.IsDir() {
		f, err := os.Create(dest)
		if err != nil {
			return err
		}
		if err := ZipPrefix(s.Path, s.Prefix, f); err != nil {
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
	} else if err := copyFileAtomic(s.Path, dest); err != nil {
		return err
	}
	return pruneBackups(dir, s.FileName)
}

func pruneBackups(dir, fileName string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var mine []string
	for _, en := range entries {
		if strings.HasSuffix(en.Name(), "_"+fileName) {
			mine = append(mine, en.Name())
		}
	}
	sort.Strings(mine) // timestamp prefix sorts oldest first
	for len(mine) > maxBackups {
		if err := os.Remove(filepath.Join(dir, mine[0])); err != nil {
			return err
		}
		mine = mine[1:]
	}
	return nil
}

// Status describes the save of one downloaded game.
type Status struct {
	RomID    int    `json:"rom_id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Emulator string `json:"emulator,omitempty"`
	Path     string `json:"path,omitempty"` // empty: no local save yet
	Conflict bool   `json:"conflict"`
	Problem  string `json:"problem,omitempty"`
}

// Status lists the local save state of games, without contacting the server.
func (e *Engine) Status(games []Game) ([]Status, error) {
	conflicts, err := e.Store.Conflicts()
	if err != nil {
		return nil, err
	}
	inConflict := map[int]bool{}
	for _, c := range conflicts {
		inConflict[c.RomID] = true
	}
	var out []Status
	for _, g := range games {
		st := Status{RomID: g.RomID, Name: g.Name, Platform: g.Platform, Conflict: inConflict[g.RomID]}
		h := e.Handlers.For(g.Platform)
		if h == nil {
			st.Problem = "no supported emulator found for this platform"
			out = append(out, st)
			continue
		}
		st.Emulator = h.ID()
		s, ok, err := h.Find(g)
		switch {
		case err != nil:
			st.Problem = err.Error()
		case ok:
			st.Path = s.Path
		}
		out = append(out, st)
	}
	return out, nil
}

// Resolve settles a conflict: "local" uploads the local save over the
// server's, "server" replaces the local save (after a backup).
func (e *Engine) Resolve(ctx context.Context, g Game, keep string) error {
	h := e.Handlers.For(g.Platform)
	if h == nil {
		return fmt.Errorf("no supported emulator for %s", g.Platform)
	}
	conflicts, err := e.Store.Conflicts()
	if err != nil {
		return err
	}
	var c *state.Conflict
	for i := range conflicts {
		if conflicts[i].RomID == g.RomID {
			c = &conflicts[i]
		}
	}
	if c == nil {
		return fmt.Errorf("%s has no save conflict", g.Name)
	}
	deviceID, err := e.DeviceID(ctx)
	if err != nil {
		return err
	}
	switch keep {
	case "local":
		s, ok, err := h.Find(g)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("the local save of %s is gone; resolve with --keep server", g.Name)
		}
		tmp, err := os.MkdirTemp(e.DataDir, "resolve-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmp)
		en, err := prepare(g, s, tmp)
		if err != nil {
			return err
		}
		return e.upload(ctx, en, deviceID, 0, true)
	case "server":
		op := romm.SyncOp{RomID: g.RomID, SaveID: &c.ServerSaveID, ServerUpdatedAt: &c.ServerUpdatedAt}
		var en *entry
		if s, ok, err := h.Find(g); err == nil && ok {
			en = &entry{game: g, save: s}
		}
		return e.download(ctx, g, en, op, deviceID, 0)
	}
	return fmt.Errorf("keep must be local or server, not %q", keep)
}
