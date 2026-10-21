package runner

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// gotestsum's go is ddto: run as go, ddto runs as ddto go. Every ddto leaves
// the go shims off PATH, even when gotestsum's command filtered the
// environment, and keeps the other entries as written.
func TestGoShimArgs(t *testing.T) {
	cache := t.TempDir()
	defer func(previous func() (string, error)) { userCacheDir = previous }(userCacheDir)
	userCacheDir = func() (string, error) { return cache, nil }
	sep := string(os.PathListSeparator)
	shim := filepath.Join(cache, "ddto", "go-shim", "0123456789abcdef")
	other := "/usr/local/go/bin"
	if runtime.GOOS == "windows" {
		// A quoted entry may hold the separator.
		other = `"C:\Go;1.27\bin"`
	}
	for _, tc := range []struct {
		name, path string
		args       []string
		want       []string
		fromShim   bool
	}{
		{name: "ddto", path: other, args: []string{"/bin/ddto", "test", "./..."}, want: []string{"test", "./..."}},
		{name: "go shim", path: shim + sep + other + sep + shim, args: []string{"go", "test", "-json", "./..."}, want: []string{"go", "test", "-json", "./..."}, fromShim: true},
		{name: "ddto under gotestsum", path: shim + sep + other, args: []string{"/bin/ddto", "test", "-json"}, want: []string{"test", "-json"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PATH", tc.path)
			goShimLeft = ""
			got, fromShim := GoShimArgs(tc.args)
			if !slices.Equal(got, tc.want) || fromShim != tc.fromShim {
				t.Fatalf("GoShimArgs(%q) = %q, %t; want %q, %t", tc.args, got, fromShim, tc.want, tc.fromShim)
			}
			if path := os.Getenv("PATH"); path != other {
				t.Fatalf("PATH=%q; want %q", path, other)
			}
			if strings.Contains(tc.path, shim) != (goShimLeft == shim) {
				t.Fatalf("left %q", goShimLeft)
			}
		})
	}
	// A go shim in another user cache, as in an environment that a command
	// filtered, is left too, and look-alike entries are kept.
	elsewhere := filepath.Join(t.TempDir(), "ddto", "go-shim", "fedcba9876543210")
	keep := []string{filepath.Join(t.TempDir(), "ddto", "go-shim", "not-a-shim"), filepath.Join(t.TempDir(), "go-shim", "0123456789abcdef"), "ddto/go-shim/0123456789abcdef"}
	t.Setenv("PATH", strings.Join(append([]string{elsewhere}, keep...), sep))
	if _, fromShim := GoShimArgs([]string{"go", "version"}); !fromShim || os.Getenv("PATH") != strings.Join(keep, sep) || goShimLeft != elsewhere {
		t.Fatalf("PATH=%q left %q", os.Getenv("PATH"), goShimLeft)
	}

	if !IsGotestsum([]string{"/home/user/go/bin/gotestsum", "--"}) || IsGotestsum([]string{"gotestsum-wrapper"}) || IsGotestsum(nil) {
		t.Fatal("IsGotestsum")
	}
}

