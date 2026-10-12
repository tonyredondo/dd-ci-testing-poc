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

// A batch file receives ddto's command line after ddto's name as the caller
// wrote it: cmd.exe reads quoting otherwise than Go writes it, so a quoted &
// stays an argument instead of starting another command.
func TestCLIPassesWindowsCommandLineThrough(t *testing.T) {
	dir, driver := prepareMiniFixture(t)
	script := filepath.Join(t.TempDir(), "script.cmd")
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
		if err := cmd.Run(); !errors.As(err, &exit) {
			t.Fatalf("%s: %v %s", cmdline, err, stderr.String())
		}
		return stdout.String(), exit.ExitCode()
	}
	arguments := `"` + script + `" "one&echo.INJECTED"`
	want, wantCode := run(script, arguments)
	got, code := run(driver, `"`+driver+`"  `+arguments)
	if got != want || code != wantCode || strings.Contains(got, "\nINJECTED") || !strings.Contains(got, `"one&echo.INJECTED"`) {
		t.Fatalf("through ddto: %q %d; directly: %q %d", got, code, want, wantCode)
	}
}
