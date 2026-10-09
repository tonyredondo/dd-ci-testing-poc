package integration

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
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

// Tools that ddtest chains belong to the build: unlike test processes, a user
// -toolexec must keep the temporary workspace that makes Mini resolvable.
func TestMiniTemporaryWorkspaceKeepsBuildToolEnvironment(t *testing.T) {
	driver := sharedDriver(t, "..")
	root := t.TempDir()
	wrapper, client := filepath.Join(root, "wrapper"), filepath.Join(root, "client")
	for _, dir := range []string{wrapper, client} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	writeBuildFixture(t, wrapper, map[string]string{
		"go.mod": "module example.com/toolwrapper\ngo 1.21\n",
		"main.go": `package main
import ("fmt";"os";"os/exec";"path/filepath";"strings")
func main() {
 tool := os.Args[1]
 if strings.TrimSuffix(filepath.Base(tool), ".exe") == "compile" && strings.HasPrefix(os.Getenv("TOOLEXEC_IMPORTPATH"), "example.com/toolclient") {
  out, err := exec.Command("go", "list", "github.com/tonyredondo/dd-ci-testing-poc/testopt").CombinedOutput()
  if err != nil { fmt.Fprintf(os.Stderr, "wrapper go list failed: %v\n%s", err, out); os.Exit(1) }
  if err := os.WriteFile(os.Getenv("TOOL_MARKER"), out, 0600); err != nil { panic(err) }
 }
 cmd := exec.Command(tool, os.Args[2:]...)
 cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
 if err := cmd.Run(); err != nil {
  if exit, ok := err.(*exec.ExitError); ok { os.Exit(exit.ExitCode()) }
  panic(err)
 }
}
`,
	})
	writeBuildFixture(t, client, map[string]string{
		"go.mod":         "module example.com/toolclient\ngo 1.21\n",
		"client_test.go": "package toolclient\nimport \"testing\"\nfunc TestTool(t *testing.T){}\n",
	})
	env := testEnv("GOWORK=off", "GOPROXY=off", "DD_CIVISIBILITY_ENABLED=false")
	binary := filepath.Join(root, executableName("toolwrapper"))
	if out, stderr, code := command(t, wrapper, env, "go", "build", "-o", binary, "."); code != 0 {
		t.Fatal(out, stderr)
	}
	marker := filepath.Join(root, "marker")
	// Covering testing makes ddtest's selective tool chain the user's wrapper.
	args := []string{"test", "-count=1", "-coverpkg=testing", "-toolexec=" + quoteToolArgument(t, binary), "."}
	out, stderr, code := command(t, client, append(env, "TOOL_MARKER="+marker), driver, args...)
	if code != 0 {
		t.Fatalf("exit=%d\n%s%s", code, out, stderr)
	}
	if data, err := os.ReadFile(marker); err != nil || !strings.Contains(string(data), "testopt") {
		t.Fatalf("wrapper did not resolve Mini through the build workspace: %q %v", data, err)
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

// ddtest must use the vendored sources that go test uses: a go mod vendor tree
// beside go.work belongs to its module and is ignored, while relative local
// replacements keep working when the manifest moves to a temporary workspace.
func TestMiniVendorSelectionMatchesGo(t *testing.T) {
	driver := sharedDriver(t, "..")
	for _, tc := range []struct {
		name, work, vendor string
		absolute           bool
	}{
		// Absolute replacements keep a misused vendor tree consistent, so the
		// wrong sources would run silently instead of failing.
		{"module-vendor-beside-go1.21-workspace", "go 1.21\nuse .\n", "module", true},
		{"module-vendor-beside-go1.25-workspace", "go 1.25.0\nuse .\n", "module", true},
		{"module-vendor-beside-workspace-relative-replacements", "go 1.21\nuse .\n", "module", false},
		{"module-vendor-with-relative-replacements", "", "module", false},
		{"workspace-vendor-with-relative-replacements", "go 1.22\nuse ./client\nreplace example.com/other => ./other\n", "workspace", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			client := filepath.Join(root, "client")
			for _, name := range []string{"helper", "other"} {
				dir := filepath.Join(root, name)
				if err := os.Mkdir(dir, 0700); err != nil {
					t.Fatal(err)
				}
				writeBuildFixture(t, dir, map[string]string{"go.mod": "module example.com/" + name + "\ngo 1.21\n", name + ".go": "package " + name + "\nconst Value=1\n"})
			}
			if err := os.Mkdir(client, 0700); err != nil {
				t.Fatal(err)
			}
			local := func(name string) string {
				if tc.absolute {
					return strconv.Quote(filepath.ToSlash(filepath.Join(root, name)))
				}
				return "../" + name
			}
			mod := "module example.com/vendorselection\ngo 1.21\nrequire (\nexample.com/helper v0.0.0\nexample.com/other v0.0.0\n)\nreplace example.com/helper => " + local("helper") + "\n"
			if tc.vendor == "module" {
				mod += "replace example.com/other => " + local("other") + "\n"
			}
			writeBuildFixture(t, client, map[string]string{
				"go.mod":         mod,
				"client_test.go": "package vendorselection\nimport(\"fmt\";\"testing\";\"example.com/helper\";\"example.com/other\")\nfunc TestValues(t *testing.T){fmt.Printf(\"VALUES=%d,%d\\n\",helper.Value,other.Value)}\n",
			})
			env := append(testEnv("GOPROXY=off", "DD_CIVISIBILITY_ENABLED=false"), userCacheEnv(t, t.TempDir())...)
			vendor := filepath.Join(client, "vendor")
			if tc.vendor == "module" {
				if out, stderr, code := command(t, client, append(env, "GOWORK=off"), "go", "mod", "vendor"); code != 0 {
					t.Fatal(out, stderr)
				}
			}
			switch {
			case tc.vendor == "module" && tc.work != "":
				// go.work beside the module's own vendor directory.
				writeBuildFixture(t, client, map[string]string{"go.work": tc.work})
			case tc.work != "":
				writeBuildFixture(t, root, map[string]string{"go.work": tc.work})
			}
			if tc.vendor == "workspace" {
				if out, stderr, code := command(t, root, env, "go", "work", "vendor"); code != 0 {
					t.Fatal(out, stderr)
				}
				vendor = filepath.Join(root, "vendor")
			}
			// Patched vendored copies make the selected source observable.
			for _, name := range []string{"helper", "other"} {
				path := filepath.Join(vendor, "example.com", name, name+".go")
				if err := os.WriteFile(path, []byte("package "+name+"\nconst Value=7\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if tc.work == "" {
				env = append(env, "GOWORK=off")
			}
			values := func(out string) string {
				for _, line := range strings.Split(out, "\n") {
					if strings.HasPrefix(line, "VALUES=") {
						return strings.TrimSpace(line)
					}
				}
				return ""
			}
			native, nativeErr, nativeCode := command(t, client, env, "go", "test", "-v", "-count=1", ".")
			if nativeCode != 0 {
				t.Fatalf("native: %s%s", native, nativeErr)
			}
			out, stderr, code := command(t, client, env, driver, "test", "-v", "-count=1", ".")
			if code != 0 {
				t.Fatalf("exit=%d\n%s%s", code, out, stderr)
			}
			if want, got := values(native), values(out); want == "" || got != want {
				t.Fatalf("selected sources differ: native=%q ddtest=%q", want, got)
			}
		})
	}
}
