//go:build go1.25

package gotesting

import "github.com/tonyredondo/dd-ci-testing-poc/internal/compat"

import "testing"

func TestNativeContextWithoutSDK(t *testing.T) {
	ctx := compat.Context(t)
	if allocs := testing.AllocsPerRun(100, func() { bindNativeTestContext(t, nil) }); allocs != 0 {
		t.Fatalf("SDK-free binding allocated %g objects", allocs)
	}
	if compat.Context(t) != ctx {
		t.Fatal("SDK-free binding replaced the native context")
	}
}

func BenchmarkNativeContextWithoutSDK(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		bindNativeTestContext(b, nil)
	}
}
