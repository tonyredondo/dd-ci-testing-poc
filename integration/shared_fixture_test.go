package integration

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

// Only immutable compilation inputs and binaries are shared. runParityCase
// still owns a fresh receiver, child process and retry state for every case.
type parityFixture struct {
	dir, sdk, mini, oracle string
	base, head             string
}

type sharedParityFixture struct {
	once    sync.Once
	fixture *parityFixture
}

var (
	sharedTestingParity sharedParityFixture
	sharedTestifyParity sharedParityFixture
	sharedParityRoot    string
)

// The first consumer's t.TempDir would disappear before the deferred matrix
// starts. TestMain owns these directories through all selected tests and -count
// repetitions. A filtered deferred-only invocation initializes its own build.
func TestMain(m *testing.M) {
	var err error
	sharedParityRoot, err = os.MkdirTemp("", "ddtest-parity-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "create shared parity workspace:", err)
		os.Exit(1)
	}
	code := m.Run()
	if err := os.RemoveAll(sharedParityRoot); err != nil {
		fmt.Fprintln(os.Stderr, "clean shared parity workspace:", err)
		code = 1
	}
	os.Exit(code)
}

func (s *sharedParityFixture) get(t *testing.T, name string, build func(*testing.T, func() string) *parityFixture) *parityFixture {
	t.Helper()
	s.once.Do(func() {
		start := time.Now()
		tempDir := func() string {
			t.Helper()
			dir, err := os.MkdirTemp(sharedParityRoot, "")
			if err != nil {
				t.Fatal(err)
			}
			return dir
		}
		s.fixture = build(t, tempDir)
		t.Logf("shared %s fixture built once in %s", name, time.Since(start))
	})
	if s.fixture == nil {
		// sync.Once also completes when the builder calls Fatal/Goexit. Never
		// let a later test reuse an incomplete fixture or hide the first failure.
		t.Fatalf("shared %s fixture unavailable after a failed build", name)
	}
	return s.fixture
}