// The shim is one go per ddto build in ddto's user cache, which ddto runs.
func TestUseGoShim(t *testing.T) {
	cache := t.TempDir()
	defer func(previous func() (string, error)) { userCacheDir = previous }(userCacheDir)
	userCacheDir = func() (string, error) { return cache, nil }
	defer func() { goShimFirst = "" }()
	t.Setenv("PATH", "/usr/bin")
	if command, err := UseGoShim([]string{"gotestsum", "--"}); err != nil || !slices.Equal(command, []string{"gotestsum", "--"}) {
		t.Fatal(command, err)
	}
	dir := goShimFirst
	if !strings.HasPrefix(dir, filepath.Join(cache, "ddto", "go-shim")) || os.Getenv("PATH") != dir+string(os.PathListSeparator)+"/usr/bin" {
		t.Fatalf("shim %q PATH=%q", dir, os.Getenv("PATH"))
	}
	name := "go"
	if runtime.GOOS == "windows" {
		name = "go.exe"
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	shim, err := os.Stat(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	if executable, err := os.Stat(self); err != nil || shim.Size() != executable.Size() {
		t.Fatalf("the shim is not this executable: %v", err)
	}
	// A second command reuses it.
	if again, err := goShimDir(self); err != nil || again != dir {
		t.Fatalf("goShimDir again: %q %v", again, err)
	}
}

// A shim directory that holds the PATH separator would split on PATH, so
// ddto runs gotestsum without it, after a warning.
func TestUseGoShimRefusesSeparator(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows quotes the entry")
	}
	cache := filepath.Join(t.TempDir(), "cache"+string(os.PathListSeparator)+"ci")
	defer func(previous func() (string, error)) { userCacheDir = previous }(userCacheDir)
	userCacheDir = func() (string, error) { return cache, nil }
	t.Setenv("PATH", "/usr/bin")
	if _, err := UseGoShim([]string{"make", "test"}); err == nil || os.Getenv("PATH") != "/usr/bin" {
		t.Fatalf("UseGoShim: %v, PATH=%q", err, os.Getenv("PATH"))
	}
}

// --go-shim runs any command with the go shim; gotestsum gets it in any form.
func TestGoShimCommand(t *testing.T) {
	for _, tc := range []struct {
		args, want []string
		shim       bool
	}{
		{args: []string{"--go-shim", "make", "test"}, want: []string{"make", "test"}, shim: true},
		{args: []string{"--go-shim"}, want: []string{}, shim: true},
		{args: []string{"gotestsum", "--", "./..."}, want: []string{"gotestsum", "--", "./..."}, shim: true},
		{args: []string{"go", "tool", "gotestsum", "--format", "testname"}, shim: true},
		{args: []string{"go", "tool", "gotest.tools/gotestsum"}, shim: true},
		{args: []string{"go", "run", "gotest.tools/gotestsum@v1.13.0", "--", "./..."}, shim: true},
		{args: []string{"go", "run", "-mod=mod", "gotest.tools/gotestsum"}, shim: true},
		{args: []string{"go", "run", "./cmd/gotestsum"}},
		{args: []string{"go", "tool", "cover"}},
		{args: []string{"go", "build", "gotest.tools/gotestsum"}},
		{args: []string{"make", "test"}},
		{args: []string{"make", "--go-shim"}},
	} {
		want := tc.want
		if want == nil {
			want = tc.args
		}
		if got, shim := GoShimCommand(tc.args); !slices.Equal(got, want) || shim != tc.shim {
			t.Errorf("GoShimCommand(%q) = %q, %t; want %q, %t", tc.args, got, shim, want, tc.shim)
		}
	}
}

// A go command with the go shim runs the go that PATH names before the shim,
// which then stays first on PATH for the commands that go starts.
func TestUseGoShimRunsTheToolchain(t *testing.T) {
	cache := t.TempDir()
	defer func(previous func() (string, error)) { userCacheDir = previous }(userCacheDir)
	userCacheDir = func() (string, error) { return cache, nil }
	defer func() { goShimFirst, goShimGo = "", "" }()
	toolchain, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go on PATH")
	}
	command, err := UseGoShim([]string{"go", "version"})
	if err != nil || !slices.Equal(command, []string{toolchain, "version"}) {
		t.Fatalf("UseGoShim = %q, %v; want %s first", command, err, toolchain)
	}
	if found, err := exec.LookPath("go"); err != nil || filepath.Dir(found) != goShimFirst {
		t.Fatalf("go on PATH is %q, %v; want the shim in %s", found, err, goShimFirst)
	}
}

// go run and go tool put $GOROOT/bin before the shim on their program's
// PATH: go run runs its program through ddto, and ddto runs a tool's program
// itself.
func TestGoShimGoCommand(t *testing.T) {
	self := filepath.Join(t.TempDir(), "dd to")
	run, err := goShimGoCommand(self, "/go/bin/go", []string{"go", "run", "-mod=mod", "gotest.tools/gotestsum@v1.13.0", "--", "./..."})
	if want := []string{"/go/bin/go", "run", "-exec='" + self + "' 'go-shim-exec'", "-mod=mod", "gotest.tools/gotestsum@v1.13.0", "--", "./..."}; err != nil || !slices.Equal(run, want) {
		t.Fatalf("go run: %q, %v; want %q", run, err, want)
	}
	if _, err := goShimGoCommand(self, "/go/bin/go", []string{"go", "run", "-exec=wasmtime", "./cmd"}); err == nil {
		t.Fatal("go run with the caller's -exec")
	}
	// The caller's -exec after the package is the program's argument.
	if _, err := goShimGoCommand(self, "/go/bin/go", []string{"go", "run", "./cmd", "-exec"}); err != nil {
		t.Fatal(err)
	}
	if other, err := goShimGoCommand(self, "/go/bin/go", []string{"go", "generate", "./..."}); err != nil || !slices.Equal(other, []string{"/go/bin/go", "generate", "./..."}) {
		t.Fatalf("go generate: %q, %v", other, err)
	}

	// The fake go prints its arguments, which stand for the tool's path.
	t.Setenv(fakeToolEnv, "1")
	tool, err := goShimGoCommand(self, os.Args[0], []string{"go", "tool", "gotestsum", "--format", "testname"})
	if want := []string{"tool\n-n\ngotestsum", "--format", "testname"}; err != nil || !slices.Equal(tool, want) {
		t.Fatalf("go tool: %q, %v; want %q", tool, err, want)
	}
	t.Setenv(fakeToolExitEnv, "1")
	if _, err := goShimGoCommand(self, os.Args[0], []string{"go", "tool", "nope"}); err == nil {
		t.Fatal("an unknown tool")
	}
}

// go run's -exec program runs with the go shim first on PATH again.
func TestExecWithGoShim(t *testing.T) {
	cache := t.TempDir()
	defer func(previous func() (string, error)) { userCacheDir = previous }(userCacheDir)
	userCacheDir = func() (string, error) { return cache, nil }
	defer func() { goShimFirst = "" }()
	t.Setenv(fakeToolEnv, "1")
	t.Setenv(fakeToolExitEnv, "3")
	t.Setenv("PATH", "/usr/lib/go/bin")
	if runtime.GOOS != "windows" {
		t.Skip("ExecWithGoShim replaces this process on Unix")
	}
	if code := ExecWithGoShim([]string{os.Args[0], "arg"}); code != 3 || !strings.HasPrefix(os.Getenv("PATH"), goShimFirst) {
		t.Fatalf("exit %d, PATH=%q", code, os.Getenv("PATH"))
	}
}
