package compat

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"unicode"
)

func collect(iterator StringIterator) []string {
	var values []string
	for iterator.Next() {
		values = append(values, iterator.Value())
	}
	return values
}

func TestStringIteratorsMatchStandardLibrary(t *testing.T) {
	for _, value := range []string{"", "a", ",a,,b,", "a::b:::c", "é世界", "\xffa", "\u2003a\t b\n"} {
		for _, separator := range []string{"", ",", "::", "世", "missing"} {
			got, want := collect(Split(value, separator)), strings.Split(value, separator)
			if len(want) == 0 {
				want = nil
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("Split(%q,%q): got=%q want=%q", value, separator, got, want)
			}
		}
		got, want := collect(FieldsFunc(value, unicode.IsSpace)), strings.FieldsFunc(value, unicode.IsSpace)
		if len(want) == 0 {
			want = nil
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("FieldsFunc(%q): got=%q want=%q", value, got, want)
		}
	}
}

func TestStringIteratorsDoNotAllocate(t *testing.T) {
	for _, iterator := range []StringIterator{Split("a,,b,é", ","), FieldsFunc("a \u2003b\t é", unicode.IsSpace)} {
		if got := testing.AllocsPerRun(100, func() {
			copy := iterator
			for copy.Next() {
				_ = copy.Value()
			}
		}); got != 0 {
			t.Fatalf("iterator allocated %g objects", got)
		}
	}
}

func TestWaitGroupWaitsForStartedFunctions(t *testing.T) {
	var group WaitGroup
	var calls atomic.Int32
	for i := 0; i < 32; i++ {
		group.Go(func() { calls.Add(1) })
	}
	group.Wait()
	if calls.Load() != 32 {
		t.Fatal("Wait returned before every function completed")
	}
}

type cleanupTest struct{ cleanup func() }

func (t *cleanupTest) Cleanup(f func()) { t.cleanup = f }

type nativeContextTest struct {
	cleanupTest
	ctx context.Context
}

func (t *nativeContextTest) Context() context.Context { return t.ctx }

func TestContextPreservesNativeValueAndCleanupFallback(t *testing.T) {
	native := &nativeContextTest{ctx: context.WithValue(context.Background(), struct{}{}, "native")}
	if Context(native) != native.ctx || native.cleanup != nil {
		t.Fatal("native test context replaced")
	}
	legacy := &cleanupTest{}
	ctx := Context(legacy)
	if ctx.Err() != nil || legacy.cleanup == nil {
		t.Fatal("fallback context not live until cleanup")
	}
	legacy.cleanup()
	if ctx.Err() != context.Canceled {
		t.Fatal("cleanup did not cancel fallback context")
	}
}

func TestErrorMatchingPreservesWrappedAndJoinedErrors(t *testing.T) {
	want := &fixtureError{message: "fixture"}
	for _, err := range []error{want, fmt.Errorf("wrapped: %w", want), errors.Join(errors.New("other"), want)} {
		got, ok := AsType[*fixtureError](err)
		if !ok || got != want {
			t.Fatal("matching error identity changed")
		}
	}
	for _, err := range []error{nil, errors.New("unrelated")} {
		if got, ok := AsType[*fixtureError](err); ok || got != nil {
			t.Fatal("unrelated error matched")
		}
	}
}

type fixtureError struct{ message string }

func (e *fixtureError) Error() string { return e.message }

func TestValueHelpersRetainOwnershipAndEarlyIteratorStop(t *testing.T) {
	value := "original"
	copy := Pointer(value)
	value = "changed"
	if *copy != "original" || TypeFor[string]() != reflect.TypeOf("") {
		t.Fatal("value copy or reflected type changed")
	}
	input := []int{1, 2}
	combined := Concat(input, []int{3})
	combined[0] = 9
	if input[0] != 1 || !reflect.DeepEqual(combined, []int{9, 2, 3}) || Concat[int]() != nil {
		t.Fatal("concatenation changed ownership or empty result")
	}
	calls := 0
	MapSequence(map[string]int{"a": 1, "b": 2})(func(string, int) bool {
		calls++
		return false
	})
	if calls != 1 {
		t.Fatal("map iterator ignored early stop")
	}
}
