//go:build !windows

package runner

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
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
// which env and shells use, does.
func ExecCommand(args []string) int {
	err := execvp(args)
	fmt.Fprintln(os.Stderr, version.BuildLogPrefix+" ERROR:", &fs.PathError{Op: "exec", Path: args[0], Err: err})
	return commandStatus(err)
}

// defaultPath is glibc's search path for execvp without PATH.
const defaultPath = "/bin:/usr/bin"

// execvp starts args as glibc's execvp does, and returns only if they do not
// start. A name without / is tried in each PATH directory, or defaultPath's
// without PATH, where an empty entry is the current directory. The search
// goes past a missing file, and past one that cannot run, which it then
// reports (EACCES) if nothing else starts.
func execvp(args []string) error {
	file := args[0]
	if file == "" {
		return syscall.ENOENT
	}
	if strings.Contains(file, "/") {
		return execve(file, args)
	}
	path, ok := os.LookupEnv("PATH")
	if !ok {
		path = defaultPath
	}
	var err error
	denied := false
	for _, dir := range strings.Split(path, ":") {
		candidate := file
		if dir != "" {
			candidate = dir + "/" + file
		}
		switch err = execve(candidate, args); err {
		case syscall.EACCES:
			denied = true
		case syscall.ENOENT, syscall.ESTALE, syscall.ENOTDIR, syscall.ENODEV, syscall.ETIMEDOUT:
		default:
			return err
		}
	}
	if denied {
		return syscall.EACCES
	}
	return err
}

// execve starts path with args, and an executable file without a #! line
// with /bin/sh, as execvp does.
func execve(path string, args []string) error {
	err := syscall.Exec(path, args, os.Environ())
	if err == syscall.ENOEXEC {
		err = syscall.Exec("/bin/sh", append([]string{"/bin/sh", path}, args[1:]...), os.Environ())
	}
	return err
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
