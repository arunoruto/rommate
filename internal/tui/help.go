package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// binding documents one action for the help screen.
type binding struct {
	action     string
	keyboard   string
	controller string
}

// helpSections is the single source of truth for the help screen. Keep it in
// sync with handleKey and the README's controller table.
var helpSections = []struct {
	title    string
	bindings []binding
}{
	{"Navigation", []binding{
		{"Switch tab", "[ ]  or  Tab", "L1 R1"},
		{"Move up / down", "↑ ↓  or  k j", "D-pad ↑ ↓"},
		{"Previous / next page", "← →  or  PgUp PgDn", "D-pad ← →"},
		{"First / last entry", "Home  End", ""},
	}},
	{"Library", []binding{
		{"Open platform / download game", "Enter", "A"},
		{"Back", "Esc  or  Backspace", "B"},
		{"Download all missing games", "y", "Y"},
		{"Remove game (press twice)", "x", "X"},
		{"Reload from server", "r", ""},
	}},
	{"Downloads", []binding{
		{"Retry failed download", "Enter", "A"},
		{"Download next / retry all failed", "y", "Y"},
		{"Cancel, remove or dismiss", "x", "X"},
	}},
	{"General", []binding{
		{"Show / hide this help", "?  or  F1  or  h", "Select"},
		{"Quit (asks first)", "q  or  Ctrl+C", "Start"},
	}},
}

func (m *model) viewHelp(w, h int) string {
	boxW := min(w, 84)
	inner := boxW - 2

	// Size the key columns to their content; the action column gets the rest.
	keyW, padW := lipgloss.Width("Keyboard"), lipgloss.Width("Controller")
	for _, sec := range helpSections {
		for _, b := range sec.bindings {
			keyW = max(keyW, lipgloss.Width(b.keyboard))
			padW = max(padW, lipgloss.Width(b.controller))
		}
	}
	actW := inner - 2 - keyW - 2 - padW - 2
	stacked := actW < 22 // too narrow for three columns

	cell := func(s string, cw int, st lipgloss.Style) string {
		return st.Render(s) + strings.Repeat(" ", max(cw-lipgloss.Width(s), 0))
	}

	body := []string{""}
	if stacked {
		body = append(body, "  "+sKeyLabel.Render("keyboard")+sDim.Render("  ·  ")+sAccent.Render("controller"))
	} else {
		body = append(body, "  "+cell("", actW, sDim)+"  "+cell("Keyboard", keyW, sBold)+"  "+sBold.Render("Controller"))
	}
	for _, sec := range helpSections {
		body = append(body, "", "  "+sAccent.Bold(true).Render(sec.title))
		for _, b := range sec.bindings {
			c := b.controller
			if c == "" {
				c = "–"
			}
			if stacked {
				body = append(body, "  "+sText.Render(b.action),
					"    "+sKeyLabel.Render(b.keyboard)+sDim.Render("  ·  ")+sAccent.Render(c))
				continue
			}
			body = append(body, "  "+cell(b.action, actW, sText)+"  "+cell(b.keyboard, keyW, sKeyLabel)+"  "+sKeyLabel.Render(c))
		}
	}
	body = append(body, "")
	for _, l := range wrap("Controller buttons assume a Steam Input layout that maps them to these keys (see the README).", inner-4) {
		body = append(body, "  "+sDim.Render(l))
	}
	body = append(body, "  "+sDim.Render("Press ")+sKeyCap.Render(m.key("?", "B"))+sDim.Render(" to close."))

	boxH := min(len(body)+2, h)
	info := ""
	if rows := boxH - 2; len(body) > rows {
		m.helpTop = min(m.helpTop, len(body)-rows)
		body = body[m.helpTop:]
		info = fmt.Sprintf("↑↓ scroll  %d%%", (m.helpTop+rows)*100/(len(body)+m.helpTop))
	} else {
		m.helpTop = 0
	}
	panel := box("Help", info, body, boxW, boxH, true)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, panel)
}
