package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/arunoruto/rommate/internal/saves"
)

// The Saves tab: save state of downloaded games, manual and automatic sync
// (on start and before quitting) and conflict resolution.

type (
	savesStatusMsg struct {
		status []saves.Status
		err    error
	}
	savesSyncedMsg struct {
		res  saves.Result
		err  error
		quit bool // the sync before quitting finished
	}
	savesResolvedMsg struct {
		name, keep string
		err        error
	}
)

func (m *model) loadSaveStatus() tea.Msg {
	se, games, err := m.savesFn()
	if err != nil {
		return savesStatusMsg{err: err}
	}
	st, err := se.Status(games)
	return savesStatusMsg{status: st, err: err}
}

// syncSaves starts a sync in the background.
func (m *model) syncSaves(quit bool) tea.Cmd {
	if m.savesFn == nil || m.savesSyncing {
		return nil
	}
	m.savesSyncing = true
	return func() tea.Msg {
		se, games, err := m.savesFn()
		if err != nil {
			return savesSyncedMsg{err: err, quit: quit}
		}
		res, err := se.Sync(m.ctx, games, func(saves.Event) {})
		return savesSyncedMsg{res: res, err: err, quit: quit}
	}
}

func (m *model) resolveSave(st saves.Status, keep string) tea.Cmd {
	return func() tea.Msg {
		se, games, err := m.savesFn()
		if err != nil {
			return savesResolvedMsg{name: st.Name, keep: keep, err: err}
		}
		for _, g := range games {
			if g.RomID == st.RomID {
				return savesResolvedMsg{name: st.Name, keep: keep, err: se.Resolve(m.ctx, g, keep)}
			}
		}
		return savesResolvedMsg{name: st.Name, keep: keep, err: fmt.Errorf("%s is no longer downloaded", st.Name)}
	}
}

func (m *model) handleSavesMsg(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case savesStatusMsg:
		m.saveStatus, m.savesErr, m.savesLoaded = msg.status, msg.err, true
		m.savesCursor = min(m.savesCursor, max(len(m.saveStatus)-1, 0))
		return nil, true
	case savesSyncedMsg:
		m.savesSyncing = false
		if msg.quit {
			return tea.Quit, true
		}
		if msg.err != nil {
			return m.setToast("Save sync failed: "+msg.err.Error(), toastBad), true
		}
		m.lastSync, m.lastSyncAt = &msg.res, time.Now()
		kind, text := toastGood, syncSummary(msg.res)
		if msg.res.Conflicts > 0 || msg.res.Failed > 0 {
			kind = toastBad
		}
		return tea.Batch(m.loadSaveStatus, m.setToast(text, kind)), true
	case savesResolvedMsg:
		if msg.err != nil {
			return m.setToast(msg.name+": "+msg.err.Error(), toastBad), true
		}
		where := "this device's"
		if msg.keep == "server" {
			where = "RomM's"
		}
		return tea.Batch(m.loadSaveStatus, m.setToast("Kept "+where+" save of "+msg.name, toastGood)), true
	}
	return nil, false
}

func syncSummary(r saves.Result) string {
	var parts []string
	add := func(n int, one, many string) {
		switch {
		case n == 1:
			parts = append(parts, "1 "+one)
		case n > 1:
			parts = append(parts, fmt.Sprintf("%d %s", n, many))
		}
	}
	add(r.Uploaded, "uploaded", "uploaded")
	add(r.Downloaded, "downloaded", "downloaded")
	add(r.Conflicts, "conflict", "conflicts")
	add(r.Failed, "failed", "failed")
	if len(parts) == 0 {
		return "Saves are in sync"
	}
	return "Saves: " + strings.Join(parts, ", ")
}

func (m *model) conflictCount() int {
	n := 0
	for _, st := range m.saveStatus {
		if st.Conflict {
			n++
		}
	}
	return n
}

// ── keys ─────────────────────────────────────────────────────────────────────

func (m *model) savesKey(key string) (tea.Model, tea.Cmd) {
	if m.savesFn == nil {
		return m, nil
	}
	if m.moveCursor(key, &m.savesCursor, len(m.saveStatus)) {
		return m, nil
	}
	switch key {
	case "y":
		if m.savesSyncing {
			return m, m.setToast("A save sync is already running", toastInfo)
		}
		return m, tea.Batch(m.syncSaves(false), m.setToast("Syncing saves…", toastInfo))
	case "enter":
		if st, ok := m.selectedSave(); ok && st.Conflict {
			m.resolving = &st
		}
	case "r":
		return m, m.loadSaveStatus
	}
	return m, nil
}

