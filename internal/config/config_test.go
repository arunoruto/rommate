package config

import (
	"path/filepath"
	"testing"
)

func TestPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	t.Setenv("XDG_CONFIG_HOME", "")
	if got, _ := Path(); got != filepath.Join(home, ".config", "rommate", "config.toml") {
		t.Errorf("Path() = %q, want ~/.config/rommate/config.toml", got)
	}

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	if got, _ := Path(); got != filepath.Join(xdg, "rommate", "config.toml") {
		t.Errorf("Path() with XDG_CONFIG_HOME = %q", got)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("ROMMATE_SERVER", "")
	t.Setenv("ROMMATE_TOKEN", "")
	t.Setenv("ROMMATE_ROM_DIR", "")

	in := &Config{Server: "https://romm.example", Token: "rmm_x", Profile: "emudeck",
		Folders: map[string]string{"gba": "gba-hacks"}}
	if err := in.Save(); err != nil {
		t.Fatal(err)
	}
	out, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if out.Server != in.Server || out.Token != in.Token || out.Profile != in.Profile || out.Folders["gba"] != "gba-hacks" {
		t.Errorf("round trip mismatch: %+v", out)
	}
}
