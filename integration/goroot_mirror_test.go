package integration

import (
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// The fixture prints what tests that inspect their toolchain see.
const gorootMirrorFixture = `package mirror

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func TestToolchainEnvironment(t *testing.T) {
	goPath, _ := exec.LookPath("go")
	out, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		t.Fatal(err)
	}
	first := strings.SplitN(os.Getenv("PATH"), string(os.PathListSeparator), 2)[0]
	goroot, set := os.LookupEnv("GOROOT")
	t.Logf("TOOLCHAIN goroot_env=%q goroot_set=%t first_path=%q lookpath=%q go_env=%q runtime=%q",
		goroot, set, first, goPath, strings.TrimSpace(string(out)), runtime.GOROOT())
}
`

var toolchainLine = regexp.MustCompile(`TOOLCHAIN goroot_env="(.*)" goroot_set=(\w+) first_path="(.*)" lookpath="(.*)" go_env="(.*)" runtime="(.*)"`)

type toolchainView struct{ gorootEnv, gorootSet, firstPath, lookPath, goEnv, runtime string }

func parseToolchainView(t *testing.T, output string) toolchainView {
	t.Helper()
	m := toolchainLine.FindStringSubmatch(output)
	if m == nil {
		t.Fatalf("no toolchain line:\n%s", output)
	}
	return toolchainView{m[1], m[2], m[3], m[4], m[5], m[6]}
}

// isolatedUserCache points os.UserCacheDir at cache on every platform, and
// keeps the build cache warm.
func isolatedUserCache(t *testing.T, cache string) []string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOCACHE").Output()
	if err != nil {
		t.Fatal(err)
	}
	return []string{"XDG_CACHE_HOME=" + cache, "HOME=" + cache, "LOCALAPPDATA=" + cache, "GOCACHE=" + strings.TrimSpace(string(out))}
}

// cloneToolchain lays out the running toolchain as a downloaded one in a
// module cache: hard links where possible, copies otherwise, and read-only
// directories as Go leaves them.
func cloneToolchain(t *testing.T, modcache string) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOROOT", "GOVERSION").Output()
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(string(out))
	source := fields[0]
	// Released versions are path-safe; a distribution's, such as
	// go1.27.0-X:nodwarf5, would split PATH at its colon.
	version := strings.Map(func(r rune) rune {
		if r == ':' || r == ';' || r == filepath.Separator {
			return '_'
		}
		return r
	}, fields[1])
	target := filepath.Join(modcache, "golang.org", "toolchain@v0.0.1-"+version+"."+runtime.GOOS+"-"+runtime.GOARCH)
	link := runtime.GOOS != "windows"
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(source, path)
		dest := filepath.Join(target, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case info.IsDir():
			return os.MkdirAll(dest, 0755)
		case info.Mode()&fs.ModeSymlink != 0:
			to, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(to, dest)
		}
		if link && os.Link(path, dest) == nil {
			return nil
		}
		link = false
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		outFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
		if err != nil {
			return err
		}
		if _, err := io.Copy(outFile, in); err != nil {
			outFile.Close()
			return err
		}
		return outFile.Close()
	})
	if err != nil {
		t.Fatal(err)
	}
	setDirectoriesWritable(modcache, false)
	t.Cleanup(func() { setDirectoriesWritable(modcache, true) })
	return target
}

func setDirectoriesWritable(root string, writable bool) {
	mode := fs.FileMode(0555)
	if writable {
		mode = 0755
	}
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && entry.IsDir() {
			_ = os.Chmod(path, mode)
		}
		return nil
	})
}

