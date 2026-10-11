package tui

import (
	"context"
	"errors"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/arunoruto/rommate/internal/core"
)

// The download queue lives in the model and is only touched from Update, so it
// needs no locking. Downloads run in goroutines started by tea.Cmds; progress
// arrives as eventMsg and completion as jobDoneMsg.

// maxParallel is the number of simultaneous downloads.
const maxParallel = 1

// maxHistory caps the finished and failed lists.
const maxHistory = 50

type job struct {
	rom    core.RomInfo
	cancel context.CancelFunc
}

type failedJob struct {
	rom core.RomInfo
	err string
	at  time.Time
}

type finishedJob struct {
	rom core.RomInfo
	at  time.Time
}

// jobDoneMsg reports that a download goroutine returned.
type jobDoneMsg struct {
	rom      core.RomInfo
	err      error
	canceled bool
}

// download holds live progress of a running job.
type download struct {
	id          int
	name        string
	done, total int64
	// For the speed estimate: bytes and time when this run started (resumed
	// downloads start above zero).
	startBytes int64
	startTime  time.Time
}

func (d *download) speed() float64 {
	secs := time.Since(d.startTime).Seconds()
	if secs < 0.5 {
		return 0
	}
	return float64(d.done-d.startBytes) / secs
}

// eta estimates the remaining time, or 0 if unknown.
func (d *download) eta() time.Duration {
	s := d.speed()
	if s <= 0 || d.total <= d.done {
		return 0
	}
	return time.Duration(float64(d.total-d.done)/s) * time.Second
}

func (m *model) isQueued(id int) bool {
	for _, r := range m.pending {
		if r.ID == id {
			return true
		}
	}
	return false
}

func (m *model) isBusy(id int) bool { return m.running[id] != nil || m.isQueued(id) }

// enqueue adds a game to the end of the queue and starts it if a slot is free.
func (m *model) enqueue(r core.RomInfo) tea.Cmd {
	if r.Installed || m.isBusy(r.ID) {
		return nil
	}
	m.dropFailed(r.ID)
	m.pending = append(m.pending, r)
	return m.startNext()
}

// startNext starts queued downloads while slots are free.
func (m *model) startNext() tea.Cmd {
	var cmds []tea.Cmd
	for len(m.running) < maxParallel && len(m.pending) > 0 {
		r := m.pending[0]
		m.pending = m.pending[1:]
		ctx, cancel := context.WithCancel(m.ctx)
		m.running[r.ID] = &job{rom: r, cancel: cancel}
		m.active[r.ID] = &download{id: r.ID, name: r.Title(), total: r.FSSizeBytes, startTime: time.Now()}
		cmds = append(cmds, m.runJob(ctx, r))
	}
	return tea.Batch(cmds...)
}

func (m *model) runJob(ctx context.Context, r core.RomInfo) tea.Cmd {
	return func() tea.Msg {
		err := m.engine.Pull(ctx, r.Rom, func(ev core.Event) {
			select {
			case m.events <- ev:
			case <-ctx.Done():
			}
		})
		return jobDoneMsg{rom: r, err: err, canceled: errors.Is(ctx.Err(), context.Canceled)}
	}
}

// finishJob records the outcome of a download and starts the next one.
func (m *model) finishJob(msg jobDoneMsg) tea.Cmd {
	id := msg.rom.ID
	if j := m.running[id]; j != nil {
		j.cancel()
	}
	delete(m.running, id)
	delete(m.active, id)

	var cmds []tea.Cmd
	switch {
	case msg.canceled:
		cmds = append(cmds, m.setToast("Cancelled "+msg.rom.Title()+" (it will resume next time)", toastInfo))
	case msg.err != nil:
		m.failed = prepend(m.failed, failedJob{rom: msg.rom, err: msg.err.Error(), at: time.Now()})
		cmds = append(cmds, m.setToast(msg.rom.Title()+": "+msg.err.Error(), toastBad))
	default:
		m.markInstalled(id, true)
		m.finished = prepend(m.finished, finishedJob{rom: msg.rom, at: time.Now()})
		cmds = append(cmds, m.loadCounts, m.setToast("Downloaded "+msg.rom.Title(), toastGood))
	}
	m.clampDownloadsCursor()
	return tea.Batch(append(cmds, m.startNext())...)
}

// cancel stops a running download or removes a queued one.
func (m *model) cancel(id int) {
	if j := m.running[id]; j != nil {
		j.cancel() // finishJob runs when the goroutine returns
		return
	}
	for i, r := range m.pending {
		if r.ID == id {
			m.pending = append(m.pending[:i], m.pending[i+1:]...)
			return
		}
	}
}

// moveToFront makes a queued game the next one to download.
func (m *model) moveToFront(id int) bool {
	for i, r := range m.pending {
		if r.ID == id && i > 0 {
			copy(m.pending[1:i+1], m.pending[:i])
			m.pending[0] = r
			return true
		}
	}
	return false
}

func (m *model) retry(id int) tea.Cmd {
	for _, f := range m.failed {
		if f.rom.ID == id {
			return m.enqueue(f.rom)
		}
	}
	return nil
}

func (m *model) dropFailed(id int) {
	for i, f := range m.failed {
		if f.rom.ID == id {
			m.failed = append(m.failed[:i], m.failed[i+1:]...)
			return
		}
	}
}

func (m *model) dropFinished(id int) {
	for i, f := range m.finished {
		if f.rom.ID == id {
			m.finished = append(m.finished[:i], m.finished[i+1:]...)
			return
		}
	}
}

// cancelAll stops everything; used when quitting.
func (m *model) cancelAll() {
	m.pending = nil
	for _, j := range m.running {
		j.cancel()
	}
}

func prepend[T any](s []T, v T) []T {
	s = append([]T{v}, s...)
	if len(s) > maxHistory {
		s = s[:maxHistory]
	}
	return s
}

// ── downloads tab entries ───────────────────────────────────────────────────

type entryKind int

const (
	entryActive entryKind = iota
	entryQueued
	entryFailed
	entryDone
)

type dlEntry struct {
	kind entryKind
	rom  core.RomInfo
	pos  int    // queue position for entryQueued (1-based)
	err  string // for entryFailed
	at   time.Time
}

// downloadEntries lists everything in the Downloads tab in display order:
// running, queued, failed, finished.
func (m *model) downloadEntries() []dlEntry {
	var out []dlEntry
	for _, d := range m.activeDownloads() {
		out = append(out, dlEntry{kind: entryActive, rom: m.running[d.id].rom})
	}
	for i, r := range m.pending {
		out = append(out, dlEntry{kind: entryQueued, rom: r, pos: i + 1})
	}
	for _, f := range m.failed {
		out = append(out, dlEntry{kind: entryFailed, rom: f.rom, err: f.err, at: f.at})
	}
	for _, f := range m.finished {
		out = append(out, dlEntry{kind: entryDone, rom: f.rom, at: f.at})
	}
	return out
}

func (m *model) clampDownloadsCursor() {
	n := len(m.downloadEntries())
	m.dlCursor = min(m.dlCursor, max(n-1, 0))
}

// pendingCount is the number of running plus queued downloads (header badge).
func (m *model) pendingCount() int { return len(m.running) + len(m.pending) }
