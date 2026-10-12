package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/arunoruto/rommate/internal/core"
	"github.com/arunoruto/rommate/internal/gamepad"
	"github.com/arunoruto/rommate/internal/romm"
)

func typeText(m *model, s string) tea.Cmd {
	var cmd tea.Cmd
	for _, r := range s {
		_, cmd = m.handleKey(keyRune(r))
	}
	return cmd
}

func TestFilterGames(t *testing.T) {
	m := withRoms(testModel(100, 30))
	m.handleKey(keyRune('/'))
	if !m.searching || m.screen != screenRoms {
		t.Fatal("/ in a platform should start filtering its games")
	}
	typeText(m, "zel q") // q and space are typed, not commands
	if m.query != "zel q" || m.quitting {
		t.Fatalf("query = %q, quitting = %v", m.query, m.quitting)
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyBackspace})
	m.handleKey(tea.KeyMsg{Type: tea.KeyBackspace})
	if rows := m.rows(); len(rows) != 1 || m.roms[rows[0]].Name != "The Legend of Zelda" {
		t.Fatalf("filter %q matched %v", m.query, rows)
	}
	v := m.View()
	if !strings.Contains(v, "/ zel") || !strings.Contains(v, "1 / 1 of 3") {
		t.Fatalf("filter not shown:\n%s", v)
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.searching || m.query != "zel" {
		t.Fatal("enter should keep the filter and focus the list")
	}
	// Esc first clears the filter, keeping Zelda selected, then goes back.
	m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if m.query != "" || m.screen != screenRoms || m.roms[m.cursor].Name != "The Legend of Zelda" {
		t.Fatalf("esc should clear the filter: query %q, cursor %d", m.query, m.cursor)
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen != screenPlatforms {
		t.Fatal("second esc should go back to the platforms")
	}
}

func TestFilterMatchesAllWords(t *testing.T) {
	m := withRoms(testModel(100, 30))
	m.query = "bros super"
	if rows := m.rows(); len(rows) != 1 || m.roms[rows[0]].ID != 2 {
		t.Fatalf("rows = %v", rows)
	}
}

func TestSearchAllPlatforms(t *testing.T) {
	m := testModel(100, 30)
	m.cursor = 2
	m.handleKey(keyRune('/'))
	if m.screen != screenSearch || !m.searching {
		t.Fatal("/ on the platforms should open the search")
	}
	if cmd := typeText(m, "ma"); cmd == nil {
		t.Fatal("typing should schedule a search")
	}
	// A tick for an older keystroke is ignored.
	if cmd, _ := m.handleSearchMsg(searchTickMsg(m.searchSeq - 1)); cmd != nil {
		t.Fatal("stale tick should not search")
	}
	results := []core.RomInfo{
		{Rom: romm.Rom{ID: 2, PlatformID: 2, PlatformSlug: "nes", Name: "Super Mario Bros.", FSSizeBytes: 40976}, Installed: true},
		{Rom: romm.Rom{ID: 6, PlatformID: 9, PlatformSlug: "switch", Name: "Super Mario Odyssey", FSSizeBytes: 6 << 30}},
	}
	m.handleSearchMsg(searchMsg{query: "m", roms: results[:1]}) // stale
	if len(m.roms) != 0 {
		t.Fatal("results for an older query should be ignored")
	}
	m.handleSearchMsg(searchMsg{query: "ma", roms: results})
	v := m.View()
	if len(m.rows()) != 2 || !strings.Contains(v, "Search all platforms") || !strings.Contains(v, "switch") {
		t.Fatalf("results not shown:\n%s", v)
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter}) // done typing
	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter}) // download
	if !m.isBusy(6) {
		t.Fatal("enter on a result should queue it")
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen != screenPlatforms || m.cursor != 2 || m.query != "" {
		t.Fatalf("esc should return to the platforms at the old cursor, got screen %d cursor %d", m.screen, m.cursor)
	}
}

func TestSearchNeedsTwoCharacters(t *testing.T) {
	m := testModel(100, 30)
	m.handleKey(keyRune('/'))
	typeText(m, "m")
	if cmd := m.runSearch(); cmd != nil || m.loading {
		t.Fatal("a one-letter search should not ask the server")
	}
	if !strings.Contains(m.View(), "Search every platform by game name") {
		t.Fatal("expected a hint for the empty search")
	}
}

func TestControllerButtonsDontType(t *testing.T) {
	m := withRoms(testModel(100, 30))
	m.handleKey(keyRune('/'))
	typeText(m, "zel")
	m.handlePad(gamepad.Y)
	if m.query != "zel" || m.searching {
		t.Fatalf("Y should end typing, not type: query %q searching %v", m.query, m.searching)
	}
}
