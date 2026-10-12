package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arunoruto/rommate/internal/romm"
)

func TestPullFetchesCover(t *testing.T) {
	data := []byte("rom")
	jpeg := []byte("\xff\xd8\xff\xe0 a jpeg")
	rom := romm.Rom{ID: 9, PlatformSlug: "3ds", Name: "Omega Ruby", FSName: "Omega Ruby (Europe).3ds",
		PathCoverLarge: "/assets/romm/resources/roms/6/9/cover/big.png?ts=2026-10-11 18:55:54+00:00",
		Files:          []romm.RomFile{{ID: 90, FileName: "Omega Ruby (Europe).3ds", FileSizeBytes: 3}}}
	srv := &fakeRomM{
		roms:     map[int]romm.Rom{9: rom},
		contents: map[int][]byte{90: data},
		assets:   map[string][]byte{"/assets/romm/resources/roms/6/9/cover/big.png": jpeg},
	}
	e, _ := newTestEngine(t, srv)
	e.Covers, e.Mapper.Media = true, t.TempDir()

	if err := e.Pull(context.Background(), rom, func(Event) {}); err != nil {
		t.Fatal(err)
	}
	// Named after the ROM file, with the extension of the actual format.
	cover := filepath.Join(e.Mapper.Media, "n3ds", "covers", "Omega Ruby (Europe).jpg")
	if got, err := os.ReadFile(cover); err != nil || string(got) != string(jpeg) {
		t.Fatalf("cover at %s: %v", cover, err)
	}
	if err := e.Remove(rom); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cover); !os.IsNotExist(err) {
		t.Fatal("removing the game should remove its cover")
	}
}

func TestCoverProblemsDontFailDownloads(t *testing.T) {
	rom := romm.Rom{ID: 9, PlatformSlug: "nes", Name: "Zelda", FSName: "zelda.nes",
		PathCoverLarge: "/assets/cover.png",
		Files:          []romm.RomFile{{ID: 90, FileName: "zelda.nes", FileSizeBytes: 3}}}
	srv := &fakeRomM{roms: map[int]romm.Rom{9: rom}, contents: map[int][]byte{90: []byte("rom")},
		assets: map[string][]byte{"/assets/cover.png": []byte("<html>not an image</html>")}}
	e, _ := newTestEngine(t, srv)
	e.Covers, e.Mapper.Media = true, t.TempDir()
	var warnings []string
	e.Warn = func(msg string) { warnings = append(warnings, msg) }

	if err := e.Pull(context.Background(), rom, func(Event) {}); err != nil {
		t.Fatalf("a bad cover must not fail the download: %v", err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "not a PNG or JPEG") {
		t.Fatalf("warnings = %v", warnings)
	}
	if entries, _ := os.ReadDir(filepath.Join(e.Mapper.Media, "nes", "covers")); len(entries) != 0 {
		t.Fatalf("nothing should be written, found %v", entries)
	}

	// With covers off nothing is fetched at all.
	e.Covers, warnings = false, nil
	if p, err := e.FetchCover(context.Background(), rom, "/roms/nes/zelda.nes"); p != "" || err != nil {
		t.Fatalf("covers off: %q %v", p, err)
	}
}
