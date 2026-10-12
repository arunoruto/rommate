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

func TestSyncTables(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if v, _ := s.GetValue("device_id"); v != "" {
		t.Fatal("unset value should be empty")
	}
	_ = s.SetValue("device_id", "abc")
	if v, _ := s.GetValue("device_id"); v != "abc" {
		t.Fatalf("device_id = %q", v)
	}
	k := SaveKey{RomID: 5, Slot: "autosave", Emulator: "retroarch"}
	_ = s.PutBaseline(Baseline{SaveKey: k, Hash: "h1", SyncedAt: time.Unix(10, 0)})
	_ = s.PutBaseline(Baseline{SaveKey: k, Hash: "h2", SyncedAt: time.Unix(20, 0)})
	if b, ok, _ := s.GetBaseline(k); !ok || b.Hash != "h2" {
		t.Fatalf("baseline = %+v %v", b, ok)
	}
	_ = s.PutConflict(Conflict{SaveKey: k, ServerSaveID: 9, ServerUpdatedAt: time.Unix(30, 0), LocalHash: "l", DetectedAt: time.Unix(40, 0)})
	if cs, _ := s.Conflicts(); len(cs) != 1 || cs[0].ServerSaveID != 9 {
		t.Fatalf("conflicts = %+v", cs)
	}
	_ = s.DeleteConflict(k)
	if cs, _ := s.Conflicts(); len(cs) != 0 {
		t.Fatal("conflict not deleted")
	}
}
