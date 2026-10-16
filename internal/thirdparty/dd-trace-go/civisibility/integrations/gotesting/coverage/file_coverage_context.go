package coverage

import (
	"math/bits"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/log"
)

// Context file coverage is ddto's --experimental-file-coverage=context. Each
// goroutine carries a context, the tests and singleflight calls (its sinks)
// that it works for, with one bit per instrumented file, which a site sets
// when it runs. ddto's edits of the runtime give a goroutine its creator's
// context and add a channel's senders to its receivers. A goroutine that
// reads a socket, pipe or terminal may work for anyone from then on, as may
// one that never had a context:
//   - without calls, it has no context, and its sites set bits in the window
//     of the test that started last, which every test that runs meanwhile
//     reads;
//   - with calls, it keeps them in a public context, whose bits every test
//     reads, and the calls' joiners too.
//
// A test's files are those of every context that held it, or held a call that
// it joined, of every public context, and of every window since it started.
//
// The runtime and the banks reach these variables through linkname. The
// runtime initializes fileCoverageUnknown and the two context functions, and
// calls the hooks, which this package sets before a test can start.
var (
	fileCoverageWords       uint64
	fileCoverageBanksV2     unsafe.Pointer
	fileCoverageUnknown     unsafe.Pointer
	fileCoverageGetContext  func() unsafe.Pointer
	fileCoverageSetContext  func(unsafe.Pointer)
	fileCoverageSendHook    func(slot *uintptr, ctx unsafe.Pointer) (sent unsafe.Pointer)
	fileCoverageReceiveHook func(union uintptr, ctx unsafe.Pointer) (merged, seen unsafe.Pointer)
	fileCoverageIOHook      func(ctx unsafe.Pointer) (next unsafe.Pointer)
)

// fileCoverageContextBankVersion is the layout of fileCoverageContextBank,
// which ddto's generated code repeats
// (instrument.FileCoverageContextBankVersion).
const fileCoverageContextBankVersion = 2

// fileCoverageContextBank is one package variant's record. Its files' bits
// start at word *base; prefix is the package's symbol prefix, and path the
// import path that -trimpath gives its directory.
type fileCoverageContextBank struct {
	next    unsafe.Pointer
	version uintptr
	base    *uintptr
	names   *string
	n       uintptr
	always  *string
	nAlways uintptr
	prefix  string
	path    string
}

// maxContextSinks bounds a context. A goroutine whose context would hold more
// live sinks keeps only its calls, in a public context; with too many calls,
// it keeps none, and they leak.
const maxContextSinks = 32

// A contextSink is a test or a singleflight call.
type contextSink struct {
	id   uint64
	call bool
	// epoch is the current window's when a test starts.
	epoch uint64
	// dead: a test after its capture; a call once retired, when no live test
	// and no call in flight reaches it through joins.
	dead    bool
	retired bool
	// A call leaks once some of its work goes to windows, from the window
	// leakyEpoch on; unknownJoined, once a goroutine that may work for anyone
	// joined it.
	leaky         bool
	leakyEpoch    uint64
	unknownJoined bool
	// joined are the calls that this sink joined, and contexts those that
	// hold it.
	joined   []*contextSink
	contexts []*fileContext
}

// A fileContext is an interned set of sinks with its file bits. Goroutines
// point at the bits' first word. A public context holds calls only, and
// every test reads its bits.
type fileContext struct {
	bits   []uint64
	sinks  []*contextSink
	public bool
	key    string
}

func (c *fileContext) pointer() unsafe.Pointer {
	if c == nil {
		return nil
	}
	return unsafe.Pointer(&c.bits[0])
}

type fileWindow struct {
	epoch uint64
	bits  []uint64
}

type contextState struct {
	once      sync.Once
	available bool

	// Set once, read without the lock.
	words   int
	names   []string
	always  []string
	files   map[string]int
	symbols map[string]bool

	mu      sync.Mutex
	byBits  map[uintptr]*fileContext
	byKey   map[string]*fileContext
	public  []*fileContext
	tests   map[uint64]*contextSink
	calls   map[*contextSink]bool
	windows []fileWindow
	epoch   uint64
	nextID  uint64
	// void holds no live sink; overflow stands for a channel union with more
	// live sinks than a context can hold.
	void, overflow *fileContext
	stats          struct{ merges, walks, incomplete, overflows, io, leaky int }
}

