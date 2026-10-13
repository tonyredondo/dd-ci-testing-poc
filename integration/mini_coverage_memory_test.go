package integration

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const coverageMemoryLibrary = `package fixture

func CoveredFirst() int  { return 1 }
func CoveredSecond() int { return 2 }
func CoveredCleanup() int { return 3 }
func CoveredRetry(attempt int) int {
	if attempt == 1 {
		return 0
	}
	return 4
}
`

const coverageMemoryTests = `package fixture_test

import (
	"testing"

	fixture "example.com/dd-ci-testing-fixture"
)

var retryAttempts int

func TestCoveredFirst(t *testing.T) {
	if fixture.CoveredFirst() != 1 {
		t.Fatal("first")
	}
}

func TestCoveredParallel(t *testing.T) {
	t.Run("a", func(t *testing.T) { t.Parallel(); fixture.CoveredFirst() })
	t.Run("b", func(t *testing.T) { t.Parallel(); fixture.CoveredSecond() })
}

func TestCoveredCleanup(t *testing.T) {
	t.Cleanup(func() { fixture.CoveredCleanup() })
}

func TestCoveredRetry(t *testing.T) {
	retryAttempts++
	if fixture.CoveredRetry(retryAttempts) == 0 {
		t.Error("first attempt")
	}
}
`

// Per-test coverage reads the counters in memory when ddto adds its testing
// hook. Each test's coverage is the same as with text profiles, in count and
// atomic modes and under -race, and test processes print only native go
// test's coverage lines. A coverage selection that instruments testing keeps
// text profiles.
func TestMiniPerTestCoverageInMemory(t *testing.T) {
	dir, driver := prepareMiniFixture(t)
	writeBuildFixture(t, dir, map[string]string{"memory_cov.go": coverageMemoryLibrary, "memory_cov_test.go": coverageMemoryTests})
	for _, build := range []struct {
		name   string
		flags  []string
		memory bool
	}{
		{"atomic", []string{"-covermode=atomic", "-coverpkg=./..."}, true},
		{"count", []string{"-covermode=count", "-coverpkg=./..."}, true},
		{"race", []string{"-race", "-covermode=atomic", "-coverpkg=./..."}, true},
		{"covered-testing", []string{"-covermode=atomic", "-coverpkg=testing,./..."}, false},
	} {
		t.Run(build.name, func(t *testing.T) {
			bin := filepath.Join(t.TempDir(), executableName("memory.test"))
			args := append(append([]string{"test", "--runtime=mini", "-c", "-o", bin}, build.flags...), ".")
			out, stderr, code := command(t, dir, testEnv("DD_CIVISIBILITY_ENABLED=false", "DD_TRACE_DEBUG=true"), driver, args...)
			if code != 0 {
				t.Fatal(out, stderr)
			}
			if want := fmt.Sprintf("per-test coverage in_memory_hook=%t", build.memory); !strings.Contains(stderr, want) {
				t.Fatalf("build log lacks %q:\n%s", want, stderr)
			}
			run := func(policy policySettings, env ...string) ([]string, int, execution) {
				t.Helper()
				env = append(env, "DD_CIVISIBILITY_FLAKY_RETRY_ENABLED=true", "DD_CIVISIBILITY_FLAKY_RETRY_COUNT=1", "DD_TRACE_DEBUG=true")
				got, exec := runParityCase(t, dir, bin, parityCase{Args: []string{"-test.v", "-test.run=^TestCovered"}, Policy: policy, Coverage: policy.Coverage, Env: env})
				if exec.code != 0 {
					t.Fatalf("exit %d:\n%s\n%s", exec.code, exec.out, exec.stderr)
				}
				lines := 0
				for _, line := range strings.Split(exec.out, "\n") {
					if strings.HasPrefix(strings.TrimSpace(line), "coverage: ") {
						lines++
					}
				}
				return normalizedMiniCoverage(t, &got.miniWireCapture), lines, exec
			}
			policy := policySettings{Coverage: true, Retry: true}
			memory, memoryLines, exec := run(policy)
			if got := strings.Contains(exec.stderr, "per-test coverage in memory: true"); got != build.memory {
				t.Fatalf("in memory=%t, want %t:\n%s", got, build.memory, exec.stderr)
			}
			text, textLines, _ := run(policy, "DD_CIVISIBILITY_INTERNAL_COVERAGE_IN_MEMORY=false")
			if len(memory) == 0 || !slices.Equal(memory, text) {
				t.Fatalf("per-test coverage differs\nin memory: %v\ntext profiles: %v", memory, text)
			}
			_, plainLines, _ := run(policySettings{Retry: true})
			if build.memory && memoryLines != plainLines || textLines <= plainLines {
				t.Fatalf("coverage lines: in memory %d, text profiles %d, without per-test coverage %d", memoryLines, textLines, plainLines)
			}
		})
	}
}
