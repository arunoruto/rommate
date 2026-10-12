package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/arunoruto/rommate/internal/folders"
	"github.com/arunoruto/rommate/internal/romm"
	"github.com/arunoruto/rommate/internal/state"
)

// fakeRomM serves /api/roms/{id} and /api/roms/{id}/content/{name}?file_ids=N
// with Range support, like RomM behind nginx.
type fakeRomM struct {
	roms     map[int]romm.Rom
	contents map[int][]byte    // file id -> bytes
	ranges   int               // number of Range requests seen
	assets   map[string][]byte // /assets/... path -> bytes
}

func (f *fakeRomM) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer rmm_test" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/assets/") {
		data, ok := f.assets[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(data)
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/roms/"), "/")
	id, _ := strconv.Atoi(parts[0])
	if len(parts) == 1 {
		_ = json.NewEncoder(w).Encode(f.roms[id])
		return
	}
	fid, _ := strconv.Atoi(r.URL.Query().Get("file_ids"))
	data, ok := f.contents[fid]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if rg := r.Header.Get("Range"); rg != "" {
		f.ranges++
		var start int
		if _, err := fmt.Sscanf(rg, "bytes=%d-", &start); err != nil || start > len(data) {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(data)-1, len(data)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(data[start:])
		return
	}
	_, _ = w.Write(data)
}

func newTestEngine(t *testing.T, srv *fakeRomM) (*Engine, string) {
	t.Helper()
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	store, err := state.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	root := t.TempDir()
	return &Engine{
		Client: romm.New(ts.URL, "rmm_test"),
		Mapper: folders.Mapper{Root: root},
		Store:  store,
	}, root
}

func collect(evs *[]Event) Emit { return func(e Event) { *evs = append(*evs, e) } }

func TestPullSingleFileUsesESDEFolder(t *testing.T) {
	data := []byte(strings.Repeat("3ds!", 1000))
	rom := romm.Rom{ID: 9, PlatformSlug: "3ds", Name: "Pokémon Omega Ruby", FSName: "omega.3ds",
		Files: []romm.RomFile{{ID: 90, FileName: "omega.3ds", FileSizeBytes: int64(len(data))}}}
	srv := &fakeRomM{roms: map[int]romm.Rom{9: rom}, contents: map[int][]byte{90: data}}
	e, root := newTestEngine(t, srv)

	var evs []Event
	if err := e.Pull(context.Background(), rom, collect(&evs)); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "n3ds", "omega.3ds")
	got, err := os.ReadFile(want)
	if err != nil || string(got) != string(data) {
		t.Fatalf("file at %s wrong: %v", want, err)
	}
	if last := evs[len(evs)-1]; last.Type != EventDone || last.Path != want {
		t.Fatalf("last event = %+v", last)
	}
	if _, ok, _ := e.Store.Get(9); !ok {
		t.Fatal("download not recorded in state")
	}

	// Pulling again is a no-op.
	evs = nil
	if err := e.Pull(context.Background(), rom, collect(&evs)); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 || evs[0].Type != EventSkipped {
		t.Fatalf("second pull events = %+v", evs)
	}
}

func TestPullResumesPartialDownload(t *testing.T) {
	data := []byte(strings.Repeat("abcdefgh", 512))
	rom := romm.Rom{ID: 1, PlatformSlug: "gba", FSName: "game.gba",
		Files: []romm.RomFile{{ID: 10, FileName: "game.gba", FileSizeBytes: int64(len(data))}}}
	srv := &fakeRomM{roms: map[int]romm.Rom{1: rom}, contents: map[int][]byte{10: data}}
	e, root := newTestEngine(t, srv)

	dest := filepath.Join(root, "gba", "game.gba")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest+".part", data[:1000], 0o644); err != nil {
		t.Fatal(err)
	}
	if err := e.Pull(context.Background(), rom, func(Event) {}); err != nil {
		t.Fatal(err)
	}
	if srv.ranges != 1 {
		t.Fatalf("expected 1 Range request, got %d", srv.ranges)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != string(data) {
		t.Fatal("resumed file content mismatch")
	}
	if _, err := os.Stat(dest + ".part"); !os.IsNotExist(err) {
		t.Fatal(".part file left behind")
	}
}

func TestPullMultiFileKeepsStructure(t *testing.T) {
	cue, bin := []byte("FILE track.bin"), []byte(strings.Repeat("\x00", 2048))
	rom := romm.Rom{ID: 2, PlatformSlug: "ps", HasMultipleFiles: true, FSName: "Game (Europe)",
		Files: []romm.RomFile{
			{ID: 20, FileName: "game.cue", FilePath: "psx/Game (Europe)", FileSizeBytes: int64(len(cue))},
			{ID: 21, FileName: "track.bin", FilePath: "psx/Game (Europe)/data", FileSizeBytes: int64(len(bin))},
		}}
	srv := &fakeRomM{roms: map[int]romm.Rom{2: rom}, contents: map[int][]byte{20: cue, 21: bin}}
	e, root := newTestEngine(t, srv)
	e.Mapper.Overrides = map[string]string{"ps": "psx"}

	if err := e.Pull(context.Background(), rom, func(Event) {}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"psx/Game (Europe)/game.cue", "psx/Game (Europe)/data/track.bin"} {
		if _, err := os.Stat(filepath.Join(root, p)); err != nil {
			t.Errorf("missing %s: %v", p, err)
		}
	}
}

func TestPullRejectsPathTraversal(t *testing.T) {
	rom := romm.Rom{ID: 3, PlatformSlug: "nes", HasMultipleFiles: true, FSName: "evil",
		Files: []romm.RomFile{{ID: 30, FileName: "../../../escape", FilePath: "nes/evil", FileSizeBytes: 1}}}
	srv := &fakeRomM{roms: map[int]romm.Rom{3: rom}, contents: map[int][]byte{30: []byte("x")}}
	e, _ := newTestEngine(t, srv)

	if err := e.Pull(context.Background(), rom, func(Event) {}); err == nil {
		t.Fatal("expected unsafe path error")
	}
}
