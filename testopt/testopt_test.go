package testopt

import (
	"context"
	"testing"
)

// Options are functions: callers can write their own and apply them directly.
func TestStartSpanOptionsAreFunctions(t *testing.T) {
	client, err := New(Config{Transport: TransportConfig{Endpoint: "http://127.0.0.1:9"}})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close(context.Background())
	var team StartSpanOption = func(s *Span) { s.SetTag("team", "payments") }
	options := []StartSpanOption{ResourceName("resource"), SpanType("test"), Tag("key", "value"), team}
	span, _ := client.StartSpan(context.Background(), "test", options...)
	Tag("applied", "directly")(span)
	for key, want := range map[string]string{"team": "payments", "key": "value", "applied": "directly"} {
		if got, _ := span.Meta(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}
