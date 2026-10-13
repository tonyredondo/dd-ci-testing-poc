// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2026 Datadog, Inc.

package coverage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/testassert/require"
)

// fakeTestingCoverage registers a hook over three blocks in two files whose
// counts the test sets. counters copies the counts; counts returns the copy.
type fakeTestingCoverage struct {
	files   []string
	entries []uint32
	counts  []uint32
	text    string
	refuse  string
	failed  bool
}

func newFakeTestingCoverage() *fakeTestingCoverage {
	f := &fakeTestingCoverage{
		files: []string{"example.com/app/a.go", "example.com/app/b.go"},
		entries: []uint32{
			0, 3, 2, 5, 3, 2,
			0, 7, 1, 9, 2, 1,
			1, 4, 1, 6, 10, 3,
		},
		counts: []uint32{0, 0, 0},
	}
	f.text = f.profile(nil)
	return f
}

// profile renders the text profile for counts, or with every count at its
// maximum for nil.
func (f *fakeTestingCoverage) profile(counts []uint32) string {
	var b strings.Builder
	b.WriteString("mode: atomic\n")
	for i := 0; i < len(f.entries)/6; i++ {
		e := f.entries[i*6:]
		count := ^uint32(0)
		if counts != nil {
			count = counts[i]
		}
		fmt.Fprintf(&b, "%s:%d.%d,%d.%d %d %d\n", f.files[e[0]], e[1], e[2], e[3], e[4], e[5], count)
	}
	return b.String()
}

func (f *fakeTestingCoverage) register() {
	registerTestingCoverage(
		func() ([]string, []uint32, []byte, string) { return f.files, f.entries, []byte(f.text), f.refuse },
		func(dst []uint32) ([]uint32, bool) { return append(dst[:0], f.counts...), !f.failed },
		func(raw, dst []uint32) ([]uint32, bool) { return append(dst[:0], raw...), true },
		func(raw []uint32) []byte { return []byte(f.profile(raw)) },
	)
}

func setUpMemoryCoverage(t *testing.T) *fakeTestingCoverage {
	t.Helper()
	ResetForTesting()
	t.Cleanup(ResetForTesting)
	mode = "atomic"
	f := newFakeTestingCoverage()
	f.register()
	return f
}

// A collector reads the counters in memory and finds the files and lines
// that coveredFilesBetween finds in text profiles of the same counts, and
// releases its baseline.
func TestMemoryCoverageMatchesTextProfiles(t *testing.T) {
	f := setUpMemoryCoverage(t)
	f.counts = []uint32{2, 0, 5}
	before := append([]uint32(nil), f.counts...)
	collector := &testCoverage{testFile: "example.com/app/a_test.go"}
	require.NoError(t, collector.collectCoverageBeforeTestExecution())
	require.NotNil(t, collector.memory, "in-memory collector")
	require.Equal(t, int64(12), memoryCoverage.held.Load())
	f.counts = []uint32{3, 1, 5}
	require.NoError(t, collector.collectCoverageAfterTestExecution())
	require.Zero(t, memoryCoverage.held.Load(), "baseline released")
	require.True(t, collector.loadCoverageData())

	beforeProfile, err := parseCoverProfileText([]byte(f.profile(before)))
	require.NoError(t, err)
	afterProfile, err := parseCoverProfileText([]byte(f.profile(f.counts)))
	require.NoError(t, err)
	require.Equal(t, coveredFilesBetween(collector.testFile, beforeProfile, afterProfile), collector.filesCovered)
	require.Len(t, collector.filesCovered, 2, "the test file and a.go")
}

