package runner

import (
	"context"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
)

// ownRuntimeKey holds, for one invocation, the roots of the CLI's own runtime
// sources that cliMiniRoot returned: its checkout, the sources that a release
// build carries, or its exact version in GOMODCACHE. Only provisioning
// extracts a release's sources, so only its root is noted here; the others
// are found again by cliMiniRoots.
type ownRuntimeKey struct{}

type ownRuntimeRoots struct {
	mu    sync.Mutex
	roots []string
}

func withOwnRuntimeRoots(ctx context.Context) context.Context {
	return context.WithValue(ctx, ownRuntimeKey{}, &ownRuntimeRoots{})
}

// noteOwnRuntimeRoot records root, unless it is empty or ctx records none.
func noteOwnRuntimeRoot(ctx context.Context, root string) {
	roots, _ := ctx.Value(ownRuntimeKey{}).(*ownRuntimeRoots)
	if roots == nil || root == "" {
		return
	}
	roots.mu.Lock()
	defer roots.mu.Unlock()
	roots.roots = append(roots.roots, filepath.Clean(root))
}

// cliMiniRoots returns the CLI's own sources that need no extraction: its
// checkout and its exact version in moduleCache, when they exist.
func cliMiniRoots(moduleCache string) []string {
	var version string
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Path == miniModule && publishedVersion(info.Main.Version) {
		version = info.Main.Version
	}
	_, sourceFile, _, _ := runtime.Caller(0)
	var roots []string
	for _, root := range []string{miniSourceRoot(sourceFile, "", ""), miniSourceRoot("", moduleCache, version)} {
		if root != "" {
			roots = append(roots, filepath.Clean(root))
		}
	}
	return roots
}

// markOwnRuntime marks the listed packages of Mini's module that come from the
// CLI's own sources: those that provisioning used, and those of cliMiniRoots,
// which a client's replacement or requirement can also select. It returns how
// many it marked. Another runtime that the client selects lies elsewhere and
// stays unmarked.
func markOwnRuntime(ctx context.Context, listed []goPackage, moduleCache string) int {
	own := cliMiniRoots(moduleCache)
	if noted, _ := ctx.Value(ownRuntimeKey{}).(*ownRuntimeRoots); noted != nil {
		noted.mu.Lock()
		own = append(own, noted.roots...)
		noted.mu.Unlock()
	}
	marked := 0
	for i := range listed {
		p := &listed[i]
		if p.Module == nil || p.Module.Path != miniModule {
			continue
		}
		for _, root := range own {
			if strings.HasPrefix(filepath.Clean(p.Dir)+string(filepath.Separator), root+string(filepath.Separator)) {
				p.ownRuntime = true
				marked++
				break
			}
		}
	}
	return marked
}

// ownSources reports whether p's files are the CLI's own runtime sources,
// unchanged by any overlay. The CLI's tests check those sources against every
// runtime check (TestMiniRuntimePassesEveryRuntimeCheck in internal/instrument),
// so the checks need not read them again on every invocation.
func ownSources(p *goPackage, replacements map[string]string) bool {
	if !p.ownRuntime {
		return false
	}
	for _, name := range p.GoFiles {
		if _, replaced := replacements[filepath.Join(p.Dir, name)]; replaced {
			return false
		}
	}
	return true
}
