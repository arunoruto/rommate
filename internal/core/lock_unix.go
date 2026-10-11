//go:build unix

package core

import (
	"os"
	"syscall"
)

// lockFile takes a non-blocking exclusive lock on f. The lock is released
// when f is closed.
func lockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}
