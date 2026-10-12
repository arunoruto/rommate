package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/arunoruto/rommate/internal/gamepad"
)

// padMsg is a button press read directly from a controller.
type padMsg gamepad.Button

// padEcho is how long a key that matches a recent controller press is treated
// as the same press arriving again through a Steam Input keyboard layout.
const padEcho = 150 * time.Millisecond

// padKeys maps controller buttons to the keys the TUI already understands.
var padKeys = map[gamepad.Button]tea.KeyMsg{
	gamepad.Up:     {Type: tea.KeyUp},
	gamepad.Down:   {Type: tea.KeyDown},
	gamepad.Left:   {Type: tea.KeyLeft},
	gamepad.Right:  {Type: tea.KeyRight},
	gamepad.A:      {Type: tea.KeyEnter},
	gamepad.B:      {Type: tea.KeyEsc},
	gamepad.X:      {Type: tea.KeyRunes, Runes: []rune{'x'}},
	gamepad.Y:      {Type: tea.KeyRunes, Runes: []rune{'y'}},
	gamepad.L1:     {Type: tea.KeyRunes, Runes: []rune{'['}},
	gamepad.R1:     {Type: tea.KeyRunes, Runes: []rune{']'}},
	gamepad.Select: {Type: tea.KeyF1},
	gamepad.Start:  {Type: tea.KeyRunes, Runes: []rune{'q'}},
}

// startGamepad forwards controller presses to the program, unless disabled.
func startGamepad(ctx context.Context, opts Options, p *tea.Program) {
	if opts.Gamepad == "off" {
		return
	}
	if devices := gamepad.Find(); len(devices) > 0 {
		gamepad.Listen(ctx, devices, func(b gamepad.Button) { p.Send(padMsg(b)) })
	}
}

// handlePad turns a controller press into a key press.
func (m *model) handlePad(b gamepad.Button) (tea.Model, tea.Cmd) {
	k, ok := padKeys[b]
	if !ok {
		return m, nil
	}
	m.input = inputController
	if m.padSeen == nil {
		m.padSeen = map[string]time.Time{}
	}
	m.padSeen[k.String()] = time.Now()
	return m.handleKey(k)
}

// echoOfPad reports whether a keyboard key is a controller press we already
// handled, delivered again by Steam Input.
func (m *model) echoOfPad(k tea.KeyMsg) bool {
	t, ok := m.padSeen[k.String()]
	return ok && time.Since(t) < padEcho
}
