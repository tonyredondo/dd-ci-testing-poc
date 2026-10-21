package runner

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// gotestsum's go is ddto: ddto leaves the shim's directory off PATH, and the
// go shim runs as ddto go.
func TestGoShimArgs(t *testing.T) {
	sep := string(os.PathListSeparator)
	shim := filepath.Join(t.TempDir(), "go-shim")
	for _, tc := range []struct {
		name string
		args []string
		shim bool
		want []string
		atGo bool
	}{
		{name: "ddto", args: []string{"/bin/ddto", "test", "./..."}, want: []string{"test", "./..."}},
		{name: "go shim", args: []string{"go", "test", "-json", "./..."}, shim: true, want: []string{"go", "test", "-json", "./..."}, atGo: true},
		{name: "ddto under gotestsum", args: []string{"/bin/ddto", "test", "-json"}, shim: true, want: []string{"test", "-json"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PATH", "/usr/local/go/bin"+sep+"/usr/bin")
			goShimLeft = ""
			if tc.shim {
				t.Setenv(goShimVariable, shim)
				t.Setenv("PATH", shim+sep+"/usr/local/go/bin"+sep+shim+sep+"/usr/bin")
			}
			if got := GoShimArgs(tc.args); !slices.Equal(got, tc.want) {
				t.Fatalf("GoShimArgs(%q) = %q; want %q", tc.args, got, tc.want)
			}
			if _, set := os.LookupEnv(goShimVariable); set {
				t.Fatalf("%s stays set", goShimVariable)
			}
			if path := os.Getenv("PATH"); path != "/usr/local/go/bin"+sep+"/usr/bin" {
				t.Fatalf("PATH=%q", path)
			}
			if tc.shim != (goShimLeft == shim) {
				t.Fatalf("left %q", goShimLeft)
			}
		})
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
	t.Setenv(goShimVariable, "")
	t.Setenv("PATH", "/usr/bin")
	if err := UseGoShim(); err != nil {
		t.Fatal(err)
	}
	dir := os.Getenv(goShimVariable)
	if !strings.HasPrefix(dir, filepath.Join(cache, "ddto", "go-shim")) || os.Getenv("PATH") != dir+string(os.PathListSeparator)+"/usr/bin" {
		t.Fatalf("%s=%q PATH=%q", goShimVariable, dir, os.Getenv("PATH"))
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
