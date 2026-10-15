package instrument

import (
	"bytes"
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
//
// Reset returns signals to their default action for every channel, Mini's
// included. TransformSignal edits it to register Mini's channel again for the
// signals that Mini wanted, so a signal that later ends the process still
// reaches Mini first. Mini's own stop goes through the hook, so that Reset
// does not register a channel that Mini has stopped.
//
// The hook sets Mini's variables instead of calling Mini: os/signal
// initializes before Mini, and a Mini function that coverage instruments
// would record its counters under package ID zero if it ran first.
const SignalHook = `package signal

import (
	"os"
	"sync"
	_ "unsafe"
)

//go:linkname __dd_civisibility_signalNotify github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/civisibility/integrations.signalNotify
var __dd_civisibility_signalNotify func(chan<- os.Signal, ...os.Signal)

//go:linkname __dd_civisibility_signalStop github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/civisibility/integrations.signalStop
var __dd_civisibility_signalStop func(chan<- os.Signal)

var (
	// __dd_civisibility_registration orders Mini's registration, its stop and
	// Reset's registration. Take it before handlers' lock.
	__dd_civisibility_registration sync.Mutex
	// Mini's channel and the signals it wants, written holding both locks.
	__dd_civisibility_signals chan<- os.Signal
	__dd_civisibility_wanted  []os.Signal
)

func init() {
	__dd_civisibility_signalNotify = func(c chan<- os.Signal, sig ...os.Signal) {
		__dd_civisibility_registration.Lock()
		defer __dd_civisibility_registration.Unlock()
		handlers.Lock()
		__dd_civisibility_signals, __dd_civisibility_wanted = c, sig
		handlers.Unlock()
		Notify(c, sig...)
	}
	__dd_civisibility_signalStop = func(c chan<- os.Signal) {
		__dd_civisibility_registration.Lock()
		defer __dd_civisibility_registration.Unlock()
		handlers.Lock()
		if __dd_civisibility_signals == c {
			__dd_civisibility_signals, __dd_civisibility_wanted = nil, nil
		}
		handlers.Unlock()
		Stop(c)
	}
}

// __dd_civisibility_signalWantedElsewhere reports whether c is Mini's channel
// and another channel wants signal n. process calls it holding handlers' lock.
func __dd_civisibility_signalWantedElsewhere(c chan<- os.Signal, n int) bool {
	if c != __dd_civisibility_signals {
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

// __dd_civisibility_reset runs Reset's reset of sig, all signals when empty,
// then registers Mini's channel again for the signals of sig that it wanted.
func __dd_civisibility_reset(sig []os.Signal, reset func()) {
	__dd_civisibility_registration.Lock()
	defer __dd_civisibility_registration.Unlock()
	reset()
	var again []os.Signal
	for _, s := range __dd_civisibility_wanted {
		for _, r := range sig {
			if r == s {
				again = append(again, s)
				break
			}
		}
		if len(sig) == 0 {
			again = append(again, s)
		}
	}
	if len(again) != 0 {
		Notify(__dd_civisibility_signals, again...)
	}
}
`

// signalHookAPI is every declaration of package os/signal that SignalHook
// uses, rendered as sourceDeclarations renders them. A toolchain that lacks
// one, or declares it otherwise, builds without the hook.
var signalHookAPI = []string{
	"embed handlers sync.Mutex",
	"field handlers.m map[chan<- os.Signal]*handler",
	"field handlers.stopping []stopping",
	"field stopping.c chan<- os.Signal",
	"field stopping.h *handler",
	"func (*handler) want(int) bool",
	"func Notify(chan<- os.Signal, ...os.Signal)",
	"func Stop(chan<- os.Signal)",
}

// signalRuntimeAPI is what SignalHook's linknames name in Mini's integrations
// package, rendered as sourceDeclarations renders it.
var signalRuntimeAPI = []string{
	"var signalNotify func(chan<- os.Signal, ...os.Signal)",
	"var signalStop func(chan<- os.Signal)",
}

// TransformSignal checks that package os/signal, given as the sources that the
// build compiles, declares what SignalHook uses, and returns its file whose
// process asks the hook about Mini's channel and whose Reset registers Mini's
// channel again. The edits stay on their lines, so os/signal keeps its
// positions.
func TransformSignal(files map[string][]byte) (map[string][]byte, error) {
	if err := declarationsError(files, signalHookAPI, "os/signal lacks %s"); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	deliveries, resets := 0, 0
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
		text := func(n ast.Node) string {
			return string(src[fs.Position(n.Pos()).Offset:fs.Position(n.End()).Offset])
		}
		var edits []edit
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Body == nil {
				continue
			}
			switch fn.Name.Name {
			case "process":
				for _, d := range processDeliveries(fn.Body) {
					edits = append(edits, edit{fs.Position(d.call.Pos()).Offset, fs.Position(d.call.End()).Offset,
						fmt.Sprintf("%s && !__dd_civisibility_signalWantedElsewhere(%s, %s)", text(d.call), d.channel, text(d.signal))})
					deliveries++
				}
			case "Reset":
				if call, param := resetCancel(fn); call != nil {
					edits = append(edits, edit{fs.Position(call.Pos()).Offset, fs.Position(call.End()).Offset,
						fmt.Sprintf("__dd_civisibility_reset(%s, func() { %s })", param, text(call))})
					resets++
				}
			}
		}
		if len(edits) == 0 {
			continue
		}
		sort.Slice(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
		var buf bytes.Buffer
		buf.WriteString("//line " + name + ":1\n")
		cursor := 0
		for _, e := range edits {
			buf.Write(src[cursor:e.start])
			buf.WriteString(e.text)
			cursor = e.end
		}
		buf.Write(src[cursor:])
		output[name] = buf.Bytes()
	}
	switch {
	case deliveries != 1:
		return nil, fmt.Errorf("os/signal's process delivers to handlers.m in %d places that TransformSignal recognizes, want 1", deliveries)
	case resets != 1:
		return nil, fmt.Errorf("os/signal declares %d Reset functions that TransformSignal recognizes, want 1", resets)
	}
	return output, nil
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

// resetCancel returns Reset's only statement, "cancel(sig, disableSignal)",
// and the name of its variadic os.Signal parameter, or nil.
func resetCancel(fn *ast.FuncDecl) (*ast.CallExpr, string) {
	params := fn.Type.Params.List
	if len(params) != 1 || len(params[0].Names) != 1 || render(params[0].Type) != "...os.Signal" || len(fn.Body.List) != 1 {
		return nil, ""
	}
	stmt, ok := fn.Body.List[0].(*ast.ExprStmt)
	if !ok {
		return nil, ""
	}
	call, ok := stmt.X.(*ast.CallExpr)
	if !ok || len(call.Args) != 2 || call.Ellipsis.IsValid() {
		return nil, ""
	}
	cancel, ok := call.Fun.(*ast.Ident)
	param, paramOK := call.Args[0].(*ast.Ident)
	action, actionOK := call.Args[1].(*ast.Ident)
	if !ok || !paramOK || !actionOK || cancel.Name != "cancel" || param.Name != params[0].Names[0].Name || action.Name != "disableSignal" {
		return nil, ""
	}
	return call, param.Name
}

// SignalHookRuntimeError reports why the selected runtime's integrations
// package, given as the sources that the build compiles, cannot receive
// SignalHook's registration, or nil.
func SignalHookRuntimeError(sources map[string][]byte) error {
	return declarationsError(sources, signalRuntimeAPI, "the runtime's integrations package lacks %s")
}
