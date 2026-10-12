//go:build linux

package gamepad

import (
	"os"
	"testing"
	"time"
)

func TestParseDevicesFindsGamepads(t *testing.T) {
	f, err := os.Open("testdata/odin2-devices.txt") // /proc/bus/input/devices of an AYN Odin 2 on Armada
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got := map[string]bool{}
	for _, d := range parseDevices(f) {
		got[d.Name] = true
	}
	for _, want := range []string{"AYN Odin2 Gamepad", "Microsoft X-Box 360 pad 0"} {
		if !got[want] {
			t.Errorf("missing %q in %v", want, got)
		}
	}
	for _, not := range []string{"InputPlumber Keyboard", "gpio-keys", "generic ft5x06 (8d)"} {
		if got[not] {
			t.Errorf("%q is not a gamepad", not)
		}
	}
}

func TestHasBit(t *testing.T) {
	// BTN_SOUTH (0x130 = 304) lives in the 5th 64-bit word from the right.
	if !hasBit("7cdb000000000000 0 0 0 0", btnSouth) {
		t.Error("BTN_SOUTH not found")
	}
	if hasBit("3 0 0 0 0", btnSouth) || hasBit("", btnSouth) {
		t.Error("false positive")
	}
}

func TestRepeaterPressRepeatRelease(t *testing.T) {
	var got []Button
	ch := make(chan struct{}, 64)
	r := &repeater{emit: func(b Button) { got = append(got, b); ch <- struct{}{} }}
	r.key(A, 1)
	r.key(A, 0)
	r.axis(Up, Down, 1) // D-pad down via hat axis, held
	time.Sleep(repeatDelay + 2*repeatEvery + 30*time.Millisecond)
	r.axis(Up, Down, 0)
	n := len(got)
	time.Sleep(2 * repeatEvery)
	r.stop()
	if got[0] != A || got[1] != Down {
		t.Fatalf("first presses = %v", got[:2])
	}
	if n < 4 {
		t.Fatalf("expected repeats while held, got %v", got)
	}
	if len(got) != n {
		t.Fatalf("kept repeating after release: %v", got)
	}
}