var contextCoverage contextState

// contextCoverageAvailable reports registered context banks of the known
// layout, with the runtime's edits.
func contextCoverageAvailable() bool {
	c := &contextCoverage
	c.once.Do(func() {
		if fileCoverageGetContext == nil || fileCoverageSetContext == nil || atomic.LoadPointer(&fileCoverageUnknown) == nil {
			return
		}
		banks := 0
		for p := atomic.LoadPointer(&fileCoverageBanksV2); p != nil; p = (*fileCoverageContextBank)(p).next {
			if bank := (*fileCoverageContextBank)(p); bank.version != fileCoverageContextBankVersion {
				log.Debug("civisibility.cov: context file coverage off: bank layout %d, want %d", bank.version, fileCoverageContextBankVersion)
				return
			}
			banks++
		}
		if banks == 0 {
			return
		}
		c.setup()
		c.available = true
		log.Debug("civisibility.cov: per-test context file coverage: true [banks:%d words:%d]", banks, c.words)
	})
	return c.available
}

// setup indexes the banks' files. Every package has initialized, so every
// bank has reserved its words.
func (c *contextState) setup() {
	c.words = max(int(atomic.LoadUint64(&fileCoverageWords)), 1)
	c.names = make([]string, c.words*64)
	c.files, c.symbols = map[string]int{}, map[string]bool{}
	for p := atomic.LoadPointer(&fileCoverageBanksV2); p != nil; p = (*fileCoverageContextBank)(p).next {
		bank := (*fileCoverageContextBank)(p)
		c.symbols[bank.prefix] = true
		if bank.n > 0 {
			for k, name := range unsafe.Slice(bank.names, bank.n) {
				i := int(*bank.base)*64 + k
				c.names[i] = name
				c.files[filepath.ToSlash(name)] = i
				// -trimpath gives a file the import path of its directory.
				c.files[bank.path+"/"+filepath.Base(name)] = i
			}
		}
		if bank.nAlways > 0 {
			c.always = append(c.always, unsafe.Slice(bank.always, bank.nAlways)...)
		}
	}
	c.byBits, c.byKey = map[uintptr]*fileContext{}, map[string]*fileContext{}
	c.tests, c.calls = map[uint64]*contextSink{}, map[*contextSink]bool{}
	c.void = c.intern(nil, false)
	c.overflow = &fileContext{bits: make([]uint64, 1), key: "overflow"}
	c.byBits[uintptr(c.overflow.pointer())] = c.overflow
	fileCoverageSendHook, fileCoverageReceiveHook, fileCoverageIOHook = contextSend, contextReceive, contextIO
}

// beginContextCoverage starts a test: a new window becomes current, and the
// test's goroutine gets the context of the test alone. One lock orders the
// epochs and the windows' publication.
func beginContextCoverage() uint64 {
	c := &contextCoverage
	c.mu.Lock()
	c.epoch++
	window := fileWindow{c.epoch, make([]uint64, c.words)}
	c.windows = append(c.windows, window)
	atomic.StorePointer(&fileCoverageUnknown, unsafe.Pointer(&window.bits[0]))
	c.nextID++
	test := &contextSink{id: c.nextID, epoch: c.epoch}
	c.tests[c.epoch] = test
	ctx := c.intern([]*contextSink{test}, false)
	c.mu.Unlock()
	fileCoverageSetContext(ctx.pointer())
	return window.epoch
}

