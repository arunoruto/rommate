package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/arunoruto/rommate/internal/core"
	"github.com/arunoruto/rommate/internal/romm"
)

func rom(id int, name string) core.RomInfo {
	return core.RomInfo{Rom: romm.Rom{ID: id, Name: name, PlatformSlug: "nes", FSSizeBytes: 1024}}
}

func ids(rs []core.RomInfo) []int {
	var out []int
	for _, r := range rs {
		out = append(out, r.ID)
	}
	return out
}

func TestQueueLifecycle(t *testing.T) {
	m := testModel(100, 30)
	a, b, c := rom(1, "A"), rom(2, "B"), rom(3, "C")
	m.enqueue(a)
	m.enqueue(b)
	m.enqueue(c)
	m.enqueue(b) // duplicate is ignored

	if m.running[1] == nil || len(m.running) != maxParallel {
		t.Fatalf("first game should be running, got %v", m.running)
	}
	if got := ids(m.pending); len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Fatalf("pending = %v, want [2 3]", got)
	}

	// Move C to the front.
	if !m.moveToFront(3) || ids(m.pending)[0] != 3 {
		t.Fatalf("moveToFront failed: %v", ids(m.pending))
	}

	// A finishes: C starts next.
	m.finishJob(jobDoneMsg{rom: a})
	if m.running[3] == nil || len(m.finished) != 1 {
		t.Fatalf("after A: running=%v finished=%d", m.running, len(m.finished))
	}

	// C fails: B starts, C lands in failed.
	m.finishJob(jobDoneMsg{rom: c, err: errors.New("boom")})
	if m.running[2] == nil || len(m.failed) != 1 || m.failed[0].err != "boom" {
		t.Fatalf("after C failed: running=%v failed=%v", m.running, m.failed)
	}

	// B is cancelled: nothing failed, nothing finished added.
	m.finishJob(jobDoneMsg{rom: b, err: errors.New("context canceled"), canceled: true})
	if len(m.running) != 0 || len(m.failed) != 1 || len(m.finished) != 1 {
		t.Fatalf("after cancel: running=%v failed=%d finished=%d", m.running, len(m.failed), len(m.finished))
	}

	// Retry C: it leaves the failed list and runs again.
	m.retry(3)
	if m.running[3] == nil || len(m.failed) != 0 {
		t.Fatalf("retry: running=%v failed=%d", m.running, len(m.failed))
	}
}

func TestCancelQueued(t *testing.T) {
	m := testModel(100, 30)
	m.enqueue(rom(1, "A"))
	m.enqueue(rom(2, "B"))
	m.cancel(2)
	if len(m.pending) != 0 || m.running[1] == nil {
		t.Fatalf("cancel queued: pending=%v running=%v", ids(m.pending), m.running)
	}
}

func TestTabsAndQuitConfirm(t *testing.T) {
	m := testModel(100, 30)
	m.handleKey(keyRune(']'))
	if m.tab != tabDownloads {
		t.Fatal("] should switch to Downloads")
	}
	m.handleKey(keyRune(']'))
	if m.tab != tabLibrary {
		t.Fatal("] should wrap back to Library")
	}
	m.handleKey(keyRune('['))
	if m.tab != tabDownloads {
		t.Fatal("[ should go to the previous tab")
	}

	m.enqueue(rom(1, "A"))
	if _, cmd := m.handleKey(keyRune('q')); cmd == nil || !m.quitArmed {
		t.Fatal("first q with downloads should ask for confirmation")
	}
	if !strings.Contains(m.toast.text, "press q again") {
		t.Fatalf("toast = %q", m.toast.text)
	}
	m.handleKey(keyDown()) // any other key disarms
	if m.quitArmed {
		t.Fatal("other key should disarm quit")
	}
}

func TestDownloadBar(t *testing.T) {
	m := withRoms(testModel(120, 24))
	bar := m.viewDownloadBar()
	if len(bar) != 1 || !strings.Contains(bar[0], "The Legend of Zelda") || !strings.Contains(bar[0], "1 queued") {
		t.Fatalf("bar = %q", bar)
	}
	m.tab = tabDownloads
	if len(m.viewDownloadBar()) != 1 {
		t.Fatal("bar should be visible on every tab")
	}

	idle := testModel(120, 24)
	if len(idle.viewDownloadBar()) != 0 {
		t.Fatal("bar should be hidden when idle")
	}
	idle.failed = []failedJob{{rom: rom(9, "X"), err: "boom"}}
	if bar := idle.viewDownloadBar(); len(bar) != 1 || !strings.Contains(bar[0], "1 download failed") {
		t.Fatalf("idle failed bar = %q", bar)
	}
}
