package coverage

import (
	"runtime"
	"slices"
	"sync/atomic"
	"testing"
	"unsafe"
)

// contextHarness stands in for the runtime's edits: one goroutine, whose
// context the test switches, and the scratch window.
type contextHarness struct {
	t       *testing.T
	current unsafe.Pointer
	names   []string
	base    uintptr
	scratch [8]uint64
}

// withContextCoverage registers one context bank of names and makes context
// coverage available, restoring the process's state afterwards.
func withContextCoverage(t *testing.T, names ...string) *contextHarness {
	t.Helper()
	h := &contextHarness{t: t, names: names}
	saved := struct {
		words          uint64
		banks, unknown unsafe.Pointer
		get            func() unsafe.Pointer
		set            func(unsafe.Pointer)
		send           func(*uintptr, unsafe.Pointer) unsafe.Pointer
		receive        func(uintptr, unsafe.Pointer) (unsafe.Pointer, unsafe.Pointer)
		io             func(unsafe.Pointer) unsafe.Pointer
		uploadEnabled  bool
		previousMode   string
	}{fileCoverageWords, fileCoverageBanksV2, fileCoverageUnknown, fileCoverageGetContext, fileCoverageSetContext, fileCoverageSendHook, fileCoverageReceiveHook, fileCoverageIOHook, coverageUploadEnabled, mode}
	t.Cleanup(func() {
		fileCoverageWords, fileCoverageBanksV2, fileCoverageUnknown = saved.words, saved.banks, saved.unknown
		fileCoverageGetContext, fileCoverageSetContext = saved.get, saved.set
		fileCoverageSendHook, fileCoverageReceiveHook, fileCoverageIOHook = saved.send, saved.receive, saved.io
		coverageUploadEnabled, mode = saved.uploadEnabled, saved.previousMode
		contextCoverage = contextState{}
	})
	contextCoverage = contextState{}
	fileCoverageWords = uint64(len(names)+63) / 64
	fileCoverageUnknown = unsafe.Pointer(&h.scratch[0])
	fileCoverageGetContext = func() unsafe.Pointer { return h.current }
	fileCoverageSetContext = func(ctx unsafe.Pointer) { h.current = ctx }
	bank := &fileCoverageContextBank{version: fileCoverageContextBankVersion, base: &h.base, n: uintptr(len(names)), prefix: "example.com/p", path: "example.com/p"}
	if len(names) > 0 {
		bank.names = &names[0]
	}
	fileCoverageBanksV2 = unsafe.Pointer(bank)
	coverageUploadEnabled, mode = true, ""
	if !FileCoverageActive() || !contextCoverageAvailable() {
		t.Fatal("context file coverage is not active with a bank")
	}
	return h
}

// hit sets file i's bit in the current goroutine's file set, as a site does.
func (h *contextHarness) hit(i int) {
	c := h.current
	if c == nil {
		c = atomic.LoadPointer(&fileCoverageUnknown)
	}
	setBit(unsafe.Slice((*uint64)(c), contextCoverage.words), i)
}

// send and receive run the runtime's hooks for a channel's union.
func (h *contextHarness) send(slot *uintptr) {
	if h.current != nil {
		fileCoverageSendHook(slot, h.current)
	}
}

func (h *contextHarness) receive(slot *uintptr) {
	if *slot != 0 {
		h.current, _ = fileCoverageReceiveHook(*slot, h.current)
	}
}

// in runs f as a goroutine that starts with ctx, and returns its context
// afterwards.
func (h *contextHarness) in(ctx unsafe.Pointer, f func()) unsafe.Pointer {
	saved := h.current
	h.current = ctx
	f()
	ctx, h.current = h.current, saved
	return ctx
}

func (h *contextHarness) files(t *testing.T, epoch uint64) []string {
	t.Helper()
	got := scanContextCoverage(epoch)
	slices.Sort(got)
	return got
}

func TestContextCoverageAttributesToEachTest(t *testing.T) {
	h := withContextCoverage(t, "/repo/a.go", "/repo/b.go", "/repo/c.go")
	a := beginContextCoverage()
	ctxA := h.current
	b := beginContextCoverage()
	ctxB := h.current
	h.in(ctxA, func() { h.hit(0) })
	h.in(ctxB, func() { h.hit(1) })
	// A goroutine without a context runs for every running test.
	h.in(nil, func() { h.hit(2) })
	if got := h.files(t, a); !slices.Equal(got, []string{"/repo/a.go", "/repo/c.go"}) {
		t.Errorf("A: %v", got)
	}
	if got := h.files(t, b); !slices.Equal(got, []string{"/repo/b.go", "/repo/c.go"}) {
		t.Errorf("B: %v", got)
	}
}

