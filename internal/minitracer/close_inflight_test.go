package minitracer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/citransport"
)

// Close stops the background senders and then sends the remaining backlog
// concurrently, not one batch at a time: up to maxConcurrentSends requests,
// counting a background delivery still in flight. Every sender finishes
// before Close returns.
func TestCloseDrainsBacklogConcurrently(t *testing.T) {
	var received deliveries
	var requests, active, peak atomic.Int32
	release := make(chan struct{})
	var releaseOnce sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 2 {
			// A failure starts the backoff: the backlog waits for Close while
			// the first request stays in flight.
			w.WriteHeader(400)
			return
		}
		n := active.Add(1)
		defer active.Add(-1)
		for p := peak.Load(); n > p && !peak.CompareAndSwap(p, n); p = peak.Load() {
		}
		if n == maxConcurrentSends {
			// Leave time for a request beyond the bound to arrive.
			time.AfterFunc(100*time.Millisecond, func() { releaseOnce.Do(func() { close(release) }) })
		}
		select {
		case <-release:
		case <-time.After(2 * time.Second):
		}
		received.record(t, r)
		w.WriteHeader(202)
	}))
	defer server.Close()
	client, err := New(Config{MaxEvents: 2, Transport: citransport.Config{Endpoint: server.URL}})
	if err != nil {
		t.Fatal(err)
	}
	finish := func(n int) {
		for range n {
			span, _ := client.StartSpan(context.Background(), "test", SpanType("test"))
			span.Finish()
		}
	}
	finish(4) // One batch in flight, one failed.
	waitFor(t, func() bool { return client.LastError() != nil })
	// Six more sealed batches reach the pending bound; an open one remains,
	// which Close seals beyond it.
	finish(13)
	client.mu.Lock()
	ready, inflight, open := len(client.ready), client.inflight, len(client.events)
	client.mu.Unlock()
	if ready != 7 || inflight != 1 || open != 1 {
		t.Fatalf("ready=%d inflight=%d open=%d before Close", ready, inflight, open)
	}
	if err := client.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if delivered := received.distinct(t); delivered != 17 || client.DroppedEvents() != 0 {
		t.Fatalf("delivered=%d dropped=%d of 17", delivered, client.DroppedEvents())
	}
	if p := peak.Load(); p != maxConcurrentSends {
		t.Fatalf("concurrent requests: %d", p)
	}
	waitForNoDrainWorkers(t)
}

func TestCloseReportsFailedBackgroundDelivery(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	c, err := New(Config{MaxEvents: 1, Transport: citransport.Config{Endpoint: server.URL, Attempts: 1}})
	if err != nil {
		t.Fatal(err)
	}
	s, _ := c.StartSpan(context.Background(), "test", SpanType("test"))
	s.Finish()
	<-entered
	result := make(chan error, 1)
	go func() { result <- c.Close(context.Background()) }()
	waitFor(t, func() bool { c.mu.Lock(); defer c.mu.Unlock(); return c.closed })
	close(release)
	if err := <-result; err == nil {
		t.Fatal("Close hid a failed background delivery")
	}
	if c.DroppedEvents() != 1 {
		t.Fatal("wrong dropped count", c.DroppedEvents())
	}
	if err := c.Close(context.Background()); err != nil {
		t.Fatal("a later empty close reported an old failure", err)
	}
}
