package integration

import (
	"bytes"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// passthroughProgram reports its arguments, input and environment, then exits
// with the status in its first argument.
const passthroughProgram = `package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
)

func main() {
	input, _ := io.ReadAll(os.Stdin)
	civisibility, set := os.LookupEnv("DD_CIVISIBILITY_ENABLED")
	if !set {
		civisibility = "unset"
	}
	fmt.Printf("args=%q stdin=%q mark=%q civisibility=%s\n", os.Args[1:], input, os.Getenv("PASSTHROUGH_MARK"), civisibility)
	fmt.Fprintln(os.Stderr, "to stderr")
	status, _ := strconv.Atoi(os.Args[1])
	os.Exit(status)
}
`

// ddto runs every command but go test unchanged: with its arguments, standard
// streams, environment and exit status, and without the CI Visibility default
// that ddto test sets. A go command runs as go would.
func TestCLIPassesOtherCommandsThrough(t *testing.T) {
	dir, driver := prepareMiniFixture(t)
	source := t.TempDir()
	for name, data := range map[string]string{"go.mod": "module example.com/passthrough\n\ngo 1.25\n", "main.go": passthroughProgram} {
		if err := os.WriteFile(filepath.Join(source, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	program := filepath.Join(t.TempDir(), executableName("passthrough"))
	if out, stderr, code := command(t, source, testEnv(), "go", "build", "-o", program, "."); code != 0 {
		t.Fatalf("build: %s %s", out, stderr)
	}
	env := testEnv("PASSTHROUGH_MARK=kept")

	cmd := exec.Command(driver, program, "7", "a b")
	cmd.Dir, cmd.Env, cmd.Stdin = dir, env, strings.NewReader("input")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	var exit *exec.ExitError
	if err := cmd.Run(); !errors.As(err, &exit) || exit.ExitCode() != 7 {
		t.Fatalf("exit %v; want status 7", err)
	}
	if want := `args=["7" "a b"] stdin="input" mark="kept" civisibility=unset` + "\n"; stdout.String() != want {
		t.Fatalf("stdout %q; want %q", stdout.String(), want)
	}
	if got := strings.TrimSpace(stderr.String()); got != "to stderr" {
		t.Fatalf("stderr %q", got)
	}

	for _, args := range [][]string{{"version"}, {"env", "GOFLAGS", "GOWORK"}} {
		want, wantErr, wantCode := command(t, dir, env, "go", args...)
		got, gotErr, gotCode := command(t, dir, env, driver, append([]string{"go"}, args...)...)
		if got != want || gotErr != wantErr || gotCode != wantCode {
			t.Fatalf("ddto go %v: %q %q %d; go: %q %q %d", args, got, gotErr, gotCode, want, wantErr, wantCode)
		}
	}

	if _, stderr, code := command(t, dir, env, driver, "ddto-no-such-command"); code != 127 || !strings.Contains(stderr, "ddto-no-such-command") {
		t.Fatalf("a missing command: %d %q; want 127", code, stderr)
	}
	if _, stderr, code := command(t, dir, env, driver); code != 2 || !strings.Contains(stderr, "usage: ddto") {
		t.Fatalf("no command: %d %q; want the usage and 2", code, stderr)
	}
}

// ddto go test is ddto test: both report the same tests.
func TestCLIGoTestIsTest(t *testing.T) {
	dir, driver := prepareMiniFixture(t)
	for _, form := range [][]string{{"test"}, {"go", "test"}} {
		receiver := &capture{}
		server := newServer(http.HandlerFunc(receiver.handler))
		env := testEnv("DD_CIVISIBILITY_AGENTLESS_ENABLED=true", "DD_CIVISIBILITY_AGENTLESS_URL="+server.URL, "DD_TRACE_AGENT_URL="+server.URL, "DD_API_KEY=poc-not-a-real-key")
		out, stderr, code := command(t, dir, env, driver, append(form, "--runtime=mini", "-count=1", "-run=^TestPass$", ".")...)
		server.Close()
		if code != 0 || !strings.Contains(out, "ok") {
			t.Fatalf("ddto %v: %d\n%s\n%s", form, code, out, stderr)
		}
		receiver.mu.Lock()
		tests := 0
		for _, event := range receiver.events {
			if event["type"] == "test" {
				tests++
			}
		}
		failures := receiver.failures
		receiver.mu.Unlock()
		if len(failures) != 0 || tests != 1 {
			t.Fatalf("ddto %v reported %d tests, failures %v\n%s\n%s", form, tests, failures, out, stderr)
		}
	}
}