func (m *model) resolveKey(key string) (tea.Model, tea.Cmd) {
	st := *m.resolving
	switch key {
	case "x":
		m.resolving = nil
		return m, m.resolveSave(st, "local")
	case "y":
		m.resolving = nil
		return m, m.resolveSave(st, "server")
	case "esc", "backspace", "q":
		m.resolving = nil
	case "ctrl+c":
		m.cancelAll()
		return m, tea.Quit
	}
	return m, nil
}

func (m *model) selectedSave() (saves.Status, bool) {
	if len(m.saveStatus) == 0 {
		return saves.Status{}, false
	}
	return m.saveStatus[min(m.savesCursor, len(m.saveStatus)-1)], true
}

// quit leaves the TUI, syncing saves first when automatic sync is on.
func (m *model) quit() tea.Cmd {
	m.quitting = false
	m.cancelAll()
	if m.autoSync && m.savesFn != nil && !m.quitSyncing {
		m.quitSyncing = true
		m.savesSyncing = false // a running sync's result is replaced by this one
		return m.syncSaves(true)
	}
	return tea.Quit
}

// savesHints returns the footer hints of the Saves tab.
func (m *model) savesHints(tabs, quit hint) []hint {
	k := m.key
	hints := []hint{{k("y", "Y"), "Sync now"}}
	if st, ok := m.selectedSave(); ok && st.Conflict {
		hints = append([]hint{{k("Enter", "A"), "Resolve"}}, hints...)
	}
	return append(hints, tabs, quit)
}

// ── views ────────────────────────────────────────────────────────────────────

func (m *model) viewSaves(w, h int) string {
	inner, rows := w-2, h-2
	title := "Saves"
	if m.savesSyncing {
		title = "Saves · syncing…"
	}
	var body []string
	switch {
	case m.savesFn == nil:
		body = []string{"", sDim.Render("  Save sync is not available here.")}
	case m.savesErr != nil:
		body = []string{"", sBad.Render("  ! Could not read saves")}
		for _, l := range wrap(m.savesErr.Error(), inner-4) {
			body = append(body, "  "+sDim.Render(l))
		}
	case !m.savesLoaded:
		body = []string{"", " " + m.spinner.View() + sDim.Render(" Looking for saves…")}
	case len(m.saveStatus) == 0:
		body = []string{"", sDim.Render("  No downloaded games yet.")}
	default:
		if m.savesCursor < m.savesOffset {
			m.savesOffset = m.savesCursor
		}
		if m.savesCursor >= m.savesOffset+rows {
			m.savesOffset = m.savesCursor - rows + 1
		}
		for i := m.savesOffset; i < min(m.savesOffset+rows, len(m.saveStatus)); i++ {
			body = append(body, m.viewSaveRow(m.saveStatus[i], i == m.savesCursor, inner))
		}
		return box(title, fmt.Sprintf("%d / %d", m.savesCursor+1, len(m.saveStatus)), body, w, h, true)
	}
	return box(title, "", body, w, h, true)
}

func (m *model) viewSaveRow(st saves.Status, selected bool, w int) string {
	var iconPlain, icon, right string
	rightStyle := sDim
	switch {
	case st.Conflict:
		iconPlain, icon, right, rightStyle = "!", sBad.Render("!"), "conflict", sBad
	case st.Problem != "":
		iconPlain, icon, right, rightStyle = "?", sWarn.Render("?"), "not supported", sWarn
	case st.Path != "":
		iconPlain, icon, right = "✓", sGood.Render("✓"), st.Emulator
	default:
		iconPlain, icon, right = "·", sDim.Render("·"), "no save yet"
	}
	marker := " "
	if selected {
		marker = "›"
	}
	space := w - 5 - lipgloss.Width(right)
	left := ansi.Truncate(st.Name, max(space-1, 1), "…")
	gap := strings.Repeat(" ", max(space-lipgloss.Width(left), 1))
	if selected {
		return sSelected.Render(marker + " " + iconPlain + " " + left + gap + right + " ")
	}
	return "  " + icon + " " + sText.Render(left) + gap + rightStyle.Render(right) + " "
}

