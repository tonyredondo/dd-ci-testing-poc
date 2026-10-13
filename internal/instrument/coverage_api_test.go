package instrument

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The hook's registration matches Mini's own coverage package.
func TestCoverageHookMatchesMiniRegistration(t *testing.T) {
	if err := CoverageHookRuntimeError(filepath.Join("..", "thirdparty", "dd-trace-go", "civisibility", "integrations", "gotesting", "coverage")); err != nil {
		t.Fatal(err)
	}
}

// A selected runtime without the registration, or with another signature,
// keeps text profiles instead of failing to link.
func TestCoverageHookRuntimeErrorRejectsOtherRuntimes(t *testing.T) {
	for name, source := range map[string]string{
		"missing":   "package coverage\nfunc other() {}\n",
		"signature": "package coverage\nfunc registerTestingCoverage(func() ([]string, []uint32, []byte, string), func([]uint32) ([]uint32, bool), func([]uint32, []uint32) ([]uint32, bool)) {}\n",
		"method":    "package coverage\ntype r struct{}\nfunc (r) registerTestingCoverage(func() ([]string, []uint32, []byte, string), func([]uint32) ([]uint32, bool), func([]uint32, []uint32) ([]uint32, bool), func([]uint32) []byte) {}\n",
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "coverage.go"), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
		if err := CoverageHookRuntimeError(dir); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// A toolchain that lacks any declaration the hook uses keeps text profiles.
func TestCoverageHookToolchainErrorReportsMissingDeclarations(t *testing.T) {
	out, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		t.Fatal(err)
	}
	goroot := strings.TrimSpace(string(out))
	for dir, decls := range coverageHookAPI {
		for _, decl := range decls {
			fake := t.TempDir()
			for copyDir := range coverageHookAPI {
				src := filepath.Join(goroot, "src", filepath.FromSlash(copyDir))
				dst := filepath.Join(fake, "src", filepath.FromSlash(copyDir))
				if err := os.MkdirAll(dst, 0700); err != nil {
					t.Fatal(err)
				}
				entries, err := os.ReadDir(src)
				if err != nil {
					t.Fatal(err)
				}
				for _, entry := range entries {
					name := entry.Name()
					if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || copyDir == "runtime" && name != "covercounter.go" {
						continue
					}
					data, err := os.ReadFile(filepath.Join(src, name))
					if err != nil {
						t.Fatal(err)
					}
					if copyDir == dir {
						data = []byte(removeDeclaration(t, string(data), decl))
					}
					if err := os.WriteFile(filepath.Join(dst, name), data, 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			// Renaming can remove more than one declaration; any is reported.
			if err := CoverageHookToolchainError(fake); err == nil {
				t.Errorf("accepted without %q in %s", decl, dir)
			}
		}
	}
}

// removeDeclaration renames what a declaration fingerprint names, so that the
// file no longer declares it.
func removeDeclaration(t *testing.T, source, decl string) string {
	t.Helper()
	kind, rest, _ := strings.Cut(decl, " ")
	switch kind {
	case "linkname":
		return strings.ReplaceAll(source, "//go:linkname "+strings.Fields(rest)[0], "//go:linkname renamed_"+strings.Fields(rest)[0])
	case "field":
		owner, field, _ := strings.Cut(strings.Fields(rest)[0], ".")
		_ = owner
		return replaceIdentifier(source, field, field+"Renamed")
	case "func":
		name := strings.Fields(strings.TrimPrefix(rest, "(*"))[0]
		if strings.HasPrefix(rest, "(") {
			name = strings.Fields(rest)[1]
		}
		name, _, _ = strings.Cut(name, "(")
		return replaceIdentifier(source, name, name+"Renamed")
	default: // type, const
		name := strings.Fields(rest)[0]
		return replaceIdentifier(source, name, name+"Renamed")
	}
}

// replaceIdentifier renames whole-word occurrences of name.
func replaceIdentifier(source, name, renamed string) string {
	var out strings.Builder
	for i := 0; i < len(source); {
		j := strings.Index(source[i:], name)
		if j < 0 {
			out.WriteString(source[i:])
			break
		}
		j += i
		end := j + len(name)
		before := j == 0 || !isIdentByte(source[j-1])
		after := end == len(source) || !isIdentByte(source[end])
		out.WriteString(source[i:j])
		if before && after {
			out.WriteString(renamed)
		} else {
			out.WriteString(name)
		}
		i = end
	}
	return out.String()
}

func isIdentByte(b byte) bool {
	return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}
