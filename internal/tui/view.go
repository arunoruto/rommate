package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/dustin/go-humanize"

	"github.com/arunoruto/rommate/internal/core"
)

// Layout constants.
const wideWidth = 96 // show the details pane from this width on

func (m *model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	header := m.viewHeader()
	downloads := m.viewDownloadBar()
	footer := m.viewFooter()

	panelH := max(m.height-1-len(downloads)-1, 4)
	var list, details func(w, h int) string
	switch m.tab {
	case tabDownloads:
		list, details = m.viewQueue, m.viewQueueDetails
	case tabSaves:
		list, details = m.viewSaves, m.viewSavesDetails
	case tabSettings:
		list, details = m.viewSettings, m.viewSettingsDetails
	default:
		list, details = m.viewList, m.viewDetails
	}
	var panels string
	switch {
	case m.quitSyncing:
		panels = m.viewQuitSync(m.width, panelH)
	case m.resolving != nil:
		panels = m.viewResolve(m.width, panelH)
	case m.quitting:
		panels = m.viewQuit(m.width, panelH)
	case m.help:
		panels = m.viewHelp(m.width, panelH)
	case m.width >= wideWidth:
		listW := m.width * 3 / 5
		panels = lipgloss.JoinHorizontal(lipgloss.Top, list(listW, panelH), details(m.width-listW, panelH))
	default:
		panels = list(m.width, panelH)
	}

	lines := []string{header}
	lines = append(lines, strings.Split(panels, "\n")...)
	lines = append(lines, downloads...)
	lines = append(lines, footer)
	return strings.Join(lines, "\n")
}

// pageSize is the number of visible list rows.
func (m *model) pageSize() int {
	return max(m.height-1-len(m.viewDownloadBar())-1-2, 1)
}

// ── header & footer ─────────────────────────────────────────────────────────

func (m *model) viewHeader() string {
	left := sLogo.Render("rommate") + " "
	for t := tab(0); t < numTabs; t++ {
		label := t.title()
		if t == tabDownloads {
			if n := m.pendingCount(); n > 0 {
				label += fmt.Sprintf(" %d", n)
			} else if len(m.failed) > 0 {
				label += " !"
			}
		}
		if t == tabSaves {
			if m.savesSyncing {
				label += " …"
			} else if n := m.conflictCount(); n > 0 {
				label += fmt.Sprintf(" !%d", n)
			}
		}
		if t == m.tab {
			left += " " + sTabActive.Render(label)
		} else {
			left += " " + sTab.Render(label)
		}
	}
	if lipgloss.Width(left) < m.width/2 {
		left += "  " + sDim.Render(m.key("[ ]", "L1 R1"))
	}

	dot := sGood.Render("●")
	if m.err != nil {
		dot = sBad.Render("●")
	}
	host := m.host
	if m.engine.Client.Local {
		host += " · local"
	}
	right := dot + " " + sDim.Render(host)
	if m.loading || len(m.active) > 0 {
		right = m.spinner.View() + " " + right
	}
	return spread(left, right+" ", m.width)
}

type hint struct{ key, label string }

