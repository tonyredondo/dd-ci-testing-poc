package citransport

import (
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestDeliveryModesAndRetries(t *testing.T) {
	for _, agentless := range []bool{false, true} {
		t.Run(map[bool]string{false: "agent", true: "agentless"}[agentless], func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reader := io.Reader(r.Body)
				if agentless {
					if r.Header.Get("dd-api-key") != "fixture-key" || r.Header.Get("Content-Encoding") != "gzip" {
						t.Error("missing agentless headers")
					}
					gz, err := gzip.NewReader(r.Body)
					if err != nil {
						t.Error(err)
						w.WriteHeader(400)
						return
					}
					defer gz.Close()
					reader = gz
				} else {
					if r.Header.Get("dd-api-key") != "" || r.Header.Get("X-Datadog-EVP-Subdomain") != "citestcycle-intake" {
						t.Error("invalid agent headers")
					}
				}
				data, err := io.ReadAll(reader)
				if err != nil || string(data) != "payload" {
					t.Error("retry changed body")
				}
				if calls.Add(1) == 1 {
					w.WriteHeader(503)
				} else {
					w.WriteHeader(202)
				}
			}))
			defer server.Close()
			transport, err := New(Config{Endpoint: server.URL, Agentless: agentless, APIKey: "fixture-key", RetryDelay: time.Millisecond})
			if err != nil {
				t.Fatal(err)
			}
			if err = transport.Send(context.Background(), []byte("payload")); err != nil {
				t.Fatal(err)
			}
			if calls.Load() != 2 {
				t.Fatal("wrong retry count")
			}
		})
	}
}
func TestPermanentFailureRedirectAndCancellation(t *testing.T) {
	for _, status := range []int{401, 302} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Location", "/redirect")
				w.WriteHeader(status)
			}))
			defer server.Close()
			transport, err := New(Config{Endpoint: server.URL, Agentless: true, APIKey: "private-fixture-key", RetryDelay: time.Millisecond})
			if err != nil {
				t.Fatal(err)
			}
			if err = transport.Send(context.Background(), []byte("payload")); err == nil {
				t.Fatal("accepted failed delivery")
			}
			if calls.Load() != 1 {
				t.Fatal("retried permanent failure or followed redirect")
			}
		})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer server.Close()
	transport, err := New(Config{Endpoint: server.URL, RetryDelay: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err = transport.Send(ctx, []byte("payload")); err != context.DeadlineExceeded {
		t.Fatalf("lost cancellation: %v", err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("backoff ignored context")
	}
}
func TestInvalidConfiguration(t *testing.T) {
	for _, c := range []Config{{Endpoint: "file:///tmp/file"}, {Endpoint: "http://user:secret@localhost"}, {Endpoint: "http://localhost", Agentless: true}, {Endpoint: "http://localhost", Attempts: -1}} {
		if _, err := New(c); err == nil {
			t.Fatal("accepted invalid configuration")
		}
	}
}
