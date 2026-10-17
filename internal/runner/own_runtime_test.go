package runner

import (
	"context"
	"errors"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/instrument"
)

// The CLI's own runtime sources pass every runtime check that ddto runs, on
// the targets that it builds for. ownSources skips these checks for them on
// that basis, so a change to Mini that fails one fails here first.
func TestMiniRuntimePassesEveryRuntimeCheck(t *testing.T) {
	checks := map[string]struct {
		dir   string
		check func(map[string][]byte) error
	}{
		"SignalHookRuntimeError":          {"integrations", instrument.SignalHookRuntimeError},
		"ExitHookRuntimeError":            {"integrations", instrument.ExitHookRuntimeError},
		"PanicHookRuntimeError":           {"integrations", instrument.PanicHookRuntimeError},
		"TimeoutHookRuntimeError":         {"integrations/gotesting", instrument.TimeoutHookRuntimeError},
		"SessionRuntimeError":             {"utils", instrument.SessionRuntimeError},
		"FileCoverageRuntimeError":        {"integrations/gotesting/coverage", instrument.FileCoverageRuntimeError},
		"FileCoverageContextRuntimeError": {"integrations/gotesting/coverage", instrument.FileCoverageContextRuntimeError},
		"CoverageHookRuntimeError":        {"integrations/gotesting/coverage", instrument.CoverageHookRuntimeError},
	}
	for name := range runtimeChecksUsed(t) {
		if _, ok := checks[name]; !ok {
			t.Errorf("the runner calls instrument.%s, which this test does not run on Mini's sources", name)
		}
	}
	mini := filepath.Join("..", "thirdparty", "dd-trace-go", "civisibility")
	for _, target := range []struct{ goos, goarch string }{{"linux", "amd64"}, {"linux", "386"}, {"darwin", "arm64"}, {"windows", "amd64"}} {
		context := build.Default
		context.GOOS, context.GOARCH = target.goos, target.goarch
		for name, check := range checks {
			pkg, err := context.ImportDir(filepath.Join(mini, filepath.FromSlash(check.dir)), 0)
			if err != nil {
				t.Fatal(err)
			}
			sources := map[string][]byte{}
			for _, file := range pkg.GoFiles {
				path := filepath.Join(pkg.Dir, file)
				if sources[path], err = os.ReadFile(path); err != nil {
					t.Fatal(err)
				}
			}
			if err := check.check(sources); err != nil {
				t.Errorf("%s/%s: %s: %v", target.goos, target.goarch, name, err)
			}
		}
	}
}

// runtimeChecksUsed returns the instrument functions named *RuntimeError that
// the runner's sources call.
func runtimeChecksUsed(t *testing.T) map[string]bool {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	used := map[string]bool{}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(parsed, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "instrument" && strings.HasSuffix(sel.Sel.Name, "RuntimeError") {
					used[sel.Sel.Name] = true
				}
			}
			return true
		})
	}
	if len(used) < 8 {
		t.Fatalf("found only %d runtime checks: %v", len(used), used)
	}
	return used
}

func ownTestPackage(dir, module string) goPackage {
	p := goPackage{Dir: dir, GoFiles: []string{"a.go"}}
	p.Module = &struct {
		Path, Version, Dir string
		Main               bool
		Replace            *struct{ Path, Dir, Version string }
	}{Path: module}
	return p
}

// Only the packages of Mini's module under a root that cliMiniRoot returned
// come from the CLI's own sources.
func TestMarkOwnRuntime(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "mini")
	ctx := withOwnRuntimeRoots(context.Background())
	noteOwnRuntimeRoot(ctx, "")
	noteOwnRuntimeRoot(ctx, root)
	listed := []goPackage{
		ownTestPackage(filepath.Join(root, "internal", "integrations"), miniModule),
		ownTestPackage(filepath.Join(base, "client-mini", "internal", "integrations"), miniModule),
		ownTestPackage(filepath.Join(base, "mini-other", "internal"), miniModule),
		ownTestPackage(filepath.Join(root, "testdata", "fixture"), "example.com/fixture"),
		{Dir: filepath.Join(root, "internal", "nomodule")},
	}
	if marked := markOwnRuntime(ctx, listed, ""); marked != 1 {
		t.Fatalf("marked %d packages, want 1", marked)
	}
	for i, want := range []bool{true, false, false, false, false} {
		if listed[i].ownRuntime != want {
			t.Errorf("package %s: ownRuntime=%t, want %t", listed[i].Dir, listed[i].ownRuntime, want)
		}
	}
	other := []goPackage{ownTestPackage(filepath.Join(root, "internal"), miniModule)}
	if markOwnRuntime(context.Background(), other, "") != 0 || other[0].ownRuntime {
		t.Fatal("a context without roots marked a package")
	}
}

// A client's replacement or requirement that selects the CLI's own checkout
// or exact version needs no provisioning to count as its own sources.
func TestMarkOwnRuntimeFindsTheCLISources(t *testing.T) {
	roots := cliMiniRoots("")
	if len(roots) == 0 {
		t.Fatal("no own sources for this checkout")
	}
	listed := []goPackage{ownTestPackage(filepath.Join(roots[0], "internal", "thirdparty"), miniModule)}
	if markOwnRuntime(context.Background(), listed, "") != 1 {
		t.Fatalf("the checkout's packages under %s were not marked", roots[0])
	}
	cache := t.TempDir()
	cached := filepath.Join(cache, miniModule+"@v9.9.9")
	listed = []goPackage{ownTestPackage(filepath.Join(cached, "internal"), miniModule)}
	if markOwnRuntime(context.Background(), listed, cache) != 0 {
		t.Fatal("a cached version that did not build the CLI was marked")
	}
}

// cliMiniRoot records the root that it returns, such as a release's
// extracted sources, which cliMiniRoots does not find.
func TestCLIMiniRootRecordsItsRoot(t *testing.T) {
	ctx := withOwnRuntimeRoots(context.Background())
	source, err := filepath.Abs("provide.go")
	if err != nil {
		t.Fatal(err)
	}
	root := cliMiniRoot(ctx, source, "", "")
	if root == "" {
		t.Fatal("no root for this checkout")
	}
	noted := ctx.Value(ownRuntimeKey{}).(*ownRuntimeRoots).roots
	if len(noted) != 1 || noted[0] != filepath.Clean(root) {
		t.Fatalf("noted %v, want %s", noted, root)
	}
}

// A check runs unless the package is the CLI's own and no overlay replaces
// one of its files.
func TestCheckSourcesSkipsOnlyUnreplacedOwnPackages(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0600); err != nil {
		t.Fatal(err)
	}
	refuse := func(map[string][]byte) error { return errors.New("refused") }
	p := ownTestPackage(dir, miniModule)
	if got := checkSources(&p, nil, refuse); got != "refused" {
		t.Fatalf("a package that is not the CLI's own skipped its check: %q", got)
	}
	p.ownRuntime = true
	if got := checkSources(&p, nil, refuse); got != "" {
		t.Fatalf("the CLI's own package was checked: %q", got)
	}
	replaced := map[string]string{filepath.Join(dir, "a.go"): filepath.Join(dir, "a.go")}
	if got := checkSources(&p, replaced, refuse); got != "refused" {
		t.Fatalf("an overlaid package skipped its check: %q", got)
	}
}
