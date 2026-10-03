package runner

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/instrument"
)

const minimumTestifyVersion = "v1.11.1"

// Bump when compiler-side edits change without changing prepared inputs.
const testifyContractVersion = "ddtest-testify-entry-v1"

// TestifyTool contains only the prepared suite inputs. Other packages never
// need to read the tool plan. Covered sources are transformed after go cover.
type TestifyTool struct {
	Sources     map[string]string
	HookFile    string
	Fingerprint string
}

func prepareTestify(ctx context.Context, dir string, opts options, packages []goPackage, replacements map[string]string, runtime Runtime, temp string) (*TestifyTool, error) {
	var suite *goPackage
	for _, p := range packages {
		if p.ImportPath == instrument.TestifySuiteImport {
			copy := p
			suite = &copy
			break
		}
	}
	if suite == nil {
		paths, hasSuite := testifyDependencyImports(packages)
		if hasSuite {
			paths = []string{instrument.TestifySuiteImport}
		}
		if len(paths) == 0 {
			return nil, nil
		}
		// Walk actual test-import dependencies, including helpers in other modules.
		// A go.mod requirement or an assert-only import does not enable the wrapper.
		mode := "-deps"
		if hasSuite {
			// Reachability is already known. Find only the selected library's
			// source and module metadata; its dependencies need no second walk.
			mode = "-find"
		}
		args := []string{"list", mode, "-json=Dir,Name,ImportPath,GoFiles,Module,Error"}
		args = append(args, opts.buildFlags...)
		args = append(args, paths...)
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = dir
		dependencies, err := readPackages(cmd, "resolve Testify test dependencies")
		if err != nil {
			return nil, err
		}
		for _, p := range dependencies {
			if p.ImportPath == instrument.TestifySuiteImport {
				copied := p
				suite = &copied
			}
		}
		if suite == nil {
			return nil, nil
		}
	}
	selected := ""
	if suite.Module != nil {
		selected = suite.Module.Version
		if suite.Module.Replace != nil && suite.Module.Replace.Version != "" {
			selected = suite.Module.Replace.Version
		}
	}
	if !instrument.SupportsTestifyVersion(selected) {
		return nil, fmt.Errorf("Testify %q is unsupported: suite.Run instrumentation requires >=%s and <v2.0.0", selected, minimumTestifyVersion)
	}
	files := map[string][]byte{}
	actualPaths := map[string]string{}
	for _, name := range suite.GoFiles {
		logical := filepath.Join(suite.Dir, name)
		actual := logical
		if to, ok := replacements[logical]; ok {
			actual = to
		}
		if actual == "" {
			continue
		}
		src, err := os.ReadFile(actual)
		if err != nil {
			return nil, err
		}
		files[logical] = src
		actualPaths[logical] = actual
	}
	rewritten, err := instrument.TransformTestifyPackage(files)
	if err != nil {
		return nil, err
	}
	hook := "github.com/DataDog/dd-trace-go/v2/internal/civisibility/integrations/gotesting.instrumentTestifySuiteRun"
	if runtime == Mini {
		hook = strings.Replace(hook, "github.com/DataDog/dd-trace-go/v2/internal/civisibility/", "github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/civisibility/", 1)
	}
	tool := &TestifyTool{Sources: map[string]string{}, HookFile: filepath.Join(temp, "testify-hook.go")}
	hookSource := instrument.TestifyEntryHook(hook)
	hash := sha256.New()
	hash.Write([]byte(testifyContractVersion))
	hash.Write([]byte(hookSource))
	names := make([]string, 0, len(rewritten))
	for name := range rewritten {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		src := rewritten[name]
		backing := filepath.Join(temp, "testify-entry.go")
		if err := os.WriteFile(backing, src, 0600); err != nil {
			return nil, err
		}
		tool.Sources[name] = backing
		tool.Sources[actualPaths[name]] = backing
		hash.Write(src)
	}
	if len(tool.Sources) == 0 {
		return nil, fmt.Errorf("Testify suite.Run entry was not found")
	}
	if err := os.WriteFile(tool.HookFile, []byte(hookSource), 0600); err != nil {
		return nil, err
	}
	tool.Fingerprint = fmt.Sprintf("%x", hash.Sum(nil))
	return tool, nil
}

// testifyDependencyImports walks only test imports whose dependency closures
// were not already inspected by the first go list. Unknown imports remain in
// the query: a helper in another module may call suite.Run on the client's behalf.
func testifyDependencyImports(packages []goPackage) ([]string, bool) {
	known := map[string]bool{"testing": true}
	hasSuite := false
	for _, p := range packages {
		if p.ImportPath == sdkPackage || p.ImportPath == miniPackage {
			// The runtime's graph says nothing about the client's test helpers.
			continue
		}
		known[p.ImportPath] = true
		for _, path := range p.Deps {
			known[path] = true
			if p.ImportPath != "testing" && path == instrument.TestifySuiteImport {
				hasSuite = true
			}
		}
	}
	requests := map[string]bool{}
	for _, p := range packages {
		if p.ImportPath == "testing" || p.ImportPath == sdkPackage || p.ImportPath == miniPackage {
			continue
		}
		for _, imports := range [][]string{p.TestImports, p.XTestImports} {
			for _, path := range imports {
				// Check reachability before pruning: known imports still need
				// suite metadata and version validation on every invocation.
				if path == instrument.TestifySuiteImport {
					hasSuite = true
				}
				if !known[path] {
					requests[path] = true
				}
			}
		}
	}
	paths := make([]string, 0, len(requests))
	for path := range requests {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, hasSuite
}

func prepareTestifyCompile(plan *TestifyTool, args []string) ([]string, func(), error) {
	if plan == nil {
		return nil, nil, fmt.Errorf("missing Testify tool plan")
	}
	result := append([]string(nil), args...)
	var temporary []string
	cleanup := func() {
		for _, path := range temporary {
			os.Remove(path)
		}
	}
	changed := false
	// Go places compiler source inputs last. Flags may themselves contain '.go'.
	start := len(args)
	for start > 1 && strings.HasSuffix(args[start-1], ".go") {
		start--
	}
	for i := start; i < len(args); i++ {
		if replacement, ok := plan.Sources[filepath.Clean(args[i])]; ok {
			result[i] = replacement
			changed = true
			continue
		}
		// Covered source paths belong to $WORK, not the original module. Prepending
		// after cover preserves the original coverage counters and denominator.
		src, err := os.ReadFile(args[i])
		if err != nil {
			cleanup()
			return nil, nil, err
		}
		transformed, found, err := instrument.TransformTestifyEntry(args[i], src)
		if err != nil {
			cleanup()
			return nil, nil, err
		}
		if !found {
			continue
		}
		file, err := os.CreateTemp(filepath.Dir(args[i]), "ddtest-suite-*.go")
		if err != nil {
			cleanup()
			return nil, nil, err
		}
		temporary = append(temporary, file.Name())
		_, err = file.Write(transformed)
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			cleanup()
			return nil, nil, err
		}
		result[i] = file.Name()
		changed = true
	}
	if !changed {
		cleanup()
		return nil, nil, fmt.Errorf("Testify compiler inputs contain no suite.Run entry")
	}
	return append(result, plan.HookFile), cleanup, nil
}
