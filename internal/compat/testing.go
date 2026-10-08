package compat

import (
	"context"
	"io"
	"sync"
)

// WaitGroup supplies Go for sources compiled with an older module language.
// The function must not panic, matching sync.WaitGroup.Go's contract.
type WaitGroup struct{ sync.WaitGroup }

// Go starts f and accounts for it until it returns.
func (w *WaitGroup) Go(f func()) {
	w.Add(1)
	go func() {
		defer w.Done()
		f()
	}()
}

// Context retains the native test context where available. The fallback is
// cancelled by cleanup for test implementations without a Context method.
func Context(t interface{ Cleanup(func()) }) context.Context {
	if provider, ok := t.(interface{ Context() context.Context }); ok {
		return provider.Context()
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}

// Chdir preserves the native test's process-directory and cleanup behavior.
func Chdir(t interface{ Chdir(string) }, dir string) { t.Chdir(dir) }

// Output retains the native test output writer and its lifetime.
func Output(t interface{ Output() io.Writer }) io.Writer { return t.Output() }

// Attr uses native validation and writes to the original test's attributes.
func Attr(t interface{ Attr(string, string) }, key, value string) { t.Attr(key, value) }

// ClearMap retains sync.Map's atomic Clear operation on supported toolchains.
func ClearMap(m interface{ Clear() }) { m.Clear() }
