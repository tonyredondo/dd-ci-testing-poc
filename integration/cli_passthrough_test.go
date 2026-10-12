package integration

import (
	"bytes"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
// that ddto test sets or any output of its own. A go command runs as go would,
// and a command that does not run exits as from env.
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
	// An invalid DD_LOGGING_RATE made ddto's initialization warn.
	env := testEnv("PASSTHROUGH_MARK=kept", "DD_LOGGING_RATE=bad")

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

	for _, args := range [][]string{{"version"}, {"env", "GOFLAGS", "GOWORK"}, {"tool", "orchestrion", "go"}} {
		want, wantErr, wantCode := command(t, dir, env, "go", args...)
		got, gotErr, gotCode := command(t, dir, env, driver, append([]string{"go"}, args...)...)
		if got != want || gotErr != wantErr || gotCode != wantCode {
			t.Fatalf("ddto go %v: %q %q %d; go: %q %q %d", args, got, gotErr, gotCode, want, wantErr, wantCode)
		}
	}

	// Orchestrion's commands other than go test are Orchestrion's.
	bin := t.TempDir()
	copyExecutable(t, program, filepath.Join(bin, executableName("orchestrion")))
	pathEnv := append(env[:len(env):len(env)], "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if out, _, code := command(t, dir, pathEnv, driver, "orchestrion", "go", "build", "./..."); code != 0 || !strings.HasPrefix(out, `args=["go" "build" "./..."]`) {
		t.Fatalf("ddto orchestrion go build: %d %q", code, out)
	}

	for _, missing := range []string{"ddto-no-such-command", filepath.Join(t.TempDir(), "ddto-no-such-command")} {
		if _, stderr, code := command(t, dir, env, driver, missing); code != 127 || !strings.Contains(stderr, "ddto-no-such-command") {
			t.Fatalf("a missing command %s: %d %q; want 127", missing, code, stderr)
		}
	}
	if runtime.GOOS == "windows" {
		// Batch files run through cmd.exe, as Windows starts them.
		script := filepath.Join(t.TempDir(), "script.cmd")
		if err := os.WriteFile(script, []byte("@echo batch %*\r\n@exit /b 5\r\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if out, stderr, code := command(t, dir, env, driver, script, "one"); code != 5 || strings.TrimSpace(out) != "batch one" {
			t.Fatalf("a batch file: %d %q %q; want its output and 5", code, out, stderr)
		}
	} else {
		notExecutable := filepath.Join(t.TempDir(), "notes")
		if err := os.WriteFile(notExecutable, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, stderr, code := command(t, dir, env, driver, notExecutable); code != 126 {
			t.Fatalf("a file that cannot run: %d %q; want 126", code, stderr)
		}
		// Also when PATH names it.
		notExecutableEnv := append(env[:len(env):len(env)], "PATH="+filepath.Dir(notExecutable))
		if _, stderr, code := command(t, dir, notExecutableEnv, driver, "notes"); code != 126 {
			t.Fatalf("a file in PATH that cannot run: %d %q; want 126", code, stderr)
		}
		// An executable file without a #! line runs with /bin/sh.
		script := filepath.Join(t.TempDir(), "script")
		if err := os.WriteFile(script, []byte("echo \"script $1\"\nexit 7\n"), 0o700); err != nil {
			t.Fatal(err)
		}
		if out, stderr, code := command(t, dir, env, driver, script, "one"); code != 7 || out != "script one\n" {
			t.Fatalf("a script without #!: %d %q %q; want its output and 7", code, out, stderr)
		}
		// A relative PATH entry, which only the caller sets, finds commands
		// as from a shell, and so does an empty PATH, as the current
		// directory.
		relative := t.TempDir()
		copyExecutable(t, program, filepath.Join(relative, "bin", "passthrough-relative"))
		copyExecutable(t, program, filepath.Join(relative, "passthrough-here"))
		for name, path := range map[string]string{"passthrough-relative": "PATH=bin", "passthrough-here": "PATH="} {
			if out, stderr, code := command(t, relative, append(env[:len(env):len(env)], path), driver, name, "3"); code != 3 || !strings.HasPrefix(out, `args=["3"]`) {
				t.Fatalf("%s with %s: %d %q %q; want 3", name, path, code, out, stderr)
			}
		}
		// Without PATH, execvp searches its default path.
		var noPath []string
		for _, v := range env {
			if !strings.HasPrefix(v, "PATH=") {
				noPath = append(noPath, v)
			}
		}
		if _, stderr, code := command(t, dir, noPath, driver, "true"); code != 0 {
			t.Fatalf("true without PATH: %d %q; want 0", code, stderr)
		}
		if _, stderr, code := command(t, dir, env, driver, ""); code != 127 {
			t.Fatalf("an empty command name: %d %q; want 127", code, stderr)
		}
		// The search skips a PATH entry longer than PATH_MAX, as glibc does,
		// and finds nothing in such entries alone.
		long := "/" + strings.Repeat("a", 4096)
		for path, want := range map[string]int{long + string(os.PathListSeparator) + filepath.Join(relative, "bin"): 3, long: 127} {
			if _, stderr, code := command(t, dir, append(env[:len(env):len(env)], "PATH="+path), driver, "passthrough-relative", "3"); code != want {
				t.Fatalf("with a long PATH entry: %d %q; want %d", code, stderr, want)
			}
		}
		// The search goes past a PATH directory that cannot be searched, and
		// reports it if the command is nowhere else.
		if os.Geteuid() != 0 {
			closed := t.TempDir()
			if err := os.Chmod(closed, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(closed, 0o700) })
			for later, want := range map[string]int{t.TempDir(): 126, filepath.Join(relative, "bin"): 3} {
				path := "PATH=" + closed + string(os.PathListSeparator) + later
				if _, stderr, code := command(t, dir, append(env[:len(env):len(env)], path), driver, "passthrough-relative", "3"); code != want {
					t.Fatalf("with %s: %d %q; want %d", path, code, stderr, want)
				}
			}
		}
	}
	if _, stderr, code := command(t, dir, env, driver); code != 2 || !strings.Contains(stderr, "usage: ddto") {
		t.Fatalf("no command: %d %q; want the usage and 2", code, stderr)
	}
}

// ddto go test is ddto test: both report the same tests, also with go's -C
// before test.
func TestCLIGoTestIsTest(t *testing.T) {
	dir, driver := prepareMiniFixture(t)
	elsewhere := t.TempDir()
	for _, form := range [][]string{{"test"}, {"go", "test"}, {"go", "-C", dir, "test"}} {
		receiver := &capture{}
		server := newServer(http.HandlerFunc(receiver.handler))
		env := testEnv("DD_CIVISIBILITY_AGENTLESS_ENABLED=true", "DD_CIVISIBILITY_AGENTLESS_URL="+server.URL, "DD_TRACE_AGENT_URL="+server.URL, "DD_API_KEY=poc-not-a-real-key")
		cwd := dir
		if len(form) > 2 {
			cwd = elsewhere
		}
		out, stderr, code := command(t, cwd, env, driver, append(form, "--runtime=mini", "-count=1", "-run=^TestPass$", ".")...)
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

// copyExecutable copies the executable at source to target, creating its directory.
func copyExecutable(t *testing.T, source, target string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(source)
	if err == nil {
		err = os.WriteFile(target, data, 0o700)
	}
	if err != nil {
		t.Fatal(err)
	}
}
