package runner

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/goenv"
)

// fakeModuleCacheToolchain lays out the files a mirror needs, read-only like
// the module cache's, and returns its environment.
func fakeModuleCacheToolchain(t *testing.T) *goEnvironment {
	t.Helper()
	return fakeModuleCacheToolchainFor(t, goruntime.GOOS)
}

func fakeModuleCacheToolchainFor(t *testing.T, hostOS string) *goEnvironment {
	t.Helper()
	modcache := filepath.Join(t.TempDir(), "mod")
	goroot := filepath.Join(modcache, "golang.org", "toolchain@v0.0.1-go1.99.0.test")
	exe := ""
	if hostOS == "windows" {
		exe = ".exe"
	}
	tools := filepath.Join("pkg", "tool", hostOS+"_"+goruntime.GOARCH)
	for rel, content := range map[string]string{
		filepath.Join("bin", "go"+exe):                "go",
		filepath.Join(tools, "compile"+exe):           "compile",
		filepath.Join(tools, "link"+exe):              "link",
		filepath.Join("src", "testing", "testing.go"): "package testing\n",
		filepath.Join("src", "fmt", "print.go"):       "package fmt\n",
		"VERSION":                                     "go1.99.0\n",
	} {
		path := filepath.Join(goroot, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0555); err != nil {
			t.Fatal(err)
		}
	}
	setTreeWritable(t, modcache, false)
	t.Cleanup(func() { setTreeWritable(t, modcache, true) })
	return &goEnvironment{GOROOT: goroot, GOMODCACHE: modcache, GOVERSION: "go1.99.0", GOHOSTOS: hostOS, GOHOSTARCH: goruntime.GOARCH}
}

// setTreeWritable makes the module cache's directories read-only, as Go does,
// or writable again so that the test can remove it.
func setTreeWritable(t *testing.T, root string, writable bool) {
	t.Helper()
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && entry.IsDir() {
			mode := fs.FileMode(0555)
			if writable {
				mode = 0755
			}
			_ = os.Chmod(path, mode)
		}
		return nil
	})
}

// sourceModes records the source's file modes and contents, which no mirror
// operation may change.
func sourceModes(t *testing.T, root string) map[string]string {
	t.Helper()
	modes := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		modes[path] = info.Mode().String() + ":" + string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return modes
}

func useTestUserCache(t *testing.T) string {
	t.Helper()
	cache := t.TempDir()
	previous := userCacheDir
	userCacheDir = func() (string, error) { return cache, nil }
	t.Cleanup(func() { userCacheDir = previous })
	return cache
}

func TestFilePathHasPrefix(t *testing.T) {
	sep := string(filepath.Separator)
	root := filepath.Join(sep+"cache", "mod")
	for _, tc := range []struct {
		path string
		want bool
	}{
		{root, true},
		{filepath.Join(root, "golang.org", "toolchain@v0.0.1-go1.27.1.linux-amd64"), true},
		{root + "2", false},
		{filepath.Join(sep+"cache", "mo"), false},
		{sep + "other", false},
	} {
		if got := filePathHasPrefix(tc.path, root); got != tc.want {
			t.Errorf("filePathHasPrefix(%q, %q) = %t; want %t", tc.path, root, got, tc.want)
		}
	}
	if goruntime.GOOS == "windows" && !filePathHasPrefix(`c:\cache\mod\x`, `C:\cache\mod`) {
		t.Error("volume names must compare without case, as Go's check does")
	}
}

func TestNeedsGOROOTMirror(t *testing.T) {
	env := fakeModuleCacheToolchain(t)
	if !needsGOROOTMirror(env) {
		t.Fatal("a toolchain in GOMODCACHE needs a mirror")
	}
	outside := *env
	outside.GOMODCACHE = filepath.Join(filepath.Dir(env.GOMODCACHE), "mod2")
	if needsGOROOTMirror(&outside) {
		t.Fatal("a toolchain outside GOMODCACHE needs no mirror")
	}
	empty := *env
	empty.GOMODCACHE = ""
	if needsGOROOTMirror(&empty) {
		t.Fatal("no GOMODCACHE, no mirror")
	}
}

