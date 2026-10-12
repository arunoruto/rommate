//go:build windows

package romm

import (
	"syscall"

	"golang.org/x/sys/windows"
)

// reusePort lets rommate share the mDNS port with the system's responder.
func reusePort(_, _ string, c syscall.RawConn) error {
	var serr error
	err := c.Control(func(fd uintptr) {
		serr = windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, windows.SO_REUSEADDR, 1)
	})
	if err != nil {
		return err
	}
	return serr
}
