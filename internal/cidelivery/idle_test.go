package cidelivery

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestIdleDeliveryWaitsForWholeParallelGroup(t *testing.T) {
	var coordinator Coordinator
	var running atomic.Int32
	var delivered atomic.Int32
	remove := coordinator.register(func() {
		if running.Load() != 0 {
			t.Error("delivery overlapped a test")
		}
	})
	defer remove()
	first := coordinator.begin()
	running.Add(1)
	second := coordinator.begin()
	running.Add(1)
	coordinator.queue(func() { delivered.Add(1) })
	running.Add(-1)
	first()
	first()
	if delivered.Load() != 0 {
		t.Fatal("sent while another test was active")
	}
	running.Add(-1)
	second()
	if delivered.Load() != 1 {
		t.Fatal("lost or duplicated queued delivery")
	}
}

func TestNextTestWaitsForDeliveryAndCanceledCallbacksAreRemoved(t *testing.T) {
	var coordinator Coordinator
	entered, finish := make(chan struct{}), make(chan struct{})
	var canceled atomic.Int32
	remove := coordinator.register(func() { canceled.Add(1) })
	remove()
	remove()
	coordinator.queue(func() { close(entered); <-finish })
	started := make(chan func(), 1)
	go func() { started <- coordinator.begin() }()
	<-entered
	select {
	case release := <-started:
		release()
		t.Fatal("test admitted before delivery finished")
	case <-time.After(10 * time.Millisecond):
	}
	close(finish)
	(<-started)()
	if canceled.Load() != 0 {
		t.Fatal("removed callback still ran")
	}
}

func TestConcurrentActivityRetainsQueuedWork(t *testing.T) {
	var coordinator Coordinator
	parent := coordinator.begin()
	var delivered atomic.Int32
	var group sync.WaitGroup
	for range 100 {
		group.Go(func() {
			release := coordinator.begin()
			coordinator.queue(func() { delivered.Add(1) })
			release()
		})
	}
	group.Wait()
	if delivered.Load() != 0 {
		t.Fatal("delivery ran before the parent completed")
	}
	parent()
	if delivered.Load() != 100 {
		t.Fatal("lost queued work", delivered.Load())
	}
}

func TestStartupDeliveryWaitsForCompletedGroup(t *testing.T) {
	var coordinator Coordinator
	var delivered atomic.Int32
	coordinator.queueAfterTests(func() { delivered.Add(1) })
	first, second := coordinator.begin(), coordinator.begin()
	if delivered.Load() != 0 {
		t.Fatal("startup delayed first admission")
	}
	first()
	if delivered.Load() != 0 {
		t.Fatal("startup overlapped the parallel group")
	}
	second()
	if delivered.Load() != 1 || coordinator.afterTestsCount.Load() != 0 {
		t.Fatal("startup work lost or remained enabled")
	}
}

func TestStartupDeliveryRetainsAdmissionWhileSending(t *testing.T) {
	var coordinator Coordinator
	entered, finish := make(chan struct{}), make(chan struct{})
	coordinator.queueAfterTests(func() { close(entered); <-finish })
	first := coordinator.begin()
	done := make(chan struct{})
	go func() { first(); close(done) }()
	<-entered
	if coordinator.afterTestsCount.Load() != 1 {
		t.Fatal("startup disabled admission before HTTP completed")
	}
	admitted := make(chan func(), 1)
	go func() { admitted <- coordinator.begin() }()
	select {
	case <-admitted:
		t.Fatal("next test overlapped delivery")
	case <-time.After(10 * time.Millisecond):
	}
	close(finish)
	<-done
	(<-admitted)()
	if coordinator.afterTestsCount.Load() != 0 {
		t.Fatal("startup admission remained enabled")
	}
}

func TestStartupDeliveryClosesWithoutTests(t *testing.T) {
	t.Setenv(DeferredEnv, "false")
	var delivered int
	QueueAfterTests(func() { delivered++ })
	if !TestAdmissionRequired() {
		t.Fatal("queued startup did not enable test admission")
	}
	Checkpoint()
	if delivered != 0 {
		t.Fatal("checkpoint delayed the first test")
	}
	Shutdown()
	Shutdown()
	if delivered != 1 || TestAdmissionRequired() {
		t.Fatal("shutdown lost or repeated startup work")
	}
}
