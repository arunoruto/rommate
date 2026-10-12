// Package gamepad reads game controllers directly, so the TUI can be driven
// with a controller even when nothing maps its buttons to keys (for example a
// non-Steam shortcut with Steam's default gamepad layout, or a plain terminal).
//
// It is implemented for Linux (evdev); on other systems Find returns nothing.
package gamepad

// Button is a controller input, already translated to rommate's actions.
type Button int

const (
	Up Button = iota
	Down
	Left
	Right
	A      // confirm
	B      // back
	X      // remove / cancel
	Y      // download all / download next
	L1     // previous tab
	R1     // next tab
	Select // help
	Start  // quit
)

func (b Button) String() string {
	return [...]string{"Up", "Down", "Left", "Right", "A", "B", "X", "Y", "L1", "R1", "Select", "Start"}[b]
}

// Device is a controller rommate can read.
type Device struct {
	Name string
	Path string
}