func (m *model) viewSavesDetails(w, h int) string {
	inner := w - 2
	st, ok := m.selectedSave()
	if !ok || m.savesFn == nil {
		return box("Details", "", m.syncInfoLines(inner), w, h, false)
	}
	lines := []string{""}
	for _, l := range wrap(st.Name, inner-4) {
		lines = append(lines, "  "+sBold.Render(l))
	}
	lines = append(lines, "")
	var status string
	switch {
	case st.Conflict:
		status = sBad.Render("! Changed here and on RomM")
	case st.Problem != "":
		status = sWarn.Render("? " + st.Problem)
	case st.Path == "":
		status = sDim.Render("No save yet")
	case !st.SyncedAt.IsZero():
		status = sGood.Render("✓ Synced " + shortAgo(st.SyncedAt))
	default:
		status = sWarn.Render("• Not synced yet")
	}
	lines = append(lines, kv("Status", status, inner)...)
	if st.Emulator != "" {
		lines = append(lines, kv("Emulator", st.Emulator, inner)...)
	}
	if st.Path != "" {
		lines = append(lines, kv("Save", st.Path, inner)...)
	}
	if st.Conflict {
		if !st.ServerUpdatedAt.IsZero() {
			lines = append(lines, kv("On RomM", "changed "+shortAgo(st.ServerUpdatedAt), inner)...)
		}
		lines = append(lines, "", "  "+sDim.Render("Press ")+sKeyCap.Render(m.key("Enter", "A"))+sDim.Render(" to choose which to keep."))
	}
	lines = append(lines, "")
	lines = append(lines, m.syncInfoLines(inner)...)
	return box("Details", "", lines, w, h, false)
}

// syncInfoLines describe the last sync and automatic sync.
func (m *model) syncInfoLines(inner int) []string {
	var lines []string
	switch {
	case m.savesSyncing:
		lines = append(lines, "  "+m.spinner.View()+sDim.Render(" Syncing with RomM…"))
	case m.lastSync != nil:
		lines = append(lines, kv("Last sync", syncSummary(*m.lastSync)+", "+shortAgo(m.lastSyncAt), inner)...)
	}
	if m.autoSync {
		lines = append(lines, "  "+sDim.Render("Saves sync on start and before quitting."))
	}
	return lines
}

func (m *model) viewResolve(w, h int) string {
	st := *m.resolving
	boxW := min(w, 64)
	inner := boxW - 2
	body := []string{""}
	for _, l := range wrap(st.Name, inner-4) {
		body = append(body, "  "+sBold.Render(l))
	}
	body = append(body, "")
	for _, l := range wrap("This save changed on this device and on RomM since the last sync. Which one do you want to keep?", inner-4) {
		body = append(body, "  "+sText.Render(l))
	}
	body = append(body, "")
	if fi, err := os.Stat(st.Path); err == nil {
		body = append(body, kv("This device", "changed "+shortAgo(fi.ModTime()), inner)...)
	}
	if !st.ServerUpdatedAt.IsZero() {
		body = append(body, kv("RomM", "changed "+shortAgo(st.ServerUpdatedAt), inner)...)
	}
	body = append(body, "")
	for _, l := range wrap("The other version is not lost: a replaced local save is backed up, and RomM keeps older versions.", inner-4) {
		body = append(body, "  "+sDim.Render(l))
	}
	body = append(body, "",
		"  "+sKeyCap.Render(m.key("x", "X"))+" "+sKeyLabel.Render("Keep this device")+"   "+
			sKeyCap.Render(m.key("y", "Y"))+" "+sKeyLabel.Render("Keep RomM")+"   "+
			sKeyCap.Render(m.key("Esc", "B"))+" "+sKeyLabel.Render("Cancel"),
		"")
	panel := box("Save conflict", "", body, boxW, min(len(body)+2, h), true)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, panel)
}

func (m *model) viewQuitSync(w, h int) string {
	body := []string{"", "  " + m.spinner.View() + " " + sBold.Render("Syncing saves before quitting…"), "",
		"  " + sDim.Render("Ctrl+C quits right away."), ""}
	panel := box("Quit", "", body, min(w, 52), min(len(body)+2, h), true)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, panel)
}
