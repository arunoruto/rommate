package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/arunoruto/rommate/internal/core"
)

// Search is keyboard-only (/): on the platform list it searches games on
// every platform through RomM, inside a platform it filters the games as you
// type.

const (
	searchDelay    = 250 * time.Millisecond // wait for a typing pause before asking RomM
	searchMinChars = 2
	searchLimit    = 100
)

type (
	searchTickMsg int // debounce: the searchSeq it was scheduled for
	searchMsg     struct {
		query string
		roms  []core.RomInfo
		err   error
	}
)

// rows returns the indices of the visible entries of the current list, in
// order: every platform, or the games matching the filter.
func (m *model) rows() []int {
	if m.screen == screenPlatforms {
		return seq(len(m.platforms))
	}
	words := strings.Fields(strings.ToLower(m.query))
	if m.screen == screenSearch || len(words) == 0 {
		return seq(len(m.roms))
	}
	out := make([]int, 0, len(m.roms))
	for i, r := range m.roms {
		if matches(strings.ToLower(r.Title()+" "+r.FSName), words) {
			out = append(out, i)
		}
	}
	return out
}

func seq(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

// matches reports whether text contains every word, in any order.
func matches(text string, words []string) bool {
	for _, w := range words {
		if !strings.Contains(text, w) {
			return false
		}
	}
	return true
}

// filtering reports whether a game filter or search is shown.
func (m *model) filtering() bool {
	return m.searching || m.query != "" || m.screen == screenSearch
}

// startSearch handles /.
func (m *model) startSearch() (tea.Model, tea.Cmd) {
	if m.loading && m.screen != screenSearch {
		return m, nil
	}
	if m.screen == screenPlatforms {
		m.searchFrom = m.cursor
		m.screen, m.roms, m.query, m.err = screenSearch, nil, "", nil
		m.cursor, m.offset = 0, 0
	}
	m.searching = true
	return m, nil
}

// leaveSearch returns from the all-platform search to the platform list.
func (m *model) leaveSearch() {
	m.screen, m.roms, m.query, m.searching, m.err = screenPlatforms, nil, "", false, nil
	m.loading = false
	m.cursor, m.offset = m.searchFrom, 0
}

// clearFilter drops the game filter, keeping the selected game selected.
func (m *model) clearFilter() {
	id := -1
	if rows := m.rows(); m.cursor < len(rows) {
		id = m.roms[rows[m.cursor]].ID
	}
	m.query, m.searching = "", false
	m.cursor = 0
	for i, r := range m.roms {
		if r.ID == id {
			m.cursor = i
		}
	}
}

// searchKey edits the query while the search line has focus.
func (m *model) searchKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "ctrl+c":
		m.cancelAll()
		return m, tea.Quit
	case "esc":
		if m.screen == screenSearch {
			m.leaveSearch()
		} else {
			m.clearFilter()
		}
		return m, nil
	case "enter":
		m.searching = false
		if m.screen == screenSearch && len(m.rows()) == 0 {
			return m, m.runSearch()
		}
		return m, nil
	case "up", "down", "pgup", "pgdown":
		m.moveCursor(k.String(), &m.cursor, m.listLen())
		return m, nil
	case "backspace":
		r := []rune(m.query)
		if len(r) == 0 {
			return m, nil
		}
		m.query = string(r[:len(r)-1])
	case "ctrl+u":
		m.query = ""
	case " ":
		m.query += " "
	default:
		if k.Type != tea.KeyRunes {
			return m, nil
		}
		m.query += string(k.Runes)
	}
	m.cursor, m.offset = 0, 0
	if m.screen != screenSearch {
		return m, nil
	}
	m.searchSeq++
	id := m.searchSeq
	return m, tea.Tick(searchDelay, func(time.Time) tea.Msg { return searchTickMsg(id) })
}

// runSearch asks RomM for games matching the query.
func (m *model) runSearch() tea.Cmd {
	q := strings.TrimSpace(m.query)
	if len([]rune(q)) < searchMinChars {
		m.roms, m.loading, m.err = nil, false, nil
		return nil
	}
	m.loading, m.err = true, nil
	query := m.query
	return func() tea.Msg {
		roms, err := m.engine.Search(m.ctx, q, searchLimit)
		return searchMsg{query: query, roms: roms, err: err}
	}
}

// handleSearchMsg applies debounce ticks and search results.
func (m *model) handleSearchMsg(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case searchTickMsg:
		if int(msg) != m.searchSeq || m.screen != screenSearch {
			return nil, true
		}
		return m.runSearch(), true
	case searchMsg:
		if m.screen != screenSearch || msg.query != m.query {
			return nil, true // stale
		}
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return nil, true
		}
		m.roms, m.err = msg.roms, nil
		m.cursor, m.offset = 0, 0
		return nil, true
	}
	return nil, false
}

// platformName returns the display name of a platform by id.
func (m *model) platformName(id int, fallback string) string {
	for _, p := range m.platforms {
		if p.ID == id {
			return p.DisplayName()
		}
	}
	return fallback
}
