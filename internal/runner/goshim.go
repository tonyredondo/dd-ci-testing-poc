package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/version"
)

// The go that ddto puts first on PATH for gotestsum, which runs go test
// itself, is ddto, named go in a directory of ddto's user cache. Run as go,
// ddto runs as ddto go, so gotestsum's go test reports to CI Visibility with
// gotestsum's own arguments.
var (
	// goShimFirst is the go shim directory that this process puts first on
	// PATH for the command that it runs.
	goShimFirst string
	// goShimLeft is the go shim directory that this process left off PATH.
	goShimLeft string
	// goShimGo is the go that a go command with the go shim runs.
	goShimGo string
)

// GoShimArgs returns ddto's arguments from its command line, and whether ddto
// runs as the go shim: then they are go and its arguments. Every ddto first
// leaves the go shims off PATH, as written otherwise, so the go that it and
// its commands run is the toolchain's, and tests see their caller's PATH.
func GoShimArgs(args []string) ([]string, bool) {
	if kept, left := withoutGoShims(os.Getenv("PATH")); left != "" {
		_ = os.Setenv("PATH", kept)
		goShimLeft = left
	}
	if isProgram(args[0], "go") {
		return append([]string{"go"}, args[1:]...), true
	}
	return args[1:], false
}

// logGoShim logs the go shim that this process left off PATH, and the one that
// it puts first for the command it runs.
func (d *cliDebug) logGoShim() {
	if goShimLeft != "" {
		d.printf("go_shim left=%q", goShimLeft)
	}
	if goShimFirst != "" {
		d.printf("go_shim first_on_path=%q", goShimFirst)
	}
	if goShimGo != "" {
		d.printf("go_shim go=%q", goShimGo)
	}
}

// goShimFlag puts the go shim first on PATH for any command, so that the go
// test that it, or a command it starts, runs reports to CI Visibility.
const goShimFlag = "--go-shim"

// GoShimCommand returns the command that ddto runs, without --go-shim, and
// whether it runs with the go shim: after --go-shim, and for gotestsum, also
// through go tool or go run.
func GoShimCommand(args []string) ([]string, bool) {
	if len(args) > 0 && args[0] == goShimFlag {
		return args[1:], true
	}
	return args, runsGotestsum(args)
}

// runsGotestsum reports whether args run gotestsum: itself, as a tool of the
// module, or with go run.
func runsGotestsum(args []string) bool {
	if IsGotestsum(args) {
		return true
	}
	if len(args) < 3 || !isProgram(args[0], "go") {
		return false
	}
	switch args[1] {
	case "tool":
		return args[2] == "gotestsum" || args[2] == "gotest.tools/gotestsum"
	case "run":
		for _, arg := range args[2:] {
			if !strings.HasPrefix(arg, "-") {
				pkg, _, _ := strings.Cut(arg, "@")
				return pkg == "gotest.tools/gotestsum"
			}
		}
	}
	return false
}

// IsGotestsum reports whether args run gotestsum.
func IsGotestsum(args []string) bool {
	return len(args) > 0 && isProgram(args[0], "gotestsum")
}

// isProgram reports whether path names the program name, with or without
// Windows' .exe.
func isProgram(path, name string) bool {
	base := filepath.Base(path)
	if runtime.GOOS == "windows" {
		base = strings.TrimSuffix(strings.ToLower(base), ".exe")
	}
	return base == name
}

// UseGoShim puts the go shim first on PATH for command, which ddto runs next,
// and returns the command to run. A go command runs the go that PATH names
// before the shim; see goShimGoCommand. On an error, PATH and the command stay
// as they were.
func UseGoShim(command []string) ([]string, error) {
	self, err := os.Executable()
	if err != nil {
		return command, err
	}
	dir, err := goShimDir(self)
	if err != nil {
		return command, err
	}
	run := command
	if isProgram(command[0], "go") {
		toolchain, err := exec.LookPath(command[0])
		if err != nil {
			return command, err
		}
		if run, err = goShimGoCommand(self, toolchain, command); err != nil {
			return command, err
		}
		goShimGo = toolchain
	}
	if err := prependGoShim(dir); err != nil {
		return command, err
	}
	return run, nil
}

// GoShimExecMode is ddto's mode as go run's -exec program with the go shim.
const GoShimExecMode = "go-shim-exec"

// goShimGoCommand returns the go command to run with the go shim. go run and
// go tool put $GOROOT/bin before the shim on their program's PATH, so go run
// runs its program through ddto's GoShimExecMode, which puts the shim first
// again, and ddto runs a tool's program itself.
func goShimGoCommand(self, toolchain string, command []string) ([]string, error) {
	run := append([]string{toolchain}, command[1:]...)
	if len(command) < 2 {
		return run, nil
	}
	switch command[1] {
	case "run":
		for _, arg := range command[2:] {
			name, _, _ := strings.Cut(strings.TrimLeft(arg, "-"), "=")
			if !strings.HasPrefix(arg, "-") {
				break
			}
			if name == "exec" {
				return nil, errors.New("go run -exec starts its program with $GOROOT/bin before the go shim")
			}
		}
		program, err := quoteToolWords([]string{self, GoShimExecMode})
		if err != nil {
			return nil, err
		}
		return append([]string{toolchain, "run", "-exec=" + program}, command[2:]...), nil
	case "tool":
		if len(command) < 3 || strings.HasPrefix(command[2], "-") {
			return nil, errors.New("go tool takes its flags before the tool with the go shim unsupported")
		}
		out, err := exec.Command(toolchain, "tool", "-n", command[2]).Output()
		if err != nil {
			return nil, fmt.Errorf("go tool -n %s: %w", command[2], err)
		}
		return append([]string{strings.TrimSpace(string(out))}, command[3:]...), nil
	}
	return run, nil
}

