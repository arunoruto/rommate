package tui

import "os"

// inputMode decides whether the footer shows keyboard keys or controller
// buttons.
//
// With Steam Input, controller presses reach the terminal as ordinary key
// presses, so the TUI cannot tell the two apart from the keys alone. Instead
// it looks at how it was launched: Steam (Game Mode or a non-Steam shortcut)
// means controller, a plain terminal means keyboard. On Linux the TUI also
// reads controllers directly (package gamepad); the first button press
// switches to controller mode.
type inputMode int

const (
	inputKeyboard inputMode = iota
	inputController
)

// inputModeMsg switches the input mode at runtime.
type inputModeMsg inputMode

// Options configures the TUI.
type Options struct {
	// Hints is "auto" (or empty), "keyboard" or "controller".
	Hints string
	// Gamepad is "auto" (or empty) to read controllers directly, or "off".
	Gamepad string
}

// steamEnv lists variables Steam or gamescope set for launched programs.
var steamEnv = []string{
	"SteamGameId",               // any game launched by Steam, incl. non-Steam shortcuts
	"SteamAppId",                // Steam games
	"SteamClientLaunch",         // launched from the Steam client
	"GAMESCOPE_WAYLAND_DISPLAY", // running inside gamescope (Game Mode)
}

// detectInputMode resolves the hints setting. getenv is os.Getenv in
// production and a stub in tests.
func detectInputMode(setting string, getenv func(string) string) inputMode {
	switch setting {
	case "keyboard":
		return inputKeyboard
	case "controller":
		return inputController
	}
	if getenv("XDG_CURRENT_DESKTOP") == "gamescope" {
		return inputController
	}
	for _, k := range steamEnv {
		if getenv(k) != "" {
			return inputController
		}
	}
	return inputKeyboard
}

func defaultInputMode(o Options) inputMode { return detectInputMode(o.Hints, os.Getenv) }
