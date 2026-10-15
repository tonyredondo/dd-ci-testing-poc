package instrument

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
)

// SignalHook is an overlay file for package os/signal. It gives Mini a Notify
// whose channel receives a signal only when no other channel wants it: when
// the signal would otherwise end the process. Mini then sends its events and
// ends the process with the same signal, and a test that handles SIGTERM or
// SIGINT itself, or the fuzzing coordinator's SIGINT, keeps the process alive
// as under go test.
//
// TransformSignal edits process, the only place that delivers signals to
// channels, to ask __dd_civisibility_signalWantedElsewhere about Mini's
// channel. The answer uses handlers' state under the lock that process
// already holds, so a channel that the test stops right after its delivery
// still counts. Channels that Stop is removing still receive the signal, as
// in process, and count too.
const SignalHook = `package signal

import (
	"os"
	"sync"
	_ "unsafe"
)

var (
	__dd_civisibility_signalsMu sync.Mutex
	__dd_civisibility_signals   chan<- os.Signal
)

//go:linkname __dd_civisibility_registerSignalNotify github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/civisibility/integrations.registerSignalNotify
func __dd_civisibility_registerSignalNotify(func(chan<- os.Signal, ...os.Signal))

func init() {
	__dd_civisibility_registerSignalNotify(func(c chan<- os.Signal, sig ...os.Signal) {
		__dd_civisibility_signalsMu.Lock()
		__dd_civisibility_signals = c
		__dd_civisibility_signalsMu.Unlock()
		Notify(c, sig...)
	})
}

// __dd_civisibility_signalWantedElsewhere reports whether c is Mini's channel
// and another channel wants signal n. process calls it holding handlers' lock.
func __dd_civisibility_signalWantedElsewhere(c chan<- os.Signal, n int) bool {
	__dd_civisibility_signalsMu.Lock()
	own := c == __dd_civisibility_signals
	__dd_civisibility_signalsMu.Unlock()
	if !own {
		return false
	}
	for other, h := range handlers.m {
		if other != c && h.want(n) {
			return true
		}
	}
	for _, d := range handlers.stopping {
		if d.c != c && d.h.want(n) {
			return true
		}
	}
	return false
}
`

// signalHookAPI is every declaration of package os/signal that SignalHook
// uses, rendered as sourceDeclarations renders them. A toolchain that lacks
// one, or declares it otherwise, builds without the hook.
var signalHookAPI = []string{
	"field handlers.m map[chan<- os.Signal]*handler",
	"field handlers.stopping []stopping",
	"field stopping.c chan<- os.Signal",
	"field stopping.h *handler",
	"func (*handler) want(int) bool",
	"func Notify(chan<- os.Signal, ...os.Signal)",
}

// signalRegistration is the function that SignalHook's linkname names in
// Mini's integrations package, rendered as sourceDeclarations renders it.
const signalRegistration = "func registerSignalNotify(func(chan<- os.Signal, ...os.Signal))"

// TransformSignal checks that package os/signal, given as the sources that the
// build compiles, declares what SignalHook uses, and returns its file whose
// process asks the hook about Mini's channel. The edit stays on its line, so
// os/signal keeps its positions.
func TransformSignal(files map[string][]byte) (map[string][]byte, error) {
	have, err := sourceDeclarations(files)
	if err != nil {
		return nil, err
	}
	for _, decl := range signalHookAPI {
		if !have[decl] {
			return nil, fmt.Errorf("os/signal lacks %s", decl)
		}
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	var found []string
	output := map[string][]byte{}
	for _, name := range names {
		src := files[name]
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
			if !ok || fn.Recv != nil || fn.Name.Name != "process" || fn.Body == nil {
				continue
			}
			for _, cond := range processDeliveries(fn.Body) {
				start, end := fs.Position(cond.call.Pos()).Offset, fs.Position(cond.call.End()).Offset
				var buf bytes.Buffer
				buf.WriteString("//line " + name + ":1\n")
				buf.Write(src[:start])
				fmt.Fprintf(&buf, "%s && !__dd_civisibility_signalWantedElsewhere(%s, %s)", src[start:end], cond.channel, src[fs.Position(cond.signal.Pos()).Offset:fs.Position(cond.signal.End()).Offset])
				buf.Write(src[end:])
				output[name] = buf.Bytes()
				found = append(found, name)
			}
		}
	}
	switch len(found) {
	case 0:
		return nil, errors.New("os/signal's process delivers to no handlers.m channel that TransformSignal recognizes")
	case 1:
		return output, nil
	default:
		return nil, fmt.Errorf("os/signal's process delivers to handlers.m in %d places", len(found))
	}
}

// signalDelivery is process's check that a channel of handlers.m wants the
// signal: the condition of "for c, h := range handlers.m { if h.want(n) {".
type signalDelivery struct {
	call    *ast.CallExpr
	channel string
	signal  ast.Expr
}

// processDeliveries returns the conditions that guard a delivery to a channel
// of handlers.m in process's body.
func processDeliveries(body *ast.BlockStmt) []signalDelivery {
	var found []signalDelivery
	ast.Inspect(body, func(n ast.Node) bool {
		loop, ok := n.(*ast.RangeStmt)
		if !ok {
			return true
		}
		over, ok := loop.X.(*ast.SelectorExpr)
		if !ok || over.Sel.Name != "m" {
			return true
		}
		if owner, ok := over.X.(*ast.Ident); !ok || owner.Name != "handlers" {
			return true
		}
		channel, ok := loop.Key.(*ast.Ident)
		if !ok {
			return true
		}
		handler, ok := loop.Value.(*ast.Ident)
		if !ok || len(loop.Body.List) == 0 {
			return true
		}
		check, ok := loop.Body.List[0].(*ast.IfStmt)
		if !ok || check.Init != nil {
			return true
		}
		call, ok := check.Cond.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return true
		}
		want, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || want.Sel.Name != "want" {
			return true
		}
		if id, ok := want.X.(*ast.Ident); !ok || id.Name != handler.Name {
			return true
		}
		found = append(found, signalDelivery{call: call, channel: channel.Name, signal: call.Args[0]})
		return true
	})
	return found
}

// SignalHookRuntimeError reports why the selected runtime's integrations
// package, given as the sources that the build compiles, cannot receive
// SignalHook's registration, or nil.
func SignalHookRuntimeError(sources map[string][]byte) error {
	have, err := sourceDeclarations(sources)
	if err != nil {
		return err
	}
	if !have[signalRegistration] || !have["linkname registerSignalNotify"] {
		return fmt.Errorf("the runtime's integrations package lacks %s", signalRegistration)
	}
	return nil
}
