package instrument

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
)

// TransformSDKMirror edits the compiler's actual SDK inputs, including covered
// sources. Opaque state belongs to SpanContext, never the recyclable Span.
func TransformSDKMirror(name string, source []byte) ([]byte, uint8, error) {
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, name, source, parser.SkipObjectResolution)
	if err != nil {
		return nil, 0, err
	}
	if bytes.Contains(source, []byte("__ddtest")) {
		return nil, 0, fmt.Errorf("%w: reserved SDK mirror name", ErrUnsupportedAPI)
	}
	var edits []edit
	var found uint8
	insert := func(pos token.Pos, code string) {
		offset := fs.PositionFor(pos, false).Offset
		edits = append(edits, edit{offset, offset, code})
	}
	for _, decl := range f.Decls {
		if group, ok := decl.(*ast.GenDecl); ok && group.Tok == token.TYPE {
			for _, spec := range group.Specs {
				typ := spec.(*ast.TypeSpec)
				if typ.Name.Name == "SpanContext" {
					structure, ok := typ.Type.(*ast.StructType)
					if !ok || found&1 != 0 {
						return nil, 0, fmt.Errorf("%w: SDK SpanContext changed", ErrUnsupportedAPI)
					}
					insert(structure.Fields.Opening+1, "\n__ddtestMirror any\n")
					found |= 1
				}
			}
		}
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		switch {
		case fn.Recv == nil && fn.Name.Name == "spanStart":
			if fn.Type.Results == nil || len(fn.Type.Results.List) != 1 || !sdkPointerType(fn.Type.Results.List[0].Type, "Span") {
				return nil, 0, fmt.Errorf("%w: SDK spanStart signature changed", ErrUnsupportedAPI)
			}
			matched := false
			for _, stmt := range fn.Body.List {
				assign, ok := stmt.(*ast.AssignStmt)
				if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 || !sdkSelector(assign.Lhs[0], "span", "context") {
					continue
				}
				call, ok := assign.Rhs[0].(*ast.CallExpr)
				if !ok || !sdkTypeName(call.Fun, "newSpanContext") || len(call.Args) != 2 || !sdkTypeName(call.Args[0], "span") || !sdkTypeName(call.Args[1], "context") {
					return nil, 0, fmt.Errorf("%w: SDK context construction changed", ErrUnsupportedAPI)
				}
				insert(stmt.End(), "\nvar __ddtestParent any; if context != nil { __ddtestParent = context.__ddtestMirror }; span.context.__ddtestMirror = __ddtestMirrorStart(pprofContext, __ddtestParent, operationName)\n")
				matched = true
			}
			if !matched || found&2 != 0 {
				return nil, 0, fmt.Errorf("%w: SDK span construction missing or ambiguous", ErrUnsupportedAPI)
			}
			found |= 2
		case sdkSpanReceiver(fn) && fn.Name.Name == "finish":
			if fn.Type.Params == nil || fn.Type.Params.NumFields() != 1 || len(fn.Type.Params.List[0].Names) != 1 || !sdkTypeName(fn.Type.Params.List[0].Type, "int64") {
				return nil, 0, fmt.Errorf("%w: SDK finish signature changed", ErrUnsupportedAPI)
			}
			// This defer is registered before the SDK's Unlock defer, so delivery
			// can never wait on Mini's queue while holding the SDK's span lock.
			insert(fn.Body.Lbrace+1, "\nvar __ddtestDelivery any; defer func(){ __ddtestMirrorDeliver(__ddtestDelivery) }()\n")
			matched := false
			locked, deferredUnlock := false, false
			for _, stmt := range fn.Body.List {
				if deferred, ok := stmt.(*ast.DeferStmt); ok && sdkLockCall(deferred.Call, "Unlock") {
					deferredUnlock = locked
				}
				expr, ok := stmt.(*ast.ExprStmt)
				if !ok {
					continue
				}
				call, ok := expr.X.(*ast.CallExpr)
				if ok && sdkLockCall(call, "Lock") {
					locked = true
				}
				if !ok || len(call.Args) != 1 || !sdkTypeName(call.Args[0], "s") {
					continue
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "finish" || !sdkSelector(selector.X, "s", "context") {
					continue
				}
				if !deferredUnlock {
					return nil, 0, fmt.Errorf("%w: SDK finish lock lifetime changed", ErrUnsupportedAPI)
				}
				insert(stmt.End(), "\nif s.context.__ddtestMirror != nil { __ddtestDelivery = __ddtestMirrorCapture(s.context.__ddtestMirror,s.name,s.service,s.resource,s.spanType,s.start,s.duration,s.error,s.meta.All(),s.metrics,s.context.TraceIDBytes(),s.spanID) }\n")
				matched = true
			}
			if !matched || found&4 != 0 {
				return nil, 0, fmt.Errorf("%w: SDK finish bookkeeping missing or ambiguous", ErrUnsupportedAPI)
			}
			found |= 4
		case fn.Recv == nil && fn.Name.Name == "ContextWithSpan":
			if fn.Type.Params == nil || fn.Type.Params.NumFields() != 2 || len(fn.Type.Params.List) != 2 || len(fn.Type.Params.List[0].Names) != 1 || fn.Type.Params.List[0].Names[0].Name != "ctx" || len(fn.Type.Params.List[1].Names) != 1 || fn.Type.Params.List[1].Names[0].Name != "s" || !sdkPointerType(fn.Type.Params.List[1].Type, "Span") {
				return nil, 0, fmt.Errorf("%w: SDK ContextWithSpan signature changed", ErrUnsupportedAPI)
			}
			matched := false
			snapshot := false
			for _, stmt := range fn.Body.List {
				if declaration, ok := stmt.(*ast.DeclStmt); ok {
					if group, ok := declaration.Decl.(*ast.GenDecl); ok && group.Tok == token.VAR {
						for _, spec := range group.Specs {
							value := spec.(*ast.ValueSpec)
							if len(value.Names) == 1 && value.Names[0].Name == "snapshot" && sdkPointerType(value.Type, "SpanContext") {
								snapshot = true
							}
						}
					}
				}
				assign, ok := stmt.(*ast.AssignStmt)
				if !ok || len(assign.Lhs) != 1 || !sdkTypeName(assign.Lhs[0], "newCtx") {
					continue
				}
				if !snapshot {
					return nil, 0, fmt.Errorf("%w: SDK context snapshot changed", ErrUnsupportedAPI)
				}
				insert(stmt.Pos(), "var __ddtestState any; if snapshot != nil { __ddtestState = snapshot.__ddtestMirror }; ctx = __ddtestMirrorContext(ctx,__ddtestState)\n")
				matched = true
			}
			if !matched || found&8 != 0 {
				return nil, 0, fmt.Errorf("%w: SDK context return missing or ambiguous", ErrUnsupportedAPI)
			}
			found |= 8
		}
	}
	if found == 0 {
		return source, 0, nil
	}
	result, err := applyTestifyEdits(name, source, edits)
	return result, found, err
}