func (m *model) viewFooter() string {
	k := m.key
	tabs := hint{k("[ ]", "L1 R1"), "Tabs"}
	quit := hint{k("q", "Start"), "Quit"}
	hints := []hint{{k("Enter", "A"), "Open"}, tabs, quit}
	switch {
	case m.quitSyncing:
		hints = []hint{{"Ctrl+C", "Quit now"}}
	case m.resolving != nil:
		hints = []hint{{k("x", "X"), "Keep this device"}, {k("y", "Y"), "Keep RomM"}, {k("Esc", "B"), "Cancel"}}
	case m.quitting:
		hints = []hint{{k("Enter", "A"), "Quit"}, {k("Esc", "B"), "Cancel"}}
	case m.tab == tabSaves:
		hints = m.savesHints(tabs, quit)
	case m.tab == tabSettings:
		hints = m.settingsHints(tabs, quit)
	case m.help:
		hints = []hint{{k("Esc", "B"), "Close help"}}
	case m.tab == tabDownloads:
		hints = m.queueHints(tabs, quit)
	case m.searching:
		hints = []hint{{"Enter", "Done"}, {"Esc", "Cancel"}, {"↑ ↓", "Move"}}
	case m.screen != screenPlatforms:
		all := "All missing"
		if m.filtering() {
			all = "All matching"
		}
		hints = []hint{{k("Enter", "A"), "Download"}, {k("y", "Y"), all},
			{k("x", "X"), "Remove"}, {k("Esc", "B"), "Back"}}
		if m.input != inputController && !m.filtering() {
			hints = append(hints, hint{"/", "Filter"})
		}
		hints = append(hints, tabs, quit)
	case m.input != inputController:
		hints = []hint{hints[0], {"/", "Search"}, tabs, quit}
	}

	// The help hint is pinned to the bottom-right corner and always shown;
	// a compact form is used on very narrow screens.
	help := sKeyCap.Render(k("?", "Select")) + " " + sKeyLabel.Render("Help") + " "
	if m.help {
		help = sKeyCap.Render(k("?", "Select")) + " " + sKeyLabel.Render("Close") + " "
	}
	if lipgloss.Width(help) > m.width/2 {
		help = sKeyCap.Render(k("?", "Select")) + " "
	}

	// Toasts sit just left of the help hint and get whatever room is left.
	toastText := ""
	if m.toast.text != "" {
		style := sAccent
		icon := "›"
		switch m.toast.kind {
		case toastGood:
			style, icon = sGood, "✓"
		case toastBad:
			style, icon = sBad, "!"
		}
		// Leave room for the primary hint (A) so the main action stays visible.
		first := sKeyCap.Render(hints[0].key) + " " + sKeyLabel.Render(hints[0].label) + "  "
		room := m.width - lipgloss.Width(help) - lipgloss.Width(first) - 4
		if room >= 8 {
			toastText = style.Render(ansi.Truncate(icon+" "+m.toast.text, min(room, max(m.width/2, 20)), "…")) + "  "
		}
	}
	right := toastText + help

	// Add hints while they fit; they are the first thing dropped.
	left := " "
	for _, h := range hints {
		part := sKeyCap.Render(h.key) + " " + sKeyLabel.Render(h.label) + "  "
		if lipgloss.Width(left+part)+lipgloss.Width(right) > m.width {
			break
		}
		left += part
	}
	return spread(left, right, m.width)
}

// ── list pane ───────────────────────────────────────────────────────────────

func (m *model) viewList(w, h int) string {
	inner := w - 2
	rows := h - 2
	title := "Platforms"
	switch m.screen {
	case screenRoms:
		title = "Platforms › " + m.platform.DisplayName()
	case screenSearch:
		title = "Search all platforms"
	}

	var body []string
	if m.filtering() {
		body = append(body, m.viewSearchLine(inner))
		rows--
	}
	visible := m.rows()
	info := ""
	switch {
	case m.err != nil:
		body = append(body, m.viewError(inner)...)
	case m.loading:
		body = append(body, "", " "+m.spinner.View()+sDim.Render(" Loading…"))
	case len(visible) == 0:
		body = append(body, "", sDim.Render("  "+m.emptyText()))
	default:
		n := len(visible)
		m.cursor = min(m.cursor, n-1)
		if m.cursor < m.offset {
			m.offset = m.cursor
		}
		if m.cursor >= m.offset+rows {
			m.offset = m.cursor - rows + 1
		}
		for i := m.offset; i < min(m.offset+rows, n); i++ {
			body = append(body, m.viewRow(visible[i], i == m.cursor, inner))
		}
		info = fmt.Sprintf("%d / %d", m.cursor+1, n)
		if n < len(m.roms) && m.screen == screenRoms {
			info += fmt.Sprintf(" of %d", len(m.roms))
		}
	}
	return box(title, info, body, w, h, true)
}

// viewSearchLine renders the query, with a cursor while typing.
func (m *model) viewSearchLine(w int) string {
	q := m.query
	if m.searching {
		q += "▏"
	}
	label := " / "
	if m.screen == screenSearch && q == "" {
		return sAccent.Render(label) + sDim.Render("type a game name")
	}
	q = ansi.TruncateLeft(q, max(lipgloss.Width(q)-(w-4), 0), "…")
	if m.searching {
		return sAccent.Render(label) + sText.Render(q)
	}
	return sDim.Render(label) + sAccent.Render(q)
}

// emptyText explains an empty list.
func (m *model) emptyText() string {
	switch {
	case m.screen == screenSearch && len([]rune(strings.TrimSpace(m.query))) < searchMinChars:
		return "Search every platform by game name."
	case m.screen == screenSearch, m.query != "":
		return "No games match “" + strings.TrimSpace(m.query) + "”."
	}
	return "Nothing here yet."
}

func (m *model) viewError(w int) []string {
	lines := []string{"", sBad.Render("  ! Couldn't talk to RomM"), ""}
	for _, l := range wrap(m.err.Error(), w-4) {
		lines = append(lines, "  "+sDim.Render(l))
	}
	return append(lines, "", sDim.Render("  Press ")+sKeyCap.Render("r")+sDim.Render(" to retry"))
}

