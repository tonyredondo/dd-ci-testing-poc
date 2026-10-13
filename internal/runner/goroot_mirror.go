package runner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/goenv"
)

// A toolchain that GOTOOLCHAIN runs from the module cache keeps GOROOT beneath
// GOMODCACHE, where Go rejects every overlay entry (cmd/go/internal/work),
// including ddto's rewritten testing sources. ddto then runs go from a mirror
// of that GOROOT in its user cache: the same files, hard-linked where the file
// system allows and copied otherwise. Windows always copies: deleting a
// read-only hard link there clears the attribute of the module cache's file.
// Standard packages' compile cache keys omit GOROOT's location, so the mirror
// reuses what the toolchain already compiled.

const (
	gorootMirrorRetention = 14 * 24 * time.Hour
	gorootMirrorMarker    = ".ddto-goroot-mirror.json"
)

// gorootMirrorLink creates a mirror's file; tests replace it to force copies.
var gorootMirrorLink = os.Link

// gorootMirrorLockOpened lets tests interleave other runs with a waiting one.
var gorootMirrorLockOpened = func() {}

// gorootMirror is the mirror one ddto test uses.
type gorootMirror struct {
	root string
	// lock is held shared while this run's go commands use the mirror, so a
	// pruner cannot remove it.
	lock *os.File
	// saved are the DDTO_ORIGINAL_ entries for the GOROOT and PATH that test
	// processes would see natively.
	saved []string
	// undo restores this process's PATH and GOROOT.
	undo func()
}

// release undoes the mirror's environment and lets pruners remove it. Call it
// after every go command of the run has finished.
func (m *gorootMirror) release() {
	if m == nil {
		return
	}
	if m.undo != nil {
		m.undo()
		m.undo = nil
	}
	if m.lock != nil {
		_ = m.lock.Close()
		m.lock = nil
	}
}

type gorootMirrorRecord struct {
	Source    string `json:"source"`
	GoVersion string `json:"go_version"`
	Files     int    `json:"files"`
	Copied    int    `json:"copied"`
}

// gorootMirrorError is a preparation failure that leaves a module-cache
// toolchain without a usable mirror. ddto then runs go test natively.
type gorootMirrorError struct{ err error }

func (e *gorootMirrorError) Error() string {
	return "toolchain in GOMODCACHE without a mirror: " + e.err.Error()
}

func (e *gorootMirrorError) Unwrap() error { return e.err }

// filePathHasPrefix is cmd/go's str.HasFilePathPrefix, the rule by which Go
// rejects an overlay entry beneath GOMODCACHE: volume names compare without
// case, the rest exactly, and only whole path elements match.
func filePathHasPrefix(s, prefix string) bool {
	sv, pv := filepath.VolumeName(s), filepath.VolumeName(prefix)
	s, prefix = s[len(sv):], prefix[len(pv):]
	if sv != pv && strings.ToUpper(sv) != strings.ToUpper(pv) {
		return false
	}
	switch {
	case len(s) == len(prefix):
		return s == prefix
	case prefix == "":
		return true
	case len(s) > len(prefix):
		if prefix[len(prefix)-1] == filepath.Separator {
			return strings.HasPrefix(s, prefix)
		}
		return s[len(prefix)] == filepath.Separator && s[:len(prefix)] == prefix
	}
	return false
}

// needsGOROOTMirror reports whether Go would reject the testing overlay.
func needsGOROOTMirror(env *goEnvironment) bool {
	return env.GOROOT != "" && env.GOMODCACHE != "" &&
		filePathHasPrefix(filepath.Clean(env.GOROOT), filepath.Clean(env.GOMODCACHE))
}

// resolvedPath resolves symbolic links in the longest existing prefix of an
// absolute path, so that containment checks see where files would go.
func resolvedPath(path string) string {
	path = filepath.Clean(path)
	rest := ""
	for current := path; ; {
		if resolved, err := filepath.EvalSymlinks(current); err == nil {
			return filepath.Join(resolved, rest)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return path
		}
		rest = filepath.Join(filepath.Base(current), rest)
		current = parent
	}
}

