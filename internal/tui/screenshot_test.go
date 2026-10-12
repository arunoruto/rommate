package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/arunoruto/rommate/internal/core"
	"github.com/arunoruto/rommate/internal/romm"
	"github.com/arunoruto/rommate/internal/saves"
)

// TestScreenshots renders TUI screens with sample data as ANSI files for the
// documentation site. It only runs when ROMMATE_SCREENSHOT_DIR is set; see
// `just screenshots`, which turns the files into PNGs.
func TestScreenshots(t *testing.T) {
	dir := os.Getenv("ROMMATE_SCREENSHOT_DIR")
	if dir == "" {
		t.Skip("set ROMMATE_SCREENSHOT_DIR to render screenshots")
	}
	lipgloss.SetColorProfile(termenv.TrueColor)
	lipgloss.SetHasDarkBackground(true)

	const w, h = 112, 26
	shots := map[string]*model{
		"library":   shotLibrary(w, h),
		"games":     shotGames(w, h),
		"downloads": shotDownloads(w, h),
		"help":      shotHelp(w, h+6),
		"saves":     shotSaves(w, h),
		"conflict":  shotConflict(w, h),
	}
	for name, m := range shots {
		if err := os.WriteFile(filepath.Join(dir, name+".ansi"), []byte(m.View()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func shotModel(w, h int) *model {
	m := testModel(w, h)
	m.engine.Client.BaseURL = "https://romm.example.com"
	m.host = "romm.example.com"
	m.bar = progress.New(progress.WithScaledGradient("#7B61FF", "#FF7AB8"),
		progress.WithoutPercentage(), progress.WithColorProfile(termenv.TrueColor))
	m.bar.Full, m.bar.Empty, m.bar.EmptyColor = '━', '━', "#3A3A48"
	// Screenshot fonts lack the braille spinner frames; use a static glyph.
	m.spinner.Spinner = spinner.Spinner{Frames: []string{"◆"}, FPS: time.Second}
	root := m.engine.Mapper.Root
	plat := func(id int, slug, name, folder string, n int, size int64) core.PlatformInfo {
		return core.PlatformInfo{
			Platform: romm.Platform{ID: id, Slug: slug, Name: name, RomCount: n, FSSizeBytes: size},
			Folder:   folder, Dir: filepath.Join(root, folder),
		}
	}
	m.platforms = []core.PlatformInfo{
		plat(1, "gba", "Game Boy Advance", "gba", 24, 210<<20),
		plat(2, "gbc", "Game Boy Color", "gbc", 18, 22<<20),
		plat(3, "3ds", "Nintendo 3DS", "n3ds", 9, 14<<30),
		plat(4, "nds", "Nintendo DS", "nds", 31, 3<<30),
		plat(5, "nes", "Nintendo Entertainment System", "nes", 42, 6<<20),
		plat(6, "switch", "Nintendo Switch", "switch", 7, 38<<30),
		plat(7, "ps2", "PlayStation 2", "ps2", 12, 41<<30),
		plat(8, "psp", "PlayStation Portable", "psp", 15, 9<<30),
		plat(9, "snes", "Super Nintendo", "snes", 36, 70<<20),
	}
	m.counts = map[string]int{"gba": 24, "3ds": 3, "nds": 12, "nes": 42, "snes": 8, "psp": 2}
	return m
}

func shotGame(id int, name, file string, size int64, installed bool) core.RomInfo {
	return core.RomInfo{
		Rom:       romm.Rom{ID: id, Name: name, FSName: file, PlatformSlug: "3ds", FSSizeBytes: size},
		Installed: installed,
		Path:      filepath.Join("/run/media/armada/Games/ROMs/n3ds", file),
	}
}

func withShotGames(m *model) *model {
	m.screen, m.platform = screenRoms, m.platforms[2]
	m.roms = []core.RomInfo{
		shotGame(11, "Animal Crossing: New Leaf", "Animal Crossing - New Leaf (Europe).3ds", 1<<30, true),
		shotGame(12, "Fire Emblem Awakening", "Fire Emblem - Awakening (Europe).3ds", 2<<30, false),
		shotGame(13, "Kid Icarus: Uprising", "Kid Icarus - Uprising (Europe).3ds", 2<<30, false),
		shotGame(14, "Mario Kart 7", "Mario Kart 7 (Europe).3ds", 1<<30, true),
		shotGame(15, "Pokémon Omega Ruby", "Pokemon Omega Ruby (Europe) (Rev 2).3ds", 2<<30, true),
		shotGame(16, "Super Mario 3D Land", "Super Mario 3D Land (Europe).3ds", 512<<20, false),
		shotGame(17, "The Legend of Zelda: A Link Between Worlds", "Legend of Zelda, The - A Link Between Worlds (Europe).3ds", 1<<30, false),
		shotGame(18, "The Legend of Zelda: Ocarina of Time 3D", "Legend of Zelda, The - Ocarina of Time 3D (Europe).3ds", 512<<20, false),
		shotGame(19, "Xenoblade Chronicles 3D", "Xenoblade Chronicles 3D (Europe).3ds", 4<<30, false),
	}
	running := m.roms[1]
	m.running[running.ID] = &job{rom: running, cancel: func() {}}
	m.active[running.ID] = &download{id: running.ID, name: running.Title(), done: 1288 << 20, total: 2 << 30,
		startTime: time.Now().Add(-48 * time.Second)}
	m.pending = []core.RomInfo{m.roms[6], m.roms[2]}
	return m
}

func shotLibrary(w, h int) *model {
	m := shotModel(w, h)
	m.cursor = 2
	return m
}

func shotGames(w, h int) *model {
	m := withShotGames(shotModel(w, h))
	m.cursor = 4
	m.counts["3ds"] = 3
	return m
}

func shotDownloads(w, h int) *model {
	m := withShotGames(shotModel(w, h))
	m.tab = tabDownloads
	m.failed = []failedJob{{rom: m.roms[8], err: "HTTP 502: bad gateway", at: time.Now().Add(-4 * time.Minute)}}
	m.finished = []finishedJob{
		{rom: m.roms[4], at: time.Now().Add(-12 * time.Minute)},
		{rom: m.roms[3], at: time.Now().Add(-15 * time.Minute)},
	}
	return m
}

func shotHelp(w, h int) *model {
	m := shotModel(w, h)
	m.help = true
	return m
}

func shotSaves(w, h int) *model {
	m := withSaves(shotModel(w, h))
	m.tab = tabSaves
	m.saveStatus = []saves.Status{
		{RomID: 11, Name: "Animal Crossing: New Leaf", Emulator: "azahar", Path: "/var/home/armada/.local/share/azahar-emu/sdmc/Nintendo 3DS/…/title/00040000/00086300/data", SyncedAt: time.Now().Add(-2 * time.Hour)},
		{RomID: 15, Name: "Pokémon Omega Ruby", Emulator: "azahar", Path: "/var/home/armada/.local/share/azahar-emu/sdmc/Nintendo 3DS/…/title/00040000/0011c400/data", SyncedAt: time.Now().Add(-5 * time.Minute)},
		{RomID: 5, Name: "Pokémon FireRed Version", Emulator: "retroarch", Path: "/var/home/armada/.var/app/org.libretro.RetroArch/config/retroarch/saves/mGBA/Pokemon - FireRed Version (USA, Europe) (Rev 1).srm", SyncedAt: time.Now().Add(-20 * time.Minute)},
		{RomID: 6, Name: "Super Mario Odyssey", Emulator: "eden", Path: "/var/home/armada/.local/share/eden/nand/user/save/0000000000000000/9E32BCFA61C0A24AC4482A10921DCFF5/0100000000010000", Conflict: true, ServerUpdatedAt: time.Now().Add(-12 * time.Minute)},
		{RomID: 2, Name: "Super Mario Bros.", Emulator: "retroarch"},
		{RomID: 3, Name: "The Legend of Zelda", Emulator: "retroarch", Path: "/var/home/armada/.var/app/org.libretro.RetroArch/config/retroarch/saves/Mesen/Legend of Zelda, The (USA) (Rev 1).srm", SyncedAt: time.Now().Add(-time.Hour)},
		{RomID: 9, Name: "SSX Tricky", Problem: "no supported emulator found for this platform"},
	}
	m.savesCursor = 3
	m.lastSync = &saves.Result{Uploaded: 1, Downloaded: 1, InSync: 4, Conflicts: 1}
	return m
}

func shotConflict(w, h int) *model {
	m := shotSaves(w, h)
	st := m.saveStatus[3]
	m.resolving = &st
	return m
}
