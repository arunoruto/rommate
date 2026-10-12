package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/arunoruto/rommate/internal/gamepad"
)

func env(kv map[string]string) func(string) string {
	return func(k string) string { return kv[k] }
}

func TestDetectInputMode(t *testing.T) {
	cases := []struct {
		name    string
		setting string
		env     map[string]string
		want    inputMode
	}{
		{"plain terminal", "", nil, inputKeyboard},
		{"auto plain terminal", "auto", nil, inputKeyboard},
		{"non-Steam shortcut", "", map[string]string{"SteamGameId": "12345"}, inputController},
		{"Game Mode", "auto", map[string]string{"XDG_CURRENT_DESKTOP": "gamescope"}, inputController},
		{"inside gamescope", "", map[string]string{"GAMESCOPE_WAYLAND_DISPLAY": "gamescope-0"}, inputController},
		{"KDE desktop", "", map[string]string{"XDG_CURRENT_DESKTOP": "KDE"}, inputKeyboard},
		{"forced keyboard under Steam", "keyboard", map[string]string{"SteamGameId": "1"}, inputKeyboard},
		{"forced controller", "controller", nil, inputController},
	}
	for _, c := range cases {
		if got := detectInputMode(c.setting, env(c.env)); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestFooterFollowsInputMode(t *testing.T) {
	m := withRoms(testModel(140, 24))
	m.toast = toast{}
	footer := func() string {
		lines := strings.Split(m.View(), "\n")
		return lines[len(lines)-1]
	}

	m.input = inputKeyboard
	if f := footer(); !strings.Contains(f, "Enter") || !strings.Contains(f, "Esc") || strings.Contains(f, "Start") {
		t.Errorf("keyboard footer: %q", f)
	}

	m.Update(inputModeMsg(inputController))
	if f := footer(); !strings.Contains(f, " A ") || !strings.Contains(f, "Select") || strings.Contains(f, "Enter") {
		t.Errorf("controller footer: %q", f)
	}
}

func TestControllerPressMapsToKeysAndDropsSteamEcho(t *testing.T) {
	m := testModel(100, 30)
	m.input = inputKeyboard
	m.handlePad(gamepad.R1) // next tab
	if m.tab != tabDownloads || m.input != inputController {
		t.Fatalf("R1: tab=%v input=%v", m.tab, m.input)
	}
	// The same press arriving again as "]" from a Steam keyboard layout is ignored.
	m.Update(keyRune(']'))
	if m.tab != tabDownloads {
		t.Fatal("Steam echo of R1 switched tabs again")
	}
	// A deliberate key press later still works.
	m.padSeen["]"] = time.Now().Add(-time.Second)
	m.Update(keyRune(']'))
	if m.tab != tabSaves {
		t.Fatal("keyboard ] after the echo window should switch tabs")
	}
}
