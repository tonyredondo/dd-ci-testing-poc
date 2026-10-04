package runner

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/instrument"
)

func prepareGoleak(pkg *goPackage, replacements map[string]string, temp string) (*LibraryEntry, error) {
	if pkg == nil {
		return nil, nil
	}
	version := ""
	if pkg.Module != nil {
		version = pkg.Module.Version
		if pkg.Module.Replace != nil && pkg.Module.Replace.Version != "" {
			version = pkg.Module.Replace.Version
		}
	}
	if !supportedGoleakVersion(version) {
		return nil, fmt.Errorf("goleak %q is unsupported: requires >=v1.3.0 and <v2.0.0", version)
	}
	tool := &LibraryEntry{Package: instrument.GoleakImport, Sources: map[string]string{}, HookFile: filepath.Join(temp, "goleak-hook.go")}
	hash := sha256.New()
	hash.Write([]byte("ddtest-goleak-find-v1"))
	hook := instrument.GoleakEntryHook()
	hash.Write([]byte(hook))
	names := append([]string(nil), pkg.GoFiles...)
	sort.Strings(names)
	for _, name := range names {
		logical := filepath.Join(pkg.Dir, name)
		actual := logical
		if replacement, ok := replacements[logical]; ok {
			actual = replacement
		}
		if actual == "" {
			continue
		}
		src, err := os.ReadFile(actual)
		if err != nil {
			return nil, err
		}
		out, found, err := instrument.TransformGoleakEntry(logical, src)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		backing := filepath.Join(temp, "goleak-entry.go")
		if err := os.WriteFile(backing, out, 0600); err != nil {
			return nil, err
		}
		tool.Sources[logical], tool.Sources[actual] = backing, backing
		hash.Write(out)
	}
	if len(tool.Sources) == 0 {
		return nil, fmt.Errorf("goleak.Find entry was not found")
	}
	if err := os.WriteFile(tool.HookFile, []byte(hook), 0600); err != nil {
		return nil, err
	}
	tool.Fingerprint = fmt.Sprintf("%x", hash.Sum(nil))
	return tool, nil
}

func supportedGoleakVersion(version string) bool {
	var minor, patch int
	if _, err := fmt.Sscanf(strings.TrimPrefix(version, "v1."), "%d.%d", &minor, &patch); err != nil || !strings.HasPrefix(version, "v1.") {
		return false
	}
	return minor > 3 || minor == 3 && (patch > 0 || !strings.Contains(version, "-"))
}

// Goleak does not import testing, so its cache key cannot inherit our testing
// marker as Testify does. A package-scoped, unused import-search directory keys
// this transform without changing the compiler identity for other packages.
// Go supplies every import through -importcfg; the sentinel is never read.
func goleakCacheFlag(dir string, opts options, pkg *goPackage, fingerprint string) (string, error) {
	flags, err := splitFlags(os.Getenv("GOFLAGS"))
	if err != nil {
		return "", err
	}
	flags = append(flags, opts.buildFlags...)
	var effective []string
	for i := 0; i < len(flags); i++ {
		name, value, assigned := strings.Cut(strings.TrimLeft(flags[i], "-"), "=")
		if name != "gcflags" {
			continue
		}
		if !assigned {
			i++
			if i == len(flags) {
				return "", fmt.Errorf("missing gcflags value")
			}
			value = flags[i]
		}
		value = strings.TrimSpace(value)
		match := false // unqualified flags apply only to command-line packages
		for _, selected := range opts.packages {
			match = match || matchesGoleakPattern(selected, dir, pkg)
		}
		if value != "" && !strings.HasPrefix(value, "-") {
			pattern, rest, ok := strings.Cut(value, "=")
			if !ok {
				return "", fmt.Errorf("invalid gcflags pattern")
			}
			match = matchesGoleakPattern(strings.TrimSpace(pattern), dir, pkg)
			value = rest
		}
		if match {
			effective, err = splitFlags(value)
			if err != nil {
				return "", err
			}
		}
	}
	effective = append(effective, "-I=ddtest-goleak-"+fingerprint)
	quoted := make([]string, len(effective))
	for i, flag := range effective {
		if strings.ContainsAny(flag, " \t\r\n") {
			if !strings.Contains(flag, "'") {
				flag = "'" + flag + "'"
			} else if !strings.Contains(flag, "\"") {
				flag = "\"" + flag + "\""
			} else {
				return "", fmt.Errorf("cannot quote goleak compiler flag")
			}
		}
		quoted[i] = flag
	}
	return "-gcflags=" + instrument.GoleakImport + "=" + strings.Join(quoted, " "), nil
}

func matchesGoleakPattern(pattern, dir string, pkg *goPackage) bool {
	if pattern == "all" {
		return true
	}
	if pattern == "std" || pattern == "cmd" {
		return false
	}
	name := pkg.ImportPath
	if filepath.IsAbs(pattern) || strings.HasPrefix(pattern, ".") {
		if filepath.IsAbs(pattern) {
			name = filepath.ToSlash(pkg.Dir)
			pattern = filepath.ToSlash(pattern)
		} else {
			relative, err := filepath.Rel(dir, pkg.Dir)
			if err != nil {
				return false
			}
			name = "./" + filepath.ToSlash(relative)
			if relative == "." {
				name = "."
			}
			if strings.Contains(name, "/vendor/") && !strings.Contains(pattern, "/vendor/") {
				return false
			}
		}
	}
	expression := regexp.QuoteMeta(pattern)
	if strings.HasSuffix(expression, `/\.\.\.`) {
		expression = strings.TrimSuffix(expression, `/\.\.\.`) + "(/.*)?"
	}
	expression = strings.ReplaceAll(expression, `\.\.\.`, ".*")
	return regexp.MustCompile("^" + expression + "$").MatchString(name)
}
