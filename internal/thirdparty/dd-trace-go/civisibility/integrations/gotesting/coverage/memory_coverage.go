// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2026 Datadog, Inc.

package coverage

import (
	"errors"
	"os"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	_ "unsafe" // for go:linkname

	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/log"
)

// Per-test coverage reads the counters in memory when ddto's testing overlay
// registers its hook. The hook renders the text profile's layout once, and
// each capture yields one count per layout entry: the counts that
// testing/internal/testdeps would write in the text profile, without files,
// text or parsing. Processes that the hook or initMemoryCoverage refuse keep
// the text profiles.
const (
	// coverageInMemoryEnv set to false keeps the text profiles.
	coverageInMemoryEnv = "DD_CIVISIBILITY_INTERNAL_COVERAGE_IN_MEMORY"
	// coverageOracleEnv set to true also computes each in-memory collector's
	// result from text profiles of the same counters, and logs both.
	coverageOracleEnv = "DD_CIVISIBILITY_INTERNAL_COVERAGE_ORACLE"
	// memoryCoverageBudget bounds the baselines that in-memory collectors
	// hold at once. Nested subtests keep their ancestors' baselines.
	memoryCoverageBudget = 64 << 20
)

// testingCoverage holds the hook that ddto's testing overlay registers.
var testingCoverage struct {
	layout   func() ([]string, []uint32, []byte, string)
	counters func([]uint32) ([]uint32, bool)
	counts   func([]uint32, []uint32) ([]uint32, bool)
	profile  func([]uint32) []byte
}

// registerTestingCoverage is called by ddto's testing overlay.
//
//go:linkname registerTestingCoverage
func registerTestingCoverage(layout func() ([]string, []uint32, []byte, string), counters func([]uint32) ([]uint32, bool), counts func([]uint32, []uint32) ([]uint32, bool), profile func([]uint32) []byte) {
	testingCoverage.layout, testingCoverage.counters, testingCoverage.counts, testingCoverage.profile = layout, counters, counts, profile
}

var memoryCoverage struct {
	once    sync.Once
	enabled atomic.Bool
	oracle  bool
	files   []string
	// entries holds six values per layout entry: the index of its file name,
	// its start line and column, its end line and column, and its statements.
	entries []uint32
	held    atomic.Int64
}

// errMemoryCoverageCapture reports a capture that the hook could not map, as
// when coverage meta-data was registered after the layout was built.
var errMemoryCoverageCapture = errors.New("coverage counters do not match the meta-data")

// memoryCollector is an in-memory collector's state.
type memoryCollector struct {
	before   []uint32
	reserved int64
	covered  []int32
	// The oracle keeps the counters of both captures.
	rawBefore, rawAfter []uint32
}

// memoryCoverageReady reports whether captures read the counters in memory,
// deciding once, before the process's first capture.
func memoryCoverageReady() bool {
	memoryCoverage.once.Do(initMemoryCoverage)
	return memoryCoverage.enabled.Load()
}

func initMemoryCoverage() {
	reason := ""
	switch {
	case testingCoverage.layout == nil:
		reason = "no testing hook"
	case mode != "count" && mode != "atomic":
		reason = "coverage mode " + mode
	case strconv.IntSize < 64:
		// Text profiles reject counts beyond the platform's int.
		reason = "32-bit counts"
	case os.Getenv(coverageInMemoryEnv) == "false":
		reason = "disabled"
	}
	if reason == "" {
		files, entries, text, refusal := testingCoverage.layout()
		switch {
		case refusal != "":
			reason = refusal
		case !matchesTextProfile(files, entries, text):
			reason = "layout differs from the text profile"
		default:
			memoryCoverage.files, memoryCoverage.entries = files, entries
			memoryCoverage.oracle = os.Getenv(coverageOracleEnv) == "true"
			memoryCoverage.enabled.Store(true)
		}
	}
	log.Debug("civisibility.cov: per-test coverage in memory: %t (%s)", reason == "", reason)
}

// matchesTextProfile reports whether parsing the hook's text profile, with
// every count at its maximum, yields exactly the layout's entries in order:
// the text path would then read the same blocks.
func matchesTextProfile(files []string, entries []uint32, text []byte) bool {
	profile, err := parseCoverProfileText(text)
	if err != nil || len(profile.blocks)*6 != len(entries) || !bytesHasMode(text, mode) {
		return false
	}
	for i := range profile.blocks {
		block, entry := &profile.blocks[i], entries[i*6:i*6+6]
		if int(entry[0]) >= len(files) || block.fileName != files[entry[0]] ||
			block.startLine != int(entry[1]) || block.startCol != int(entry[2]) ||
			block.endLine != int(entry[3]) || block.endCol != int(entry[4]) ||
			block.numStmt != int(entry[5]) || block.count != int(^uint32(0)) {
			return false
		}
	}
	return true
}

