package runner

import (
	"errors"
	"io/fs"
	"os/exec"
)

// commandStatus is the exit status of a command that ddto could not run, as
// env reports it: 127 if it is not found, 126 if it cannot run.
func commandStatus(err error) int {
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist) {
		return 127
	}
	return 126
}