// scanContextCoverage captures the test that began at epoch: the files of
// every context that held it, or held a call that it joined, of every public
// context, of the windows since it began, or since such a call leaked to
// them, and the files that every test reports. The test's goroutine is left
// without a context.
func scanContextCoverage(epoch uint64) []string {
	c := &contextCoverage
	fileCoverageSetContext(nil)
	c.mu.Lock()
	defer c.mu.Unlock()
	test := c.tests[epoch]
	if test == nil {
		return nil
	}
	delete(c.tests, epoch)
	files := make([]uint64, c.words)
	from := test.epoch
	for _, sink := range c.closure([]*contextSink{test}) {
		for _, ctx := range sink.contexts {
			orBits(files, ctx.bits)
		}
		if sink.leaky {
			from = min(from, sink.leakyEpoch)
		}
	}
	for _, ctx := range c.public {
		orBits(files, ctx.bits)
	}
	for _, window := range c.windows {
		if window.epoch >= from {
			orBits(files, window.bits)
		}
	}
	c.kill(test)
	c.prune()
	var names []string
	for w, word := range files {
		for ; word != 0; word &= word - 1 {
			if name := c.names[w*64+bits.TrailingZeros64(word)]; name != "" {
				names = append(names, name)
			}
		}
	}
	log.Debug("civisibility.cov: context coverage [epoch:%d files:%d contexts:%d public:%d merges:%d walks:%d incomplete:%d overflows:%d io:%d leaky:%d]",
		epoch, len(names), len(c.byKey), len(c.public), c.stats.merges, c.stats.walks, c.stats.incomplete, c.stats.overflows, c.stats.io, c.stats.leaky)
	return append(names, c.always...)
}

// closure returns sinks and the calls that they joined, through every level.
func (c *contextState) closure(sinks []*contextSink) []*contextSink {
	seen := map[*contextSink]bool{}
	var out []*contextSink
	for _, sink := range sinks {
		if !seen[sink] {
			seen[sink] = true
			out = append(out, sink)
		}
	}
	for i := 0; i < len(out); i++ {
		for _, call := range out[i].joined {
			if !seen[call] {
				seen[call] = true
				out = append(out, call)
			}
		}
	}
	return out
}

// kill marks sink dead and drops the contexts whose sinks are all dead. A
// goroutine that still points at one keeps it alive; nobody reads its bits.
func (c *contextState) kill(sink *contextSink) {
	sink.dead = true
	delete(c.calls, sink)
	for _, ctx := range sink.contexts {
		if !ctx.public && !slices.ContainsFunc(ctx.sinks, func(s *contextSink) bool { return !s.dead }) {
			delete(c.byBits, uintptr(ctx.pointer()))
			delete(c.byKey, ctx.key)
		}
	}
}

// prune kills the retired calls that no live test and no call in flight
// reaches, and drops the windows that no live test or leaky call reads. The
// current window stays.
func (c *contextState) prune() {
	var roots []*contextSink
	for _, test := range c.tests {
		roots = append(roots, test)
	}
	for call := range c.calls {
		if !call.retired {
			roots = append(roots, call)
		}
	}
	needed := map[*contextSink]bool{}
	for _, sink := range c.closure(roots) {
		needed[sink] = true
	}
	from := c.epoch
	for call := range c.calls {
		if call.retired && !needed[call] {
			c.kill(call)
		} else if call.leaky {
			from = min(from, call.leakyEpoch)
		}
	}
	for _, test := range c.tests {
		from = min(from, test.epoch)
	}
	i := 0
	for i < len(c.windows) && c.windows[i].epoch < from {
		i++
	}
	c.windows = slices.Delete(c.windows, 0, i)
}

// intern returns the context of sinks, sorted by id.
func (c *contextState) intern(sinks []*contextSink, public bool) *fileContext {
	var key strings.Builder
	if public {
		key.WriteString("public:")
	}
	for _, sink := range sinks {
		key.WriteString(strconv.FormatUint(sink.id, 36))
		key.WriteByte(',')
	}
	if ctx := c.byKey[key.String()]; ctx != nil {
		return ctx
	}
	ctx := &fileContext{bits: make([]uint64, c.words), sinks: sinks, public: public, key: key.String()}
	c.byKey[ctx.key], c.byBits[uintptr(ctx.pointer())] = ctx, ctx
	for _, sink := range sinks {
		sink.contexts = append(sink.contexts, ctx)
	}
	if public {
		c.public = append(c.public, ctx)
	}
	return ctx
}

