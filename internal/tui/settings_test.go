package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func withSettings(m *model) (*model, map[string]any) {
	saved := map[string]any{}
	m.tab = tabSettings
	m.confirmQuit, m.autoSync = true, true
	m.saveSetting = func(key string, value any) error { saved[key] = value; return nil }
	m.about = [][2]string{{"Server", "https://rom.example.com"}, {"Version", "v1.2.3"}}
	return m, saved
}

func TestSettingsChangeAndSave(t *testing.T) {
	m, saved := withSettings(testModel(100, 30))
	m.handleKey(tea.KeyMsg{Type: tea.KeyDown}) // Ask before quitting
	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.confirmQuit || saved["confirm_quit"] != false {
		t.Fatalf("confirm_quit not turned off: %v %v", m.confirmQuit, saved)
	}
	if _, cmd := m.handleKey(keyRune('q')); cmd == nil || m.quitting {
		t.Fatal("with confirm_quit off, q should quit right away")
	}

	m, saved = withSettings(testModel(100, 30))
	m.handleKey(tea.KeyMsg{Type: tea.KeyLeft}) // hints: auto → controller (wraps)
	if m.hints != "controller" || m.input != inputController || saved["hints"] != "controller" {
		t.Fatalf("hints = %q input = %v saved = %v", m.hints, m.input, saved)
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyRight}) // → auto
	if saved["hints"] != "auto" {
		t.Fatalf("saved = %v", saved)
	}

	m.handleKey(tea.KeyMsg{Type: tea.KeyEnd}) // gamepad
	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if saved["gamepad"] != "off" || !strings.Contains(m.toast.text, "restart") {
		t.Fatalf("gamepad: saved %v toast %q", saved, m.toast.text)
	}
}

func TestSettingsView(t *testing.T) {
	m, _ := withSettings(testModel(120, 30))
	v := m.View()
	for _, want := range []string{"Settings", "Ask before quitting", "‹ auto ›", "About", "v1.2.3", "Saved to config.toml"} {
		if !strings.Contains(v, want) {
			t.Errorf("settings view lacks %q:\n%s", want, v)
		}
	}
	// Narrow: no details pane, the description moves into the list.
	m.width = 70
	if v := m.View(); !strings.Contains(v, "Show keyboard keys") {
		t.Errorf("narrow settings view lacks the description:\n%s", v)
	}
}
