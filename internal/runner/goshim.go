package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// goShimVariable names the directory of the go that ddto puts first on PATH
// for gotestsum, which runs go test itself. That go is ddto: it leaves the
// directory off PATH and runs as ddto go, so gotestsum's go test reports to
// CI Visibility with gotestsum's own arguments.
const goShimVariable = "DDTO_GO_SHIM"

// goShimLeft is the go shim directory that this process left off PATH.
var goShimLeft string

// GoShimArgs returns ddto's arguments from its command line: go and its
// arguments when ddto runs as the go shim. Any ddto under gotestsum leaves the
// shim's directory off PATH first, so the go that it and its commands run is
// the toolchain's, and the tests see gotestsum's PATH.
func GoShimArgs(args []string) []string {
	dir, set := os.LookupEnv(goShimVariable)
	if !set {
		return args[1:]
	}
	_ = os.Unsetenv(goShimVariable)
	_ = os.Setenv("PATH", withoutPathEntry(os.Getenv("PATH"), dir))
	goShimLeft = dir
	if isProgram(args[0], "go") {
		return append([]string{"go"}, args[1:]...)
	}
	return args[1:]
}

// logGoShim logs the go shim that this process left off PATH, and the one that
// it puts first for the command it runs.
func (d *cliDebug) logGoShim() {
	if goShimLeft != "" {
		d.printf("go_shim left=%q", goShimLeft)
	}
	if dir := os.Getenv(goShimVariable); dir != "" {
		d.printf("go_shim first_on_path=%q", dir)
	}
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

// UseGoShim puts the go shim first on PATH for the command that ddto runs
// next, which inherits it.
func UseGoShim() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	dir, err := goShimDir(self)
	if err != nil {
		return err
	}
	if err := os.Setenv(goShimVariable, dir); err != nil {
		return err
	}
	return os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// goShimDir returns the directory of a go that is this ddto, in ddto's user
// cache: a symbolic link to it, or on Windows a hard link or copy. Its name
// changes with ddto's path, size and modification time, so an upgraded ddto
// gets its own.
func goShimDir(self string) (string, error) {
	info, err := os.Stat(self)
	if err != nil {
		return "", err
	}
	cache, err := userCacheDir()
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(cache) {
		return "", fmt.Errorf("user cache directory %q is not absolute", cache)
	}
	sum := sha256.Sum256([]byte(self + "\x00" + strconv.FormatInt(info.Size(), 10) + "\x00" + strconv.FormatInt(info.ModTime().UnixNano(), 10)))
	dir := filepath.Join(cache, "ddto", "go-shim", hex.EncodeToString(sum[:8]))
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

// withoutPathEntry returns the PATH list without the entries that name dir,
// as UseGoShim added it.
func withoutPathEntry(list, dir string) string {
	var kept []string
	for _, entry := range filepath.SplitList(list) {
		if filepath.Clean(entry) != filepath.Clean(dir) {
			kept = append(kept, entry)
		}
	}
	return strings.Join(kept, string(os.PathListSeparator))
}
