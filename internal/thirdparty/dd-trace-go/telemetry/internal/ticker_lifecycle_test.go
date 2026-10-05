package internal

import (
	"testing"
	"time"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/cidelivery"
)

func TestPausedTickerStartsOnlyAfterResume(t *testing.T) {
	t.Setenv(cidelivery.DeferredEnv, "false")
	ticks := make(chan struct{}, 16)
	ticker := NewPausedTicker(func() { ticks <- struct{}{} }, Range[time.Duration]{Min: time.Millisecond, Max: 20 * time.Millisecond})
	defer ticker.Stop()
	ticker.CanIncreaseSpeed()
	select {
	case <-ticks:
		t.Fatal("speed adjustment started a paused ticker")
	case <-time.After(30 * time.Millisecond):
	}
	ticker.Resume()
	select {
	case <-ticks:
	case <-time.After(2 * time.Second):
		t.Fatal("startup completion did not resume periodic telemetry")
	}
}

func TestStoppedTickerCannotResume(t *testing.T) {
	t.Setenv(cidelivery.DeferredEnv, "false")
	ticks := make(chan struct{}, 16)
	ticker := NewPausedTicker(func() { ticks <- struct{}{} }, Range[time.Duration]{Min: time.Millisecond, Max: 10 * time.Millisecond})
	ticker.Stop()
	ticker.Stop()
	ticker.Resume()
	ticker.CanIncreaseSpeed()
	select {
	case <-ticks:
		t.Fatal("closed telemetry restarted its timer")
	case <-time.After(30 * time.Millisecond):
	}
}