func bytesHasMode(text []byte, mode string) bool {
	header := "mode: " + mode + "\n"
	return len(text) >= len(header) && string(text[:len(header)]) == header
}

// beginMemoryCoverage captures an in-memory baseline, or returns nil when
// the collector must use text profiles: the process's hook is unusable, or
// the baselines held would exceed the budget.
func beginMemoryCoverage() (*memoryCollector, error) {
	if !memoryCoverageReady() {
		return nil, nil
	}
	size := int64(len(memoryCoverage.entries) / 6 * 4)
	if memoryCoverage.held.Add(size) > memoryCoverageBudget {
		memoryCoverage.held.Add(-size)
		return nil, nil
	}
	c := &memoryCollector{reserved: size}
	before, raw, err := captureMemoryCoverage()
	if err != nil {
		c.release()
		return c, err
	}
	c.before = before
	if memoryCoverage.oracle {
		c.rawBefore = raw
	}
	return c, nil
}

// finish captures the counters after the test and keeps the entries whose
// count grew, then releases the baseline.
func (c *memoryCollector) finish() error {
	defer c.release()
	if c.before == nil {
		return errMemoryCoverageCapture
	}
	after, raw, err := captureMemoryCoverage()
	if err != nil {
		return err
	}
	for i, count := range after {
		if count > c.before[i] {
			c.covered = append(c.covered, int32(i))
		}
	}
	if memoryCoverage.oracle {
		c.rawAfter = raw
	}
	return nil
}

func (c *memoryCollector) release() {
	if c.reserved != 0 {
		memoryCoverage.held.Add(-c.reserved)
		c.reserved = 0
	}
	c.before = nil
}

// captureMemoryCoverage reads one count per layout entry. A capture that
// sees new meta-data turns in-memory coverage off for later baselines.
func captureMemoryCoverage() (counts, raw []uint32, err error) {
	raw, ok := testingCoverage.counters(nil)
	if ok {
		counts, ok = testingCoverage.counts(raw, nil)
	}
	if !ok || len(counts)*6 != len(memoryCoverage.entries) {
		memoryCoverage.enabled.Store(false)
		return nil, nil, errMemoryCoverageCapture
	}
	return counts, raw, nil
}

// files returns the covered files and line bitmaps as coveredFilesBetween
// does for the same counts in text profiles.
func (c *memoryCollector) files(testFile string) []coveredFile {
	var covered coveredLines
	for _, i := range c.covered {
		entry := memoryCoverage.entries[i*6 : i*6+6]
		block := coverProfileBlock{fileName: memoryCoverage.files[entry[0]], coverageBlock: coverageBlock{
			startLine: int(entry[1]), startCol: int(entry[2]), endLine: int(entry[3]), endCol: int(entry[4]), numStmt: int(entry[5]),
		}}
		covered.add(&block)
	}
	return covered.files(testFile)
}

// checkOracle computes the result from text profiles of the same counters
// and logs whether both agree.
func (c *memoryCollector) checkOracle(testFile string, got []coveredFile) {
	if c.rawBefore == nil || c.rawAfter == nil {
		return
	}
	before, errBefore := parseCoverProfileText(testingCoverage.profile(c.rawBefore))
	after, errAfter := parseCoverProfileText(testingCoverage.profile(c.rawAfter))
	if errBefore != nil || errAfter != nil {
		log.Debug("civisibility.cov: oracle failed to parse text profiles")
		return
	}
	want := coveredFilesBetween(testFile, before, after)
	log.Debug("civisibility.cov: oracle test file %s match=%t files=%d", testFile, reflect.DeepEqual(got, want), len(want))
	c.rawBefore, c.rawAfter = nil, nil
}

// resetMemoryCoverageForTesting forgets the hook and the process's decision.
func resetMemoryCoverageForTesting() {
	testingCoverage.layout, testingCoverage.counters, testingCoverage.counts, testingCoverage.profile = nil, nil, nil, nil
	memoryCoverage.once = sync.Once{}
	memoryCoverage.enabled.Store(false)
	memoryCoverage.oracle = false
	memoryCoverage.files, memoryCoverage.entries = nil, nil
	memoryCoverage.held.Store(0)
}
