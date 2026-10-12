package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// viewQuit renders the "Quit rommate?" dialog centred in the panel area.
func (m *model) viewQuit(w, h int) string {
	boxW := min(w, 56)
	inner := boxW - 2
	body := []string{"", "  " + sBold.Render("Quit rommate?"), ""}
	if n := m.pendingCount(); n > 0 {
		msg := fmt.Sprintf("%d download is still running. It resumes the next time you open rommate.", n)
		if n > 1 {
			msg = fmt.Sprintf("%d downloads are still running. They resume the next time you open rommate.", n)
		}
		for _, l := range wrap(msg, inner-4) {
			body = append(body, "  "+sWarn.Render(l))
		}
		body = append(body, "")
	}
	body = append(body,
		"  "+sKeyCap.Render(m.key("Enter", "A"))+" "+sKeyLabel.Render("Quit")+"   "+
			sKeyCap.Render(m.key("Esc", "B"))+" "+sKeyLabel.Render("Cancel"),
		"")
	panel := box("Quit", "", body, boxW, min(len(body)+2, h), true)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, panel)
}
