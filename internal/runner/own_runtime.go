package runner

import (
	"context"
	"path/filepath"
	"runtime"
	"runtime/debug"
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

// markOwnRuntime marks the listed packages of Mini's module whose module root
// is one of the CLI's own sources: those that provisioning used, and those of
// cliMiniRoots, which a client's replacement or requirement can also select.
// It returns how many it marked. Another runtime that the client selects has
// another root, even inside the CLI's checkout, and stays unmarked.
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
		moduleRoot := p.Module.Dir
		if p.Module.Replace != nil && p.Module.Replace.Dir != "" {
			moduleRoot = p.Module.Replace.Dir
		}
		if moduleRoot == "" {
			continue
		}
		for _, root := range own {
			if filepath.Clean(moduleRoot) == root {
				p.ownRuntime = true
				marked++
				break
			}
		}
	}
	return marked
}

// ownSources reports whether p's files are the CLI's own runtime sources,
// with no overlay entry in p's directory: none replaces, adds or deletes one of
// its files. The CLI's tests check those sources against every runtime check
// (TestMiniRuntimePassesEveryRuntimeCheck), so the checks need not read them
// again on every invocation. A checkout whose files differ from the revision
// that built the CLI, through local edits or another checked-out revision,
// then fails to link a hook whose declaration it lacks, instead of going
// without the hook.
func ownSources(p *goPackage, replacements map[string]string) bool {
	if !p.ownRuntime {
		return false
	}
	dir := filepath.Clean(p.Dir)
	for path := range replacements {
		if filepath.Dir(path) == dir {
			return false
		}
	}
	return true
}
