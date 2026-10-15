package instrument

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
)

// PanicHook is the runtime file that ddto's --panic-hook adds: the variable
// that TransformPanic's gopanic calls with an unrecovered panic's value, once
// the panicking goroutine has run its deferred calls and before the runtime
// prints the panic and exits. Mini's integrations package declares the
// variable, without an initializer, and sets it while it initializes; before
// that, or in a binary without Mini, it stays nil. Like the os/signal hook,
// the runtime sets nothing in Mini and calls Mini only through the variable.
const PanicHook = `package runtime

import _ "unsafe"

//go:linkname __dd_civisibility_unhandledPanic github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/civisibility/integrations.unhandledPanic
var __dd_civisibility_unhandledPanic func(any)
`

// panicRuntimeAPI is what PanicHook's linkname names in Mini's integrations
// package. Released runtimes before it lack the variable.
var panicRuntimeAPI = []string{"var unhandledPanic func(any)"}

// panicToolchainAPI is what TransformPanic's edit uses in the runtime.
var panicToolchainAPI = []string{"field _panic.arg any", "func preprintpanics(*_panic)"}

// PanicHookRuntimeError reports why the selected runtime's integrations
// package, given as the sources that the build compiles, cannot receive
// PanicHook's call, or nil.
func PanicHookRuntimeError(sources map[string][]byte) error {
	return declarationsError(sources, panicRuntimeAPI, "the runtime's integrations package lacks %s")
}

// TransformPanic checks that the runtime, given as the sources that the build
// compiles, ends an unrecovered panic in gopanic as TransformPanic expects,
// and returns gopanic's file, edited to call PanicHook's variable right after
// its single preprintpanics call, with the panic's value as preprintpanics
// left it: the result of its Error or String method, which the runtime calls
// there for the last time before it stops the other goroutines. The hook
// thus calls no method of the value again. The edit stays on its line, so
// the runtime keeps its positions.
func TransformPanic(files map[string][]byte) (map[string][]byte, error) {
	if err := declarationsError(files, panicToolchainAPI, "the runtime lacks %s"); err != nil {
		return nil, err
	}
	output := map[string][]byte{}
	calls := 0
	for name, src := range files {
		if bytes.Contains(src, []byte("__dd_civisibility_")) {
			return nil, fmt.Errorf("%s: already instrumented", name)
		}
		fs := token.NewFileSet()
		file, err := parser.ParseFile(fs, name, src, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Body == nil || fn.Name.Name != "gopanic" {
				continue
			}
			for _, stmt := range fn.Body.List {
				expr, ok := stmt.(*ast.ExprStmt)
				if !ok {
					continue
				}
				call, ok := expr.X.(*ast.CallExpr)
				if !ok {
					continue
				}
				if id, ok := call.Fun.(*ast.Ident); !ok || id.Name != "preprintpanics" || len(call.Args) != 1 {
					continue
				}
				// preprintpanics(&p), where p is gopanic's _panic.
				arg, ok := call.Args[0].(*ast.UnaryExpr)
				if !ok || arg.Op != token.AND {
					continue
				}
				panicked, ok := arg.X.(*ast.Ident)
				if !ok {
					continue
				}
				calls++
				at := fs.Position(stmt.End()).Offset
				var buf bytes.Buffer
				buf.WriteString("//line " + name + ":1\n")
				buf.Write(src[:at])
				fmt.Fprintf(&buf, "; if __dd_civisibility_unhandledPanic != nil { __dd_civisibility_unhandledPanic(%s.arg) }", panicked.Name)
				buf.Write(src[at:])
				output[name] = buf.Bytes()
			}
		}
	}
	if calls != 1 {
		return nil, fmt.Errorf("the runtime's gopanic calls preprintpanics in %d statements that TransformPanic recognizes, want 1", calls)
	}
	return output, nil
}
