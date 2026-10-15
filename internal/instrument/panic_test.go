package instrument

import (
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The toolchain's gopanic takes the hook just before it prints an
// unrecovered panic, on preprintpanics' line. If it stops, --panic-hook
// warns and builds without the hook.
func TestToolchainGopanicTakesTheHook(t *testing.T) {
	pkg, err := build.Default.Import("runtime", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string][]byte{}
	for _, name := range pkg.GoFiles {
		if sources[name], err = os.ReadFile(filepath.Join(pkg.Dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	edited, err := TransformPanic(sources)
	if err != nil {
		t.Fatalf("%v; review TransformPanic", err)
	}
	if len(edited) != 1 {
		t.Fatalf("edited %d files, want gopanic's", len(edited))
	}
	for name, src := range edited {
		original := string(sources[name])
		got := string(src)
		if strings.Count(got, "__dd_civisibility_unhandledPanic") != 2 || strings.Count(got, "\n") != strings.Count(original, "\n")+1 {
			t.Fatalf("%s: the edit adds more than the call or moves lines", name)
		}
		var hookLine, nativeLine int
		for i, line := range strings.Split(got, "\n") {
			if strings.Contains(line, "preprintpanics(&p); if __dd_civisibility_unhandledPanic != nil { __dd_civisibility_unhandledPanic(p.arg) }") {
				hookLine = i // The line directive comes first.
			}
		}
		for i, line := range strings.Split(original, "\n") {
			if strings.TrimSpace(line) == "preprintpanics(&p)" {
				nativeLine = i + 1
			}
		}
		if hookLine == 0 || hookLine != nativeLine {
			t.Fatalf("%s: the hook is on line %d, preprintpanics on line %d", name, hookLine, nativeLine)
		}
	}
}

// PanicHook compiles as a file of package runtime, which may import only
// unsafe.
func TestPanicHookTypeChecks(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, panicHookName, PanicHook, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range file.Imports {
		if spec.Path.Value != `"unsafe"` {
			t.Errorf("PanicHook imports %s", spec.Path.Value)
		}
	}
	if _, err := (&types.Config{Importer: importer.Default()}).Check("runtime", fset, []*ast.File{file}, nil); err != nil {
		t.Fatal(err)
	}
}

const panicHookName = "zz_dd_ci_visibility_panic.go"

// PanicHook's linkname names Mini's integrations.unhandledPanic, of the same
// type.
func TestPanicHookMatchesMiniRegistration(t *testing.T) {
	dir := filepath.Join("..", "thirdparty", "dd-trace-go", "civisibility", "integrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string][]byte{}
	for _, entry := range entries {
		if name := entry.Name(); strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			if sources[name], err = os.ReadFile(filepath.Join(dir, name)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := PanicHookRuntimeError(sources); err != nil {
		t.Fatal(err)
	}
	hook, err := sourceDeclarations(map[string][]byte{panicHookName: []byte(PanicHook)})
	if err != nil {
		t.Fatal(err)
	}
	if !hook["var __dd_civisibility_unhandledPanic func(any)"] || !hook["linkname __dd_civisibility_unhandledPanic github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/civisibility/integrations.unhandledPanic"] {
		t.Error("PanicHook does not link Mini's integrations.unhandledPanic")
	}
}

// A runtime without the variable, as the released ones, builds without the
// hook, and so does a toolchain whose gopanic differs.
func TestPanicHookRejectsOtherRuntimesAndToolchains(t *testing.T) {
	for name, source := range map[string]string{
		"missing": "package integrations\nfunc other() {}\n",
		"type":    "package integrations\nvar unhandledPanic func(string)\n",
	} {
		if err := PanicHookRuntimeError(map[string][]byte{"crash.go": []byte(source)}); err == nil {
			t.Errorf("runtime %s: accepted", name)
		}
	}
	if err := PanicHookRuntimeError(map[string][]byte{"crash.go": []byte("package integrations\nvar unhandledPanic func(any)\n")}); err != nil {
		t.Errorf("runtime: %v", err)
	}
	const gopanic = "package runtime\ntype _panic struct {\n\targ any\n}\nfunc preprintpanics(p *_panic) {}\nfunc gopanic(e any) {\n\tvar p _panic\n\tpreprintpanics(&p)\n\tfatalpanic(&p)\n}\n"
	for name, source := range map[string]string{
		"no gopanic":           strings.Replace(gopanic, "gopanic", "gopanic2", 1),
		"no preprintpanics":    strings.Replace(gopanic, "\tpreprintpanics(&p)\n", "", 1),
		"nested":               strings.Replace(gopanic, "\tpreprintpanics(&p)\n", "\tif true {\n\t\tpreprintpanics(&p)\n\t}\n", 1),
		"twice":                strings.Replace(gopanic, "\tpreprintpanics(&p)\n", "\tpreprintpanics(&p)\n\tpreprintpanics(&p)\n", 1),
		"other argument":       strings.Replace(gopanic, "\tpreprintpanics(&p)", "\tpreprintpanics(nil)", 1),
		"no arg field":         strings.Replace(gopanic, "\targ any\n", "\tvalue any\n", 1),
		"preprintpanics":       strings.Replace(gopanic, "func preprintpanics(p *_panic) {}", "func preprintpanics(p _panic) {}", 1),
		"already instrumented": gopanic + "var __dd_civisibility_unhandledPanic func(any)\n",
	} {
		if _, err := TransformPanic(map[string][]byte{"panic.go": []byte(source)}); err == nil {
			t.Errorf("toolchain %s: accepted", name)
		}
	}
	edited, err := TransformPanic(map[string][]byte{"panic.go": []byte(gopanic)})
	if err != nil {
		t.Fatal(err)
	}
	if want := "\tpreprintpanics(&p); if __dd_civisibility_unhandledPanic != nil { __dd_civisibility_unhandledPanic(p.arg) }\n"; !strings.Contains(string(edited["panic.go"]), want) {
		t.Fatalf("edited gopanic:\n%s", edited["panic.go"])
	}
}
