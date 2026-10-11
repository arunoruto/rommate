package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/dustin/go-humanize"
)

// viewQueue renders the Downloads tab list.
func (m *model) viewQueue(w, h int) string {
	inner, rows := w-2, h-2
	entries := m.downloadEntries()
	if len(entries) == 0 {
		body := []string{"", sDim.Render("  No downloads yet."), ""}
		for _, l := range wrap("Open a platform in the Library tab and press "+m.key("Enter", "A")+" on a game to download it.", inner-4) {
			body = append(body, "  "+sDim.Render(l))
		}
		return box("Downloads", "", body, w, h, true)
	}

	m.clampDownloadsCursor()
	if m.dlCursor < m.dlOffset {
		m.dlOffset = m.dlCursor
	}
	if m.dlCursor >= m.dlOffset+rows {
		m.dlOffset = m.dlCursor - rows + 1
	}
	var body []string
	for i := m.dlOffset; i < min(m.dlOffset+rows, len(entries)); i++ {
		body = append(body, m.viewQueueRow(entries[i], i == m.dlCursor, inner))
	}
	return box("Downloads", fmt.Sprintf("%d / %d", m.dlCursor+1, len(entries)), body, w, h, true)
}

func (m *model) viewQueueRow(e dlEntry, selected bool, w int) string {
	var iconPlain, icon, right, rightPlain string
	switch e.kind {
	case entryActive:
		iconPlain, icon = "↓", sAccent.Render("↓")
		pct := 0.0
		if d := m.active[e.rom.ID]; d != nil && d.total > 0 {
			pct = float64(d.done) / float64(d.total)
		}
		rightPlain = fmt.Sprintf("%3.0f%%", pct*100)
		right = sAccent.Render(rightPlain)
		if barW := min(16, w/5); barW >= 6 && !selected {
			m.bar.Width = barW
			right = m.bar.ViewAs(pct) + " " + right
			rightPlain = strings.Repeat("━", barW) + " " + rightPlain
		}
	case entryQueued:
		iconPlain, icon = "•", sWarn.Render("•")
		rightPlain = fmt.Sprintf("#%d  %s", e.pos, humanize.IBytes(uint64(e.rom.FSSizeBytes)))
		right = sDim.Render(rightPlain)
	case entryFailed:
		iconPlain, icon = "!", sBad.Render("!")
		rightPlain = "failed"
		right = sBad.Render(rightPlain)
	case entryDone:
		iconPlain, icon = "✓", sGood.Render("✓")
		rightPlain = "done " + shortAgo(e.at)
		right = sDim.Render(rightPlain)
	}

	marker := " "
	if selected {
		marker = "›"
	}
	space := w - 5 - lipgloss.Width(rightPlain)
	left := ansi.Truncate(e.rom.Title(), max(space-1, 1), "…")
	gap := strings.Repeat(" ", max(space-lipgloss.Width(left), 1))
	if selected {
		return sSelected.Render(marker + " " + iconPlain + " " + left + gap + rightPlain + " ")
	}
	return "  " + icon + " " + sText.Render(left) + gap + right + " "
}

// viewQueueDetails renders the details pane for the selected download.
func (m *model) viewQueueDetails(w, h int) string {
	inner := w - 2
	e, ok := m.selectedDownload()
	if !ok {
		return box("Details", "", nil, w, h, false)
	}
	lines := []string{""}
	for _, l := range wrap(e.rom.Title(), inner-4) {
		lines = append(lines, "  "+sBold.Render(l))
	}
	lines = append(lines, "")

	size := humanize.IBytes(uint64(e.rom.FSSizeBytes))
	switch e.kind {
	case entryActive:
		d := m.active[e.rom.ID]
		pct := 0.0
		if d != nil && d.total > 0 {
			pct = float64(d.done) / float64(d.total)
		}
		m.bar.Width = max(inner-4, 4)
		lines = append(lines, "  "+m.bar.ViewAs(pct))
		if d != nil {
			lines = append(lines, "  "+sDim.Render(fmt.Sprintf("%.0f%%  ·  %s of %s", pct*100,
				humanize.IBytes(uint64(d.done)), humanize.IBytes(uint64(d.total)))), "")
			speed, eta := "–", "–"
			if s := d.speed(); s > 0 {
				speed = humanize.IBytes(uint64(s)) + "/s"
			}
			if t := d.eta(); t > 0 {
				eta = t.Round(time.Second).String()
			}
			lines = append(lines, kv("Speed", speed, inner)...)
			lines = append(lines, kv("Remaining", eta, inner)...)
		}
	case entryQueued:
		lines = append(lines, kv("Status", sWarn.Render(fmt.Sprintf("• Queued (#%d)", e.pos)), inner)...)
		lines = append(lines, kv("Size", size, inner)...)
	case entryFailed:
		lines = append(lines, kv("Status", sBad.Render("! Failed "+shortAgo(e.at)), inner)...)
		lines = append(lines, kv("Error", e.err, inner)...)
		lines = append(lines, kv("Size", size, inner)...)
		lines = append(lines, "", "  "+sDim.Render("Partial downloads resume where they stopped."))
	case entryDone:
		lines = append(lines, kv("Status", sGood.Render("✓ Downloaded "+shortAgo(e.at)), inner)...)
		lines = append(lines, kv("Size", size, inner)...)
	}
	if e.rom.PlatformSlug != "" {
		lines = append(lines, kv("Folder", m.engine.Mapper.Folder(e.rom.PlatformSlug), inner)...)
	}
	if e.rom.Path != "" {
		lines = append(lines, kv("Path", e.rom.Path, inner)...)
	}
	return box("Details", "", lines, w, h, false)
}

// queueHints returns footer hints for the selected download.
func (m *model) queueHints(tabs, quit hint) []hint {
	k := m.key
	var hints []hint
	if e, ok := m.selectedDownload(); ok {
		switch e.kind {
		case entryActive:
			hints = []hint{{k("x", "X"), "Cancel"}}
		case entryQueued:
			hints = []hint{{k("y", "Y"), "Download next"}, {k("x", "X"), "Remove"}}
		case entryFailed:
			hints = []hint{{k("Enter", "A"), "Retry"}, {k("y", "Y"), "Retry all"}, {k("x", "X"), "Dismiss"}}
		case entryDone:
			hints = []hint{{k("x", "X"), "Dismiss"}}
		}
	}
	return append(hints, tabs, quit)
}

// shortAgo formats a timestamp as "just now", "5m ago", "2h ago".
func shortAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return t.Format("Jan 2")
}
