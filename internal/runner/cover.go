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
	"regexp"
	"sort"
	"strings"
)

// RunCoverTool is ddtest's private -toolexec entrypoint. Go's cover tool opens
// logical source paths directly, unlike the compiler. Translate only its Go
// inputs and coverage metadata. Compiler/linker identities stay native. The
// cover identity also includes our transformation contract and excluded files.
func RunCoverTool(ctx context.Context, overlay string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "ddtest: missing cover tool executable")
		return 2
	}
	var finish func() error
	var cleanup func()
	if filepath.Base(args[0]) == "cover" || filepath.Base(args[0]) == "cover.exe" {
		probe := len(args) == 2 && args[1] == "-V=full"
		if probe {
			return runCoverVersion(ctx, overlay, args, stdin, stdout, stderr)
		}
		var err error
		args, finish, cleanup, err = prepareCoverInputs(overlay, args)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if cleanup != nil {
			defer cleanup()
		}
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr
	if err := cmd.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return exit.ExitCode()
		}
		fmt.Fprintln(stderr, err)
		return 2
	}
	if finish != nil {
		if err := finish(); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
	}
	return 0
}

// Bump this whenever cover translation semantics change without changing the
// rewritten client sources. Otherwise Go could reuse stale coverage objects.
const coverContractVersion = "ddtest-cover-v1"

func coverFingerprint(plan Overlay) string {
	excluded := append([]string(nil), plan.CoverExclude...)
	sort.Strings(excluded)
	data, _ := json.Marshal(struct {
		Version  string
		Excluded []string
	}{coverContractVersion, excluded})
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

func appendCoverIdentity(native, fingerprint string) string {
	native = strings.TrimSpace(native)
	if strings.Contains(native, " buildID=") {
		// Development toolchains use only the last buildID content component.
		return native + "-ddtest-cover-" + fingerprint + "\n"
	}
	return native + " ddtest-cover=" + fingerprint + "\n"
}

func runCoverVersion(ctx context.Context, overlay string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	data, err := os.ReadFile(overlay)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	var plan Overlay
	if err := json.Unmarshal(data, &plan); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	var native bytes.Buffer
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, &native, stderr
	if err := cmd.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return exit.ExitCode()
		}
		fmt.Fprintln(stderr, err)
		return 2
	}
	_, err = io.WriteString(stdout, appendCoverIdentity(native.String(), coverFingerprint(plan)))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return 0
}

// prepareCoverInputs keeps cover's input/output ordering intact. Generated
// wrappers are compiled normally but excluded from metadata and counters, so
// adding instrumentation cannot change the client's coverage percentage.
func prepareCoverInputs(overlay string, args []string) ([]string, func() error, func(), error) {
	data, err := os.ReadFile(overlay)
	if err != nil {
		return nil, nil, nil, err
	}
	var plan Overlay
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, nil, nil, err
	}
	excluded := make(map[string]bool, len(plan.CoverExclude))
	for _, path := range plan.CoverExclude {
		excluded[path] = true
	}
	result := append([]string(nil), args...)
	var inputs []int
	outListIndex := -1
	outListAssigned := false
	for i := 1; i < len(result); i++ {
		if strings.HasPrefix(result[i], "-outfilelist=") {
			outListIndex = i
			outListAssigned = true
		} else if result[i] == "-outfilelist" && i+1 < len(result) {
			outListIndex = i + 1
		}
		if strings.HasPrefix(result[i], "-") || !strings.HasSuffix(result[i], ".go") {
			continue
		}
		logical, err := filepath.Abs(result[i])
		if err != nil {
			return nil, nil, nil, err
		}
		inputs = append(inputs, i)
		if actual := plan.Replace[logical]; actual != "" {
			result[i] = actual
		}
	}
	var skipped map[int]string
	for _, i := range inputs {
		logical, _ := filepath.Abs(args[i])
		if excluded[logical] {
			if skipped == nil {
				skipped = make(map[int]string)
			}
			skipped[i] = result[i]
		}
	}
	if len(skipped) == 0 {
		return result, nil, nil, nil
	}
	if outListIndex < 0 {
		return nil, nil, nil, fmt.Errorf("cover: generated adapters require package coverage output mapping")
	}
	outList := strings.TrimPrefix(result[outListIndex], "-outfilelist=")
	data, err = os.ReadFile(outList)
	if err != nil {
		return nil, nil, nil, err
	}
	outputs := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(outputs) != len(inputs)+1 {
		return nil, nil, nil, fmt.Errorf("cover: expected %d output paths, got %d", len(inputs)+1, len(outputs))
	}
	keptOutputs := []string{outputs[0]}
	restore := make(map[string]string, len(skipped))
	for n, i := range inputs {
		if source, ok := skipped[i]; ok {
			restore[outputs[n+1]] = source
		} else {
			keptOutputs = append(keptOutputs, outputs[n+1])
		}
	}
	temporary, err := os.CreateTemp(filepath.Dir(outList), "ddtest-cover-*.txt")
	if err != nil {
		return nil, nil, nil, err
	}
	cleanup := func() { os.Remove(temporary.Name()) }
	_, err = temporary.WriteString(strings.Join(keptOutputs, "\n") + "\n")
	closeErr := temporary.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		cleanup()
		return nil, nil, nil, err
	}
	result[outListIndex] = temporary.Name()
	if outListAssigned {
		result[outListIndex] = "-outfilelist=" + temporary.Name()
	}
	filtered := make([]string, 0, len(result)-len(skipped))
	for i, arg := range result {
		if _, skip := skipped[i]; !skip {
			filtered = append(filtered, arg)
		}
	}
	finish := func() error {
		for output, source := range restore {
			content, err := os.ReadFile(source)
			if err != nil {
				return err
			}
			if err := os.WriteFile(output, content, 0600); err != nil {
				return err
			}
		}
		return nil
	}
	return filtered, finish, cleanup, nil
}