func sdkPointerType(expr ast.Expr, name string) bool {
	star, ok := expr.(*ast.StarExpr)
	return ok && sdkTypeName(star.X, name)
}
func sdkSelector(expr ast.Expr, receiver, field string) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == field && sdkTypeName(selector.X, receiver)
}
func sdkSpanReceiver(fn *ast.FuncDecl) bool {
	return fn.Recv != nil && len(fn.Recv.List) == 1 && len(fn.Recv.List[0].Names) == 1 && fn.Recv.List[0].Names[0].Name == "s" && sdkPointerType(fn.Recv.List[0].Type, "Span")
}

func sdkLockCall(call *ast.CallExpr, method string) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	return ok && len(call.Args) == 0 && selector.Sel.Name == method && sdkSelector(selector.X, "s", "mu")
}

const SDKMirrorHook = `package tracer
import (
 "context"
 _ "unsafe"
)
//go:linkname __ddtestMirrorEnable github.com/tonyredondo/dd-ci-testing-poc/internal/minitracer.sdkMirrorEnable
func __ddtestMirrorEnable()
func init(){ __ddtestMirrorEnable() }
//go:linkname __ddtestMirrorStart github.com/tonyredondo/dd-ci-testing-poc/internal/minitracer.sdkMirrorStart
func __ddtestMirrorStart(context.Context,any,string) any
//go:linkname __ddtestMirrorContext github.com/tonyredondo/dd-ci-testing-poc/internal/minitracer.sdkMirrorContext
func __ddtestMirrorContext(context.Context,any) context.Context
//go:linkname __ddtestMirrorCapture github.com/tonyredondo/dd-ci-testing-poc/internal/minitracer.sdkMirrorCapture
func __ddtestMirrorCapture(any,string,string,string,string,int64,int64,int32,func(func(string,string)bool),map[string]float64,[16]byte,uint64) any
//go:linkname __ddtestMirrorDeliver github.com/tonyredondo/dd-ci-testing-poc/internal/minitracer.sdkMirrorDeliver
func __ddtestMirrorDeliver(any)
`
