package runner

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// vendorFixture creates a caller's vendor tree and a temporary go.work, as
// preparation does before it attaches the vendor directory.
func vendorFixture(t *testing.T) (source, work string) {
	t.Helper()
	source = filepath.Join(t.TempDir(), "vendor")
	work = filepath.Join(t.TempDir(), "go.work")
	for path, data := range map[string]string{
		filepath.Join(source, "modules.txt"):                  "# example.com/dep v1.0.0\n## explicit\nexample.com/dep\n",
		filepath.Join(source, "example.com", "dep", "dep.go"): "package dep\nconst Value = 1\n",
		work: "go 1.25.0\nuse ./client\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return source, work
}

func requireSymlinks(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Symlink(dir, filepath.Join(dir, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
}

func withUserCache(t *testing.T, dir func() (string, error)) {
	t.Helper()
	previous := userCacheDir
	userCacheDir = dir
	t.Cleanup(func() { userCacheDir = previous })
}

func TestVendorWorkspaceLinksAndReusesCachedDirectory(t *testing.T) {
	requireSymlinks(t)
	cache := t.TempDir()
	withUserCache(t, func() (string, error) { return cache, nil })
	source, work := vendorFixture(t)
	backing := filepath.Join(t.TempDir(), "virtual.go")
	replacements := map[string]string{
		filepath.Join(source, "example.com", "dep", "virtual.go"): backing,
		filepath.Join(source, "example.com", "dep", "dep.go"):     "",
	}
	got, err := vendorWorkspace(t.Context(), work, source, replacements)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, filepath.Join(cache, "ddtest", "vendor-workspaces")+string(filepath.Separator)) {
		t.Fatalf("workspace outside the user cache: %s", got)
	}
	vendor := filepath.Join(filepath.Dir(got), "vendor")
	if info, err := os.Lstat(filepath.Join(vendor, "example.com")); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("vendored sources were not linked: %v %v", info, err)
	}
	if data, err := os.ReadFile(filepath.Join(vendor, "modules.txt")); err != nil || !strings.HasPrefix(string(data), "## workspace\n# example.com/dep") {
		t.Fatalf("workspace manifest: %q %v", data, err)
	}
	if replacements[filepath.Join(vendor, "example.com", "dep", "virtual.go")] != backing {
		t.Fatal("virtual overlay file was not retargeted")
	}
	if deleted, ok := replacements[filepath.Join(vendor, "example.com", "dep", "dep.go")]; !ok || deleted != "" {
		t.Fatal("overlay deletion was not retargeted")
	}
	// Links keep later edits visible without rebuilding the workspace.
	edited := "package dep\nconst Value = 2\n"
	if err := os.WriteFile(filepath.Join(source, "example.com", "dep", "dep.go"), []byte(edited), 0600); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(vendor, "example.com", "dep", "dep.go")); err != nil || string(data) != edited {
		t.Fatalf("linked source: %q %v", data, err)
	}
	// Another run with the same inputs has its own temporary go.work.
	again := filepath.Join(t.TempDir(), "go.work")
	data, err := os.ReadFile(work)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(again, data, 0600); err != nil {
		t.Fatal(err)
	}
	if reused, err := vendorWorkspace(t.Context(), again, source, map[string]string{}); err != nil || reused != got {
		t.Fatalf("unchanged inputs did not reuse %s: %s %v", got, reused, err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(again), "vendor")); !os.IsNotExist(err) {
		t.Fatalf("cached workspace also created a run vendor directory: %v", err)
	}
}

