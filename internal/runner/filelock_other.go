//go:build !(darwin || dragonfly || freebsd || linux || netbsd || openbsd || windows)

package runner

import (
	"errors"
	"os"
)

// Without file locks, vendor workspaces stay in each run's own directory.
func lockFile(*os.File, bool, bool) error { return errors.ErrUnsupported }
