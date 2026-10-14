// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2026 Datadog, Inc.

package net

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Concurrent HTTPS requests to one host share a single HTTP/2 connection, so
// they start one dial: no extra dial finishes in the transport's goroutine
// after its request took another connection. Plain HTTP keeps concurrent
// connections.
func TestCITransportDialsOnceForConcurrentHTTPSRequests(t *testing.T) {
	var mu sync.Mutex
	protocols := map[string]bool{}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		protocols[r.Proto] = true
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
	}))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()

	client := createNewHTTPClient()
	transport := client.Transport.(*ciTransport)
	defer transport.CloseIdleConnections()
	transport.secure.TLSClientConfig = server.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	var dials atomic.Int32
	dial := transport.secure.DialContext
	transport.secure.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		dials.Add(1)
		// A slow dial, as over a real network, lets the requests overlap it.
		time.Sleep(20 * time.Millisecond)
		return dial(ctx, network, address)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 5)
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := client.Get(server.URL)
			if err == nil {
				_, err = io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if n := dials.Load(); n != 1 {
		t.Fatalf("%d dials for concurrent HTTPS requests, want 1", n)
	}
	mu.Lock()
	defer mu.Unlock()
	if !protocols["HTTP/2.0"] || len(protocols) != 1 {
		t.Fatalf("protocols %v, want HTTP/2.0 only", protocols)
	}
	if transport.plain.MaxConnsPerHost != 0 {
		t.Fatalf("plain HTTP is limited to %d connections per host", transport.plain.MaxConnsPerHost)
	}
}
