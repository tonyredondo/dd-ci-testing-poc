package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"go/version"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const minimumGoVersion = "go1.25.0"

// Go assumes these versions, including their GODEBUG defaults, when go.mod or
// go.work has no go directive.
const (
	defaultGoModVersion  = "1.16"
	defaultGoWorkVersion = "1.18"
)

// crossExecHelper returns the go_$GOOS_$GOARCH_exec program that go test runs
// test binaries with when it cross-compiles and no -exec is given. go test
// looks it up from dir, its working directory.
func crossExecHelper(environment *goEnvironment, dir string) string {
	if environment.GOOS == environment.GOHOSTOS && environment.GOARCH == environment.GOHOSTARCH {
		return ""
	}
	path, err := lookPathFrom(dir, "go_"+environment.GOOS+"_"+environment.GOARCH+"_exec")
	if err != nil {
		return ""
	}
	return path
}

// lookPathFrom finds file on PATH as exec.LookPath would with dir as the
// current directory, where go test looks up -exec programs and the
// cross-compilation helper after -C. A match relative to dir returns
// exec.ErrDot: Go refuses to run it.
func lookPathFrom(dir, file string) (string, error) {
	windows := runtime.GOOS == "windows"
	var dot string
	if _, skip := os.LookupEnv("NoDefaultCurrentDirectoryInExePath"); windows && !skip {
		// Windows searches the current directory first.
		dot, _ = exec.LookPath(filepath.Join(dir, file))
	}
	for _, entry := range filepath.SplitList(os.Getenv("PATH")) {
		if entry == "" && windows {
			continue
		}
		// An empty entry is the current directory on Unix.
		relative := !filepath.IsAbs(entry)
		if relative {
			entry = filepath.Join(dir, entry)
		}
		path, err := exec.LookPath(filepath.Join(entry, file))
		if err != nil {
			continue
		}
		// Go prefers PATH's spelling of the file that it found in the current
		// directory, without an error.
		if dot == "" && !relative || dot != "" && sameLink(dot, path) {
			return path, nil
		}
		return "", &exec.Error{Name: file, Err: exec.ErrDot}
	}
	if dot != "" {
		return "", &exec.Error{Name: file, Err: exec.ErrDot}
	}
	return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
}

// sameLink reports whether a and b name the same file, without following a
// final symbolic link, as exec.LookPath compares them.
func sameLink(a, b string) bool {
	ai, err := os.Lstat(a)
	if err != nil {
		return false
	}
	bi, err := os.Lstat(b)
	return err == nil && os.SameFile(ai, bi)
}

// goDebugDefault returns the GODEBUG default=go1.N value for a Go version.
func goDebugDefault(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return ""
	}
	return "go" + strings.Join(parts[:2], ".")
}

type goEnvironment struct {
	GOMOD, GOWORK, GOMODCACHE, GOVERSION string
	GOROOT                               string
	GOOS, GOARCH, GOHOSTOS, GOHOSTARCH   string
	// GOEXPERIMENT is the setting as go env reports it, not canonical.
	GOEXPERIMENT string
}

func readGoEnvironment(ctx context.Context, dir string) (*goEnvironment, error) {
	out, err := goTool(ctx, dir, nil, "env", "-json", "GOMOD", "GOWORK", "GOMODCACHE", "GOVERSION", "GOROOT", "GOOS", "GOARCH", "GOHOSTOS", "GOHOSTARCH", "GOEXPERIMENT")
	if err != nil {
		return nil, err
	}
	var environment goEnvironment
	if err := json.Unmarshal([]byte(out), &environment); err != nil {
		return nil, fmt.Errorf("read Go environment: %w", err)
	}
	return &environment, nil
}

// testRuntimeVersion returns what runtime.Version() reports in the test
// binaries that the selected toolchain links in dir, for the requests that the
// Git worker makes as they would. goVersion, from go env, is the go command's
// own version, with the toolchain's default experiments. When goExperiment
// sets others, the linker appends their canonical list, which go tool compile
// -V prints, to the toolchain's version. A wrong result only costs the test
// processes their own requests.
func testRuntimeVersion(ctx context.Context, dir, goVersion, goExperiment string) string {
	debug := debugFromContext(ctx)
	if _, own, _ := cutExperiments(goVersion); goExperiment == own {
		debug.printf("test runtime version=%q source=go_env", goVersion)
		return goVersion
	}
	phase := debug.start("test runtime version")
	cmd := goCommand(ctx, dir, "tool", "compile", "-V")
	cmd.Env = append(cmd.Environ(), "GOFLAGS=", "GOEXPERIMENT="+goExperiment)
	out, err := cmd.Output()
	phase.finish(err)
	if err != nil {
		debug.printf("test runtime version=%q source=go_env compile_error=true", goVersion)
		return goVersion
	}
	runtimeVersion := strings.TrimPrefix(strings.TrimSpace(string(out)), "compile version ")
	if base, experiments, _ := strings.Cut(runtimeVersion, " X:"); experiments == "" {
		runtimeVersion = base
	} else {
		runtimeVersion = base + experimentSeparator(goVersion, base) + "X:" + experiments
	}
	debug.printf("test runtime version=%q source=go_tool_compile", runtimeVersion)
	return runtimeVersion
}

// cutExperiments splits a runtime version at the experiments that the linker
// appends after "-X:" or " X:".
func cutExperiments(value string) (base, experiments string, found bool) {
	for _, separator := range []string{"-X:", " X:"} {
		if base, experiments, found := strings.Cut(value, separator); found {
			return base, experiments, true
		}
	}
	return value, "", false
}

// experimentSeparator returns what the linker writes before "X:" after base.
// goVersion shows it when the toolchain has default experiments. Otherwise Go
// 1.26 and later write "-" after a version without one (go.dev/issue/75953),
// and earlier versions a space.
func experimentSeparator(goVersion, base string) string {
	if i := strings.Index(goVersion, "X:"); i > 0 {
		return goVersion[i-1 : i]
	}
	if strings.Contains(base, "-") || !version.IsValid(base) || version.Compare(base, "go1.26rc1") < 0 {
		return " "
	}
	return "-"
}

func supportsGoToolchain(value string) bool {
	// Development distributions report "devel go1.N-<revision> ...".
	fields := strings.Fields(strings.TrimPrefix(value, "devel "))
	return len(fields) != 0 && version.IsValid(fields[0]) && version.Compare(fields[0], minimumGoVersion) >= 0
}
