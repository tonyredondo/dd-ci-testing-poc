//go:build !windows

package runner

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/goenv"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/version"
)

// ExecWithCallerEnvironment restores the caller's Go settings, then replaces
// this process with args: a go test -exec program sees what native go test
// gives it, rather than ddto's temporary workspace.
func ExecWithCallerEnvironment(args []string) int {
	goenv.Restore()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, version.BuildLogPrefix+" ERROR: missing -exec program")
		return 2
	}
	executable, err := exec.LookPath(args[0])
	if err == nil {
		err = syscall.Exec(executable, args, os.Environ())
	}
	fmt.Fprintln(os.Stderr, err)
	return 2
}

// ExecCommand replaces this process with args, unchanged: ddto passes every
// command but go test through. It finds and starts the command as execvp,
// which env and shells use, does: through a relative PATH entry, which only
// the caller sets; an executable file without a #! line through /bin/sh; and
// a file in PATH that cannot run is reported as such, not as missing.
func ExecCommand(args []string) int {
	executable, err := exec.LookPath(args[0])
	if errors.Is(err, exec.ErrDot) {
		err = nil
	}
	if err == nil {
		err = syscall.Exec(executable, args, os.Environ())
		if errors.Is(err, syscall.ENOEXEC) {
			err = syscall.Exec("/bin/sh", append([]string{"/bin/sh", executable}, args[1:]...), os.Environ())
		}
	} else if errors.Is(err, exec.ErrNotFound) && inPath(args[0]) {
		err = fmt.Errorf("%s: %w", args[0], fs.ErrPermission)
	}
	fmt.Fprintln(os.Stderr, version.BuildLogPrefix+" ERROR:", err)
	return commandStatus(err)
}

// inPath reports whether a PATH directory has an entry named file, which
// LookPath skips when it cannot run.
func inPath(file string) bool {
	if strings.Contains(file, "/") {
		return false
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			dir = "."
		}
		if _, err := os.Stat(filepath.Join(dir, file)); err == nil {
			return true
		}
	}
	return false
}

// ExecNativeTool replaces the wrapper: no second process, wait loop or plan I/O.
// It is only for the CLI entrypoint, never an in-process library caller.
func ExecNativeTool(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, version.BuildLogPrefix+" ERROR: missing tool executable")
		return 2
	}
	args, err := chainUserToolexec(args)
	if err == nil {
		var executable string
		if executable, err = exec.LookPath(args[0]); err == nil {
			err = syscall.Exec(executable, args, os.Environ())
		}
	}
	fmt.Fprintln(os.Stderr, err)
	return 2
}
