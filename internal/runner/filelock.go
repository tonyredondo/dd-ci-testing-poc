package runner

import "errors"

// errLockBusy reports that a nonblocking lock request would have to wait.
var errLockBusy = errors.New("file lock is held by another process")
