package integration

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// userCacheEnv points os.UserCacheDir at dir for ddtest's persistent vendor
// workspaces. Go's own build cache and module path stay where they were.
func userCacheEnv(t *testing.T, dir string) []string {
	t.Helper()
	out, stderr, code := command(t, ".", testEnv(), "go", "env", "GOCACHE", "GOMODCACHE", "GOPATH")
	values := strings.Split(strings.TrimSpace(out), "\n")
	if code != 0 || len(values) != 3 {
		t.Fatalf("go env: %s%s", out, stderr)
	}
	env := []string{"GOCACHE=" + strings.TrimSpace(values[0]), "GOMODCACHE=" + strings.TrimSpace(values[1]), "GOPATH=" + strings.TrimSpace(values[2])}
	switch runtime.GOOS {
	case "windows":
		return append(env, "LocalAppData="+dir)
	case "darwin", "ios":
		return append(env, "HOME="+dir)
	case "plan9":
		return append(env, "home="+dir)
	}
	return append(env, "XDG_CACHE_HOME="+dir)
}

// vendorWorkspaceDirs lists the persistent workspace directories ddtest stored
// below dir.
func vendorWorkspaceDirs(t *testing.T, dir string) []string {
	t.Helper()
	var workspaces []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == "vendor-workspaces" {
			children, err := os.ReadDir(path)
			for _, child := range children {
				if child.IsDir() { // Skip each workspace's lock file.
					workspaces = append(workspaces, filepath.Join(path, child.Name()))
				}
			}
			if err != nil {
				return err
			}
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return workspaces
}

func symlinksAvailable(t *testing.T) bool {
	t.Helper()
	dir := t.TempDir()
	return os.Symlink(dir, filepath.Join(dir, "link")) == nil
}

// A temporary workspace belongs to ddtest's build. Tests, dependencies that
// initialize before testing, and the go commands they start must see the
// caller's GOWORK and GOFLAGS, as with native go test.
func TestMiniTemporaryWorkspaceRestoresGoEnvironment(t *testing.T) {
	driver := sharedDriver(t, "..")
	for _, layout := range []string{"older-module", "vendor", "workspace"} {
		t.Run(layout, func(t *testing.T) {
			root := t.TempDir()
			client := filepath.Join(root, "client")
			helper := filepath.Join(root, "helper")
			nested := filepath.Join(client, "testdata", "nested")
			for _, dir := range []string{nested, helper} {
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
			}
			// The helper does not import testing, so Go can initialize it first.
			writeBuildFixture(t, helper, map[string]string{"go.mod": "module example.com/environmenthelper\ngo 1.21\n", "helper.go": `package environmenthelper
import ("os/exec";"strings")
var Value, InitMain = 1, ""
func init() { out, _ := exec.Command("go", "list", "-m").Output(); InitMain = strings.TrimSpace(string(out)) }
`})
			writeBuildFixture(t, nested, map[string]string{"go.mod": "module example.com/nested\ngo 1.21\n", "main.go": "package main\nfunc main(){}\n"})
			writeBuildFixture(t, client, map[string]string{
				"go.mod": fmt.Sprintf("module example.com/environment\ngo 1.21\nrequire example.com/environmenthelper v0.0.0\nreplace example.com/environmenthelper => %q\n", filepath.ToSlash(helper)),
				"environment_test.go": `package environment
import ("fmt";"os";"os/exec";"strings";"testing";"example.com/environmenthelper")
func TestEnvironment(t *testing.T) {
 fmt.Printf("ENV INIT_MAIN=%q\n", environmenthelper.InitMain)
 for _, name := range []string{"GOWORK", "GOFLAGS", "DDTEST_ORIGINAL_GOWORK", "DDTEST_ORIGINAL_GOFLAGS"} {
  value, ok := os.LookupEnv(name)
  fmt.Printf("ENV %s=%t:%q\n", name, ok, value)
 }
 main, err := exec.Command("go", "list", "-m").CombinedOutput()
 fmt.Printf("ENV MAIN=%q %t\n", strings.TrimSpace(string(main)), err == nil)
 build := exec.Command("go", "build", "-o", os.DevNull, ".")
 build.Dir = "testdata/nested"
 fmt.Printf("ENV NESTED=%t\n", build.Run() == nil)
}
`,
			})
			env := testEnv("GOPROXY=off", "DD_CIVISIBILITY_ENABLED=false")
			env = append(env, userCacheEnv(t, t.TempDir())...)
			switch layout {
			case "vendor":
				if out, stderr, code := command(t, client, env, "go", "mod", "vendor"); code != 0 {
					t.Fatal(out, stderr)
				}
			case "workspace":
				writeBuildFixture(t, root, map[string]string{"go.work": "go 1.21\nuse ./client\n"})
			}
			lines := func(out string) []string {
				var selected []string
				for _, line := range strings.Split(out, "\n") {
					if strings.HasPrefix(line, "ENV ") {
						selected = append(selected, strings.TrimSpace(line))
					}
				}
				return selected
			}
			native, nativeErr, nativeCode := command(t, client, env, "go", "test", "-v", "-count=1", ".")
			if nativeCode != 0 {
				t.Fatalf("native: %s%s", native, nativeErr)
			}
			out, stderr, code := command(t, client, env, driver, "test", "-v", "-count=1", ".")
			if code != 0 {
				t.Fatalf("exit=%d\n%s%s", code, out, stderr)
			}
			if want, got := lines(native), lines(out); len(want) != 7 || !slices.Equal(got, want) {
				t.Fatalf("test environment changed:\nnative: %q\nddtest: %q", want, got)
			}
		})
	}
}

// Linked vendor workspaces live at a stable path, so Go's build cache reuses
// unchanged vendored packages. The links keep later vendor edits visible.
func TestMiniVendorWorkspaceLinksAndReusesBuildCache(t *testing.T) {
	driver := sharedDriver(t, "..")
	dir := t.TempDir()
	helper := t.TempDir()
	cache := t.TempDir()
	writeBuildFixture(t, helper, map[string]string{"go.mod": "module example.com/linkedhelper\ngo 1.21\n", "helper.go": "package linkedhelper\nconst Value=1\n"})
	writeBuildFixture(t, dir, map[string]string{
		"go.mod":         fmt.Sprintf("module example.com/linkedvendor\ngo 1.21\nrequire example.com/linkedhelper v0.0.0\nreplace example.com/linkedhelper => %q\n", filepath.ToSlash(helper)),
		"client_test.go": "package linkedvendor\nimport(\"fmt\";\"testing\";\"example.com/linkedhelper\")\nfunc TestLinked(t *testing.T){fmt.Printf(\"VALUE=%d\\n\",linkedhelper.Value)}\n",
	})
	env := append(testEnv("GOPROXY=off", "DD_CIVISIBILITY_ENABLED=false"), userCacheEnv(t, cache)...)
	if out, stderr, code := command(t, dir, env, "go", "mod", "vendor"); code != 0 {
		t.Fatal(out, stderr)
	}
	patched := filepath.Join(dir, "vendor", "example.com", "linkedhelper", "helper.go")
	links := symlinksAvailable(t)
	for i, value := range []int{7, 7, 8} {
		if i != 1 {
			if err := os.WriteFile(patched, []byte(fmt.Sprintf("package linkedhelper\nconst Value=%d\n", value)), 0600); err != nil {
				t.Fatal(err)
			}
		}
		out, stderr, code := command(t, dir, env, driver, "test", "-x", "-v", "-count=1", ".")
		if code != 0 {
			t.Fatalf("run %d exit=%d\n%s%s", i, code, out, stderr)
		}
		if !strings.Contains(out, fmt.Sprintf("VALUE=%d", value)) {
			t.Fatalf("run %d did not use the vendored source with value %d:\n%s", i, value, out)
		}
		var compiled bool
		for _, line := range compilerTraceLines(stderr) {
			compiled = compiled || strings.Contains(line, "-p example.com/linkedhelper")
		}
		if links && i == 1 && compiled {
			t.Fatalf("unchanged vendored package was recompiled:\n%s", stderr)
		}
	}
	if data, err := os.ReadFile(filepath.Join(dir, "vendor", "modules.txt")); err != nil || strings.Contains(string(data), "## workspace") {
		t.Fatalf("client vendor manifest changed: %q %v", data, err)
	}
	workspaces := vendorWorkspaceDirs(t, cache)
	if !links {
		return
	}
	if len(workspaces) != 1 {
		t.Fatalf("expected one reusable vendor workspace: %q", workspaces)
	}
	if info, err := os.Lstat(filepath.Join(workspaces[0], "vendor", "example.com")); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("vendor workspace copied sources instead of linking them: %v %v", info, err)
	}
}
