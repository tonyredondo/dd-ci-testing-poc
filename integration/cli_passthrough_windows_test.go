//go:build windows

package integration

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// A command receives ddto's command line after ddto's name as the caller
// wrote it, so a batch file starts as if the caller had started it with that
// command line. Go's own quoting left a quoted & bare, where cmd.exe, which
// runs batch files, starts another command at it.
func TestCLIPassesWindowsCommandLineThrough(t *testing.T) {
	dir, driver := prepareMiniFixture(t)
	script := filepath.Join(t.TempDir(), "script.cmd")
	if strings.ContainsAny(script, " \t\"") {
		t.Skipf("the batch file's path %s needs quoting", script)
	}
	if err := os.WriteFile(script, []byte("@echo batch %*\r\n@exit /b 5\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run := func(name, cmdline string) (string, int) {
		t.Helper()
		cmd := exec.Command(name)
		cmd.Dir, cmd.Env = dir, testEnv()
		cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: cmdline}
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		var exit *exec.ExitError
		if err := cmd.Run(); errors.As(err, &exit) {
			return stdout.String(), exit.ExitCode()
		} else if err != nil {
			t.Fatalf("%s: %v %s", cmdline, err, stderr.String())
		}
		return stdout.String(), 0
	}
	// cmd.exe keeps a quoted & in the argument. With the batch file's path
	// quoted as well, cmd /c removes the first and last quotes, directly as
	// through ddto.
	for _, arguments := range []string{script + ` "one&echo.INJECTED"`, `"` + script + `" "one&echo.INJECTED"`} {
		want, wantCode := run(script, arguments)
		got, code := run(driver, `"`+driver+`"  `+arguments)
		if got != want || code != wantCode {
			t.Fatalf("%s through ddto: %q %d; directly: %q %d", arguments, got, code, want, wantCode)
		}
		if !strings.HasPrefix(arguments, `"`) && (code != 5 || strings.TrimSpace(got) != `batch "one&echo.INJECTED"`) {
			t.Fatalf("%s: %q %d; want the quoted argument and 5", arguments, got, code)
		}
	}
}
