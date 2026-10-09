package runner

import (
	"fmt"
	"os"
	"testing"
)

// TestMain keeps persistent vendor workspaces out of the developer's cache.
func TestMain(m *testing.M) {
	cache, err := os.MkdirTemp("", "ddtest-runner-cache-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "create user cache:", err)
		os.Exit(1)
	}
	userCacheDir = func() (string, error) { return cache, nil }
	code := m.Run()
	if err := os.RemoveAll(cache); err != nil {
		fmt.Fprintln(os.Stderr, "remove user cache:", err)
		code = 1
	}
	os.Exit(code)
}