// unknown returns the context of a goroutine that may work for anyone and
// keeps sinks' calls: public, or none without calls. With too many calls, it
// keeps none, and they leak.
func (c *contextState) unknown(sinks []*contextSink) *fileContext {
	kept := calls(sinks)
	switch {
	case len(kept) == 0:
		return nil
	case len(kept) > maxContextSinks:
		c.stats.overflows++
		c.markLeaky(kept)
		return nil
	}
	return c.intern(kept, true)
}

// lookup returns the context at p, or nil for a context that was dropped, or
// that never was one. Channels keep unions as integers, which keep nothing
// alive; a dropped context's address can come back as another context's,
// which only adds that context's sinks.
func (c *contextState) lookup(p uintptr) *fileContext {
	if p == 0 {
		return nil
	}
	return c.byBits[p]
}

// live returns ctx's live sinks; nil for nil.
func live(ctx *fileContext) []*contextSink {
	if ctx == nil {
		return nil
	}
	var out []*contextSink
	for _, sink := range ctx.sinks {
		if !sink.dead {
			out = append(out, sink)
		}
	}
	return out
}

// calls returns the calls among sinks.
func calls(sinks []*contextSink) []*contextSink {
	return slices.DeleteFunc(slices.Clone(sinks), func(s *contextSink) bool { return !s.call })
}

// union merges two lists of sinks sorted by id.
func union(a, b []*contextSink) []*contextSink {
	out := make([]*contextSink, 0, len(a)+len(b))
	for len(a) > 0 || len(b) > 0 {
		switch {
		case len(b) == 0 || len(a) > 0 && a[0].id < b[0].id:
			out, a = append(out, a[0]), a[1:]
		case len(a) == 0 || b[0].id < a[0].id:
			out, b = append(out, b[0]), b[1:]
		default:
			out, a, b = append(out, a[0]), a[1:], b[1:]
		}
	}
	return out
}

// markLeaky marks the calls among sinks leaky: what runs for them from now on
// goes to windows.
func (c *contextState) markLeaky(sinks []*contextSink) {
	for _, sink := range sinks {
		if sink.call && !sink.leaky && !sink.dead {
			sink.leaky, sink.leakyEpoch = true, c.epoch
			c.stats.leaky++
		}
	}
}

// currentWindow returns the bits that goroutines without a context set.
func (c *contextState) currentWindow() []uint64 {
	if n := len(c.windows); n > 0 {
		return c.windows[n-1].bits
	}
	return nil
}

// target returns where the files of a goroutine that gets ctx go.
func (c *contextState) target(ctx *fileContext) []uint64 {
	if ctx == nil {
		return c.currentWindow()
	}
	return ctx.bits
}

// contextSend adds the sender's live sinks to a channel's union before the
// channel holds the sent value, and returns the union, which the sender's
// goroutine caches.
func contextSend(slot *uintptr, ctx unsafe.Pointer) unsafe.Pointer {
	c := &contextCoverage
	c.mu.Lock()
	defer c.mu.Unlock()
	current := c.lookup(atomic.LoadUintptr(slot))
	if current == c.overflow {
		return current.pointer()
	}
	existing := live(current)
	merged := union(existing, live(c.lookup(uintptr(ctx))))
	if current != nil && len(merged) == len(existing) {
		return current.pointer()
	}
	next := c.void
	if len(merged) > maxContextSinks {
		c.stats.overflows++
		c.markLeaky(merged)
		next = c.overflow
	} else if len(merged) > 0 {
		next = c.intern(merged, false)
	}
	atomic.StoreUintptr(slot, uintptr(next.pointer()))
	return next.pointer()
}