// A mirror links the toolchain's files, leaves the module cache unchanged and
// is reused by later runs.
func TestGOROOTMirrorCreatesAndReuses(t *testing.T) {
	env := fakeModuleCacheToolchain(t)
	useTestUserCache(t)
	before := sourceModes(t, env.GOMODCACHE)
	var links atomic.Int32
	previous := gorootMirrorLink
	gorootMirrorLink = func(old, new string) error { links.Add(1); return previous(old, new) }
	t.Cleanup(func() { gorootMirrorLink = previous })

	root, lock, err := ensureGOROOTMirror(context.Background(), env, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	lock.Close()
	if !validGOROOTMirror(root, env) {
		t.Fatal("new mirror is invalid")
	}
	mirrored, err := os.Stat(filepath.Join(root, "src", "testing", "testing.go"))
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.Stat(filepath.Join(env.GOROOT, "src", "testing", "testing.go"))
	if err != nil {
		t.Fatal(err)
	}
	if linked := os.SameFile(mirrored, original); linked != (goruntime.GOOS != "windows") {
		t.Fatalf("hard-linked=%t on %s", linked, goruntime.GOOS)
	}
	created := links.Load()
	root2, lock2, err := ensureGOROOTMirror(context.Background(), env, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	lock2.Close()
	if root2 != root || links.Load() != created {
		t.Fatalf("reuse rebuilt the mirror: %s %s links %d→%d", root, root2, created, links.Load())
	}
	if after := sourceModes(t, env.GOMODCACHE); !equalMaps(before, after) {
		t.Fatalf("the module cache changed:\n%v\n%v", before, after)
	}
}

func equalMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// Without hard links, as across file systems, the mirror copies every file.
func TestGOROOTMirrorCopiesWithoutLinks(t *testing.T) {
	env := fakeModuleCacheToolchain(t)
	useTestUserCache(t)
	previous := gorootMirrorLink
	gorootMirrorLink = func(string, string) error { return errors.New("invalid cross-device link") }
	t.Cleanup(func() { gorootMirrorLink = previous })
	root, lock, err := ensureGOROOTMirror(context.Background(), env, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	lock.Close()
	mirrored, _ := os.Stat(filepath.Join(root, "bin", filepath.Base(validToolPath(env, "go"))))
	original, _ := os.Stat(validToolPath(env, "go"))
	if mirrored == nil || original == nil || os.SameFile(mirrored, original) || mirrored.Mode() != original.Mode() {
		t.Fatalf("not a copy with the same mode: %v %v", mirrored, original)
	}
	data, err := os.ReadFile(filepath.Join(root, gorootMirrorMarker))
	if err != nil || !bytes.Contains(data, []byte(`"copied":6`)) {
		t.Fatalf("record %s: %v", data, err)
	}
}

func validToolPath(env *goEnvironment, name string) string {
	if env.GOHOSTOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(env.GOROOT, "bin", name)
}

// A mirror whose record or files are missing is replaced, not reused.
func TestGOROOTMirrorRebuildsIncompleteMirror(t *testing.T) {
	env := fakeModuleCacheToolchain(t)
	useTestUserCache(t)
	for _, damage := range []string{gorootMirrorMarker, filepath.Join("src", "testing", "testing.go"), "other source"} {
		root, lock, err := ensureGOROOTMirror(context.Background(), env, t.TempDir(), nil)
		if err != nil {
			t.Fatal(err)
		}
		lock.Close()
		if damage == "other source" {
			// A record of another toolchain, even at this path, is not reused.
			err = os.WriteFile(filepath.Join(root, gorootMirrorMarker), []byte(`{"source":"/other","go_version":"go1.99.0","files":6}`), 0600)
		} else {
			err = os.Remove(filepath.Join(root, damage))
		}
		if err != nil {
			t.Fatal(err)
		}
		if validGOROOTMirror(root, env) {
			t.Fatalf("mirror without %s is valid", damage)
		}
		root, lock, err = ensureGOROOTMirror(context.Background(), env, t.TempDir(), nil)
		if err != nil {
			t.Fatal(err)
		}
		lock.Close()
		if !validGOROOTMirror(root, env) {
			t.Fatalf("mirror without %s was not rebuilt", damage)
		}
	}
}

// Concurrent runs build one mirror; every run gets a valid one.
func TestGOROOTMirrorConcurrentCreators(t *testing.T) {
	env := fakeModuleCacheToolchain(t)
	useTestUserCache(t)
	var links atomic.Int32
	previous := gorootMirrorLink
	gorootMirrorLink = func(old, new string) error { links.Add(1); return previous(old, new) }
	t.Cleanup(func() { gorootMirrorLink = previous })
	var wg sync.WaitGroup
	roots := make([]string, 8)
	errs := make([]error, 8)
	for i := range roots {
		wg.Go(func() {
			var lock *os.File
			roots[i], lock, errs[i] = ensureGOROOTMirror(context.Background(), env, t.TempDir(), nil)
			if lock != nil {
				defer lock.Close()
				if !validGOROOTMirror(roots[i], env) {
					errs[i] = errors.New("invalid mirror while held")
				}
			}
		})
	}
	wg.Wait()
	for i := range roots {
		if errs[i] != nil || roots[i] != roots[0] {
			t.Fatalf("run %d: %s %v", i, roots[i], errs[i])
		}
	}
	if goruntime.GOOS != "windows" && links.Load() != 6 {
		t.Fatalf("%d links for 6 files: more than one build", links.Load())
	}
}

// Pruning removes unused mirrors and abandoned staging directories, keeps
// mirrors in use and never changes the module cache.
func TestGOROOTMirrorPruning(t *testing.T) {
	env := fakeModuleCacheToolchain(t)
	cache := useTestUserCache(t)
	before := sourceModes(t, env.GOMODCACHE)
	old := *env
	old.GOVERSION = "go1.98.0"
	used := *env
	used.GOVERSION = "go1.97.0"
	var roots []string
	var usedLock *os.File
	for _, e := range []*goEnvironment{&old, &used, env} {
		root, lock, err := ensureGOROOTMirror(context.Background(), e, t.TempDir(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if e == &used {
			usedLock = lock
		} else {
			lock.Close()
		}
		roots = append(roots, root)
	}
	defer usedLock.Close()
	parent := filepath.Join(cache, "ddto", "goroots")
	stale := time.Now().Add(-gorootMirrorRetention - time.Hour)
	for _, root := range roots[:2] {
		if err := os.Chtimes(root, stale, stale); err != nil {
			t.Fatal(err)
		}
	}
	abandoned := filepath.Join(parent, filepath.Base(roots[0])+".tmp-123")
	if err := os.MkdirAll(filepath.Join(abandoned, "src"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(filepath.Join(env.GOROOT, "VERSION"), filepath.Join(abandoned, "src", "VERSION")); err != nil && goruntime.GOOS != "windows" {
		t.Fatal(err)
	}
	// A creator that holds its key's lock is still building its staging.
	building := filepath.Join(parent, filepath.Base(roots[1])+".tmp-456")
	if err := os.MkdirAll(building, 0700); err != nil {
		t.Fatal(err)
	}
	creator, err := holdGOROOTMirrorLock(context.Background(), roots[1]+".lock", false)
	if err != nil {
		t.Fatal(err)
	}
	defer creator.Close()
	pruneGOROOTMirrors(parent, filepath.Base(roots[2]))
	if _, err := os.Stat(building); err != nil {
		t.Fatalf("a creator's staging was removed: %v", err)
	}
	if _, err := os.Stat(roots[0]); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("unused mirror kept: %v", err)
	}
	if _, err := os.Stat(roots[0] + ".lock"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("unused mirror's lock kept: %v", err)
	}
	if _, err := os.Stat(abandoned); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("abandoned staging kept: %v", err)
	}
	if !validGOROOTMirror(roots[1], &used) {
		t.Fatal("a held mirror was removed")
	}
	if !validGOROOTMirror(roots[2], env) {
		t.Fatal("the kept mirror was removed")
	}
	if after := sourceModes(t, env.GOMODCACHE); !equalMaps(before, after) {
		t.Fatalf("pruning changed the module cache:\n%v\n%v", before, after)
	}
}

// The mirror never goes into GOMODCACHE, the toolchain or the module.
func TestGOROOTMirrorRejectsProtectedCache(t *testing.T) {
	env := fakeModuleCacheToolchain(t)
	module := t.TempDir()
	env.GOMOD = filepath.Join(module, "go.mod")
	for _, cache := range []string{env.GOMODCACHE, filepath.Join(env.GOROOT, "cache"), filepath.Join(module, ".cache")} {
		previous := userCacheDir
		userCacheDir = func() (string, error) { return cache, nil }
		_, _, err := gorootMirrorLocation(env, t.TempDir())
		userCacheDir = previous
		if err == nil {
			t.Errorf("cache %s accepted", cache)
		}
	}
	work := t.TempDir()
	previous := userCacheDir
	userCacheDir = func() (string, error) { return filepath.Join(work, ".cache"), nil }
	defer func() { userCacheDir = previous }()
	if _, _, err := gorootMirrorLocation(env, work); err == nil {
		t.Error("a cache within the working directory was accepted")
	}
}

// Test processes see the GOROOT and PATH that go test gives them natively,
// while this process runs the mirror's go.
func TestUseGOROOTMirrorEnvironment(t *testing.T) {
	env := fakeModuleCacheToolchain(t)
	mirror := filepath.Join(t.TempDir(), "mirror")
	callerPath := strings.Join([]string{"/caller/bin", "/usr/bin"}, string(os.PathListSeparator))
	sep := string(os.PathListSeparator)

	// Switched: the caller's go is another toolchain, which hands GOROOT on.
	t.Setenv("PATH", callerPath)
	os.Unsetenv("GOROOT")
	saved, undo, err := useGOROOTMirror(mirror, env.GOROOT)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		goenv.SaveValue("GOROOT", env.GOROOT, true),
		goenv.SaveValue("PATH", filepath.Join(env.GOROOT, "bin")+sep+callerPath, true),
	}
	if strings.Join(saved, "\n") != strings.Join(want, "\n") {
		t.Fatalf("saved %q; want %q", saved, want)
	}
	if got := os.Getenv("PATH"); got != filepath.Join(mirror, "bin")+sep+callerPath {
		t.Fatalf("PATH=%q", got)
	}
	if _, set := os.LookupEnv("GOROOT"); set {
		t.Fatal("GOROOT set although the caller left it unset")
	}
	undo()
	if os.Getenv("PATH") != callerPath {
		t.Fatalf("PATH not restored: %q", os.Getenv("PATH"))
	}

	// Direct: the caller runs that toolchain's go; its own GOROOT stays.
	direct := filepath.Join(env.GOROOT, "bin") + sep + callerPath
	t.Setenv("PATH", direct)
	t.Setenv("GOROOT", "/caller/goroot")
	saved, undo, err = useGOROOTMirror(mirror, env.GOROOT)
	if err != nil {
		t.Fatal(err)
	}
	if saved[0] != goenv.SaveValue("GOROOT", "/caller/goroot", true) {
		t.Fatalf("saved %q", saved[0])
	}
	if os.Getenv("GOROOT") != mirror {
		t.Fatalf("GOROOT=%q; the caller's would select the module cache", os.Getenv("GOROOT"))
	}
	undo()
	if os.Getenv("GOROOT") != "/caller/goroot" || os.Getenv("PATH") != direct {
		t.Fatal("environment not restored")
	}
}

// Windows mirrors copy: deleting a read-only hard link there would clear the
// module cache file's attribute.
func TestGOROOTMirrorCopiesForWindows(t *testing.T) {
	env := fakeModuleCacheToolchainFor(t, "windows")
	useTestUserCache(t)
	root, lock, err := ensureGOROOTMirror(context.Background(), env, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	lock.Close()
	mirrored, err := os.Stat(filepath.Join(root, "bin", "go.exe"))
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.Stat(filepath.Join(env.GOROOT, "bin", "go.exe"))
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(mirrored, original) {
		t.Fatal("a Windows mirror shares a file with the module cache")
	}
}

// A testing package in GOMODCACHE, or outside the mirror, would make Go reject
// the overlay: preparation stops and ddto runs go test natively.
func TestCheckTestingGOROOT(t *testing.T) {
	env := fakeModuleCacheToolchain(t)
	mirror := &gorootMirror{root: filepath.Join(t.TempDir(), "mirror")}
	var mirrorErr *gorootMirrorError
	for _, tc := range []struct {
		dir    string
		mirror *gorootMirror
		ok     bool
	}{
		{filepath.Join(mirror.root, "src", "testing"), mirror, true},
		{filepath.Join(t.TempDir(), "goroot", "src", "testing"), nil, true},
		{filepath.Join(env.GOROOT, "src", "testing"), nil, false},
		{filepath.Join(t.TempDir(), "other", "src", "testing"), mirror, false},
	} {
		err := checkTestingGOROOT(tc.dir, env, tc.mirror)
		if (err == nil) != tc.ok || err != nil && !errors.As(err, &mirrorErr) {
			t.Errorf("%s with mirror %t: %v", tc.dir, tc.mirror != nil, err)
		}
	}
}