func TestContextCoverageMergesChannelUnions(t *testing.T) {
	h := withContextCoverage(t, "/repo/a.go", "/repo/b.go")
	a := beginContextCoverage()
	ctxA := h.current
	b := beginContextCoverage()
	ctxB := h.current
	var slot uintptr
	h.in(ctxB, func() { h.send(&slot) })
	// A receives B's work: what A runs next counts for both.
	merged := h.in(ctxA, func() {
		h.receive(&slot)
		h.hit(1)
	})
	if merged == ctxA {
		t.Fatal("the receive kept A's context")
	}
	if got := h.files(t, a); !slices.Equal(got, []string{"/repo/b.go"}) {
		t.Errorf("A: %v", got)
	}
	if got := h.files(t, b); !slices.Equal(got, []string{"/repo/b.go"}) {
		t.Errorf("B: %v", got)
	}
	// A receive that adds nothing keeps the context.
	h.in(ctxA, func() {
		var own uintptr
		h.send(&own)
		h.receive(&own)
		if h.current != ctxA {
			t.Error("a receive of A's own union changed A's context")
		}
	})
}

// A call's work reaches its joiners through a worker without a context and a
// second hop to a worker of another test.
func TestContextCoverageKeepsCallsAcrossUnknownWorkers(t *testing.T) {
	h := withContextCoverage(t, "/repo/f.go", "/repo/g.go")
	a := beginContextCoverage()
	ctxA := h.current
	d := beginContextCoverage()
	ctxD := h.current
	var call unsafe.Pointer
	var toWorker, toV uintptr
	leader := h.in(ctxA, func() {
		fileCoverageEnter(&call)
		h.send(&toWorker)
	})
	// W, started by TestMain, receives the call's work and hands it on.
	worker := h.in(nil, func() {
		h.receive(&toWorker)
		h.send(&toV)
	})
	if worker == nil || !contextCoverage.lookup(uintptr(worker)).public {
		t.Fatal("the worker without a context dropped the call")
	}
	h.in(ctxD, func() {
		h.receive(&toV)
		h.hit(0)
	})
	h.in(leader, func() { fileCoverageLeave(&call) })
	b := beginContextCoverage()
	h.in(h.current, func() { fileCoverageJoin(&call) })
	h.in(leader, func() { fileCoverageComplete(&call) })
	if got := h.files(t, b); !slices.Contains(got, "/repo/f.go") {
		t.Errorf("the joiner lacks the call's file from the second hop: %v", got)
	}
	h.files(t, a)
	h.files(t, d)
}

func TestContextCoverageIOKeepsCalls(t *testing.T) {
	h := withContextCoverage(t, "/repo/f.go")
	beginContextCoverage()
	var call unsafe.Pointer
	public := h.in(h.current, func() {
		fileCoverageEnter(&call)
		h.current = fileCoverageIOHook(h.current)
	})
	if ctx := contextCoverage.lookup(uintptr(public)); ctx == nil || !ctx.public || len(ctx.sinks) != 1 || !ctx.sinks[0].call {
		t.Fatalf("after IO: %+v", ctx)
	}
	// Without calls, the goroutine loses its context.
	if next := h.in(nil, func() {}); next != nil {
		t.Fatal("a goroutine without a context gained one")
	}
	beginContextCoverage()
	if next := fileCoverageIOHook(h.current); next != nil {
		t.Fatal("IO kept a test's context")
	}
}

func TestContextCoverageOverflow(t *testing.T) {
	h := withContextCoverage(t, "/repo/a.go")
	var slot uintptr
	var epochs []uint64
	for range maxContextSinks + 1 {
		epochs = append(epochs, beginContextCoverage())
		h.send(&slot)
	}
	if contextCoverage.lookup(slot) != contextCoverage.overflow {
		t.Fatal("the union did not overflow")
	}
	ctx := h.in(h.current, func() { h.receive(&slot) })
	if ctx != nil {
		t.Fatal("the receiver of an overflowed union kept a context")
	}
	h.in(nil, func() { h.hit(0) })
	for _, epoch := range epochs {
		if got := h.files(t, epoch); !slices.Contains(got, "/repo/a.go") {
			t.Errorf("test %d lacks the file of an overflowed union's receiver: %v", epoch, got)
		}
	}
}