// contextReceive returns the receiver's context once it received a value
// from a channel with union, its own plus the union's live sinks, and the
// union, which the receiver's goroutine caches. A receiver that may work for
// anyone takes only the union's calls: every test reads its files anyway.
// When the context gains sinks, the files of the receiver's stack go to it:
// the rest of each frame's statement runs for them before the next site.
func contextReceive(u uintptr, ctx unsafe.Pointer) (unsafe.Pointer, unsafe.Pointer) {
	c := &contextCoverage
	c.mu.Lock()
	incoming := c.lookup(u)
	if incoming == nil {
		c.mu.Unlock()
		return ctx, nil
	}
	receiver := c.lookup(uintptr(ctx))
	unknown := ctx == nil || receiver != nil && receiver.public
	mine := live(receiver)
	added := live(incoming)
	if unknown {
		added = calls(added)
	}
	var next *fileContext
	switch merged := union(mine, added); {
	case incoming == c.overflow && !unknown:
		// The union's calls leaked when it overflowed; the receiver may
		// now work for any of its tests.
		next = c.unknown(mine)
	case len(merged) == len(mine):
		c.mu.Unlock()
		return ctx, incoming.pointer()
	case unknown || len(merged) > maxContextSinks:
		next = c.unknown(merged)
	default:
		next = c.intern(merged, false)
	}
	c.stats.merges++
	target := c.target(next)
	c.mu.Unlock()
	c.repair(target)
	return next.pointer(), incoming.pointer()
}

// contextIO returns the context of a goroutine that is about to read a
// socket, pipe or terminal: it may work for anyone from then on, so it keeps
// only its calls, and its stack's files go where its next hits do.
func contextIO(ctx unsafe.Pointer) unsafe.Pointer {
	c := &contextCoverage
	c.mu.Lock()
	current := c.lookup(uintptr(ctx))
	if current != nil && current.public {
		c.mu.Unlock()
		return ctx
	}
	c.stats.io++
	next := c.unknown(live(current))
	target := c.target(next)
	c.mu.Unlock()
	c.repair(target)
	return next.pointer()
}

// repair sets the bits of the files of every frame of the calling goroutine's
// stack in target, or of every file when it cannot tell them all: the walk
// stops before the goroutine's first frame, or a frame of an instrumented
// package maps to no file.
func (c *contextState) repair(target []uint64) {
	if target == nil {
		return
	}
	c.count(&c.stats.walks)
	pcs := make([]uintptr, 64)
	for {
		n := runtime.Callers(1, pcs)
		if n < len(pcs) {
			pcs = pcs[:n]
			break
		}
		if len(pcs) >= 1<<16 {
			c.credit(target)
			return
		}
		pcs = make([]uintptr, 4*len(pcs))
	}
	frames := runtime.CallersFrames(pcs)
	complete := false
	for {
		frame, more := frames.Next()
		if i, ok := c.files[filepath.ToSlash(frame.File)]; ok {
			setBit(target, i)
		} else if c.symbols[symbolPrefix(frame.Function)] {
			c.credit(target)
			return
		}
		complete = frame.Function == "runtime.goexit"
		if !more {
			break
		}
	}
	if !complete {
		c.credit(target)
	}
}

func (c *contextState) count(stat *int) {
	c.mu.Lock()
	*stat++
	c.mu.Unlock()
}

// credit sets every file's bit in target.
func (c *contextState) credit(target []uint64) {
	c.count(&c.stats.incomplete)
	for i := range target {
		atomic.StoreUint64(&target[i], ^uint64(0))
	}
}

// symbolPrefix returns the package part of a function's symbol name, as the
// linker escapes it.
func symbolPrefix(function string) string {
	if i := strings.IndexByte(function, '['); i >= 0 {
		function = function[:i]
	}
	slash := strings.LastIndexByte(function, '/')
	if dot := strings.IndexByte(function[slash+1:], '.'); dot >= 0 {
		return function[:slash+1+dot]
	}
	return function
}

// The singleflight edits call these with the address of a call's sink, which
// the first of them creates. The group's mutex is held for fileCoverageJoin
// and fileCoverageComplete.

