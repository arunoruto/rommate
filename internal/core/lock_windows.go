//go:build windows

package core

import (
	"os"

	"golang.org/x/sys/windows"
)

// lockFile takes a non-blocking exclusive lock on f. The lock is released
// when f is closed.
func lockFile(f *os.File) error {
	ol := new(windows.Overlapped)
	return windows.LockFileEx(windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, ol)
}
