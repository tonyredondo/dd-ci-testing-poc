package runner

import (
	"fmt"
	"os"
	"sync"
	"testing"
)

// TestMain keeps persistent vendor workspaces out of the developer's cache.
// The directory is created on first use: helper processes that re-execute this
// binary exit without returning from m.Run, so they must not create one.
func TestMain(m *testing.M) {
	var (
		once     sync.Once
		cache    string
		cacheErr error
	)
	userCacheDir = func() (string, error) {
		once.Do(func() { cache, cacheErr = os.MkdirTemp("", "ddtest-runner-cache-") })
		return cache, cacheErr
	}
	code := m.Run()
	if cache != "" {
		if err := os.RemoveAll(cache); err != nil {
			fmt.Fprintln(os.Stderr, "remove user cache:", err)
			code = 1
		}
	}
	os.Exit(code)
}
