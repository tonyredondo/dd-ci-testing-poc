package instrument

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeclaresParallelStop(t *testing.T) {
	for _, tc := range []struct {
		src  string
		want bool
	}{
		{"package testing\nimport \"sync/atomic\"\nvar (\n\tparallelStart atomic.Int64\n\tparallelStop  atomic.Int64 // stopped\n)\n", true},
		{"package testing\nvar parallelStop int64\n", false},
		{"package testing\n// parallelStop atomic.Int64\n", false},
		{"package testing\n", false},
	} {
		file, err := parser.ParseFile(token.NewFileSet(), "testing.go", tc.src, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		if got := declaresParallelStop(file); got != tc.want {
			t.Errorf("%q: got %v, want %v", tc.src, got, tc.want)
		}
	}
}

// Supported toolchains keep the counter. If one stops, Mini builds without the
// hook and its in-process retries of parallel tests break AllocsPerRun again.
func TestToolchainTestingDeclaresParallelStop(t *testing.T) {
	out, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(strings.TrimSpace(string(out)), "src", "testing")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, entry := range entries {
		if name := entry.Name(); strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			if files[name], err = os.ReadFile(filepath.Join(dir, name)); err != nil {
				t.Fatal(err)
			}
		}
	}
	rewritten, err := Transform(files)
	if err != nil {
		t.Fatal(err)
	}
	if !rewritten.ParallelStop {
		t.Fatal("testing no longer declares parallelStop as an atomic.Int64; review ParallelStopHook")
	}
}

func TestSaveEnvironmentDistinguishesEmptyAndUnset(t *testing.T) {
	t.Setenv("GOFLAGS", "")
	if got := SaveEnvironment("GOFLAGS"); got != SavedEnvironmentPrefix+"GOFLAGS==" {
		t.Fatalf("empty value: %q", got)
	}
	os.Unsetenv("GOFLAGS")
	if got := SaveEnvironment("GOFLAGS"); got != SavedEnvironmentPrefix+"GOFLAGS=" {
		t.Fatalf("unset value: %q", got)
	}
}

// Runs the hook as it initializes testing: saved values replace ddtest's
// build-only settings, and an unset original is removed again.
func TestEnvironmentHookRestoresSavedValues(t *testing.T) {
	if !strings.Contains(Hooks, environmentHook) {
		t.Fatal("testing hooks do not restore the environment")
	}
	dir := t.TempDir()
	program := "package main\nimport (\n \"fmt\"\n __dd_ci_os \"os\"\n)\n" + environmentHook +
		"func main() {\n for _, name := range []string{\"GOWORK\", \"GOFLAGS\", \"" + SavedEnvironmentPrefix + "GOWORK\", \"" + SavedEnvironmentPrefix + "GOFLAGS\"} {\n  value, ok := __dd_ci_os.LookupEnv(name)\n  fmt.Printf(\"%s=%t:%q\\n\", name, ok, value)\n }\n}\n"
	for name, data := range map[string]string{"go.mod": "module example.com/restore\ngo 1.25\n", "main.go": program} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	binary := filepath.Join(dir, "restore.exe")
	build := exec.Command("go", "build", "-buildvcs=false", "-o", binary, ".")
	build.Dir = dir
	build.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	run := exec.Command(binary)
	run.Env = append(os.Environ(), "GOWORK=/tmp/ddtest/go.work", "GOFLAGS='-mod=readonly'",
		SavedEnvironmentPrefix+"GOWORK=", SavedEnvironmentPrefix+"GOFLAGS==-tags=a b")
	out, err := run.Output()
	if err != nil {
		t.Fatal(err)
	}
	want := "GOWORK=false:\"\"\nGOFLAGS=true:\"-tags=a b\"\n" + SavedEnvironmentPrefix + "GOWORK=false:\"\"\n" + SavedEnvironmentPrefix + "GOFLAGS=false:\"\"\n"
	if string(out) != want {
		t.Fatalf("restored environment:\n%s\nwant:\n%s", out, want)
	}
}
