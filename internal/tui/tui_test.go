package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/arunoruto/rommate/internal/core"
	"github.com/arunoruto/rommate/internal/folders"
	"github.com/arunoruto/rommate/internal/romm"
)

func testModel(w, h int) *model {
	e := &core.Engine{
		Client: romm.New("https://rom.example.com", ""),
		Mapper: folders.Mapper{Root: "/run/media/armada/Games/ROMs"},
	}
	m := newModel(context.Background(), e)
	m.width, m.height, m.loading = w, h, false
	m.platforms = []core.PlatformInfo{
		{Platform: romm.Platform{ID: 1, Slug: "gba", Name: "Game Boy Advance", RomCount: 1, FSSizeBytes: 16 << 20}, Folder: "gba", Dir: "/run/media/armada/Games/ROMs/gba"},
		{Platform: romm.Platform{ID: 6, Slug: "3ds", Name: "Nintendo 3DS", RomCount: 1, FSSizeBytes: 2 << 30}, Folder: "n3ds", Dir: "/run/media/armada/Games/ROMs/n3ds"},
		{Platform: romm.Platform{ID: 2, Slug: "nes", Name: "Nintendo Entertainment System", RomCount: 2}, Folder: "nes", Dir: "/run/media/armada/Games/ROMs/nes"},
	}
	m.counts = map[string]int{"gba": 1, "nes": 1}
	return m
}

func withRoms(m *model) *model {
	m.screen, m.platform = screenRoms, m.platforms[2]
	m.roms = []core.RomInfo{
		{Rom: romm.Rom{ID: 2, Name: "Super Mario Bros.", FSName: "25th Anniversary Super Mario Bros. (Europe) (Promo, Virtual Console).nes", FSSizeBytes: 40976}, Installed: true, Path: "/run/media/armada/Games/ROMs/nes/25th Anniversary Super Mario Bros. (Europe) (Promo, Virtual Console).nes"},
		{Rom: romm.Rom{ID: 3, Name: "The Legend of Zelda", FSName: "Legend of Zelda, The (USA) (Rev 1).nes", FSSizeBytes: 131088}},
		{Rom: romm.Rom{ID: 4, Name: "A Game With An Extremely Long Name That Will Not Fit Anywhere On A Small Screen", FSSizeBytes: 1 << 20}},
	}
	m.pending = []core.RomInfo{m.roms[2]}
	m.running[3] = &job{rom: m.roms[1], cancel: func() {}}
	m.active[3] = &download{id: 3, name: "The Legend of Zelda", done: 60000, total: 131088, startTime: time.Now().Add(-2 * time.Second)}
	m.toast = toast{text: "Queued A Game With An Extremely Long Name", kind: toastInfo}
	return m
}

// TestViewFits renders every screen at several sizes and checks the frame is
// exactly the terminal size: no overflowing lines, no missing lines.
func TestViewFits(t *testing.T) {
	sizes := [][2]int{{60, 16}, {80, 24}, {100, 30}, {160, 50}, {213, 50}} // 213x50 ≈ 1280x800 handheld
	states := map[string]func(*model) *model{
		"platforms": func(m *model) *model { return m },
		"roms":      withRoms,
		"loading":   func(m *model) *model { m.loading = true; return m },
		"error": func(m *model) *model {
			m.err = errors.New("Get \"https://rom.example.com/api/platforms\": dial tcp: lookup rom.example.com: no such host")
			return m
		},
		"empty": func(m *model) *model { m.platforms = nil; return m },
		"help":  func(m *model) *model { m.help = true; return m },
	}
	for name, setup := range states {
		for _, sz := range sizes {
			m := setup(testModel(sz[0], sz[1]))
			lines := strings.Split(m.View(), "\n")
			if len(lines) != sz[1] {
				t.Errorf("%s %dx%d: %d lines, want %d", name, sz[0], sz[1], len(lines), sz[1])
			}
			for i, l := range lines {
				if w := lipgloss.Width(l); w > sz[0] {
					t.Errorf("%s %dx%d: line %d is %d wide: %q", name, sz[0], sz[1], i, w, l)
				}
			}
		}
	}
}

func TestCursorWrapsAndScrolls(t *testing.T) {
	m := withRoms(testModel(80, 24))
	m.handleKey(keyUp())
	if m.cursor != len(m.roms)-1 {
		t.Fatalf("up from top should wrap to bottom, got %d", m.cursor)
	}
	m.handleKey(keyDown())
	if m.cursor != 0 {
		t.Fatalf("down from bottom should wrap to top, got %d", m.cursor)
	}
}

func keyUp() tea.KeyMsg   { return tea.KeyMsg{Type: tea.KeyUp} }
func keyDown() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyDown} }

func TestHelpToggle(t *testing.T) {
	m := testModel(100, 30)
	for _, open := range []tea.KeyMsg{keyRune('?'), {Type: tea.KeyF1}, keyRune('h')} {
		m.handleKey(open)
		if !m.help {
			t.Fatalf("%q should open help", open)
		}
		if !strings.Contains(m.View(), "Previous / next page") {
			t.Fatal("help screen not rendered")
		}
		// q closes help instead of quitting.
		if _, cmd := m.handleKey(keyRune('q')); cmd != nil || m.help {
			t.Fatalf("q in help should only close it")
		}
	}
}

func keyRune(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

func TestHelpScrollsOnSmallScreens(t *testing.T) {
	m := testModel(60, 16)
	m.handleKey(keyRune('?'))
	if strings.Contains(m.View(), "Quit") {
		t.Fatal("expected help to be cut off on a 16-line screen")
	}
	for i := 0; i < 100; i++ {
		m.handleKey(keyDown())
	}
	v := m.View()
	if !strings.Contains(v, "Quit") || !strings.Contains(v, "100%") {
		t.Fatalf("scrolling down should reveal the end of the help:\n%s", v)
	}
	m.handleKey(keyRune('q'))
	if m.help || m.helpTop != 0 {
		t.Fatal("closing help should reset scroll")
	}
}

func TestHelpHintAlwaysBottomRight(t *testing.T) {
	for _, w := range []int{40, 60, 80, 120, 213} {
		m := withRoms(testModel(w, 24)) // includes a long toast
		lines := strings.Split(m.View(), "\n")
		footer := strings.TrimRight(lines[len(lines)-1], " ")
		if !strings.HasSuffix(footer, "Help") && !strings.HasSuffix(footer, "?") {
			t.Errorf("width %d: footer does not end with the help hint: %q", w, footer)
		}
		if w >= 60 && !strings.Contains(footer, "Download") {
			t.Errorf("width %d: primary hint dropped: %q", w, footer)
		}
	}
}
