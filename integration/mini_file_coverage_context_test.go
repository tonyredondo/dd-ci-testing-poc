package integration

import (
	"slices"
	"strings"
	"testing"
)

// contextFixture's parallel tests run at the same time and reach different
// files: directly, through a goroutine and through a worker that a channel
// feeds.
const contextFixture = `package fixture_test

import (
	"sync/atomic"
	"testing"
	"time"

	fixture "example.com/dd-ci-testing-fixture"
	"example.com/dd-ci-testing-fixture/kit"
)

var (
	arrived    atomic.Int32
	rendezvous = make(chan struct{})
)

// meet returns once both parallel tests reach it.
func meet(t *testing.T) {
	if arrived.Add(1) == 2 {
		close(rendezvous)
	}
	select {
	case <-rendezvous:
	case <-time.After(30 * time.Second):
		t.Fatal("the parallel tests did not run at the same time")
	}
}

func TestContextA(t *testing.T) {
	t.Parallel()
	meet(t)
	done := make(chan int)
	go func() { done <- fixture.FileA() }()
	<-done
}

func TestContextB(t *testing.T) {
	t.Parallel()
	meet(t)
	jobs, done := make(chan int), make(chan int)
	go func() {
		for n := range jobs {
			done <- kit.Want(n)
		}
	}()
	jobs <- 2
	<-done
	close(jobs)
}
`

func TestMiniFileCoverageContext(t *testing.T) {
	dir, driver := prepareMiniFixture(t)
	writeFileCoverageFixture(t, dir)
	writeBuildFixture(t, dir, map[string]string{"context_test.go": contextFixture})
	got, exec := runParityCase(t, dir, driver, parityCase{
		Args:     []string{"test", "--runtime=mini", "--experimental-file-coverage=context", "-count=1", "-v", "-run=^TestContext", "-parallel=2", "."},
		Policy:   policySettings{Coverage: true},
		Coverage: true,
		Env:      []string{"DD_TRACE_DEBUG=true"},
	})
	if exec.code != 0 {
		t.Fatalf("exit %d:\n%s\n%s", exec.code, exec.out, exec.stderr)
	}
	if !strings.Contains(exec.stderr, "file coverage installed=true") || !strings.Contains(exec.stderr, "context=true") {
		t.Fatalf("build log lacks context file coverage:\n%s", exec.stderr)
	}
	files, _ := fileCoverageRows(t, &got.miniWireCapture)
	for test, want := range map[string]struct{ has, lacks []string }{
		// The goroutine that A starts reaches lib.go for A only.
		"TestContextA": {has: []string{"context_test.go", "filecov.go", "lib.go"}, lacks: []string{"kit.go"}},
		// B's worker reaches kit.go for B only.
		"TestContextB": {has: []string{"context_test.go", "kit.go"}, lacks: []string{"filecov.go", "lib.go"}},
	} {
		for _, file := range want.has {
			if !slices.Contains(files[test], file) {
				t.Errorf("%s lacks %s: %v", test, file, files[test])
			}
		}
		for _, file := range want.lacks {
			if slices.Contains(files[test], file) {
				t.Errorf("%s has %s: %v", test, file, files[test])
			}
		}
	}
}
