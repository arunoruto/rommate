package state

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestEntryJSONUsesSnakeCase(t *testing.T) {
	b, err := json.Marshal(Entry{RomID: 1, PlatformSlug: "nes", Name: "Zelda", Path: "/roms/nes/z.nes",
		SizeBytes: 2, DownloadedAt: time.Unix(0, 0).UTC()})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"rom_id"`, `"platform_slug"`, `"name"`, `"path"`, `"size_bytes"`, `"downloaded_at"`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("missing %s in %s", key, b)
		}
	}
}

func TestStoreRoundTrip(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	e := Entry{RomID: 7, PlatformSlug: "gba", Name: "FireRed", Path: "/roms/gba/f.gba", SizeBytes: 16, DownloadedAt: time.Unix(100, 0)}
	if err := s.Put(e); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.Get(7)
	if err != nil || !ok || got.Name != "FireRed" || got.PlatformSlug != "gba" {
		t.Fatalf("Get = %+v %v %v", got, ok, err)
	}
	counts, err := s.CountByPlatform()
	if err != nil || counts["gba"] != 1 {
		t.Fatalf("CountByPlatform = %v %v", counts, err)
	}
	if err := s.Delete(7); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.Get(7); ok {
		t.Fatal("entry still there after Delete")
	}
}
