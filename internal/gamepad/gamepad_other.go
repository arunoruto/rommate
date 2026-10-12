//go:build !linux

package gamepad

import "context"

// Find returns no devices: direct controller input is Linux-only.
func Find() []Device { return nil }

// Listen does nothing on this system.
func Listen(context.Context, []Device, func(Button)) {}
