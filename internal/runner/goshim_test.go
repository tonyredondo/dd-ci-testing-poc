package runner

import (
	"os"
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
	// filtered, is left too: its go is this executable.
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	elsewhere := t.TempDir()
	name := "go"
	if runtime.GOOS == "windows" {
		name = "go.exe"
	}
	if err := os.Link(self, filepath.Join(elsewhere, name)); err != nil && (runtime.GOOS == "windows" || os.Symlink(self, filepath.Join(elsewhere, name)) != nil) {
		t.Logf("no link to this executable: %v", err)
	} else {
		t.Setenv("PATH", elsewhere+sep+other)
		if _, fromShim := GoShimArgs([]string{"go", "version"}); !fromShim || os.Getenv("PATH") != other || goShimLeft != elsewhere {
			t.Fatalf("PATH=%q left %q", os.Getenv("PATH"), goShimLeft)
		}
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
	if err := UseGoShim(); err != nil {
		t.Fatal(err)
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
