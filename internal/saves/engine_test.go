package saves

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/arunoruto/rommate/internal/romm"
	"github.com/arunoruto/rommate/internal/state"
)

// fileHandler keeps one save per game at <dir>/<rom_id>.srm.
type fileHandler struct{ dir string }

func (h fileHandler) ID() string { return "test-emu" }
func (h fileHandler) path(g Game) string {
	return filepath.Join(h.dir, strconv.Itoa(g.RomID)+".srm")
}
func (h fileHandler) Find(g Game) (LocalSave, bool, error) {
	if _, err := os.Stat(h.path(g)); err != nil {
		return LocalSave{}, false, nil
	}
	s, _ := h.Target(g)
	return s, true, nil
}
func (h fileHandler) Target(g Game) (LocalSave, error) {
	return LocalSave{Emulator: h.ID(), Path: h.path(g), FileName: strconv.Itoa(g.RomID) + ".srm"}, nil
}

type oneRegistry struct{ h Handler }

func (r oneRegistry) For(string) Handler { return r.h }

// fakeRomM answers negotiate with a scripted plan and records calls.
type fakeRomM struct {
	mu        sync.Mutex
	plan      func(req romm.NegotiateRequest) []romm.SyncOp
	uploads   map[int]string // rom_id -> uploaded content
	conflict  map[int]bool   // rom_id -> answer uploads without overwrite with 409
	content   map[int]string // save_id -> content served
	confirmed map[int]string // save_id -> hash confirmed
	completed [2]int
}

func (f *fakeRomM) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case r.URL.Path == "/api/devices":
		_ = json.NewEncoder(w).Encode(map[string]string{"device_id": "dev-1"})
	case r.URL.Path == "/api/sync/negotiate":
		var req romm.NegotiateRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		_ = json.NewEncoder(w).Encode(romm.NegotiateResponse{SessionID: 7, Operations: f.plan(req)})
	case r.URL.Path == "/api/saves" && r.Method == http.MethodPost:
		rom, _ := strconv.Atoi(r.URL.Query().Get("rom_id"))
		if r.URL.Query().Get("overwrite") == "false" && f.conflict[rom] {
			w.WriteHeader(http.StatusConflict)
			return
		}
		file, _, err := r.FormFile("saveFile")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		b, _ := io.ReadAll(file)
		f.uploads[rom] = string(b)
		_ = json.NewEncoder(w).Encode(romm.Save{ID: 100 + rom, RomID: rom})
	case strings.HasSuffix(r.URL.Path, "/content"):
		id, _ := strconv.Atoi(strings.Split(r.URL.Path, "/")[3])
		_, _ = io.WriteString(w, f.content[id])
	case strings.HasSuffix(r.URL.Path, "/downloaded"):
		id, _ := strconv.Atoi(strings.Split(r.URL.Path, "/")[3])
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.confirmed[id] = body["content_hash"]
	case strings.HasSuffix(r.URL.Path, "/complete"):
		var body map[string]int
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.completed = [2]int{body["operations_completed"], body["operations_failed"]}
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func intp(i int) *int { return &i }

func newTestEngine(t *testing.T, f *fakeRomM) (*Engine, string) {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	store, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	saveDir := t.TempDir()
	return &Engine{
		Client:   romm.New(srv.URL, "rmm_test"),
		Store:    store,
		Handlers: oneRegistry{fileHandler{saveDir}},
		DataDir:  t.TempDir(),
		Now:      func() time.Time { return time.Unix(1_800_000_000, 0) },
	}, saveDir
}