// fileCoverageEnter adds the call's sink to the leader's context while it
// runs the call's function.
func fileCoverageEnter(p *unsafe.Pointer) {
	if !contextCoverageAvailable() {
		return
	}
	c := &contextCoverage
	c.mu.Lock()
	defer c.mu.Unlock()
	call := c.call(p)
	ctx := fileCoverageGetContext()
	current := c.lookup(uintptr(ctx))
	merged := union(live(current), []*contextSink{call})
	var next *fileContext
	if ctx == nil || current != nil && current.public || len(merged) > maxContextSinks {
		next = c.unknown(merged)
	} else {
		next = c.intern(merged, false)
	}
	fileCoverageSetContext(next.pointer())
}

// fileCoverageLeave removes the call's sink from the leader's context once
// the call's function returned.
func fileCoverageLeave(p *unsafe.Pointer) {
	if !contextCoverageAvailable() {
		return
	}
	c := &contextCoverage
	c.mu.Lock()
	defer c.mu.Unlock()
	call := (*contextSink)(atomic.LoadPointer(p))
	current := c.lookup(uintptr(fileCoverageGetContext()))
	if call == nil || current == nil || !slices.Contains(current.sinks, call) {
		return
	}
	rest := slices.DeleteFunc(live(current), func(s *contextSink) bool { return s == call })
	if current.public {
		fileCoverageSetContext(c.unknown(rest).pointer())
	} else {
		fileCoverageSetContext(c.intern(rest, false).pointer())
	}
}

// fileCoverageJoin subscribes the sinks of a caller that found the call in
// flight. A caller that may work for anyone makes the call publish its files
// to the window when it completes.
func fileCoverageJoin(p *unsafe.Pointer) {
	if !contextCoverageAvailable() {
		return
	}
	c := &contextCoverage
	c.mu.Lock()
	defer c.mu.Unlock()
	call := c.call(p)
	ctx := fileCoverageGetContext()
	current := c.lookup(uintptr(ctx))
	if ctx == nil || current != nil && current.public {
		call.unknownJoined = true
	}
	for _, sink := range live(current) {
		if sink != call && !slices.Contains(sink.joined, call) {
			sink.joined = append(sink.joined, call)
		}
	}
}

// fileCoverageComplete retires the call before its waiters return. A call
// that a goroutine that may work for anyone joined publishes its files to the
// window.
func fileCoverageComplete(p *unsafe.Pointer) {
	if !contextCoverageAvailable() {
		return
	}
	c := &contextCoverage
	c.mu.Lock()
	defer c.mu.Unlock()
	call := (*contextSink)(atomic.LoadPointer(p))
	if call == nil || call.retired {
		return
	}
	call.retired = true
	if window := c.currentWindow(); call.unknownJoined && window != nil {
		for _, sink := range c.closure([]*contextSink{call}) {
			for _, ctx := range sink.contexts {
				orBitsAtomic(window, ctx.bits)
			}
		}
	}
}

// call returns the call's sink, which it creates on first use. The call
// struct keeps it alive, and the mutex orders its creation.
func (c *contextState) call(p *unsafe.Pointer) *contextSink {
	if sink := (*contextSink)(atomic.LoadPointer(p)); sink != nil {
		return sink
	}
	c.nextID++
	sink := &contextSink{id: c.nextID, call: true, epoch: c.epoch}
	c.calls[sink] = true
	atomic.StorePointer(p, unsafe.Pointer(sink))
	return sink
}

func setBit(bits []uint64, i int) {
	w, bit := &bits[i/64], uint64(1)<<(i%64)
	for {
		old := atomic.LoadUint64(w)
		if old&bit != 0 || atomic.CompareAndSwapUint64(w, old, old|bit) {
			return
		}
	}
}

// orBits adds src's bits, which sites may be setting, to dst, which only the
// caller sees.
func orBits(dst, src []uint64) {
	for i := range dst {
		dst[i] |= atomic.LoadUint64(&src[i])
	}
}

// orBitsAtomic adds src's bits to dst, which sites may be setting too.
func orBitsAtomic(dst, src []uint64) {
	for i := range dst {
		if add := atomic.LoadUint64(&src[i]); add != 0 {
			for {
				old := atomic.LoadUint64(&dst[i])
				if old|add == old || atomic.CompareAndSwapUint64(&dst[i], old, old|add) {
					break
				}
			}
		}
	}
}