// gorootMirrorLocation returns the cache directory for mirrors and this
// toolchain's key. The directory must not lie within GOMODCACHE, the toolchain
// itself or the module, which must stay unchanged.
func gorootMirrorLocation(env *goEnvironment, dir string) (parent, key string, err error) {
	cache, err := userCacheDir()
	if err != nil {
		return "", "", err
	}
	if !filepath.IsAbs(cache) {
		return "", "", fmt.Errorf("user cache directory %q is not absolute", cache)
	}
	parent = filepath.Join(cache, "ddto", "goroots")
	resolved := resolvedPath(parent)
	protected := []string{env.GOMODCACHE, env.GOROOT, dir}
	for _, file := range []string{env.GOMOD, env.GOWORK} {
		if file != "" && file != os.DevNull && filepath.IsAbs(file) {
			protected = append(protected, filepath.Dir(file))
		}
	}
	for _, path := range protected {
		if path != "" && filePathHasPrefix(resolved, resolvedPath(path)) {
			return "", "", fmt.Errorf("user cache directory %s lies within %s", parent, path)
		}
	}
	sum := sha256.Sum256([]byte(env.GOROOT + "\x00" + env.GOVERSION + "\x00" + env.GOHOSTOS + "_" + env.GOHOSTARCH))
	name := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-' {
			return r
		}
		return '_'
	}, env.GOVERSION)
	return parent, name + "-" + hex.EncodeToString(sum[:8]), nil
}

// validGOROOTMirror reports a complete mirror of env's toolchain: its record,
// written last, names the source, and the files that go test needs exist.
func validGOROOTMirror(root string, env *goEnvironment) bool {
	data, err := os.ReadFile(filepath.Join(root, gorootMirrorMarker))
	if err != nil {
		return false
	}
	var record gorootMirrorRecord
	if json.Unmarshal(data, &record) != nil || record.Source != env.GOROOT || record.GoVersion != env.GOVERSION || record.Files == 0 {
		return false
	}
	exe := ""
	if env.GOHOSTOS == "windows" {
		exe = ".exe"
	}
	tools := filepath.Join("pkg", "tool", env.GOHOSTOS+"_"+env.GOHOSTARCH)
	for _, rel := range []string{filepath.Join("bin", "go"+exe), filepath.Join(tools, "compile"+exe), filepath.Join(tools, "link"+exe), filepath.Join("src", "testing", "testing.go")} {
		info, err := os.Stat(filepath.Join(root, rel))
		if err != nil || !info.Mode().IsRegular() {
			return false
		}
	}
	return true
}

// holdGOROOTMirrorLock locks path, shared or exclusive, after confirming that
// the open file is still the lock file: a pruner may have replaced it.
func holdGOROOTMirrorLock(ctx context.Context, path string, exclusive bool) (*os.File, error) {
	for attempt := 0; attempt < 3; attempt++ {
		lock, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
		if err != nil {
			return nil, err
		}
		gorootMirrorLockOpened()
		if err := waitForLock(ctx, lock, exclusive); err != nil {
			lock.Close()
			return nil, err
		}
		if sameOpenFile(lock, path) {
			return lock, nil
		}
		lock.Close()
	}
	return nil, errors.New("toolchain mirror lock was replaced repeatedly")
}

// ensureGOROOTMirror returns a valid mirror of env's toolchain and its lock,
// held shared. A missing or incomplete mirror is rebuilt under the exclusive
// lock, which waits for runs still using the old one.
func ensureGOROOTMirror(ctx context.Context, env *goEnvironment, dir string, debug *cliDebug) (root string, lock *os.File, err error) {
	parent, key, err := gorootMirrorLocation(env, dir)
	if err != nil {
		return "", nil, err
	}
	if err := os.MkdirAll(parent, 0700); err != nil {
		return "", nil, err
	}
	root = filepath.Join(parent, key)
	if lock, err = holdGOROOTMirrorLock(ctx, root+".lock", false); err != nil {
		return "", nil, err
	}
	if validGOROOTMirror(root, env) {
		touchGOROOTMirror(root)
		debug.printf("goroot mirror state=reused source=%q mirror=%q", env.GOROOT, root)
		return root, lock, nil
	}
	lock.Close()
	if lock, err = holdGOROOTMirrorLock(ctx, root+".lock", true); err != nil {
		return "", nil, err
	}
	created := false
	if !validGOROOTMirror(root, env) {
		if err := createGOROOTMirror(parent, key, root, env, debug); err != nil {
			lock.Close()
			return "", nil, err
		}
		created = true
	}
	touchGOROOTMirror(root)
	lock.Close()
	if lock, err = holdGOROOTMirrorLock(ctx, root+".lock", false); err != nil {
		return "", nil, err
	}
	// The exclusive lock was released: another run may only have reused it,
	// and its fresh time keeps pruners away.
	if !validGOROOTMirror(root, env) {
		lock.Close()
		return "", nil, errors.New("toolchain mirror changed while it was published")
	}
	if created {
		pruneGOROOTMirrors(parent, key)
	} else {
		debug.printf("goroot mirror state=reused source=%q mirror=%q", env.GOROOT, root)
	}
	return root, lock, nil
}

