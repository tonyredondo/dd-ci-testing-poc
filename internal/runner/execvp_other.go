//go:build !linux && !windows

package runner

import (
	"fmt"
	"os"
	"syscall"
)

// The BSD libcs' execvp (gen/exec.c in FreeBSD and Apple's Libc) searches
// defaultPath, _PATH_DEFPATH, without PATH.
const defaultPath = "/usr/bin:/bin"

// skipEntry reports whether the libc skips a PATH entry, with the message it
// prints: one whose candidate is too long for its buffer, of PATH_MAX.
func skipEntry(dir, file string) bool {
	if len(dir)+len(file)+2 <= 1024 {
		return false
	}
	fmt.Fprintf(os.Stderr, "execvp: %s: path too long\n", dir)
	return true
}

// searchNext reports whether the libc goes on to the next PATH entry after
// err, and whether err reports a candidate that could not run. It takes
// EACCES for the candidate's only if the candidate exists, not for a
// directory that cannot be searched.
func searchNext(candidate string, err error) (next, denied bool) {
	switch err {
	case syscall.E2BIG, syscall.ENOMEM, syscall.ETXTBSY:
		return false, false
	case syscall.ELOOP, syscall.ENAMETOOLONG, syscall.ENOENT, syscall.ENOTDIR:
		return true, false
	}
	if _, statErr := os.Stat(candidate); statErr != nil {
		return true, false
	}
	return err == syscall.EACCES, err == syscall.EACCES
}

// notFound is the libc's error when no candidate started.
func notFound(error) error {
	return syscall.ENOENT
}
