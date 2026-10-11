package folders

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFolder(t *testing.T) {
	m := Mapper{Root: "/roms", Overrides: map[string]string{"gba": "gameboyadvance"}}
	cases := map[string]string{
		"3ds":     "n3ds",           // built-in alias
		"ngc":     "gc",             // built-in alias
		"nes":     "nes",            // identical name
		"gba":     "gameboyadvance", // user override wins
		"unknown": "unknown",        // falls back to slug
	}
	for slug, want := range cases {
		if got := m.Folder(slug); got != want {
			t.Errorf("Folder(%q) = %q, want %q", slug, got, want)
		}
	}
	if got := m.Dir("3ds"); got != filepath.Join("/roms", "n3ds") {
		t.Errorf("Dir(3ds) = %q", got)
	}
}

func TestESDEROMDirectory(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "ES-DE", "settings")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	settings := `<?xml version="1.0"?>
<bool name="Foo" value="true" />
<string name="ROMDirectory" value="/run/media/armada/Games/ROMs" />
`
	if err := os.WriteFile(filepath.Join(dir, "es_settings.xml"), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := esdeROMDirectory(home); got != "/run/media/armada/Games/ROMs" {
		t.Errorf("esdeROMDirectory = %q", got)
	}
}