func coverToolCommand(executable, overlay string) (string, error) {
	return toolCommand(executable, overlay, "cover")
}

func toolCommand(executable, overlay, mode string) (string, error) {
	quote := func(s string) (string, error) {
		// cmd/go's quoted.Split preserves backslashes; strconv.Quote would double
		// Windows separators. Choose a delimiter absent from the path instead.
		if !strings.Contains(s, "'") {
			return "'" + s + "'", nil
		}
		if !strings.Contains(s, `"`) {
			return `"` + s + `"`, nil
		}
		return "", fmt.Errorf("cannot quote cover tool path containing both quote characters: %s", s)
	}
	exe, err := quote(executable)
	if err != nil {
		return "", err
	}
	path, err := quote(overlay)
	if err != nil {
		return "", err
	}
	return exe + " tool-overlay " + mode + " " + path, nil
}

// Match only the packages whose regular sources changed. Test files are not
// covered. Without -coverpkg, Go covers the requested test packages only.
func needsCoverOverlay(dir string, opts options, targets, rewritten []goPackage) bool {
	if !opts.coverage {
		return false
	}
	for _, p := range rewritten {
		if len(opts.coverPatterns) == 0 {
			for _, target := range targets {
				if target.ImportPath == p.ImportPath && target.ImportPath != "testing" && target.ImportPath != sdkPackage && target.ImportPath != miniPackage {
					return true
				}
			}
			continue
		}
		for _, pattern := range opts.coverPatterns {
			if pattern == "all" || pattern == "std" && p.Module == nil {
				return true
			}
			name := p.ImportPath
			if pattern == "." || pattern == ".." || strings.HasPrefix(pattern, "./") || strings.HasPrefix(pattern, "../") || filepath.IsAbs(pattern) {
				// Resolve the directory before the wildcard. A relative pattern
				// must not cover testing in an unrelated GOROOT outside that tree.
				prefix, tail := pattern, ""
				if wildcard := strings.Index(pattern, "..."); wildcard >= 0 {
					slash := strings.LastIndex(pattern[:wildcard], "/")
					prefix, tail = pattern[:slash], pattern[slash+1:]
				}
				base := prefix
				if !filepath.IsAbs(base) {
					base = filepath.Join(dir, base)
				}
				relative, err := filepath.Rel(base, p.Dir)
				if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
					continue
				}
				name, pattern = filepath.ToSlash(relative), tail
				if name == "." {
					name = ""
				}
			}
			expression := regexp.QuoteMeta(pattern)
			expression = strings.ReplaceAll(expression, `/\.\.\.`, `(?:/.*)?`)
			expression = strings.ReplaceAll(expression, `\.\.\.`, ".*")
			if regexp.MustCompile("^" + expression + "$").MatchString(name) {
				return true
			}
		}
	}
	return false
}
