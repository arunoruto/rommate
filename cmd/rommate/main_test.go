package main

import (
	"encoding/json"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestParseFlagsAnywhere(t *testing.T) {
	cases := [][]string{
		{"nes", "--json"},
		{"--json", "nes"},
		{"-json", "nes", "--", "--literal"},
	}
	want := [][]string{{"nes"}, {"nes"}, {"nes", "--literal"}}
	for i, args := range cases {
		fs, asJSON := newFlags("t")
		pos, err := parse(fs, args)
		if err != nil {
			t.Fatal(err)
		}
		if !*asJSON || !reflect.DeepEqual(pos, want[i]) {
			t.Errorf("parse(%q) = %q json=%v, want %q json=true", args, pos, *asJSON, want[i])
		}
	}
}

func TestConfigNeverPrintsToken(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("ROMMATE_SERVER", "https://romm.example")
	t.Setenv("ROMMATE_TOKEN", "rmm_secret")

	out := captureStdout(t, func() {
		if err := cmdConfig([]string{"--json"}); err != nil {
			t.Fatal(err)
		}
	})
	var info configInfo
	if err := json.Unmarshal([]byte(out), &info); err != nil {
		t.Fatalf("bad JSON %q: %v", out, err)
	}
	if !info.Configured || !info.TokenSet || info.Server != "https://romm.example" {
		t.Errorf("info = %+v", info)
	}
	if strings.Contains(out, "rmm_secret") {
		t.Fatal("config output leaked the token")
	}

	t.Setenv("ROMMATE_TOKEN", "")
	out = captureStdout(t, func() { _ = cmdConfig([]string{"--json"}) })
	if err := json.Unmarshal([]byte(out), &info); err != nil || info.Configured || info.Problem == "" {
		t.Errorf("unconfigured: %+v (%v)", info, err)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	fn()
	os.Stdout = old
	w.Close()
	b, _ := io.ReadAll(r)
	return string(b)
}
