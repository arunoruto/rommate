package saves

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func md5hex(b []byte) string { s := md5.Sum(b); return hex.EncodeToString(s[:]) }

func TestHashPlainFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "game.srm")
	data := []byte("save data")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Hash(p)
	if err != nil || got != md5hex(data) {
		t.Fatalf("Hash = %s, %v; want %s", got, err, md5hex(data))
	}
}

func TestZipIsDeterministicAndHashedLikeRomM(t *testing.T) {
	dir := t.TempDir()
	save := filepath.Join(dir, "save")
	_ = os.MkdirAll(filepath.Join(save, "sub"), 0o755)
	_ = os.WriteFile(filepath.Join(save, "b.bin"), []byte("bee"), 0o644)
	_ = os.WriteFile(filepath.Join(save, "sub", "a.bin"), []byte("ay"), 0o644)

	var z1, z2 bytes.Buffer
	if err := ZipDir(save, &z1); err != nil {
		t.Fatal(err)
	}
	// Touch a file: same content, new mtime -> identical zip bytes.
	_ = os.Chtimes(filepath.Join(save, "b.bin"), timeNowPlus(), timeNowPlus())
	if err := ZipDir(save, &z2); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(z1.Bytes(), z2.Bytes()) {
		t.Fatal("zip is not deterministic")
	}

	zp := filepath.Join(dir, "save.zip")
	_ = os.WriteFile(zp, z1.Bytes(), 0o644)
	got, err := Hash(zp)
	if err != nil {
		t.Fatal(err)
	}
	// RomM: md5("\n".join(f"{name}:{md5(entry)}" for name in sorted(names)))
	want := md5hex([]byte(fmt.Sprintf("b.bin:%s\nsub/a.bin:%s", md5hex([]byte("bee")), md5hex([]byte("ay")))))
	if got != want {
		t.Fatalf("zip hash = %s, want %s", got, want)
	}

	// Round trip.
	restored := filepath.Join(dir, "restored")
	if err := UnzipDir(zp, restored); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(restored, "sub", "a.bin")); string(b) != "ay" {
		t.Fatalf("restored content = %q", b)
	}
}

func timeNowPlus() (t time.Time) { return time.Now().Add(time.Hour) }

func TestPrefixZipAndRestore(t *testing.T) {
	dir := t.TempDir()
	sd := filepath.Join(dir, "SAVEDATA")
	for _, f := range []string{"ULUS10041DATA00/DATA.BIN", "ULUS10041SYS/PARAM.SFO", "ULES00151SAVE/DATA.BIN"} {
		p := filepath.Join(sd, filepath.FromSlash(f))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte(f), 0o644)
	}
	zp := filepath.Join(dir, "g.zip")
	f, _ := os.Create(zp)
	if err := ZipPrefix(sd, "ULUS10041", f); err != nil {
		t.Fatal(err)
	}
	f.Close()

	// Change the game's save and add another one, then restore.
	_ = os.WriteFile(filepath.Join(sd, "ULUS10041DATA00", "DATA.BIN"), []byte("changed"), 0o644)
	_ = os.MkdirAll(filepath.Join(sd, "ULUS10041EXTRA"), 0o755)
	if err := UnzipPrefix(zp, sd, "ULUS10041"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(sd, "ULUS10041DATA00", "DATA.BIN")); string(b) != "ULUS10041DATA00/DATA.BIN" {
		t.Errorf("game save not restored: %q", b)
	}
	if _, err := os.Stat(filepath.Join(sd, "ULUS10041EXTRA")); !os.IsNotExist(err) {
		t.Error("entries of the game not in the zip should be removed")
	}
	if b, _ := os.ReadFile(filepath.Join(sd, "ULES00151SAVE", "DATA.BIN")); string(b) != "ULES00151SAVE/DATA.BIN" {
		t.Error("another game's save was touched")
	}
}
