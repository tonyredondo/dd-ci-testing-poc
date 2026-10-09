package instrument

import (
	"go/ast"
	"go/token"
	"os"
)

// Hooks uses the exact link targets and signatures in the SDK's Orchestrion
// advice. Keeping these symbols also preserves the SDK's woven ownership gate.
const Hooks = `package testing
import (
 _ "unsafe"
 __dd_ci_os "os"
)
//go:linkname __dd_civisibility_instrumentTestingBuiltWithOrchestrion github.com/DataDog/dd-trace-go/v2/internal/civisibility/integrations/gotesting.instrumentTestingBuiltWithOrchestrion
func __dd_civisibility_instrumentTestingBuiltWithOrchestrion()
//go:linkname __dd_civisibility_instrumentTestingMWithControl github.com/DataDog/dd-trace-go/v2/internal/civisibility/integrations/gotesting.instrumentTestingMWithControl
func __dd_civisibility_instrumentTestingMWithControl(*M) (bool, func(int) int)
//go:linkname __dd_civisibility_instrumentTestingMAbnormalExitCode github.com/DataDog/dd-trace-go/v2/internal/civisibility/integrations/gotesting.instrumentTestingMAbnormalExitCode
func __dd_civisibility_instrumentTestingMAbnormalExitCode() int
//go:linkname __dd_civisibility_instrumentTestingTFunc github.com/DataDog/dd-trace-go/v2/internal/civisibility/integrations/gotesting.instrumentTestingTFunc
func __dd_civisibility_instrumentTestingTFunc(func(*T)) func(*T)
//go:linkname __dd_civisibility_instrumentSetErrorInfo github.com/DataDog/dd-trace-go/v2/internal/civisibility/integrations/gotesting.instrumentSetErrorInfo
func __dd_civisibility_instrumentSetErrorInfo(TB, string, string, int)
//go:linkname __dd_civisibility_instrumentCaptureFormattedError github.com/DataDog/dd-trace-go/v2/internal/civisibility/integrations/gotesting.instrumentCaptureFormattedError
func __dd_civisibility_instrumentCaptureFormattedError(TB, string, string, int) string
//go:linkname __dd_civisibility_instrumentCaptureFormattedSkip github.com/DataDog/dd-trace-go/v2/internal/civisibility/integrations/gotesting.instrumentCaptureFormattedSkip
func __dd_civisibility_instrumentCaptureFormattedSkip(TB, string, string) string
//go:linkname __dd_civisibility_instrumentCloseAndSkip github.com/DataDog/dd-trace-go/v2/internal/civisibility/integrations/gotesting.instrumentCloseAndSkip
func __dd_civisibility_instrumentCloseAndSkip(TB, string)
//go:linkname __dd_civisibility_instrumentSkipNow github.com/DataDog/dd-trace-go/v2/internal/civisibility/integrations/gotesting.instrumentSkipNow
func __dd_civisibility_instrumentSkipNow(TB)
//go:linkname __dd_civisibility_instrumentTestingParallel github.com/DataDog/dd-trace-go/v2/internal/civisibility/integrations/gotesting.instrumentTestingParallel
func __dd_civisibility_instrumentTestingParallel(*T) bool
//go:linkname __dd_civisibility_instrumentTestingBFunc github.com/DataDog/dd-trace-go/v2/internal/civisibility/integrations/gotesting.instrumentTestingBFunc
func __dd_civisibility_instrumentTestingBFunc(*B, string, func(*B)) (string, func(*B))
func init() { __dd_civisibility_instrumentTestingBuiltWithOrchestrion() }
` + environmentHook

// SavedEnvironmentPrefix names the variables that carry a caller's Go command
// settings while ddtest replaces them for its own go test invocation.
const SavedEnvironmentPrefix = "DDTEST_ORIGINAL_"

// SaveEnvironment records name's current value for environmentHook: "=value"
// when it is set, or an empty value when it is unset.
func SaveEnvironment(name string) string {
	if value, ok := os.LookupEnv(name); ok {
		return SavedEnvironmentPrefix + name + "==" + value
	}
	return SavedEnvironmentPrefix + name + "="
}

// environmentHook restores the settings that ddtest replaces only for go test.
// Packages that import testing initialize after it, so tests and the go
// commands they start see the caller's workspace and flags.
const environmentHook = `func init() {
 for _, name := range [...]string{"GOWORK", "GOFLAGS"} {
  key := "` + SavedEnvironmentPrefix + `" + name
  saved, ok := __dd_ci_os.LookupEnv(key)
  if !ok {
   continue
  }
  _ = __dd_ci_os.Unsetenv(key)
  if saved != "" && saved[0] == '=' {
   _ = __dd_ci_os.Setenv(name, saved[1:])
  } else {
   _ = __dd_ci_os.Unsetenv(name)
  }
 }
}
`

// ParallelStopHook lets Mini's in-process retries record the end of a parallel
// test in testing's own accounting, which only testing's tRunner updates. A
// retry attempt that calls Parallel ends outside tRunner; without the record,
// testing.AllocsPerRun panics in every later test of the binary. The linker
// rejects any other access to the counter.
const ParallelStopHook = `//go:linkname __dd_civisibility_registerTestingParallelStop github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/civisibility/integrations/gotesting.registerTestingParallelStop
func __dd_civisibility_registerTestingParallelStop(func())
func init() { __dd_civisibility_registerTestingParallelStop(func() { parallelStop.Add(1) }) }
`

// declaresParallelStop reports whether this AST declares the counter with the
// exact type used by ParallelStopHook. Locals, comments and other types do not
// enable the hook.
func declaresParallelStop(file *ast.File) bool {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			value := spec.(*ast.ValueSpec)
			selector, ok := value.Type.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "Int64" {
				continue
			}
			if pkg, ok := selector.X.(*ast.Ident); !ok || pkg.Name != "atomic" {
				continue
			}
			for _, ident := range value.Names {
				if ident.Name == "parallelStop" {
					return true
				}
			}
		}
	}
	return false
}

// FuzzHook preserves the callback's concrete type through the native F.Fuzz hook.
const FuzzHook = `
//go:linkname __dd_civisibility_instrumentTestingFuzzFunc github.com/DataDog/dd-trace-go/v2/internal/civisibility/integrations/gotesting.instrumentTestingFuzzFunc
func __dd_civisibility_instrumentTestingFuzzFunc(any) any
`