// viewRow renders entry i of the current list at width w.
func (m *model) viewRow(i int, selected bool, w int) string {
	var icon, iconPlain, left, right string
	var rightStyle lipgloss.Style
	if m.screen == screenPlatforms {
		p := m.platforms[i]
		n := m.counts[p.Slug]
		iconPlain, icon = " ", " "
		left = p.DisplayName()
		right = fmt.Sprintf("%d/%d", n, p.RomCount)
		switch {
		case n >= p.RomCount:
			rightStyle, iconPlain, icon = sGood, "✓", sGood.Render("✓")
		case n > 0:
			rightStyle = sAccent
		default:
			rightStyle = sDim
		}
	} else {
		r := m.roms[i]
		iconPlain, icon = m.romIcon(r)
		left = r.Title()
		right = humanize.IBytes(uint64(r.FSSizeBytes))
		if m.screen == screenSearch {
			right = r.PlatformSlug + "  " + right
		}
		rightStyle = sDim
	}

	marker := " "
	if selected {
		marker = "›"
	}
	space := w - 5 - lipgloss.Width(right) // marker, icon, spaces
	left = ansi.Truncate(left, max(space-1, 1), "…")
	gap := strings.Repeat(" ", max(space-lipgloss.Width(left), 1))

	if selected {
		return sSelected.Render(marker + " " + iconPlain + " " + left + gap + right + " ")
	}
	return " " + " " + icon + " " + sText.Render(left) + gap + rightStyle.Render(right) + " "
}

// romIcon returns the plain and styled status icon of a game.
func (m *model) romIcon(r core.RomInfo) (string, string) {
	switch {
	case m.active[r.ID] != nil:
		return "↓", sAccent.Render("↓")
	case m.isQueued(r.ID):
		return "•", sWarn.Render("•")
	case r.Installed:
		return "✓", sGood.Render("✓")
	}
	return " ", " "
}

// ── details pane ────────────────────────────────────────────────────────────

func (m *model) viewDetails(w, h int) string {
	inner := w - 2
	var body []string
	if rows := m.rows(); !m.loading && m.err == nil && m.cursor < len(rows) {
		if m.screen == screenPlatforms {
			body = m.platformDetails(m.platforms[rows[m.cursor]], inner)
		} else {
			body = m.romDetails(m.roms[rows[m.cursor]], inner)
		}
	}
	return box("Details", "", body, w, h, false)
}

func (m *model) platformDetails(p core.PlatformInfo, w int) []string {
	n := m.counts[p.Slug]
	lines := []string{""}
	for _, l := range wrap(p.DisplayName(), w-4) {
		lines = append(lines, "  "+sBold.Render(l))
	}
	lines = append(lines, "")

	m.bar.Width = max(w-4, 4)
	ratio := 0.0
	if p.RomCount > 0 {
		ratio = float64(n) / float64(p.RomCount)
	}
	lines = append(lines, "  "+m.bar.ViewAs(ratio),
		"  "+sDim.Render(fmt.Sprintf("%d of %d games on this device", n, p.RomCount)), "")

	folder := p.Folder
	if p.Folder != p.Slug {
		folder = p.Slug + " → " + p.Folder
	}
	lines = append(lines, kv("Folder", folder, w)...)
	lines = append(lines, kv("On server", humanize.IBytes(uint64(p.FSSizeBytes)), w)...)
	lines = append(lines, kv("Path", p.Dir, w)...)
	return lines
}

func (m *model) romDetails(r core.RomInfo, w int) []string {
	lines := []string{""}
	for _, l := range wrap(r.Title(), w-4) {
		lines = append(lines, "  "+sBold.Render(l))
	}
	lines = append(lines, "")

	var status string
	switch d := m.active[r.ID]; {
	case d != nil:
		pct := 0.0
		if d.total > 0 {
			pct = float64(d.done) / float64(d.total)
		}
		status = sAccent.Render(fmt.Sprintf("↓ Downloading %.0f%%", pct*100))
	case m.isQueued(r.ID):
		status = sWarn.Render("• Queued")
	case r.Installed:
		status = sGood.Render("✓ On this device")
	default:
		status = sDim.Render("Not downloaded")
	}
	lines = append(lines, kv("Status", status, w)...)
	if m.screen == screenSearch {
		lines = append(lines, kv("Platform", m.platformName(r.PlatformID, r.PlatformSlug), w)...)
	}
	lines = append(lines, kv("Size", humanize.IBytes(uint64(r.FSSizeBytes)), w)...)
	if len(r.Files) > 1 {
		lines = append(lines, kv("Files", fmt.Sprintf("%d", len(r.Files)), w)...)
	}
	lines = append(lines, kv("File", r.FSName, w)...)
	lines = append(lines, kv("Path", r.Path, w)...)
	return lines
}