// createGOROOTMirror replaces an incomplete mirror with a new one. The caller
// holds the mirror's exclusive lock.
func createGOROOTMirror(parent, key, root string, env *goEnvironment, debug *cliDebug) error {
	started := time.Now()
	if err := removeGOROOTMirror(root); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(parent, key+".tmp-")
	if err != nil {
		return err
	}
	defer removeGOROOTMirror(staging) // Already gone after a successful rename.
	files, copied, err := buildGOROOTMirror(env.GOROOT, staging, env.GOHOSTOS != "windows")
	if err != nil {
		return err
	}
	data, err := json.Marshal(gorootMirrorRecord{Source: env.GOROOT, GoVersion: env.GOVERSION, Files: files, Copied: copied})
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(staging, gorootMirrorMarker), data, 0600); err != nil {
		return err
	}
	if err := os.Rename(staging, root); err != nil {
		return err
	}
	if !validGOROOTMirror(root, env) {
		return errors.New("new toolchain mirror is incomplete")
	}
	mode := "hardlink"
	switch {
	case copied == files:
		mode = "copy"
	case copied > 0:
		mode = "mixed"
	}
	debug.printf("goroot mirror state=created source=%q mirror=%q mode=%s files=%d copied=%d duration=%s", env.GOROOT, root, mode, files, copied, time.Since(started).Round(time.Microsecond))
	return nil
}

// buildGOROOTMirror recreates source's tree in staging. Directories are made
// writable for their owner so removal never changes a file's mode: a
// hard-linked file shares its mode with the module cache. After the first
// failed link, the remaining files are copied.
func buildGOROOTMirror(source, staging string, link bool) (files, copied int, err error) {
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil || rel == "." {
			return err
		}
		target := filepath.Join(staging, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case info.IsDir():
			return os.Mkdir(target, info.Mode().Perm()|0700)
		case info.Mode()&fs.ModeSymlink != 0:
			destination, err := os.Readlink(path)
			if err != nil {
				return err
			}
			files++
			return os.Symlink(destination, target)
		case info.Mode().IsRegular():
			files++
			if link {
				if gorootMirrorLink(path, target) == nil {
					return nil
				}
				link = false // Another file system, or links unsupported.
			}
			copied++
			return copyGOROOTFile(path, target, info.Mode().Perm())
		}
		return fmt.Errorf("unsupported file in GOROOT: %s", path)
	})
	return files, copied, err
}

func copyGOROOTFile(source, target string, perm fs.FileMode) (err error) {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := out.Close(); err == nil {
			err = closeErr
		}
	}()
	// Copy the bytes. io.Copy would use copy_file_range, which btrfs turns
	// into a reflink; a GOROOT of reflinks made the next go list wait about
	// 25 s on the first access to its files, against 0.1 s after a real copy.
	_, err = io.CopyBuffer(struct{ io.Writer }{out}, struct{ io.Reader }{in}, make([]byte, 256<<10))
	return err
}

// removeGOROOTMirror deletes a mirror or staging tree. It makes directories
// writable and never changes a file's mode, which a hard link shares with the
// module cache. Windows mirrors hold copies only.
func removeGOROOTMirror(path string) error {
	_ = filepath.WalkDir(path, func(p string, entry fs.DirEntry, err error) error {
		if err == nil && entry.IsDir() {
			_ = os.Chmod(p, 0700)
		}
		return nil
	})
	return os.RemoveAll(path)
}

func touchGOROOTMirror(root string) {
	now := time.Now()
	_ = os.Chtimes(root, now, now)
}

// pruneGOROOTMirrors removes mirrors unused for the retention period and
// staging directories that no creator is still building.
func pruneGOROOTMirrors(parent, keep string) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-gorootMirrorRetention)
	for _, entry := range entries {
		name := entry.Name()
		key, isLock := strings.CutSuffix(name, ".lock")
		if staged, _, ok := strings.Cut(name, ".tmp-"); ok {
			key = staged
		}
		if key == keep {
			continue
		}
		if strings.Contains(name, ".tmp-") {
			// A creator holds its key's exclusive lock while it builds.
			removeAbandonedGOROOTStaging(parent, key, name)
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.ModTime().Before(cutoff) {
			continue
		}
		switch {
		case isLock:
			if _, err := os.Lstat(filepath.Join(parent, key)); errors.Is(err, fs.ErrNotExist) {
				removeUnusedGOROOTMirror(parent, key, cutoff)
			}
		case entry.IsDir():
			removeUnusedGOROOTMirror(parent, name, cutoff)
		}
	}
}

