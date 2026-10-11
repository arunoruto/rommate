// Package tui is rommate's controller-friendly terminal UI.
//
// Every action is bound to a small set of keys that Steam Input can map to a
// gamepad: arrows (D-pad), enter (A), esc (B), y (Y), x (X), [ and ] (L1/R1),
// f1 (Select) and q (Start).
package tui

import (
	"context"
	"net/url"
	"sort"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/arunoruto/rommate/internal/core"
)

// Run starts the TUI.
func Run(ctx context.Context, e *core.Engine, opts Options) error {
	m := newModel(ctx, e)
	m.input = defaultInputMode(opts)
	_, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(ctx)).Run()
	if err == tea.ErrProgramKilled && ctx.Err() != nil {
		return nil
	}
	return err
}

type screen int

const (
	screenPlatforms screen = iota
	screenRoms
)

// tab is a top-level page in the header tab bar.
type tab int

const (
	tabLibrary tab = iota
	tabDownloads
	numTabs
)

func (t tab) title() string {
	switch t {
	case tabDownloads:
		return "Downloads"
	default:
		return "Library"
	}
}

// toastDuration is how long status messages stay visible.
const toastDuration = 4 * time.Second

type toastKind int

const (
	toastInfo toastKind = iota
	toastGood
	toastBad
)

type toast struct {
	text string
	kind toastKind
	id   int
}

type model struct {
	ctx    context.Context
	engine *core.Engine
	host   string

	tab       tab
	screen    screen
	platforms []core.PlatformInfo
	roms      []core.RomInfo
	platform  core.PlatformInfo
	counts    map[string]int // downloaded games per platform slug
	cursor    int
	offset    int
	loading   bool
	err       error
	confirmID int  // rom awaiting a second X press to confirm removal
	quitArmed bool // q pressed once while downloads are running
	help      bool
	input     inputMode
	helpTop   int // first visible help line when the help is scrolled

	toast   toast
	toastID int

	events    chan core.Event
	pending   []core.RomInfo    // queued, in order
	running   map[int]*job      // running downloads by rom id
	active    map[int]*download // live progress of running downloads
	failed    []failedJob       // newest first
	finished  []finishedJob     // newest first
	dlCursor  int               // cursor in the Downloads tab
	dlOffset  int
	romCursor map[int]int // remembered cursor per platform

	spinner spinner.Model
	bar     progress.Model

	width, height int
}

func newModel(ctx context.Context, e *core.Engine) *model {
	host := e.Client.BaseURL
	if u, err := url.Parse(e.Client.BaseURL); err == nil && u.Host != "" {
		host = u.Host
	}
	sp := spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(sAccent))
	bar := progress.New(progress.WithScaledGradient("#7B61FF", "#FF7AB8"), progress.WithoutPercentage())
	bar.Full, bar.Empty = '━', '━'
	bar.EmptyColor = "#3A3A48"
	return &model{
		ctx:       ctx,
		engine:    e,
		host:      host,
		loading:   true,
		counts:    map[string]int{},
		events:    make(chan core.Event, 64),
		running:   map[int]*job{},
		active:    map[int]*download{},
		romCursor: map[int]int{},
		spinner:   sp,
		bar:       bar,
	}
}

type (
	platformsMsg []core.PlatformInfo
	romsMsg      []core.RomInfo
	countsMsg    map[string]int
	errMsg       struct{ error }
	eventMsg     core.Event
	clearToast   int
)

func (m *model) loadPlatforms() tea.Msg {
	ps, err := m.engine.Platforms(m.ctx)
	if err != nil {
		return errMsg{err}
	}
	return platformsMsg(ps)
}

func (m *model) loadRoms(id int) tea.Cmd {
	return func() tea.Msg {
		rs, err := m.engine.Roms(m.ctx, id)
		if err != nil {
			return errMsg{err}
		}
		return romsMsg(rs)
	}
}

func (m *model) loadCounts() tea.Msg {
	c, err := m.engine.DownloadedCounts()
	if err != nil {
		return errMsg{err}
	}
	return countsMsg(c)
}

func (m *model) waitEvent() tea.Msg {
	select {
	case ev := <-m.events:
		return eventMsg(ev)
	case <-m.ctx.Done():
		return nil
	}
}

func (m *model) setToast(text string, kind toastKind) tea.Cmd {
	m.toastID++
	id := m.toastID
	m.toast = toast{text: text, kind: kind, id: id}
	return tea.Tick(toastDuration, func(time.Time) tea.Msg { return clearToast(id) })
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(m.loadPlatforms, m.loadCounts, m.waitEvent, m.spinner.Tick)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case platformsMsg:
		m.platforms, m.loading, m.err = msg, false, nil
		m.cursor = min(m.cursor, max(len(m.platforms)-1, 0))
	case romsMsg:
		m.roms, m.loading, m.err = msg, false, nil
		m.cursor = min(m.romCursor[m.platform.ID], max(len(m.roms)-1, 0))
		m.offset = 0
	case countsMsg:
		m.counts = msg
	case inputModeMsg:
		m.input = inputMode(msg)
	case errMsg:
		m.loading, m.err = false, msg.error
	case clearToast:
		if int(msg) == m.toast.id {
			m.toast = toast{}
		}
	case eventMsg:
		m.handleEvent(core.Event(msg))
		return m, m.waitEvent
	case jobDoneMsg:
		return m, m.finishJob(msg)
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// handleEvent applies progress from a running download. Completion is
// handled by finishJob, so late events for finished jobs are ignored.
func (m *model) handleEvent(ev core.Event) {
	d := m.active[ev.RomID]
	if d == nil || (ev.Type != core.EventStart && ev.Type != core.EventProgress) {
		return
	}
	if ev.Type == core.EventProgress && d.done == 0 && ev.Done > 0 && d.startBytes == 0 {
		// First progress of a resumed download reports the resume offset.
		d.startBytes, d.startTime = ev.Done, time.Now()
	}
	if ev.Total > 0 {
		d.total = ev.Total
	}
	d.done = ev.Done
}

func (m *model) markInstalled(id int, installed bool) {
	for i := range m.roms {
		if m.roms[i].ID == id {
			m.roms[i].Installed = installed
		}
	}
}

func (m *model) listLen() int {
	if m.screen == screenPlatforms {
		return len(m.platforms)
	}
	return len(m.roms)
}

// key returns the keyboard or controller name of a binding for the current
// input mode.
func (m *model) key(keyboard, controller string) string {
	if m.input == inputController {
		return controller
	}
	return keyboard
}

func (m *model) platformIndex() int {
	for i, p := range m.platforms {
		if p.ID == m.platform.ID {
			return i
		}
	}
	return 0
}

// activeDownloads returns running downloads in a stable order.
func (m *model) activeDownloads() []*download {
	out := make([]*download, 0, len(m.active))
	for _, d := range m.active {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out
}
