package cidelivery

import (
	"sync/atomic"
	"testing"
)

func TestLeakCheckWaitsForActiveSendAndPausesNewSends(t *testing.T) {
	var closed atomic.Int32
	remove := RegisterConnectionCloser(func() { closed.Add(1) })
	defer remove()
	BeginSend()
	ready := make(chan func(), 1)
	go func() { ready <- PrepareLeakCheck() }()
	select {
	case resume := <-ready:
		resume()
		t.Fatal("leak check overlapped delivery")
	default:
	}
	EndSend()
	resume := <-ready
	if closed.Load() != 1 {
		resume()
		t.Fatal("owned connections were not closed")
	}
	started, done := make(chan struct{}), make(chan struct{})
	go func() { close(started); BeginSend(); EndSend(); close(done) }()
	<-started
	select {
	case <-done:
		resume()
		t.Fatal("new send ran during leak check")
	default:
	}
	resume()
	<-done
}