// ExecWithGoShim runs args, the program that go run built and its arguments,
// with the go shim first on its PATH again.
func ExecWithGoShim(args []string) int {
	self, err := os.Executable()
	if err == nil {
		var dir string
		if dir, err = goShimDir(self); err == nil {
			err = prependGoShim(dir)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, version.BuildLogPrefix+" WARN:", err, "- running", args[0], "without the go shim")
	}
	return ExecWithCallerEnvironment("", args)
}

// prependGoShim puts the go shim directory first on PATH.
func prependGoShim(dir string) error {
	entry := dir
	if strings.ContainsRune(dir, os.PathListSeparator) {
		if runtime.GOOS != "windows" {
			return fmt.Errorf("the go shim directory %s holds the PATH separator", dir)
		}
		entry = `"` + dir + `"`
	}
	if err := os.Setenv("PATH", entry+string(os.PathListSeparator)+os.Getenv("PATH")); err != nil {
		return err
	}
	goShimFirst = dir
	return nil
}

// goShimRoot is the directory of every go shim, in ddto's user cache.
func goShimRoot() (string, error) {
	cache, err := userCacheDir()
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(cache) {
		return "", fmt.Errorf("user cache directory %q is not absolute", cache)
	}
	return filepath.Join(cache, "ddto", "go-shim"), nil
}

// goShimDir returns the directory of a go that is this ddto: a symbolic link
// to it, or on Windows a hard link or copy. Its name changes with ddto's
// path, size and modification time, so an upgraded ddto gets its own.
func goShimDir(self string) (string, error) {
	info, err := os.Stat(self)
	if err != nil {
		return "", err
	}
	root, err := goShimRoot()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(self + "\x00" + strconv.FormatInt(info.Size(), 10) + "\x00" + strconv.FormatInt(info.ModTime().UnixNano(), 10)))
	dir := filepath.Join(root, hex.EncodeToString(sum[:8]))
	shim := filepath.Join(dir, "go")
	if runtime.GOOS == "windows" {
		shim += ".exe"
	}
	if _, err := os.Stat(shim); err == nil {
		return dir, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	// Another ddto may create the same shim: build it aside, then rename.
	temporary := shim + ".tmp" + strconv.Itoa(os.Getpid())
	_ = os.Remove(temporary)
	if runtime.GOOS != "windows" {
		err = os.Symlink(self, temporary)
	} else if err = os.Link(self, temporary); err != nil {
		err = copyExecutableFile(self, temporary)
	}
	if err != nil {
		return "", err
	}
	if err := os.Rename(temporary, shim); err != nil {
		_ = os.Remove(temporary)
		if _, statErr := os.Stat(shim); statErr != nil {
			return "", err
		}
	}
	return dir, nil
}

// copyExecutableFile copies the executable at source to target.
func copyExecutableFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	return errors.Join(err, out.Close())
}

// withoutGoShims returns the PATH list without its go shims, keeping the
// other entries as written, and the last entry that it left.
func withoutGoShims(list string) (kept, left string) {
	var entries []string
	for _, raw := range pathListEntries(list) {
		entry := raw
		if runtime.GOOS == "windows" {
			entry = strings.ReplaceAll(raw, `"`, "")
		}
		if isGoShimEntry(entry) {
			left = entry
			continue
		}
		entries = append(entries, raw)
	}
	return strings.Join(entries, string(os.PathListSeparator)), left
}

// isGoShimEntry reports whether a PATH entry is a directory that goShimDir
// makes, ddto/go-shim/ and 16 hexadecimal digits, in any user cache: an
// environment that a command filtered may name another. It reads no file, so
// a slow PATH entry does not delay ddto.
func isGoShimEntry(entry string) bool {
	if entry == "" || !filepath.IsAbs(entry) {
		return false
	}
	entry = filepath.Clean(entry)
	name, parent := filepath.Base(entry), filepath.Dir(entry)
	if len(name) != 16 || strings.Trim(name, "0123456789abcdef") != "" {
		return false
	}
	return filepath.Base(parent) == "go-shim" && filepath.Base(filepath.Dir(parent)) == "ddto"
}

// pathListEntries splits a PATH list where filepath.SplitList does, but keeps
// each entry as written: on Windows, quotes may enclose a separator.
func pathListEntries(list string) []string {
	if list == "" {
		return nil
	}
	if runtime.GOOS != "windows" {
		return strings.Split(list, string(os.PathListSeparator))
	}
	var entries []string
	start, quoted := 0, false
	for i := 0; i < len(list); i++ {
		switch list[i] {
		case '"':
			quoted = !quoted
		case os.PathListSeparator:
			if !quoted {
				entries = append(entries, list[start:i])
				start = i + 1
			}
		}
	}
	return append(entries, list[start:])
}