// The process keeps text profiles when the hook refuses, when its layout does
// not read back from its text profile as the same blocks, when settings turn
// it off, and for a counter mode without per-test coverage.
func TestMemoryCoverageRefusals(t *testing.T) {
	for name, change := range map[string]func(t *testing.T, f *fakeTestingCoverage){
		"hook refusal":   func(t *testing.T, f *fakeTestingCoverage) { f.refuse = "blocks share a position" },
		"reordered text": func(t *testing.T, f *fakeTestingCoverage) { f.entries[1], f.entries[7] = f.entries[7], f.entries[1] },
		"missing block": func(t *testing.T, f *fakeTestingCoverage) {
			f.text = strings.Join(strings.SplitN(f.text, "\n", 4)[:3], "\n") + "\n"
		},
		"line break": func(t *testing.T, f *fakeTestingCoverage) {
			f.files[1] = "example.com/app/b\nc.go"
			f.text = f.profile(nil)
		},
		"statements": func(t *testing.T, f *fakeTestingCoverage) { f.entries[5] = 9 },
		"wrong mode": func(t *testing.T, f *fakeTestingCoverage) {
			f.text = strings.Replace(f.text, "mode: atomic", "mode: count", 1)
		},
		"settings": func(t *testing.T, f *fakeTestingCoverage) { t.Setenv(coverageInMemoryEnv, "false") },
		"set mode": func(t *testing.T, f *fakeTestingCoverage) { mode = "set" },
		"no hook":  func(t *testing.T, f *fakeTestingCoverage) { resetMemoryCoverageForTesting() },
		"oversized line": func(t *testing.T, f *fakeTestingCoverage) {
			f.files[0] = strings.Repeat("a", 70000) + ".go"
			f.text = f.profile(nil)
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := setUpMemoryCoverage(t)
			change(t, f)
			require.False(t, memoryCoverageReady())
		})
	}
	setUpMemoryCoverage(t)
	require.True(t, memoryCoverageReady(), "the unchanged hook")
}

// Over the budget for baselines, a collector uses text profiles, and its
// after-capture does too.
func TestMemoryCoverageBudget(t *testing.T) {
	setUpMemoryCoverage(t)
	temporaryDir = t.TempDir()
	written := 0
	tearDown = func(profile, _ string) (string, error) {
		written++
		return "", os.WriteFile(profile, []byte("mode: atomic\nexample.com/app/a.go:3.2,5.3 2 1\n"), 0o600)
	}
	memoryCoverage.once.Do(initMemoryCoverage)
	memoryCoverage.held.Store(memoryCoverageBudget - 4)
	collector := &testCoverage{testFile: "example.com/app/a_test.go"}
	require.NoError(t, collector.collectCoverageBeforeTestExecution())
	require.Nil(t, collector.memory)
	require.Equal(t, int64(memoryCoverageBudget-4), memoryCoverage.held.Load())
	require.NoError(t, collector.collectCoverageAfterTestExecution())
	require.Equal(t, 2, written)
	require.FileExists(t, filepath.Join(temporaryDir, filepath.Base(collector.postCoverageFilename)))
}

// A capture that does not match the layout fails the collector, releases its
// baseline and keeps later collectors on text profiles.
func TestMemoryCoverageCaptureFailure(t *testing.T) {
	f := setUpMemoryCoverage(t)
	collector := &testCoverage{testFile: "example.com/app/a_test.go"}
	require.NoError(t, collector.collectCoverageBeforeTestExecution())
	f.failed = true
	require.Error(t, collector.collectCoverageAfterTestExecution())
	require.Zero(t, memoryCoverage.held.Load())
	require.False(t, memoryCoverageReady())

	f = setUpMemoryCoverage(t)
	f.failed = true
	collector = &testCoverage{testFile: "example.com/app/a_test.go"}
	require.Error(t, collector.collectCoverageBeforeTestExecution())
	require.NotNil(t, collector.memory)
	require.Zero(t, memoryCoverage.held.Load())
	require.Error(t, collector.collectCoverageAfterTestExecution())
}

// Process collectors pause and resume through the same captures, and each
// segment releases its baseline.
func TestMemoryCoverageProcessSegments(t *testing.T) {
	f := setUpMemoryCoverage(t)
	collector := BeginProcessTestCoverage("example.com/app/a_test.go")
	require.NotNil(t, collector)
	f.counts = []uint32{1, 0, 0}
	collector.Pause()
	require.Zero(t, memoryCoverage.held.Load())
	f.counts = []uint32{5, 0, 0}
	collector.Resume()
	f.counts = []uint32{5, 0, 2}
	files := collector.Finish()
	require.Zero(t, memoryCoverage.held.Load())
	names := []string{}
	for _, file := range files {
		names = append(names, file.Name)
	}
	require.Len(t, names, 3, "the test file, a.go from the first segment and b.go from the second: %v", names)
}
