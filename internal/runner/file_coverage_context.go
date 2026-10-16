package runner

import (
	"os"
	"path/filepath"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/instrument"
)

// --experimental-file-coverage=context: goroutines carry the tests that they
// work for, so tests keep t.Parallel and each reports its own files. See
// instrument.TransformContextRuntime.

// contextHookFile is the file that context file coverage adds to each package
// that it edits.
const contextHookFile = "zz_ddto_context.go"

// contextEdit is what context file coverage changes in one package.
type contextEdit struct {
	pkg       string
	transform func(map[string][]byte) (map[string][]byte, error)
	hook      string
	// optional packages are edited only when the build lists them.
	optional bool
}

// contextEdits returns the edits of context file coverage.
func contextEdits() []contextEdit {
	return []contextEdit{
		{pkg: "runtime", transform: instrument.TransformContextRuntime, hook: instrument.ContextRuntimeHook(instrument.FileCoveragePackage)},
		{pkg: "internal/poll", transform: instrument.TransformContextPoll, hook: instrument.ContextPollHook},
		{pkg: "context", transform: instrument.TransformContextContext, hook: instrument.ContextCaptureHook("context")},
		{pkg: "net/http", transform: instrument.TransformContextHTTP, hook: instrument.ContextCaptureHook("http"), optional: true},
		{pkg: "internal/singleflight", transform: instrument.TransformContextSingleflight, hook: instrument.ContextSingleflightHook("singleflight", instrument.FileCoveragePackage), optional: true},
	}
}

// contextChanges are the overlay changes of context file coverage, which the
// caller applies once file coverage is installed: replacements of edited
// files and added hook files.
type contextChanges struct {
	replace map[string]string
	add     map[string]string
}

// prepareContextCoverage edits the toolchain packages that context file
// coverage needs, as the build with tests lists them, and returns the
// changes, or why the build goes without context file coverage. listed has
// the runtime's packages. Nothing changes the overlay yet.
func prepareContextCoverage(tested, listed []goPackage, replacements map[string]string, temp string) (contextChanges, string, error) {
	changes := contextChanges{replace: map[string]string{}, add: map[string]string{}}
	if coverage := findPackage(listed, miniCoveragePackage); coverage == nil {
		return changes, "the runtime's coverage package not listed", nil
	} else if refusal := checkSources(coverage, replacements, instrument.FileCoverageContextRuntimeError); refusal != "" {
		return changes, refusal, nil
	}
	for _, edit := range contextEdits() {
		pkg := findPackage(tested, edit.pkg)
		if pkg == nil {
			if edit.optional {
				continue
			}
			return changes, edit.pkg + " not listed", nil
		}
		sources, err := packageSources(pkg, replacements)
		if err != nil {
			return changes, err.Error(), nil
		}
		edited, err := edit.transform(sources)
		if err != nil {
			return changes, err.Error(), nil
		}
		for logical, src := range edited {
			backing := filepath.Join(temp, "context-"+filepath.Base(pkg.Dir)+"-"+filepath.Base(logical))
			if err := os.WriteFile(backing, src, 0600); err != nil {
				return changes, "", err
			}
			changes.replace[logical] = backing
		}
		changes.add[filepath.Join(pkg.Dir, contextHookFile)] = edit.hook
	}
	return changes, "", nil
}
