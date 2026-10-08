//go:build go1.25

package compat

import (
	"testing"
	"testing/synctest"
)

// Synctest forwards native virtual-time testing on supported toolchains. This
// version guard belongs to the API boundary rather than every calling source.
func Synctest(t *testing.T, f func(*testing.T)) { synctest.Test(t, f) }

// SynctestWait waits until the native bubble's other goroutines are blocked.
func SynctestWait() { synctest.Wait() }