func TestVendorWorkspaceKeyFollowsManifestAndWorkspace(t *testing.T) {
	requireSymlinks(t)
	cache := t.TempDir()
	withUserCache(t, func() (string, error) { return cache, nil })
	source, work := vendorFixture(t)
	first, err := vendorWorkspace(t.Context(), work, source, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(source, "modules.txt")
	if err := os.WriteFile(manifest, []byte("# example.com/dep v1.0.1\n## explicit\nexample.com/dep\n"), 0600); err != nil {
		t.Fatal(err)
	}
	second, err := vendorWorkspace(t.Context(), work, source, map[string]string{})
	if err != nil || second == first {
		t.Fatalf("manifest change reused %s: %s %v", first, second, err)
	}
	if err := os.WriteFile(work, []byte("go 1.25.0\nuse ./other\n"), 0600); err != nil {
		t.Fatal(err)
	}
	third, err := vendorWorkspace(t.Context(), work, source, map[string]string{})
	if err != nil || third == second {
		t.Fatalf("workspace change reused %s: %s %v", second, third, err)
	}
	if data, err := os.ReadFile(filepath.Join(filepath.Dir(first), "vendor", "modules.txt")); err != nil || !strings.Contains(string(data), "v1.0.0") {
		t.Fatalf("existing workspace changed: %q %v", data, err)
	}
}

func TestVendorWorkspaceNeverReusesUnexpectedContents(t *testing.T) {
	requireSymlinks(t)
	cache := t.TempDir()
	withUserCache(t, func() (string, error) { return cache, nil })
	source, work := vendorFixture(t)
	cached, err := vendorWorkspace(t.Context(), work, source, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	extra := filepath.Join(filepath.Dir(cached), "vendor", "extra.example")
	if err := os.WriteFile(extra, nil, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := vendorWorkspace(t.Context(), work, source, map[string]string{})
	if err != nil || got != work {
		t.Fatalf("unexpected cached contents were reused: %s %v", got, err)
	}
	if info, err := os.Lstat(filepath.Join(filepath.Dir(work), "vendor", "example.com")); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("run workspace was not linked: %v %v", info, err)
	}
	if _, err := os.Stat(extra); err != nil {
		t.Fatalf("existing cache entry was modified: %v", err)
	}
}

func TestVendorWorkspaceWithoutUserCacheUsesRunDirectory(t *testing.T) {
	requireSymlinks(t)
	withUserCache(t, func() (string, error) { return "", errors.New("no home directory") })
	source, work := vendorFixture(t)
	got, err := vendorWorkspace(t.Context(), work, source, map[string]string{})
	if err != nil || got != work {
		t.Fatalf("run workspace: %s %v", got, err)
	}
	if info, err := os.Lstat(filepath.Join(filepath.Dir(work), "vendor", "example.com")); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("run workspace was not linked: %v %v", info, err)
	}
}

func TestPruneVendorWorkspacesKeepsRecentAndLinkedSources(t *testing.T) {
	requireSymlinks(t)
	parent := t.TempDir()
	source := t.TempDir()
	original := filepath.Join(source, "dep.go")
	if err := os.WriteFile(original, []byte("package dep\n"), 0600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * vendorWorkspaceRetention)
	for _, name := range []string{"old", "kept", "recent"} {
		dir := filepath.Join(parent, name)
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(source, filepath.Join(dir, "linked")); err != nil {
			t.Fatal(err)
		}
		if name != "recent" {
			if err := os.Chtimes(dir, old, old); err != nil {
				t.Fatal(err)
			}
		}
	}
	pruneVendorWorkspaces(parent, "kept")
	for name, want := range map[string]bool{"old": false, "kept": true, "recent": true} {
		if _, err := os.Lstat(filepath.Join(parent, name)); (err == nil) != want {
			t.Fatalf("%s present=%t want %t", name, err == nil, want)
		}
	}
	if _, err := os.Stat(original); err != nil {
		t.Fatalf("pruning removed a linked source: %v", err)
	}
}

func TestModuleWorkspaceInputWithoutGoDirectiveKeepsGo116Defaults(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/nogo\n"), 0600); err != nil {
		t.Fatal(err)
	}
	input, err := moduleWorkspaceInput(t.Context(), root, root, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "go 1.25.0\n") || !strings.Contains(string(data), "godebug default=go1.16\n") {
		t.Fatalf("workspace changes defaults of a module without a go directive: %s", data)
	}
}

func TestUseModuleWorkspaceFlagsRemovesEveryModfileSpelling(t *testing.T) {
	t.Setenv("GOFLAGS", "-modfile=env.mod -tags=fixture")
	opts := options{buildFlags: []string{"-modfile", "alt.mod", "--modfile=other.mod", "-race"}}
	var plan Plan
	if err := useModuleWorkspaceFlags(&opts, &plan); err != nil {
		t.Fatal(err)
	}
	if strings.Join(opts.buildFlags, " ") != "-race" {
		t.Fatalf("build flags: %q", opts.buildFlags)
	}
	if plan.workspaceGoFlags != "'-tags=fixture'" || !plan.moduleWorkspace || opts.workspaceGoFlags == nil || *opts.workspaceGoFlags != plan.workspaceGoFlags {
		t.Fatalf("GOFLAGS for the workspace: %q %+v", plan.workspaceGoFlags, opts.workspaceGoFlags)
	}
}