// removeAbandonedGOROOTStaging removes a staging directory that no creator
// is building. Creators lock their key before they create one, so without a
// lock file no creator can be building it.
func removeAbandonedGOROOTStaging(parent, key, name string) {
	path := filepath.Join(parent, key+".lock")
	lock, err := os.OpenFile(path, os.O_RDWR, 0)
	if err == nil {
		defer lock.Close()
		if tryLockFile(lock, true) != nil || !sameOpenFile(lock, path) {
			return
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return
	}
	_ = removeGOROOTMirror(filepath.Join(parent, name))
}

// removeUnusedGOROOTMirror removes a mirror only while no run holds it. A run
// that reused it after the directory scan refreshed its time, so the time is
// checked again under the exclusive lock.
func removeUnusedGOROOTMirror(parent, key string, cutoff time.Time) {
	path := filepath.Join(parent, key+".lock")
	lock, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return
	}
	defer lock.Close()
	if tryLockFile(lock, true) != nil || !sameOpenFile(lock, path) {
		return
	}
	root := filepath.Join(parent, key)
	if info, err := os.Lstat(root); err == nil && !info.ModTime().Before(cutoff) {
		return
	}
	if err := removeGOROOTMirror(root); err != nil {
		return
	}
	if os.Remove(path) != nil {
		// Windows cannot remove an open file, including this handle.
		lock.Close()
		_ = os.Remove(path)
	}
}

// switchedToolchain reports whether the caller's go switched to goroot's
// toolchain rather than being it: the switch hands that toolchain GOROOT.
func switchedToolchain(goroot string) bool {
	path, err := exec.LookPath("go")
	if err != nil {
		return true
	}
	bin, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return true
	}
	native, err := filepath.EvalSymlinks(filepath.Join(goroot, "bin"))
	return err != nil || bin != native
}

func prependPath(dir, list string) string {
	if list == "" {
		return dir
	}
	return dir + string(os.PathListSeparator) + list
}

// useGOROOTMirror makes this process's go commands, and those of a launcher
// such as Orchestrion, run the mirror's go, through PATH and, when the caller
// sets it, GOROOT. It returns the GOROOT and PATH that go test would give test
// processes natively: the switched-to toolchain's GOROOT, or the caller's,
// and that toolchain's bin before the caller's PATH. Mini's test runtime
// restores them before any package initializes.
func useGOROOTMirror(root, goroot string) (saved []string, undo func(), err error) {
	callerPath, pathSet := os.LookupEnv("PATH")
	callerGOROOT, gorootSet := os.LookupEnv("GOROOT")
	nativeGOROOT, nativeSet := callerGOROOT, gorootSet
	if switchedToolchain(goroot) {
		nativeGOROOT, nativeSet = goroot, true
	}
	saved = []string{
		goenv.SaveValue("GOROOT", nativeGOROOT, nativeSet),
		goenv.SaveValue("PATH", prependPath(filepath.Join(goroot, "bin"), callerPath), true),
	}
	undo = func() {
		if pathSet {
			_ = os.Setenv("PATH", callerPath)
		} else {
			_ = os.Unsetenv("PATH")
		}
		if gorootSet {
			_ = os.Setenv("GOROOT", callerGOROOT)
		}
	}
	if err := os.Setenv("PATH", prependPath(filepath.Join(root, "bin"), callerPath)); err != nil {
		undo()
		return nil, nil, err
	}
	if gorootSet {
		if err := os.Setenv("GOROOT", root); err != nil {
			undo()
			return nil, nil, err
		}
	}
	return saved, undo, nil
}

// checkTestingGOROOT rejects a testing package that Go would not accept an
// overlay for. Provisioning can raise the Go version that the module needs,
// and go then switches to another toolchain, which may lie in GOMODCACHE or
// outside the mirror.
func checkTestingGOROOT(testingDir string, env *goEnvironment, mirror *gorootMirror) error {
	dir := filepath.Clean(testingDir)
	if env.GOMODCACHE != "" && filePathHasPrefix(dir, filepath.Clean(env.GOMODCACHE)) ||
		mirror != nil && !filePathHasPrefix(dir, mirror.root) {
		return &gorootMirrorError{fmt.Errorf("go selected testing from %s, outside a usable GOROOT", testingDir)}
	}
	return nil
}

// prepareGOROOTMirror selects the mirror for a module-cache toolchain and
// switches this process's go commands to it.
func prepareGOROOTMirror(ctx context.Context, env *goEnvironment, dir string, debug *cliDebug) (*gorootMirror, error) {
	phase := debug.start("goroot mirror")
	root, lock, err := ensureGOROOTMirror(ctx, env, dir, debug)
	phase.finish(err)
	if err != nil {
		return nil, err
	}
	saved, undo, err := useGOROOTMirror(root, env.GOROOT)
	if err != nil {
		lock.Close()
		return nil, err
	}
	return &gorootMirror{root: root, lock: lock, saved: saved, undo: undo}, nil
}
