package runner

import "syscall"

// glibc's execvp (posix/execvpe.c) searches defaultPath, confstr(_CS_PATH),
// without PATH.
const defaultPath = "/bin:/usr/bin"

// skipEntry reports whether glibc skips a PATH entry: one too long for its
// buffer, of Linux's PATH_MAX.
func skipEntry(dir, _ string) bool {
	return len(dir) >= 4096
}

// searchNext reports whether glibc goes on to the next PATH entry after err,
// and whether err reports a candidate that could not run.
func searchNext(_ string, err error) (next, denied bool) {
	switch err {
	case syscall.EACCES:
		return true, true
	case syscall.ENOENT, syscall.ESTALE, syscall.ENOTDIR, syscall.ENODEV, syscall.ETIMEDOUT:
		return true, false
	}
	return false, false
}

// notFound is glibc's error when no candidate started: the last one's.
func notFound(last error) error {
	return last
}
