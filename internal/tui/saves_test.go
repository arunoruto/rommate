package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/arunoruto/rommate/internal/saves"
)

func TestSavesConflictDialog(t *testing.T) {
	m := withSaves(testModel(110, 30))
	m.tab = tabSaves
	m.savesCursor = 0
	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.resolving != nil {
		t.Fatal("Enter on a save without conflict should not open the dialog")
	}
	m.savesCursor = 1
	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.resolving == nil || m.resolving.RomID != 6 {
		t.Fatal("Enter on a conflict should open the resolve dialog")
	}
	if v := m.View(); !strings.Contains(v, "Keep this device") || !strings.Contains(v, "Keep RomM") {
		t.Fatal("resolve dialog not shown")
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if m.resolving != nil {
		t.Fatal("Esc should cancel")
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if _, cmd := m.handleKey(keyRune('y')); cmd == nil || m.resolving != nil {
		t.Fatal("Y should resolve with RomM's save")
	}
}

func TestQuitSyncsSavesFirst(t *testing.T) {
	m := withSaves(testModel(110, 30))
	m.confirmQuit = false
	if _, cmd := m.handleKey(keyRune('q')); cmd == nil || !m.quitSyncing {
		t.Fatal("quitting with automatic sync should sync saves first")
	}
	if _, cmd := m.handleKey(keyRune('q')); cmd != nil {
		t.Fatal("keys other than Ctrl+C are ignored while syncing before quit")
	}
	if cmd, _ := m.handleSavesMsg(savesSyncedMsg{quit: true}); cmd == nil {
		t.Fatal("the finished quit sync should quit")
	}

	m = withSaves(testModel(110, 30))
	m.confirmQuit, m.autoSync = false, false
	if _, cmd := m.handleKey(keyRune('q')); cmd == nil || m.quitSyncing {
		t.Fatal("without automatic sync, q should quit directly")
	}
}

func TestSyncSummary(t *testing.T) {
	if got := syncSummary(saves.Result{}); got != "Saves are in sync" {
		t.Errorf("empty: %q", got)
	}
	if got := syncSummary(saves.Result{Uploaded: 1, Conflicts: 2}); got != "Saves: 1 uploaded, 2 conflicts" {
		t.Errorf("mixed: %q", got)
	}
	if got := syncSummary(saves.Result{Conflicts: 1}); got != "Saves: 1 conflict" {
		t.Errorf("singular: %q", got)
	}
}
