package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *model) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()
	if m.quitting {
		return m.quitKey(key)
	}
	if m.help {
		return m.helpKey(key)
	}
	if key != "x" {
		m.confirmID = 0
	}

	switch key {
	case "ctrl+c":
		m.cancelAll()
		return m, tea.Quit
	case "q":
		if m.confirmQuit || m.pendingCount() > 0 {
			m.quitting = true
			return m, nil
		}
		return m, tea.Quit
	case "?", "f1", "h":
		m.help = true
		return m, nil
	case "tab", "]":
		m.tab = (m.tab + 1) % numTabs
		return m, nil
	case "shift+tab", "[":
		m.tab = (m.tab + numTabs - 1) % numTabs
		return m, nil
	}

	if m.tab == tabDownloads {
		return m.downloadsKey(key)
	}
	return m.libraryKey(key)
}

// quitKey handles the "Quit rommate?" dialog: A/Enter (or q/Start again)
// quits, anything else cancels.
func (m *model) quitKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter", "q", "ctrl+c":
		m.cancelAll()
		return m, tea.Quit
	}
	m.quitting = false
	return m, nil
}

func (m *model) helpKey(key string) (tea.Model, tea.Cmd) {
	// Any of help/back/confirm closes the help screen; q closes it instead of
	// quitting so a stray Start press is harmless.
	switch key {
	case "?", "f1", "h", "esc", "backspace", "enter", "q":
		m.help, m.helpTop = false, 0
	case "up", "k":
		m.helpTop = max(m.helpTop-1, 0)
	case "down", "j":
		m.helpTop++ // clamped when rendering
	case "pgup", "left":
		m.helpTop = max(m.helpTop-m.pageSize(), 0)
	case "pgdown", "right":
		m.helpTop += m.pageSize()
	case "ctrl+c":
		m.cancelAll()
		return m, tea.Quit
	}
	return m, nil
}

// moveCursor applies navigation keys to a list cursor and reports whether the
// key was a navigation key.
func (m *model) moveCursor(key string, cursor *int, n int) bool {
	switch key {
	case "up", "k":
		if *cursor > 0 {
			*cursor--
		} else if n > 0 {
			*cursor = n - 1 // wrap around, handy on a D-pad
		}
	case "down", "j":
		if *cursor < n-1 {
			*cursor++
		} else {
			*cursor = 0
		}
	case "pgup", "left":
		*cursor = max(*cursor-m.pageSize(), 0)
	case "pgdown", "right":
		*cursor = min(*cursor+m.pageSize(), max(n-1, 0))
	case "home":
		*cursor = 0
	case "end":
		*cursor = max(n-1, 0)
	default:
		return false
	}
	return true
}

func (m *model) libraryKey(key string) (tea.Model, tea.Cmd) {
	n := m.listLen()
	if m.moveCursor(key, &m.cursor, n) {
		return m, nil
	}
	switch key {
	case "esc", "backspace":
		if m.screen == screenRoms {
			m.romCursor[m.platform.ID] = m.cursor
			m.screen, m.roms, m.err = screenPlatforms, nil, nil
			m.cursor = m.platformIndex()
			m.offset = 0
		}
	case "enter":
		if n == 0 || m.loading {
			break
		}
		if m.screen == screenPlatforms {
			m.platform = m.platforms[m.cursor]
			m.screen, m.loading = screenRoms, true
			return m, m.loadRoms(m.platform.ID)
		}
		r := m.roms[m.cursor]
		switch {
		case r.Installed:
			return m, m.setToast(r.Title()+" is already downloaded", toastInfo)
		case m.isBusy(r.ID):
			return m, m.setToast(r.Title()+" is already in the queue", toastInfo)
		}
		return m, tea.Batch(m.enqueue(r), m.setToast("Queued "+r.Title(), toastInfo))
	case "y":
		// Download every missing game of the current platform.
		if m.screen != screenRoms || m.loading {
			break
		}
		var cmds []tea.Cmd
		count := 0
		for _, r := range m.roms {
			if !r.Installed && !m.isBusy(r.ID) {
				cmds = append(cmds, m.enqueue(r))
				count++
			}
		}
		if count == 0 {
			return m, m.setToast("Everything is already downloaded", toastInfo)
		}
		return m, tea.Batch(append(cmds, m.setToast(fmt.Sprintf("Queued %d games", count), toastInfo))...)
	case "x":
		if m.screen != screenRoms || n == 0 {
			break
		}
		r := &m.roms[m.cursor]
		if !r.Installed || m.isBusy(r.ID) {
			break
		}
		if m.confirmID != r.ID {
			m.confirmID = r.ID
			return m, m.setToast("Press "+m.key("x", "X")+" again to delete "+r.Title(), toastBad)
		}
		m.confirmID = 0
		if err := m.engine.Remove(r.Rom); err != nil {
			return m, m.setToast(err.Error(), toastBad)
		}
		r.Installed = false
		m.dropFinished(r.ID)
		return m, tea.Batch(m.loadCounts, m.setToast("Removed "+r.Title(), toastInfo))
	case "r":
		m.loading, m.err = true, nil
		if m.screen == screenPlatforms {
			return m, tea.Batch(m.loadPlatforms, m.loadCounts)
		}
		return m, m.loadRoms(m.platform.ID)
	}
	return m, nil
}

func (m *model) downloadsKey(key string) (tea.Model, tea.Cmd) {
	entries := m.downloadEntries()
	if m.moveCursor(key, &m.dlCursor, len(entries)) {
		return m, nil
	}
	if len(entries) == 0 {
		return m, nil
	}
	e := entries[min(m.dlCursor, len(entries)-1)]
	switch key {
	case "enter":
		if e.kind == entryFailed {
			return m, tea.Batch(m.retry(e.rom.ID), m.setToast("Retrying "+e.rom.Title(), toastInfo))
		}
	case "y":
		switch e.kind {
		case entryQueued:
			if m.moveToFront(e.rom.ID) {
				m.dlCursor = len(m.running) // follow the item to the top of the queue
				return m, m.setToast(e.rom.Title()+" is next", toastInfo)
			}
		case entryFailed:
			var cmds []tea.Cmd
			for _, f := range append([]failedJob(nil), m.failed...) {
				cmds = append(cmds, m.retry(f.rom.ID))
			}
			return m, tea.Batch(append(cmds, m.setToast("Retrying all failed downloads", toastInfo))...)
		}
	case "x":
		switch e.kind {
		case entryActive, entryQueued:
			m.cancel(e.rom.ID)
			m.clampDownloadsCursor()
			if e.kind == entryQueued {
				return m, m.setToast("Removed "+e.rom.Title()+" from the queue", toastInfo)
			}
		case entryFailed:
			m.dropFailed(e.rom.ID)
		case entryDone:
			m.dropFinished(e.rom.ID)
		}
		m.clampDownloadsCursor()
	}
	return m, nil
}

// selectedDownload returns the entry under the Downloads cursor.
func (m *model) selectedDownload() (dlEntry, bool) {
	entries := m.downloadEntries()
	if len(entries) == 0 {
		return dlEntry{}, false
	}
	return entries[min(m.dlCursor, len(entries)-1)], true
}
