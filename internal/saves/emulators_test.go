package saves

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mkfile(t *testing.T, path string, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fakeHome(t *testing.T) string {
	t.Helper()
	// Handlers resolve symlinks (macOS: /var -> /private/var), so use the
	// resolved path as home too.
	h, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", h)
	return h
}

func TestRetroArchFindAndTarget(t *testing.T) {
	h := fakeHome(t)
	ra := filepath.Join(h, ".var/app/org.libretro.RetroArch/config/retroarch")
	mkfile(t, filepath.Join(ra, "retroarch.cfg"), "savefile_directory = \"default\"\nsort_savefiles_enable = \"true\"\n")
	g := Game{RomID: 5, Platform: "gba", Path: "/roms/gba/Pokemon - FireRed Version (USA, Europe) (Rev 1).gba"}

	r := newRetroArch()
	if r == nil {
		t.Fatal("RetroArch not detected")
	}
	if _, ok, _ := r.Find(g); ok {
		t.Fatal("no save yet")
	}
	target, err := r.Target(g)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(ra, "saves", "mGBA", "Pokemon - FireRed Version (USA, Europe) (Rev 1).srm")
	if target.Path != want {
		t.Fatalf("target = %s, want %s", target.Path, want)
	}
	mkfile(t, want, "sram")
	s, ok, err := r.Find(g)
	if err != nil || !ok || s.Path != want || s.FileName != filepath.Base(want) {
		t.Fatalf("find = %+v %v %v", s, ok, err)
	}

	// Saves next to the ROM.
	mkfile(t, filepath.Join(ra, "retroarch.cfg"), "savefiles_in_content_dir = \"true\"\n")
	if s, _ := r.Target(g); s.Path != "/roms/gba/Pokemon - FireRed Version (USA, Europe) (Rev 1).srm" {
		t.Fatalf("content-dir target = %s", s.Path)
	}
}

func TestMelonDSSaveFilePath(t *testing.T) {
	h := fakeHome(t)
	cfg := filepath.Join(h, ".config/melonDS/melonDS.toml")
	mkfile(t, cfg, "[Instance0]\nSaveFilePath = \"\"\n")
	g := Game{Platform: "nds", Path: filepath.Join(h, "roms/nds/Mario Kart DS.nds")}
	m := newMelonDS()
	if s, _ := m.Target(g); s.Path != filepath.Join(h, "roms/nds/Mario Kart DS.sav") {
		t.Fatalf("next to ROM: %s", s.Path)
	}
	mkfile(t, cfg, "[Instance0]\nSaveFilePath = \"~/Emulation/saves/melonds/saves\"\n")
	if s, _ := m.Target(g); s.Path != filepath.Join(h, "Emulation/saves/melonds/saves/Mario Kart DS.sav") {
		t.Fatalf("SaveFilePath: %s", s.Path)
	}
}

func TestAzaharTitleFolder(t *testing.T) {
	h := fakeHome(t)
	zeros := strings.Repeat("0", 32)
	saveDir := filepath.Join(h, ".local/share/azahar-emu/sdmc/Nintendo 3DS", zeros, zeros, "title/00040000/00055d00/data")
	mkfile(t, filepath.Join(saveDir, "00000001", "main"), "save")

	rom := make([]byte, 0x200)
	copy(rom[0x100:], "NCCH")
	binary.LittleEndian.PutUint64(rom[0x118:], 0x0004000000055D00)
	romPath := filepath.Join(h, "roms/n3ds/game.cxi")
	mkfile(t, romPath, string(rom))

	a := newAzahar()
	s, ok, err := a.Find(Game{Platform: "3ds", Path: romPath})
	if err != nil || !ok || s.Path != saveDir || !s.IsDir || s.FileName != "0004000000055D00.zip" {
		t.Fatalf("find = %+v %v %v", s, ok, err)
	}
}

func TestPPSSPPPrefix(t *testing.T) {
	h := fakeHome(t)
	sd := filepath.Join(h, ".var/app/org.ppsspp.PPSSPP/config/ppsspp/PSP/SAVEDATA")
	mkfile(t, filepath.Join(sd, "ULUS10041DATA00", "DATA.BIN"), "x")
	romPath := filepath.Join(h, "roms/psp/game.iso")
	mkfile(t, romPath, string(isoWithUMD("ULUS-10041|X|0001|G")))
	p := newPPSSPP()
	s, ok, err := p.Find(Game{Platform: "psp", Path: romPath})
	if err != nil || !ok || s.Prefix != "ULUS10041" || s.Path != sd {
		t.Fatalf("find = %+v %v %v", s, ok, err)
	}
}

func TestEdenTitleFolder(t *testing.T) {
	h := fakeHome(t)
	user := "8D1C3B5A9F2E4C6B0A1B2C3D4E5F6071"
	root := filepath.Join(h, ".local/share/eden/nand/user/save/0000000000000000")
	mkfile(t, filepath.Join(root, user, "0100000000010000", "File1"), "save")
	y := newYuzuFamily()
	if y == nil || y.ID() != "eden" {
		t.Fatal("Eden not detected")
	}
	g := Game{Platform: "switch", Path: filepath.Join(h, "roms/switch/Super Mario Odyssey [0100000000010000].xci")}
	s, ok, err := y.Find(g)
	if err != nil || !ok || filepath.Base(filepath.Dir(s.Path)) != user {
		t.Fatalf("find = %+v %v %v", s, ok, err)
	}
	other := Game{Platform: "switch", Path: filepath.Join(h, "roms/switch/Other [0100000000020000].nsp")}
	tgt, err := y.Target(other)
	if err != nil || tgt.Path != filepath.Join(root, user, "0100000000020000") {
		t.Fatalf("target = %+v %v", tgt, err)
	}
}

func TestRegistryPicksHandlers(t *testing.T) {
	h := fakeHome(t)
	mkfile(t, filepath.Join(h, ".config/retroarch/retroarch.cfg"), "")
	d := &Detected{}
	if got := d.For("gba"); got == nil || got.ID() != "retroarch" {
		t.Errorf("gba -> %v", got)
	}
	if got := d.For("nds"); got == nil || got.ID() != "retroarch" {
		t.Errorf("nds without melonDS should fall back to RetroArch: %v", got)
	}
	if got := d.For("ps2"); got != nil {
		t.Errorf("ps2 should have no handler yet: %v", got)
	}
	if got := d.For("3ds"); got != nil {
		t.Errorf("3ds without Azahar: %v", got)
	}
}

func TestAzaharCustomStorageNeedsSwitch(t *testing.T) {
	h := fakeHome(t)
	custom := filepath.Join(h, "custom-sdmc")
	_ = os.MkdirAll(custom, 0o755)
	_ = os.MkdirAll(filepath.Join(h, ".local/share/azahar-emu/sdmc"), 0o755)
	cfg := filepath.Join(h, ".config/azahar-emu/qt-config.ini")
	write := func(on string) {
		mkfile(t, cfg, "[Data%20Storage]\nsdmc_directory="+custom+"\nsdmc_directory\\default=false\nuse_custom_storage="+on+"\n")
	}
	write("false")
	if a := newAzahar().(*azahar); a.sdmc != filepath.Join(h, ".local/share/azahar-emu/sdmc") {
		t.Errorf("custom path used although use_custom_storage=false: %s", a.sdmc)
	}
	write("true")
	if a := newAzahar().(*azahar); a.sdmc != custom {
		t.Errorf("custom path ignored: %s", a.sdmc)
	}
}