// kv renders a label/value pair, wrapping long plain values under the value
// column.
func kv(label, value string, w int) []string {
	const labelW = 11
	pad := strings.Repeat(" ", labelW)
	valW := max(w-2-labelW-1, 8)
	var vals []string
	if strings.Contains(value, "\x1b") { // already styled: don't wrap
		vals = []string{ansi.Truncate(value, valW, "…")}
	} else {
		for _, l := range wrap(value, valW) {
			vals = append(vals, sText.Render(l))
		}
	}
	out := []string{"  " + sDim.Render(fmt.Sprintf("%-*s", labelW, label)) + vals[0]}
	for _, v := range vals[1:] {
		out = append(out, "  "+pad+v)
	}
	return out
}

// ── downloads ───────────────────────────────────────────────────────────────

// viewDownloadBar is the Steam-style status bar above the footer, shown on
// every tab: the current download with a progress bar, plus how many more are
// queued. When idle it only appears to point out failed downloads.
func (m *model) viewDownloadBar() []string {
	ds := m.activeDownloads()
	if len(ds) == 0 {
		if n := len(m.failed); n > 0 && m.tab != tabDownloads {
			msg := fmt.Sprintf("%d download failed", n)
			if n > 1 {
				msg = fmt.Sprintf("%d downloads failed", n)
			}
			return []string{" " + sBad.Render("! "+msg) + sDim.Render("  ·  see the Downloads tab ("+m.key("]", "R1")+")")}
		}
		return nil
	}
	d := ds[0]
	pct := 0.0
	if d.total > 0 {
		pct = float64(d.done) / float64(d.total)
	}
	stats := fmt.Sprintf("%3.0f%%  %s / %s", pct*100,
		humanize.IBytes(uint64(d.done)), humanize.IBytes(uint64(d.total)))
	if sp := d.speed(); sp > 0 {
		stats += "  " + humanize.IBytes(uint64(sp)) + "/s"
	}
	if more := m.pendingCount() - 1; more > 0 {
		stats += fmt.Sprintf("  ·  %d queued", more)
	}
	// Drop details from the right until the bar keeps a useful width.
	nameW := max(min(m.width/4, 32), 8)
	for m.width-nameW-lipgloss.Width(stats)-6 < 10 && strings.Contains(stats, "  ") {
		stats = stats[:strings.LastIndex(stats, "  ")]
	}
	barW := max(m.width-nameW-lipgloss.Width(stats)-6, 4)
	m.bar.Width = barW
	name := ansi.Truncate(d.name, nameW, "…")
	name += strings.Repeat(" ", max(nameW-lipgloss.Width(name), 0))
	return []string{" " + sAccent.Render("↓") + " " + sText.Render(name) + " " + m.bar.ViewAs(pct) + " " + sDim.Render(stats)}
}

// ── helpers ─────────────────────────────────────────────────────────────────

// box draws a rounded panel with a title in the top border and optional info
// in the bottom border. Body lines are padded or cut to fit.
func box(title, info string, body []string, w, h int, focused bool) string {
	border := sFaint
	if focused {
		border = sAccent
	}
	inner := w - 2

	titleText := " " + title + " "
	topFill := max(inner-1-lipgloss.Width(titleText), 0)
	top := border.Render("╭─") + sBold.Render(titleText) + border.Render(strings.Repeat("─", topFill)+"╮")

	var bottom string
	if info != "" {
		infoText := " " + info + " "
		fill := max(inner-1-lipgloss.Width(infoText), 0)
		bottom = border.Render("╰"+strings.Repeat("─", fill)) + sDim.Render(infoText) + border.Render("─╯")
	} else {
		bottom = border.Render("╰" + strings.Repeat("─", inner) + "╯")
	}

	lines := []string{top}
	for i := 0; i < h-2; i++ {
		line := ""
		if i < len(body) {
			line = ansi.Truncate(body[i], inner, "")
		}
		pad := max(inner-lipgloss.Width(line), 0)
		lines = append(lines, border.Render("│")+line+strings.Repeat(" ", pad)+border.Render("│"))
	}
	lines = append(lines, bottom)
	return strings.Join(lines, "\n")
}

// spread places left and right at the edges of a w-wide line.
func spread(left, right string, w int) string {
	gap := w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		left = ansi.Truncate(left, max(w-lipgloss.Width(right)-1, 0), "…")
		gap = max(w-lipgloss.Width(left)-lipgloss.Width(right), 0)
	}
	return ansi.Truncate(left+strings.Repeat(" ", gap)+right, w, "")
}

// wrap breaks plain text into lines of at most w cells, splitting long words.
func wrap(s string, w int) []string {
	if w < 1 {
		return []string{s}
	}
	out := strings.Split(ansi.Wrap(s, w, " /-_"), "\n")
	for i, l := range out {
		out[i] = strings.TrimRight(l, " ")
	}
	return out
}
