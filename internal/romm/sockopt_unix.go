//go:build unix

package romm

import (
	"syscall"

	"golang.org/x/sys/unix"
)

// reusePort lets rommate share the mDNS port with the system's responder
// (Avahi, mDNSResponder).
func reusePort(_, _ string, c syscall.RawConn) error {
	var serr error
	err := c.Control(func(fd uintptr) {
		if serr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1); serr != nil {
			return
		}
		serr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
	})
	if err != nil {
		return err
	}
	return serr
}
