//go:build linux

package gamepad

import (
	"bufio"
	"context"
	"encoding/binary"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Linux input event types and codes (linux/input-event-codes.h).
const (
	evKey = 0x01
	evAbs = 0x03

	btnSouth  = 0x130 // A
	btnEast   = 0x131 // B
	btnNorth  = 0x133 // reported as BTN_X by xpad-style pads (left face button)
	btnWest   = 0x134 // reported as BTN_Y by xpad-style pads (top face button)
	btnTL     = 0x136
	btnTR     = 0x137
	btnSelect = 0x13a
	btnStart  = 0x13b
	btnDUp    = 0x220
	btnDDown  = 0x221
	btnDLeft  = 0x222
	btnDRight = 0x223

	absHat0X = 0x10
	absHat0Y = 0x11
)

var keyButtons = map[uint16]Button{
	btnSouth: A, btnEast: B, btnNorth: X, btnWest: Y,
	btnTL: L1, btnTR: R1, btnSelect: Select, btnStart: Start,
	btnDUp: Up, btnDDown: Down, btnDLeft: Left, btnDRight: Right,
}

// eventSize is sizeof(struct input_event) on 64-bit Linux: a 16-byte timeval,
// then type (u16), code (u16) and value (s32).
const eventSize = 24

// Find lists the game controllers this user can read: devices in
// /proc/bus/input/devices with an event handler whose key capabilities include
// BTN_SOUTH (the A button).
func Find() []Device {
	f, err := os.Open("/proc/bus/input/devices")
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []Device
	for _, d := range parseDevices(f) {
		if fh, err := os.Open(d.Path); err == nil {
			fh.Close()
			out = append(out, d)
		}
	}
	return out
}

// parseDevices returns every device with gamepad buttons, readable or not.
func parseDevices(r io.Reader) []Device {
	var out []Device
	var name, handler, keys string
	flush := func() {
		if handler != "" && hasBit(keys, btnSouth) {
			out = append(out, Device{Name: name, Path: filepath.Join("/dev/input", handler)})
		}
		name, handler, keys = "", "", ""
	}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "":
			flush()
		case strings.HasPrefix(line, "N: Name="):
			name = strings.Trim(strings.TrimPrefix(line, "N: Name="), `"`)
		case strings.HasPrefix(line, "H: Handlers="):
			for _, h := range strings.Fields(strings.TrimPrefix(line, "H: Handlers=")) {
				if strings.HasPrefix(h, "event") {
					handler = h
				}
			}
		case strings.HasPrefix(line, "B: KEY="):
			keys = strings.TrimPrefix(line, "B: KEY=")
		}
	}
	flush()
	return out
}

// hasBit reports whether bit is set in a /proc capability bitmap: space
// separated hex words, most significant first.
func hasBit(bitmap string, bit uint) bool {
	words := strings.Fields(bitmap)
	if len(words) == 0 {
		return false
	}
	n := new(big.Int)
	for _, w := range words {
		v, ok := new(big.Int).SetString(w, 16)
		if !ok {
			return false
		}
		n.Lsh(n, 64)
		n.Or(n, v)
	}
	// /proc prints longs: 64-bit words on 64-bit kernels.
	return n.Bit(int(bit)) == 1
}

// Repeat timing for held D-pad directions.
const (
	repeatDelay = 400 * time.Millisecond
	repeatEvery = 110 * time.Millisecond
	// dedupeWindow drops a press that arrives again from another device.
	dedupeWindow = 50 * time.Millisecond
)

// Listen reads all devices and calls fn for every button press (plus repeats
// while a direction is held) until ctx ends. It returns immediately.
func Listen(ctx context.Context, devices []Device, fn func(Button)) {
	var mu sync.Mutex // fn is called from several goroutines
	last := map[Button]time.Time{}
	emit := func(b Button) {
		mu.Lock()
		defer mu.Unlock()
		// The same physical press can arrive from two devices (the hardware
		// pad and a virtual pad such as InputPlumber's); keep one.
		now := time.Now()
		if now.Sub(last[b]) < dedupeWindow {
			return
		}
		last[b] = now
		fn(b)
	}
	for _, d := range devices {
		go listen(ctx, d.Path, emit)
	}
}

func listen(ctx context.Context, path string, emit func(Button)) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	go func() {
		<-ctx.Done()
		f.Close() // unblocks Read
	}()

	r := &repeater{emit: emit}
	defer r.stop()
	buf := make([]byte, eventSize*16)
	for {
		n, err := f.Read(buf)
		if err != nil {
			return
		}
		for off := 0; off+eventSize <= n; off += eventSize {
			typ := binary.LittleEndian.Uint16(buf[off+16:])
			code := binary.LittleEndian.Uint16(buf[off+18:])
			value := int32(binary.LittleEndian.Uint32(buf[off+20:]))
			switch typ {
			case evKey:
				if b, ok := keyButtons[code]; ok {
					r.key(b, value)
				}
			case evAbs:
				switch code {
				case absHat0X:
					r.axis(Left, Right, value)
				case absHat0Y:
					r.axis(Up, Down, value)
				}
			}
		}
	}
}

// repeater emits presses and repeats held directions.
type repeater struct {
	emit  func(Button)
	mu    sync.Mutex
	held  Button
	timer *time.Timer
	on    bool
}

func isDirection(b Button) bool { return b <= Right }

func (r *repeater) key(b Button, value int32) {
	switch value {
	case 1: // press
		r.emit(b)
		if isDirection(b) {
			r.start(b)
		}
	case 0: // release
		if isDirection(b) {
			r.release(b)
		}
	}
}

func (r *repeater) axis(neg, pos Button, value int32) {
	switch {
	case value < 0:
		r.key(neg, 1)
	case value > 0:
		r.key(pos, 1)
	default:
		r.release(neg)
		r.release(pos)
	}
}

func (r *repeater) start(b Button) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.timer != nil {
		r.timer.Stop()
	}
	r.held, r.on = b, true
	var tick func()
	tick = func() {
		r.mu.Lock()
		if !r.on || r.held != b {
			r.mu.Unlock()
			return
		}
		r.timer = time.AfterFunc(repeatEvery, tick)
		r.mu.Unlock()
		r.emit(b)
	}
	r.timer = time.AfterFunc(repeatDelay, tick)
}

func (r *repeater) release(b Button) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.on && r.held == b {
		r.on = false
		if r.timer != nil {
			r.timer.Stop()
		}
	}
}

func (r *repeater) stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.on = false
	if r.timer != nil {
		r.timer.Stop()
	}
}