// treeModes records every file's mode below root, which the mirror must not
// change: hard links share it with the module cache.
func treeModes(t *testing.T, root string) map[string]fs.FileMode {
	t.Helper()
	modes := map[string]fs.FileMode{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		info, err := os.Lstat(path)
		if err == nil {
			modes[path] = info.Mode()
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return modes
}

func sameModes(a, b map[string]fs.FileMode) bool {
	if len(a) != len(b) {
		return false
	}
	for path, mode := range a {
		if b[path] != mode {
			return false
		}
	}
	return true
}

func gorootMirrorFixtureDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeBuildFixture(t, dir, map[string]string{
		"go.mod":         "module example.com/mirror\n\ngo 1.25.0\n",
		"mirror_test.go": gorootMirrorFixture,
	})
	return dir
}

// runMirroredToolchain runs native go test, then ddto test twice, and checks
// that ddto mirrors the toolchain once, reports tests, keeps the result cache
// and shows tests the toolchain view of native go test, except
// runtime.GOROOT, which names the mirror.
// goCommand is the caller's go: os/exec resolves a bare name with this
// process's PATH, not env's.
func runMirroredToolchain(t *testing.T, dir string, env []string, goCommand, cache, modcache string) {
	t.Helper()
	before := treeModes(t, modcache)
	out, stderr, code := command(t, dir, env, goCommand, "test", "-count=1", "-v", "./...")
	if code != 0 {
		t.Fatalf("native go test: %s%s", out, stderr)
	}
	native := parseToolchainView(t, out)
	driver := sharedDriver(t, "..")
	receiver := &parityReceiver{side: map[string][][]byte{}, requests: map[string]int{}}
	server := newServer(http.HandlerFunc(receiver.handler))
	defer server.Close()
	ciEnv := append(append([]string(nil), env...), "DD_TRACE_DEBUG=true", "DD_CIVISIBILITY_ENABLED=true", "DD_CIVISIBILITY_AGENTLESS_ENABLED=false", "DD_TRACE_AGENT_URL="+server.URL)
	for i, want := range []string{"state=created", "state=reused"} {
		out, stderr, code := command(t, dir, ciEnv, driver, "test", "-count=1", "-v", "./...")
		if code != 0 || !strings.Contains(stderr, "goroot mirror "+want) {
			t.Fatalf("run %d: exit %d, want %s:\n%s%s", i, code, want, out, stderr)
		}
		got := parseToolchainView(t, out)
		mirrors := filepath.Join(cache, "ddto", "goroots")
		if !strings.HasPrefix(got.runtime, mirrors) {
			t.Errorf("runtime.GOROOT %q is not a mirror in %s", got.runtime, mirrors)
		}
		got.runtime, native.runtime = "", ""
		if got != native {
			t.Errorf("tests see %+v; natively %+v", got, native)
		}
	}
	receiver.sideMu.Lock()
	events := 0
	for path, n := range receiver.requests {
		if strings.HasSuffix(path, "/citestcycle") {
			events += n
		}
	}
	receiver.sideMu.Unlock()
	if events == 0 {
		t.Fatal("no test events through the mirrored toolchain")
	}
	// The saved native environment keeps Go's result cache valid.
	for i := 0; i < 2; i++ {
		out, stderr, code = command(t, dir, ciEnv, driver, "test", "./...")
		if code != 0 {
			t.Fatalf("cached run %d: %s%s", i, out, stderr)
		}
	}
	if !strings.Contains(out, "(cached)") {
		t.Fatalf("result not cached:\n%s", out)
	}
	if after := treeModes(t, modcache); !sameModes(before, after) {
		t.Fatal("the module cache's files changed")
	}
}

// A toolchain whose go runs directly from the module cache.
func TestMiniGOROOTMirrorDirectToolchain(t *testing.T) {
	modcache := filepath.Join(t.TempDir(), "mod")
	goroot := cloneToolchain(t, modcache)
	cache := t.TempDir()
	env := testEnv(append(isolatedUserCache(t, cache), "PATH="+filepath.Join(goroot, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"),
		"GOMODCACHE="+modcache, "GOTOOLCHAIN=local", "GOPROXY=off")...)
	runMirroredToolchain(t, gorootMirrorFixtureDir(t), env, filepath.Join(goroot, "bin", executableName("go")), cache, modcache)
}

// Without a usable mirror location, ddto warns and runs go test natively,
// with its exit status.
func TestMiniGOROOTMirrorUnavailableRunsGoTest(t *testing.T) {
	modcache := filepath.Join(t.TempDir(), "mod")
	goroot := cloneToolchain(t, modcache)
	// A user cache inside GOMODCACHE: the mirror would write there.
	cache := filepath.Join(modcache, "cache")
	env := testEnv(append(isolatedUserCache(t, cache), "PATH="+filepath.Join(goroot, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"),
		"GOMODCACHE="+modcache, "GOTOOLCHAIN=local", "GOPROXY=off")...)
	dir := gorootMirrorFixtureDir(t)
	driver := sharedDriver(t, "..")
	for _, fail := range []bool{false, true} {
		if fail {
			writeBuildFixture(t, dir, map[string]string{"fail_test.go": "package mirror\nimport \"testing\"\nfunc TestFails(t *testing.T) { t.Fatal(\"fails\") }\n"})
		}
		_, _, native := command(t, dir, env, filepath.Join(goroot, "bin", executableName("go")), "test", "-count=1", "./...")
		out, stderr, code := command(t, dir, env, driver, "test", "-count=1", "./...")
		if code != native || !strings.Contains(stderr, "running go test without CI Visibility") {
			t.Fatalf("fail=%t: exit %d, native %d:\n%s%s", fail, code, native, out, stderr)
		}
	}
}
