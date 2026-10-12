package tui

import (
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// setting is one option of the Settings tab, stored in config.toml.
type setting struct {
	key     string // config.toml key
	label   string
	desc    string
	choices []string // "on"/"off" settings are stored as booleans
	restart bool     // takes effect the next time rommate starts
}

var settingsList = []setting{
	{"hints", "Footer hints",
		"Show keyboard keys or controller buttons in the footer. auto shows buttons when rommate was started from Steam.",
		[]string{"auto", "keyboard", "controller"}, false},
	{"confirm_quit", "Ask before quitting",
		"Ask before rommate quits. Running downloads always ask.",
		[]string{"on", "off"}, false},
	{"auto_sync_saves", "Sync saves on open and quit",
		"Sync emulator saves with RomM when rommate starts and before it quits.",
		[]string{"on", "off"}, false},
	{"covers", "Download cover art",
		"Save each game's cover into ES-DE's media folder when it downloads. For games you already have, run rommate covers.",
		[]string{"on", "off"}, false},
	{"gamepad", "Read controllers directly",
		"On Linux, read buttons straight from the controller, so no Steam Input layout is needed. off relies on keys only.",
		[]string{"auto", "off"}, true},
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// settingValue returns the current value of a setting.
func (m *model) settingValue(key string) string {
	switch key {
	case "hints":
		if m.hints == "" {
			return "auto"
		}
		return m.hints
	case "confirm_quit":
		return onOff(m.confirmQuit)
	case "auto_sync_saves":
		return onOff(m.autoSync)
	case "covers":
		return onOff(m.engine.Covers)
	case "gamepad":
		if m.gamepadSetting == "" {
			return "auto"
		}
		return m.gamepadSetting
	}
	return ""
}

// applySetting changes a setting for this session and saves it.
func (m *model) applySetting(s setting, v string) tea.Cmd {
	switch s.key {
	case "hints":
		m.hints = v
		m.input = detectInputMode(v, os.Getenv)
	case "confirm_quit":
		m.confirmQuit = v == "on"
	case "auto_sync_saves":
		m.autoSync = v == "on"
	case "covers":
		m.engine.Covers = v == "on"
	case "gamepad":
		m.gamepadSetting = v
	}
	if m.saveSetting == nil {
		return nil
	}
	var value any = v
	if s.choices[0] == "on" {
		value = v == "on"
	}
	if err := m.saveSetting(s.key, value); err != nil {
		return m.setToast("Could not save: "+err.Error(), toastBad)
	}
	msg := s.label + ": " + v
	if s.restart {
		msg += " (after a restart)"
	}
	return m.setToast(msg, toastGood)
}

func (m *model) settingsKey(key string) (tea.Model, tea.Cmd) {
	step := 0
	switch key {
	case "enter", "right":
		step = 1
	case "left":
		step = -1
	case "up", "k", "down", "j", "home", "end":
		m.moveCursor(key, &m.setCursor, len(settingsList))
		return m, nil
	}
	if step == 0 {
		return m, nil
	}
	s := settingsList[m.setCursor]
	i := 0
	for j, c := range s.choices {
		if c == m.settingValue(s.key) {
			i = j
		}
	}
	i = (i + step + len(s.choices)) % len(s.choices)
	return m, m.applySetting(s, s.choices[i])
}

func (m *model) settingsHints(tabs, quit hint) []hint {
	k := m.key
	return []hint{{k("Enter", "A"), "Change"}, {k("← →", "D-pad ← →"), "Choose"}, tabs, quit}
}

func (m *model) viewSettings(w, h int) string {
	inner := w - 2
	body := []string{""}
	for i, s := range settingsList {
		body = append(body, m.viewSettingRow(s, i == m.setCursor, inner))
	}
	if m.width < wideWidth {
		// No details pane: describe the selected setting here.
		body = append(body, "")
		for _, l := range wrap(settingsList[m.setCursor].desc, inner-4) {
			body = append(body, "  "+sDim.Render(l))
		}
	}
	body = append(body, "", "  "+sAccent.Bold(true).Render("About"))
	for _, a := range m.about {
		body = append(body, kv(a[0], a[1], inner)...)
	}
	return box("Settings", "", body, w, h, true)
}

func (m *model) viewSettingRow(s setting, selected bool, w int) string {
	value := m.settingValue(s.key)
	right := value
	if selected {
		right = "‹ " + value + " ›"
	}
	space := w - 3 - lipgloss.Width(right)
	left := ansi.Truncate(s.label, max(space-1, 1), "…")
	gap := strings.Repeat(" ", max(space-lipgloss.Width(left), 1))
	if selected {
		return sSelected.Render("› " + left + gap + right + " ")
	}
	style := sAccent
	if value == "off" {
		style = sDim
	}
	return "  " + sText.Render(left) + gap + style.Render(right) + " "
}

func (m *model) viewSettingsDetails(w, h int) string {
	inner := w - 2
	s := settingsList[m.setCursor]
	lines := []string{""}
	for _, l := range wrap(s.label, inner-4) {
		lines = append(lines, "  "+sBold.Render(l))
	}
	lines = append(lines, "")
	for _, l := range wrap(s.desc, inner-4) {
		lines = append(lines, "  "+sText.Render(l))
	}
	lines = append(lines, "")
	var opts []string
	for _, c := range s.choices {
		if c == m.settingValue(s.key) {
			opts = append(opts, sAccent.Render("● "+c))
		} else {
			opts = append(opts, sDim.Render("○ "+c))
		}
	}
	lines = append(lines, "  "+strings.Join(opts, "   "))
	if s.restart {
		lines = append(lines, "", "  "+sWarn.Render("Takes effect the next time rommate starts."))
	}
	lines = append(lines, "", "  "+sDim.Render("Saved to config.toml right away."))
	return box("Details", "", lines, w, h, false)
}
