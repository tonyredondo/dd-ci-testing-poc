package runner

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
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
	requests := map[string]bool{}
	hasSuite := false
	standard := map[string]bool{"testing": true}
	for _, p := range packages {
		if p.ImportPath == "testing" {
			for _, path := range p.Deps {
				standard[path] = true
			}
		}
	}
	for _, p := range packages {
		if p.ImportPath == instrument.TestifySuiteImport {
			copy := p
			suite = &copy
			break
		}
		if p.ImportPath == "testing" || p.ImportPath == sdkPackage || p.ImportPath == miniPackage {
			continue
		}
		for _, path := range p.Deps {
			if path == instrument.TestifySuiteImport {
				hasSuite = true
			}
		}
		for _, path := range append(append([]string{}, p.TestImports...), p.XTestImports...) {
			if path == instrument.TestifySuiteImport {
				hasSuite = true
			}
			if !standard[path] {
				requests[path] = true
			}
		}
	}
	if suite == nil {
		if hasSuite {
			requests = map[string]bool{instrument.TestifySuiteImport: true}
		}
		if len(requests) == 0 {
			return nil, nil
		}
		paths := make([]string, 0, len(requests))
		for path := range requests {
			paths = append(paths, path)
		}
		sort.Strings(paths)
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
		var stderr strings.Builder
		cmd.Stderr = &stderr
		data, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("resolve Testify test dependencies: %w\n%s", err, stderr.String())
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		for {
			var p goPackage
			if err := decoder.Decode(&p); err != nil {
				if err == io.EOF {
					break
				}
				return nil, err
			}
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
		if err := instrument.CheckTestifyNames(logical, src); err != nil {
			return nil, err
		}
		files[logical] = src
		actualPaths[logical] = actual
	}
	if err := instrument.TestifyAPI(files); err != nil {
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
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		src, changed, err := instrument.TransformTestifyEntry(name, files[name])
		if err != nil {
			return nil, err
		}
		if !changed {
			continue
		}
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
