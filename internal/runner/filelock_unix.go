//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package runner

import (
	"errors"
	"os"
	"syscall"
)

// lockFile takes an advisory lock on the whole file. Locks belong to the open
// file, so separate opens conflict even within one process; closing releases.
func lockFile(file *os.File, exclusive, wait bool) error {
	how := syscall.LOCK_SH
	if exclusive {
		how = syscall.LOCK_EX
	}
	if !wait {
		how |= syscall.LOCK_NB
	}
	for {
		err := syscall.Flock(int(file.Fd()), how)
		switch {
		case errors.Is(err, syscall.EINTR):
			continue
		case errors.Is(err, syscall.EWOULDBLOCK):
			return errLockBusy
		}
		return err
	}
}
