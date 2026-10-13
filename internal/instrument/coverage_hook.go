package instrument

// CoverageHook is an overlay file for package testing that lets Mini's
// per-test coverage read the coverage counters in memory instead of asking
// testing/internal/testdeps for a text profile twice per test.
//
// It compiles against the toolchain's own internal/coverage packages. The
// layout reproduces what cfile.ProcessCoverTestDir and cformat would write for
// this process: units without a parent, merged by file, function, literal flag
// and position, packages in import path order and units in cformat's order.
// It refuses, and Mini keeps the text profiles, for counter modes or
// granularities that the text path would reject or that per-test coverage does
// not use, for file names with line breaks and for blocks that share a
// position in a file, where the text path's result depends on map order.
//
// counters copies every counter segment with atomic loads, each segment
// preceded by its length. counts maps such a copy to one count per layout
// entry, summing merged units as cformat does. profile renders a copy as the
// text profile that the text path would write, for Mini's debug comparison.
const CoverageHook = `package testing

import (
	"bytes"
	"cmp"
	"internal/coverage"
	"internal/coverage/cformat"
	"internal/coverage/decodemeta"
	"internal/coverage/rtcov"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"
)

//go:linkname __dd_civisibility_coverageCounterList internal/coverage/cfile.getCovCounterList
func __dd_civisibility_coverageCounterList() []rtcov.CovCounterBlob

//go:linkname __dd_civisibility_registerTestingCoverage github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/civisibility/integrations/gotesting/coverage.registerTestingCoverage
func __dd_civisibility_registerTestingCoverage(func() ([]string, []uint32, []byte, string), func([]uint32) ([]uint32, bool), func([]uint32, []uint32) ([]uint32, bool), func([]uint32) []byte)

func init() {
	__dd_civisibility_registerTestingCoverage(__dd_civisibility_coverageLayout, __dd_civisibility_coverageCounters, __dd_civisibility_coverageCounts, __dd_civisibility_coverageProfile)
}

type __dd_civisibility_coverageFunc struct{ start, units int32 }

var __dd_civisibility_coverage struct {
	once      sync.Once
	mode      coverage.CounterMode
	metaLen   int
	files     []string
	entries   []uint32
	text      []byte
	refusal   string
	funcs     [][]__dd_civisibility_coverageFunc
	unitEntry []int32
}

func __dd_civisibility_coverageLayout() ([]string, []uint32, []byte, string) {
	c := &__dd_civisibility_coverage
	c.once.Do(__dd_civisibility_coverageBuild)
	return c.files, c.entries, c.text, c.refusal
}

func __dd_civisibility_coverageBuild() {
	c := &__dd_civisibility_coverage
	ml := rtcov.Meta.List
	c.metaLen = len(ml)
	for _, b := range ml {
		switch m := coverage.CounterMode(b.CounterMode); {
		case m == coverage.CtrModeRegOnly || m == coverage.CtrModeTestMain:
		case c.mode == coverage.CtrModeInvalid:
			c.mode = m
		case m != c.mode:
			c.refusal = "counter mode clash"
			return
		}
		if coverage.CounterGranularity(b.CounterGranularity) != coverage.CtrGranularityPerBlock {
			c.refusal = "counter granularity"
			return
		}
	}
	if c.mode != coverage.CtrModeCount && c.mode != coverage.CtrModeAtomic {
		c.refusal = "counter mode"
		return
	}
	type unitKey struct {
		file, fname string
		lit         bool
		unit        coverage.CoverableUnit
	}
	type pkgUnits struct {
		keys   []unitKey
		index  map[unitKey]int32
		global []int32
	}
	type unitRef struct {
		pkg   *pkgUnits
		local int32
	}
	pkgs := map[string]*pkgUnits{}
	var paths []string
	var refs []unitRef
	formatter := cformat.NewFormatter(c.mode)
	c.funcs = make([][]__dd_civisibility_coverageFunc, len(ml))
	var fd coverage.FuncDesc
	for slot, b := range ml {
		d, err := decodemeta.NewCoverageMetaDataDecoder(unsafe.Slice(b.P, int(b.Len)), true)
		if err != nil {
			c.refusal = "meta-data: " + err.Error()
			return
		}
		path := d.PackagePath()
		p := pkgs[path]
		if p == nil {
			p = &pkgUnits{index: map[unitKey]int32{}}
			pkgs[path] = p
			paths = append(paths, path)
		}
		formatter.SetPackage(path)
		c.funcs[slot] = make([]__dd_civisibility_coverageFunc, d.NumFuncs())
		for fn := range c.funcs[slot] {
			if err := d.ReadFunc(uint32(fn), &fd); err != nil {
				c.refusal = "meta-data: " + err.Error()
				return
			}
			c.funcs[slot][fn] = __dd_civisibility_coverageFunc{start: int32(len(refs)), units: int32(len(fd.Units))}
			for _, u := range fd.Units {
				if u.Parent != 0 {
					refs = append(refs, unitRef{local: -1})
					continue
				}
				key := unitKey{fd.Srcfile, fd.Funcname, fd.Lit, u}
				local, ok := p.index[key]
				if !ok {
					local = int32(len(p.keys))
					p.index[key] = local
					p.keys = append(p.keys, key)
				}
				refs = append(refs, unitRef{p, local})
				formatter.AddUnit(fd.Srcfile, fd.Funcname, fd.Lit, u, ^uint32(0))
			}
		}
	}
	type position struct {
		file                         string
		stLine, stCol, enLine, enCol uint32
	}
	seen := map[position]bool{}
	fileIndex := map[string]uint32{}
	slices.Sort(paths)
	for _, path := range paths {
		p := pkgs[path]
		order := make([]int32, len(p.keys))
		for i := range order {
			order[i] = int32(i)
		}
		slices.SortFunc(order, func(i, j int32) int {
			a, b := &p.keys[i], &p.keys[j]
			return cmp.Or(strings.Compare(a.file, b.file),
				cmp.Compare(a.unit.StLine, b.unit.StLine), cmp.Compare(a.unit.EnLine, b.unit.EnLine),
				cmp.Compare(a.unit.StCol, b.unit.StCol), cmp.Compare(a.unit.EnCol, b.unit.EnCol),
				cmp.Compare(a.unit.NxStmts, b.unit.NxStmts))
		})
		p.global = make([]int32, len(p.keys))
		for _, local := range order {
			k := &p.keys[local]
			if strings.ContainsAny(k.file, "\r\n") {
				c.refusal = "file name with a line break"
				return
			}
			at := position{k.file, k.unit.StLine, k.unit.StCol, k.unit.EnLine, k.unit.EnCol}
			if seen[at] {
				c.refusal = "blocks share a position"
				return
			}
			seen[at] = true
			file, ok := fileIndex[k.file]
			if !ok {
				file = uint32(len(c.files))
				fileIndex[k.file] = file
				c.files = append(c.files, k.file)
			}
			p.global[local] = int32(len(c.entries) / 6)
			c.entries = append(c.entries, file, k.unit.StLine, k.unit.StCol, k.unit.EnLine, k.unit.EnCol, k.unit.NxStmts)
		}
	}
	c.unitEntry = make([]int32, len(refs))
	for i, r := range refs {
		c.unitEntry[i] = -1
		if r.pkg != nil {
			c.unitEntry[i] = r.pkg.global[r.local]
		}
	}
	var text bytes.Buffer
	if err := formatter.EmitTextual(nil, &text); err != nil {
		c.refusal = "text profile: " + err.Error()
		return
	}
	c.text = text.Bytes()
}

func __dd_civisibility_coverageCounters(dst []uint32) ([]uint32, bool) {
	__dd_civisibility_coverage.once.Do(__dd_civisibility_coverageBuild)
	dst = dst[:0]
	for _, segment := range __dd_civisibility_coverageCounterList() {
		counters := unsafe.Slice((*atomic.Uint32)(unsafe.Pointer(segment.Counters)), int(segment.Len))
		dst = append(dst, uint32(len(counters)))
		for i := range counters {
			dst = append(dst, counters[i].Load())
		}
	}
	return dst, len(rtcov.Meta.List) == __dd_civisibility_coverage.metaLen
}

// __dd_civisibility_coverageWalk visits each function with a header in a copy
// made by counters, as cfile's VisitFuncs reads the counter segments.
func __dd_civisibility_coverageWalk(raw []uint32, visit func(slot int, fn uint32, counters []uint32) bool) bool {
	for len(raw) > 0 {
		n := int(raw[0])
		if 1+n > len(raw) {
			return false
		}
		segment := raw[1 : 1+n]
		raw = raw[1+n:]
		for i := 0; i < len(segment); i++ {
			if segment[i] == 0 {
				continue
			}
			if i+coverage.FirstCtrOffset > len(segment) {
				return false
			}
			ctrs := int(segment[i+coverage.NumCtrsOffset])
			pkg := int32(segment[i+coverage.PkgIdOffset])
			fn := segment[i+coverage.FuncIdOffset]
			start := i + coverage.FirstCtrOffset
			if start+ctrs > len(segment) {
				return false
			}
			slot := 0
			switch {
			case pkg > 0:
				slot = int(pkg) - 1
			case pkg < 0:
				mapped, ok := rtcov.Meta.PkgMap[int(pkg)]
				if !ok {
					return false
				}
				slot = mapped
			default:
				return false
			}
			if !visit(slot, fn, segment[start:start+ctrs]) {
				return false
			}
			i = start + ctrs - 1
		}
	}
	return true
}

func __dd_civisibility_coverageCounts(raw, dst []uint32) ([]uint32, bool) {
	c := &__dd_civisibility_coverage
	c.once.Do(__dd_civisibility_coverageBuild)
	if c.refusal != "" {
		return dst[:0], false
	}
	n := len(c.entries) / 6
	dst = slices.Grow(dst[:0], n)[:n]
	clear(dst)
	ok := __dd_civisibility_coverageWalk(raw, func(slot int, fn uint32, counters []uint32) bool {
		if slot >= len(c.funcs) || int(fn) >= len(c.funcs[slot]) {
			return false
		}
		f := c.funcs[slot][fn]
		if len(counters) < int(f.units) {
			return false
		}
		for i, entry := range c.unitEntry[f.start : f.start+f.units] {
			if entry < 0 {
				continue
			}
			if sum := dst[entry] + counters[i]; sum >= dst[entry] {
				dst[entry] = sum
			} else {
				dst[entry] = ^uint32(0)
			}
		}
		return true
	})
	return dst, ok
}

func __dd_civisibility_coverageProfile(raw []uint32) []byte {
	c := &__dd_civisibility_coverage
	c.once.Do(__dd_civisibility_coverageBuild)
	type pkfunc struct {
		slot int
		fn   uint32
	}
	live := map[pkfunc][]uint32{}
	__dd_civisibility_coverageWalk(raw, func(slot int, fn uint32, counters []uint32) bool {
		for _, v := range counters {
			if v != 0 {
				live[pkfunc{slot, fn}] = counters
				break
			}
		}
		return true
	})
	formatter := cformat.NewFormatter(c.mode)
	var fd coverage.FuncDesc
	for slot, b := range rtcov.Meta.List {
		d, err := decodemeta.NewCoverageMetaDataDecoder(unsafe.Slice(b.P, int(b.Len)), true)
		if err != nil {
			return nil
		}
		formatter.SetPackage(d.PackagePath())
		for fn := uint32(0); fn < d.NumFuncs(); fn++ {
			if err := d.ReadFunc(fn, &fd); err != nil {
				return nil
			}
			counters, have := live[pkfunc{slot, fn}]
			for i, u := range fd.Units {
				if u.Parent != 0 {
					continue
				}
				count := uint32(0)
				if have {
					count = counters[i]
				}
				formatter.AddUnit(fd.Srcfile, fd.Funcname, fd.Lit, u, count)
			}
		}
	}
	var text bytes.Buffer
	if err := formatter.EmitTextual(nil, &text); err != nil {
		return nil
	}
	return text.Bytes()
}
`