func TestSyncUploadDownloadConflictAndIgnoreForeign(t *testing.T) {
	f := &fakeRomM{uploads: map[int]string{}, content: map[int]string{55: "server save"},
		confirmed: map[int]string{}, conflict: map[int]bool{}}
	e, dir := newTestEngine(t, f)
	_ = os.WriteFile(filepath.Join(dir, "1.srm"), []byte("local one"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "2.srm"), []byte("old local two"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "3.srm"), []byte("local three"), 0o644)
	serverTime := time.Unix(1_700_000_000, 0).UTC()
	f.plan = func(req romm.NegotiateRequest) []romm.SyncOp {
		if len(req.Saves) != 3 || req.Saves[0].Slot != Slot || req.Saves[0].ContentHash == "" {
			t.Errorf("unexpected negotiate request: %+v", req)
		}
		return []romm.SyncOp{
			{Action: "upload", RomID: 1, Slot: Slot},
			{Action: "download", RomID: 2, Slot: Slot, SaveID: intp(55), ServerUpdatedAt: &serverTime},
			{Action: "conflict", RomID: 3, Slot: Slot, SaveID: intp(66), ServerUpdatedAt: &serverTime},
			{Action: "download", RomID: 99, Slot: Slot, SaveID: intp(77)}, // not on this device
		}
	}
	games := []Game{{RomID: 1, Name: "One"}, {RomID: 2, Name: "Two"}, {RomID: 3, Name: "Three"}}
	var events []Event
	res, err := e.Sync(context.Background(), games, func(ev Event) { events = append(events, ev) })
	if err != nil {
		t.Fatal(err)
	}
	if res.Uploaded != 1 || res.Downloaded != 1 || res.Conflicts != 1 || res.Failed != 0 {
		t.Fatalf("result = %+v, events %+v", res, events)
	}
	if f.uploads[1] != "local one" {
		t.Errorf("uploaded %q", f.uploads[1])
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "2.srm")); string(b) != "server save" {
		t.Errorf("downloaded save = %q", b)
	}
	if st, _ := os.Stat(filepath.Join(dir, "2.srm")); !st.ModTime().Equal(serverTime) {
		t.Errorf("mtime = %v, want server time", st.ModTime())
	}
	backups, _ := os.ReadDir(e.BackupDir(2))
	if len(backups) != 1 {
		t.Fatalf("backups = %v", backups)
	}
	if b, _ := os.ReadFile(filepath.Join(e.BackupDir(2), backups[0].Name())); string(b) != "old local two" {
		t.Errorf("backup = %q", b)
	}
	if f.confirmed[55] == "" {
		t.Error("download not confirmed")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "3.srm")); string(b) != "local three" {
		t.Error("conflicting save must stay untouched")
	}
	cs, _ := e.Store.Conflicts()
	if len(cs) != 1 || cs[0].RomID != 3 || cs[0].ServerSaveID != 66 {
		t.Errorf("conflicts = %+v", cs)
	}
	if f.completed != [2]int{3, 0} {
		t.Errorf("session completed with %v", f.completed)
	}
	if id, _ := e.Store.GetValue("device_id"); id != "dev-1" {
		t.Errorf("device id = %q", id)
	}
}

func TestNoOpWithMissedLocalChange(t *testing.T) {
	f := &fakeRomM{uploads: map[int]string{}, content: map[int]string{}, confirmed: map[int]string{},
		conflict: map[int]bool{2: true}}
	e, dir := newTestEngine(t, f)
	for _, rom := range []int{1, 2} {
		_ = e.Store.PutBaseline(state.Baseline{SaveKey: state.SaveKey{RomID: rom, Slot: Slot, Emulator: "test-emu"}, Hash: "old"})
		_ = os.WriteFile(filepath.Join(dir, strconv.Itoa(rom)+".srm"), []byte("edited"), 0o644)
	}
	f.plan = func(romm.NegotiateRequest) []romm.SyncOp {
		return []romm.SyncOp{
			{Action: "no_op", RomID: 1, Slot: Slot, SaveID: intp(11), ServerContentHash: "server"},
			{Action: "no_op", RomID: 2, Slot: Slot, SaveID: intp(12), ServerContentHash: "server"},
		}
	}
	res, err := e.Sync(context.Background(), []Game{{RomID: 1}, {RomID: 2}}, func(Event) {})
	if err != nil {
		t.Fatal(err)
	}
	if res.Uploaded != 1 || f.uploads[1] != "edited" {
		t.Errorf("missed local change should be pushed: %+v %v", res, f.uploads)
	}
	if res.Conflicts != 1 {
		t.Errorf("server changed too (409) should be a conflict: %+v", res)
	}
}

func TestResolveConflict(t *testing.T) {
	f := &fakeRomM{uploads: map[int]string{}, content: map[int]string{66: "server three"},
		confirmed: map[int]string{}, conflict: map[int]bool{}}
	e, dir := newTestEngine(t, f)
	k := func(rom int) state.SaveKey { return state.SaveKey{RomID: rom, Slot: Slot, Emulator: "test-emu"} }
	for _, rom := range []int{3, 4} {
		_ = os.WriteFile(filepath.Join(dir, strconv.Itoa(rom)+".srm"), []byte("local"), 0o644)
		_ = e.Store.PutConflict(state.Conflict{SaveKey: k(rom), ServerSaveID: 66, ServerUpdatedAt: time.Unix(1, 0)})
	}
	if err := e.Resolve(context.Background(), Game{RomID: 3, Name: "Three"}, "server"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "3.srm")); string(b) != "server three" {
		t.Errorf("keep server: local save = %q", b)
	}
	if bs, _ := os.ReadDir(e.BackupDir(3)); len(bs) != 1 {
		t.Errorf("keep server should back up the local save, got %v", bs)
	}
	if err := e.Resolve(context.Background(), Game{RomID: 4, Name: "Four"}, "local"); err != nil {
		t.Fatal(err)
	}
	if f.uploads[4] != "local" {
		t.Errorf("keep local: uploaded %q", f.uploads[4])
	}
	if cs, _ := e.Store.Conflicts(); len(cs) != 0 {
		t.Errorf("conflicts left: %+v", cs)
	}
	if err := e.Resolve(context.Background(), Game{RomID: 4, Name: "Four"}, "local"); err == nil {
		t.Error("resolving a game without a conflict should fail")
	}
}
