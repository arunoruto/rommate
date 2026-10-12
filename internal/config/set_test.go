package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSetLine(t *testing.T) {
	cases := []struct{ name, doc, want string }{
		{"replace", "# my server\nserver = \"x\"\nhints = \"auto\" # comment\n", "# my server\nserver = \"x\"\nhints = \"keyboard\"\n"},
		{"add", "server = \"x\"\n", "server = \"x\"\nhints = \"keyboard\"\n"},
		{"before table", "server = \"x\"\n\n[folders]\ngba = \"gba2\"\nhints = \"no\"\n", "server = \"x\"\nhints = \"keyboard\"\n\n[folders]\ngba = \"gba2\"\nhints = \"no\"\n"},
		{"commented out", "# hints = \"auto\"\n", "# hints = \"auto\"\nhints = \"keyboard\"\n"},
		{"empty", "", "hints = \"keyboard\"\n"},
	}
	for _, c := range cases {
		if got := setLine(c.doc, "hints", `"keyboard"`); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

func TestSetValueKeepsFileLoadable(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path := filepath.Join(dir, "rommate", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("server = \"https://r\"\ntoken = \"t\"\n\n[folders]\ngba = \"x\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SetValue("confirm_quit", false); err != nil {
		t.Fatal(err)
	}
	if err := SetValue("hints", "controller"); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConfirmQuit || cfg.Hints != "controller" || cfg.Folders["gba"] != "x" || cfg.Server != "https://r" {
		t.Fatalf("unexpected config %+v", cfg)
	}
	if st, _ := os.Stat(path); runtime.GOOS != "windows" && st.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", st.Mode().Perm())
	}
}