// Contexts and windows of finished tests are dropped, so a long run keeps
// what its running tests need.
func TestContextCoverageRetention(t *testing.T) {
	h := withContextCoverage(t, "/repo/a.go")
	var slot uintptr
	for range 1000 {
		epoch := beginContextCoverage()
		h.send(&slot)
		h.hit(0)
		h.files(t, epoch)
	}
	if n, w := len(contextCoverage.byKey), len(contextCoverage.windows); n > 3 || w > 1 {
		t.Fatalf("after 1000 tests: %d contexts, %d windows", n, w)
	}
}

// A call that joins itself, or two calls that join each other, are dropped
// once retired and no live test reaches them.
func TestContextCoverageReclaimsCallCycles(t *testing.T) {
	h := withContextCoverage(t, "/repo/a.go")
	epoch := beginContextCoverage()
	var c, d unsafe.Pointer
	h.in(h.current, func() {
		fileCoverageEnter(&c)
		fileCoverageJoin(&c)
		fileCoverageEnter(&d)
		fileCoverageJoin(&c)
		fileCoverageLeave(&d)
		fileCoverageJoin(&d)
		fileCoverageLeave(&c)
	})
	fileCoverageComplete(&c)
	fileCoverageComplete(&d)
	h.files(t, epoch)
	if n := len(contextCoverage.calls); n != 0 {
		t.Fatalf("%d calls left after their test finished", n)
	}
}

// A joiner that may work for anyone gets the call's files in the window
// before the call's waiters return.
func TestContextCoverageUnknownJoiner(t *testing.T) {
	h := withContextCoverage(t, "/repo/f.go")
	a := beginContextCoverage()
	var call unsafe.Pointer
	leader := h.in(h.current, func() { fileCoverageEnter(&call) })
	b := beginContextCoverage()
	h.in(nil, func() { fileCoverageJoin(&call) })
	h.in(leader, func() {
		h.hit(0)
		fileCoverageLeave(&call)
	})
	fileCoverageComplete(&call)
	if got := h.files(t, b); !slices.Contains(got, "/repo/f.go") {
		t.Errorf("a test running while an unknown goroutine joined lacks the call's file: %v", got)
	}
	h.files(t, a)
}

// A merge credits the files of the receiver's stack: here, this file's,
// which a bank names. A frame of an instrumented package whose file maps to
// none credits every file.
func TestContextCoverageRepairsTheStack(t *testing.T) {
	_, self, _, _ := runtime.Caller(0)
	h := withContextCoverage(t, self, "/repo/other.go")
	a := beginContextCoverage()
	ctxA := h.current
	b := beginContextCoverage()
	var slot uintptr
	h.send(&slot)
	h.in(ctxA, func() { h.receive(&slot) })
	if got := h.files(t, b); !slices.Equal(got, []string{self}) {
		t.Errorf("B after A received its work: %v", got)
	}
	h.files(t, a)
	contextCoverage.symbols[symbolPrefix(funcName(t))] = true
	delete(contextCoverage.files, self)
	target := make([]uint64, contextCoverage.words)
	contextCoverage.repair(target)
	if target[0] != ^uint64(0) {
		t.Errorf("an unmapped frame of an instrumented package credited %#x", target[0])
	}
}

func funcName(t *testing.T) string {
	pc, _, _, _ := runtime.Caller(1)
	return runtime.FuncForPC(pc).Name()
}

func TestSymbolPrefix(t *testing.T) {
	for function, want := range map[string]string{
		"example.com/m.v2/dot%2epkg_test.TestWhere": "example.com/m.v2/dot%2epkg_test",
		"example.com/p.(*T).M":                      "example.com/p",
		"example.com/p.F[...]":                      "example.com/p",
		"example.com/p.F.func1":                     "example.com/p",
		"runtime.goexit":                            "runtime",
		"example.com/p.G[go.shape.struct { x.T }]":  "example.com/p",
	} {
		if got := symbolPrefix(function); got != want {
			t.Errorf("symbolPrefix(%q) = %q, want %q", function, got, want)
		}
	}
}
