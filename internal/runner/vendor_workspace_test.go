package runner

import (
	"context"
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

// moduleVendor describes a go mod vendor tree beside its module.
func moduleVendor(source string) vendorManifest {
	return vendorManifest{dir: source, base: filepath.Dir(source), module: true}
}

func requireSymlinks(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Symlink(dir, filepath.Join(dir, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
}

// useVendorWorkspace attaches source and holds any cached workspace until the
// test ends, as a run holds it until go test exits.
func useVendorWorkspace(t *testing.T, work, source string, replacements map[string]string) (string, error) {
	t.Helper()
	got, lock, err := vendorWorkspace(t.Context(), work, moduleVendor(source), replacements)
	if lock != nil {
		t.Cleanup(func() { lock.Close() })
	}
	return got, err
}

// assertLinkedVendor checks the fixture's layout: real directories, so cmd/go
// can traverse them in package patterns, and a linked source file.
func assertLinkedVendor(t *testing.T, vendor string) {
	t.Helper()
	if info, err := os.Lstat(filepath.Join(vendor, "example.com", "dep")); err != nil || !info.IsDir() {
		t.Fatalf("vendored directory was not recreated: %v %v", info, err)
	}
	if info, err := os.Lstat(filepath.Join(vendor, "example.com", "dep", "dep.go")); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("vendored source was not linked: %v %v", info, err)
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
	got, err := useVendorWorkspace(t, work, source, replacements)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, filepath.Join(cache, "ddtest", "vendor-workspaces")+string(filepath.Separator)) {
		t.Fatalf("workspace outside the user cache: %s", got)
	}
	vendor := filepath.Join(filepath.Dir(got), "vendor")
	assertLinkedVendor(t, vendor)
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
	if reused, err := useVendorWorkspace(t, again, source, map[string]string{}); err != nil || reused != got {
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
	first, err := useVendorWorkspace(t, work, source, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(source, "modules.txt")
	if err := os.WriteFile(manifest, []byte("# example.com/dep v1.0.1\n## explicit\nexample.com/dep\n"), 0600); err != nil {
		t.Fatal(err)
	}
	second, err := useVendorWorkspace(t, work, source, map[string]string{})
	if err != nil || second == first {
		t.Fatalf("manifest change reused %s: %s %v", first, second, err)
	}
	if err := os.WriteFile(work, []byte("go 1.25.0\nuse ./other\n"), 0600); err != nil {
		t.Fatal(err)
	}
	third, err := useVendorWorkspace(t, work, source, map[string]string{})
	if err != nil || third == second {
		t.Fatalf("workspace change reused %s: %s %v", second, third, err)
	}
	// A file added to the vendored tree needs its own link.
	added := filepath.Join(source, "example.com", "dep", "added.go")
	if err := os.WriteFile(added, []byte("package dep\n"), 0600); err != nil {
		t.Fatal(err)
	}
	fourth, err := useVendorWorkspace(t, work, source, map[string]string{})
	if err != nil || fourth == third {
		t.Fatalf("added vendored file reused %s: %s %v", third, fourth, err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(fourth), "vendor", "example.com", "dep", "added.go")); err != nil {
		t.Fatalf("added vendored file is missing: %v", err)
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
	cached, err := useVendorWorkspace(t, work, source, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	extra := filepath.Join(filepath.Dir(cached), "vendor", "extra.example")
	if err := os.WriteFile(extra, nil, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := useVendorWorkspace(t, work, source, map[string]string{})
	if err != nil || got != work {
		t.Fatalf("unexpected cached contents were reused: %s %v", got, err)
	}
	assertLinkedVendor(t, filepath.Join(filepath.Dir(work), "vendor"))
	if _, err := os.Stat(extra); err != nil {
		t.Fatalf("existing cache entry was modified: %v", err)
	}
}

func TestVendorWorkspaceWithoutUserCacheUsesRunDirectory(t *testing.T) {
	requireSymlinks(t)
	withUserCache(t, func() (string, error) { return "", errors.New("no home directory") })
	source, work := vendorFixture(t)
	got, err := useVendorWorkspace(t, work, source, map[string]string{})
	if err != nil || got != work {
		t.Fatalf("run workspace: %s %v", got, err)
	}
	assertLinkedVendor(t, filepath.Join(filepath.Dir(work), "vendor"))
}

func requireFileLocks(t *testing.T) {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "lock")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := tryLockFile(file, false); errors.Is(err, errors.ErrUnsupported) {
		t.Skip("file locks unavailable")
	} else if err != nil {
		t.Fatal(err)
	}
}

func TestFileLocksConflictAcrossOpens(t *testing.T) {
	requireFileLocks(t)
	path := filepath.Join(t.TempDir(), "lock")
	open := func() *os.File {
		file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { file.Close() })
		return file
	}
	first, second, pruner := open(), open(), open()
	if err := tryLockFile(first, false); err != nil {
		t.Fatal(err)
	}
	if err := tryLockFile(second, false); err != nil {
		t.Fatalf("shared locks conflict: %v", err)
	}
	if err := tryLockFile(pruner, true); !errors.Is(err, errLockBusy) {
		t.Fatalf("exclusive lock while shared: %v", err)
	}
	first.Close()
	second.Close()
	if err := tryLockFile(pruner, true); err != nil {
		t.Fatalf("exclusive lock after release: %v", err)
	}
}

func TestPruneVendorWorkspacesKeepsRecentAndLinkedSources(t *testing.T) {
	requireSymlinks(t)
	requireFileLocks(t)
	parent := t.TempDir()
	source := t.TempDir()
	original := filepath.Join(source, "dep.go")
	if err := os.WriteFile(original, []byte("package dep\n"), 0600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * vendorWorkspaceRetention)
	for _, name := range []string{"old", "kept", "recent", "staging.tmp-1"} {
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
	// A lock left by a removed workspace is removed as well.
	orphan := filepath.Join(parent, "orphan.lock")
	if err := os.WriteFile(orphan, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(orphan, old, old); err != nil {
		t.Fatal(err)
	}
	pruneVendorWorkspaces(parent, "kept")
	for name, want := range map[string]bool{"old": false, "old.lock": false, "orphan.lock": false, "staging.tmp-1": false, "kept": true, "recent": true} {
		if _, err := os.Lstat(filepath.Join(parent, name)); (err == nil) != want {
			t.Fatalf("%s present=%t want %t", name, err == nil, want)
		}
	}
	if _, err := os.Stat(original); err != nil {
		t.Fatalf("pruning removed a linked source: %v", err)
	}
}

// A run holds its workspace until go test exits. However old it looks, another
// run's prune must leave it in place; once released, it can be removed.
func TestPruneSkipsVendorWorkspaceInUse(t *testing.T) {
	requireSymlinks(t)
	requireFileLocks(t)
	cache := t.TempDir()
	withUserCache(t, func() (string, error) { return cache, nil })
	source, work := vendorFixture(t)
	path, lock, err := vendorWorkspace(t.Context(), work, moduleVendor(source), map[string]string{})
	if err != nil || lock == nil {
		t.Fatalf("cached workspace: %s %v %v", path, lock, err)
	}
	defer lock.Close()
	root := filepath.Dir(path)
	old := time.Now().Add(-2 * vendorWorkspaceRetention)
	if err := os.Chtimes(root, old, old); err != nil {
		t.Fatal(err)
	}
	pruneVendorWorkspaces(filepath.Dir(root), "another")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("prune removed a workspace in use: %v", err)
	}
	lock.Close()
	pruneVendorWorkspaces(filepath.Dir(root), "another")
	for _, removed := range []string{root, root + ".lock"} {
		if _, err := os.Lstat(removed); !os.IsNotExist(err) {
			t.Fatalf("unused workspace remains: %s %v", removed, err)
		}
	}
}

// Preparation stops when its context ends, even while a pruner holds the lock.
func TestVendorWorkspaceLockWaitHonorsCancellation(t *testing.T) {
	requireSymlinks(t)
	requireFileLocks(t)
	cache := t.TempDir()
	withUserCache(t, func() (string, error) { return cache, nil })
	source, work := vendorFixture(t)
	path, lock, err := vendorWorkspace(t.Context(), work, moduleVendor(source), map[string]string{})
	if err != nil || lock == nil {
		t.Fatalf("cached workspace: %s %v %v", path, lock, err)
	}
	lock.Close()
	pruner, err := os.OpenFile(filepath.Dir(path)+".lock", os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer pruner.Close()
	if err := tryLockFile(pruner, true); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, held, err := vendorWorkspace(ctx, work, moduleVendor(source), map[string]string{})
		if held != nil {
			held.Close()
		}
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("canceled preparation returned %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("waiting for the workspace lock ignored cancellation")
	}
}

// A run that opened the lock while another run pruned the workspace must not
// report the removed directory: it rebuilds and holds the current lock.
func TestVendorWorkspaceReuseAfterConcurrentPruneRebuilds(t *testing.T) {
	requireSymlinks(t)
	requireFileLocks(t)
	cache := t.TempDir()
	withUserCache(t, func() (string, error) { return cache, nil })
	source, work := vendorFixture(t)
	path, lock, err := vendorWorkspace(t.Context(), work, moduleVendor(source), map[string]string{})
	if err != nil || lock == nil {
		t.Fatalf("cached workspace: %s %v %v", path, lock, err)
	}
	lock.Close()
	root := filepath.Dir(path)
	lockPath := root + ".lock"
	pruner, err := os.OpenFile(lockPath, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer pruner.Close()
	if err := tryLockFile(pruner, true); err != nil {
		t.Fatal(err)
	}
	opened := make(chan struct{}, 1)
	previous := vendorWorkspaceLockOpened
	vendorWorkspaceLockOpened = func() {
		select {
		case opened <- struct{}{}:
		default:
		}
	}
	t.Cleanup(func() { vendorWorkspaceLockOpened = previous })
	type result struct {
		path string
		lock *os.File
		err  error
	}
	done := make(chan result, 1)
	go func() {
		path, lock, err := vendorWorkspace(t.Context(), work, moduleVendor(source), map[string]string{})
		done <- result{path, lock, err}
	}()
	<-opened
	// The pruner removes the workspace and its lock while the run waits.
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(lockPath) // Windows keeps a file that the run has open.
	pruner.Close()
	var got result
	select {
	case got = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("run did not finish after the prune")
	}
	if got.err != nil || got.lock == nil {
		t.Fatalf("rebuilt workspace: %v %v", got.lock, got.err)
	}
	defer got.lock.Close()
	if _, err := os.Stat(got.path); err != nil {
		t.Fatalf("run reported a removed workspace: %v", err)
	}
	if !sameOpenFile(got.lock, lockPath) {
		t.Fatal("run holds a lock that no longer protects the workspace")
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

// goSelectsVendor mirrors cmd/go's setDefaultBuildMod for an existing vendor
// directory: the explicit flag, the go directive and the manifest's mode.
func TestGoSelectsVendorMirrorsCmdGo(t *testing.T) {
	moduleManifest := "# example.com/dep v1.0.0\n## explicit\nexample.com/dep\n"
	workspaceManifest := "## workspace\n" + moduleManifest
	for _, tc := range []struct {
		name, mod, goVersion, manifest string
		workspace, missing, want       bool
	}{
		{name: "module default", goVersion: "1.21", manifest: moduleManifest, want: true},
		{name: "module before 1.14", goVersion: "1.13", manifest: moduleManifest},
		{name: "module without go directive", manifest: moduleManifest},
		{name: "module with workspace manifest", goVersion: "1.21", manifest: workspaceManifest},
		{name: "module without manifest", goVersion: "1.21", want: true},
		{name: "workspace with module manifest", goVersion: "1.25", manifest: moduleManifest, workspace: true},
		{name: "workspace manifest", goVersion: "1.22", manifest: workspaceManifest, workspace: true, want: true},
		{name: "workspace without manifest", goVersion: "1.22", workspace: true},
		{name: "explicit vendor", mod: "vendor", manifest: moduleManifest, workspace: true, want: true},
		{name: "explicit readonly", mod: "readonly", goVersion: "1.21", manifest: moduleManifest},
		{name: "explicit mod", mod: "mod", goVersion: "1.21", manifest: moduleManifest},
		{name: "missing directory", mod: "vendor", missing: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "vendor")
			if !tc.missing {
				if err := os.Mkdir(dir, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if tc.manifest != "" {
				if err := os.WriteFile(filepath.Join(dir, "modules.txt"), []byte(tc.manifest), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if got := goSelectsVendor(tc.mod, tc.goVersion, dir, tc.workspace, nil); got != tc.want {
				t.Fatalf("selected=%t want %t", got, tc.want)
			}
		})
	}
}

// Go canonicalizes a module's relative replacement against the go.work
// directory and compares a go.work replacement verbatim; the temporary go.work
// makes the latter absolute.
func TestVendorManifestRewritesReplacementsForWorkspace(t *testing.T) {
	base := filepath.Join(t.TempDir(), "client")
	workspace := filepath.Join(t.TempDir(), "workspace")
	absolute := filepath.Join(t.TempDir(), "absolute")
	relative := func(path string) string {
		rel, err := filepath.Rel(workspace, filepath.Join(base, path))
		if err != nil {
			t.Fatal(err)
		}
		return toDirectoryPath(rel)
	}
	original := strings.Join([]string{
		"# example.com/helper v0.0.0 => ./helper",
		"## explicit; go 1.21",
		"example.com/helper",
		"# example.com/shared v1.0.0 => ../shared",
		"# example.com/fork v1.0.0 => example.com/forked v1.2.0",
		"# example.com/abs v0.0.0 => " + absolute,
		"# example.com/work => ./work",
		"# example.com/pinned v0.0.0 => ./pinned",
		"# example.com/pinned v0.0.1 => ./other",
		"# example.com/pinned => ./pinned",
		"# example.com/helper => ./helper",
		"",
	}, "\n")
	// go.work replaces every version of work, but only pinned v0.0.1.
	vendor := vendorManifest{base: base, module: true, workReplaced: map[moduleVersion]bool{
		{path: "example.com/work"}:                      true,
		{path: "example.com/pinned", version: "v0.0.1"}: true,
	}}
	want := strings.Join([]string{
		"## workspace",
		"# example.com/helper v0.0.0 => " + relative("helper"),
		"## explicit; go 1.21",
		"example.com/helper",
		"# example.com/shared v1.0.0 => " + relative(filepath.Join("..", "shared")),
		"# example.com/fork v1.0.0 => example.com/forked v1.2.0",
		"# example.com/abs v0.0.0 => " + absolute,
		"# example.com/work => " + filepath.Join(base, "work"),
		"# example.com/pinned v0.0.0 => " + relative("pinned"),
		"# example.com/pinned v0.0.1 => " + filepath.Join(base, "other"),
		"# example.com/pinned => " + relative("pinned"),
		"# example.com/helper => " + relative("helper"),
		"",
	}, "\n")
	if got := string(vendor.contents([]byte(original), workspace)); got != want {
		t.Fatalf("workspace manifest:\n%s\nwant:\n%s", got, want)
	}
	// A workspace manifest keeps its header; only paths move with the workspace.
	vendor.module = false
	if got := string(vendor.contents([]byte("## workspace\n"), workspace)); got != "## workspace\n" {
		t.Fatalf("workspace header changed: %q", got)
	}
}

// cmd/go splits modules.txt lines on whitespace, so a moved relative path that
// gains the project's spaces is unreadable. Such targets use whitespace-free
// links declared in the temporary go.work, which Go compares verbatim.
func TestVendorManifestAliasesTargetsWithWhitespace(t *testing.T) {
	base := filepath.Join(t.TempDir(), "my project", "client")
	original := strings.Join([]string{
		"# example.com/helper v0.0.0 => ../helper",
		"## explicit; go 1.21",
		"example.com/helper",
		"# example.com/plain v0.0.0 => ../../plain",
		"# example.com/helper => ../helper",
		"",
	}, "\n")
	vendor := vendorManifest{base: base, module: true}
	vendor.assignAliases([]byte(original))
	if len(vendor.aliasTargets) != 1 || vendor.aliasTargets[0] != filepath.Join(filepath.Dir(base), "helper") {
		t.Fatalf("alias targets: %q", vendor.aliasTargets)
	}
	workspace := t.TempDir()
	plain, err := filepath.Rel(workspace, filepath.Join(base, "..", "..", "plain"))
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"## workspace",
		"# example.com/helper v0.0.0 => ./replacements/0",
		"## explicit; go 1.21",
		"example.com/helper",
		"# example.com/plain v0.0.0 => " + toDirectoryPath(plain),
		"# example.com/helper => ./replacements/0",
		"",
	}, "\n")
	if got := string(vendor.contents([]byte(original), workspace)); got != want {
		t.Fatalf("workspace manifest:\n%s\nwant:\n%s", got, want)
	}
	work := filepath.Join(workspace, "go.work")
	if err := os.WriteFile(work, []byte("go 1.25.0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := vendor.declareAliases(t.Context(), work); err != nil {
		t.Fatal(err)
	}
	out, err := goTool(t.Context(), workspace, nil, "work", "edit", "-json", work)
	if err != nil {
		t.Fatal(err)
	}
	for _, replace := range []string{`"Path": "example.com/helper"`, `"Version": "v0.0.0"`, `"Path": "./replacements/0"`} {
		if !strings.Contains(out, replace) {
			t.Fatalf("go.work lacks %s:\n%s", replace, out)
		}
	}
	if strings.Count(out, `"Path": "./replacements/0"`) != 2 {
		t.Fatalf("go.work needs both the versioned and the all-version alias:\n%s", out)
	}
}
