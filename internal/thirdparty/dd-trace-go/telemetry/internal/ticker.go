// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2025 Datadog, Inc.

package internal

import (
	"sync"
	"time"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/cidelivery"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/log"
)

type TickFunc func()

type Ticker struct {
	ticker *time.Ticker

	tickSpeedMu sync.Mutex
	tickSpeed   time.Duration
	paused      bool

	interval Range[time.Duration]

	tickFunc TickFunc

	stopChan chan struct{}
	stopped  bool
	stopIdle func()
	lastTick time.Time // Deferred mode: when the checkpoint ticker last ran.
}

func NewTicker(tickFunc TickFunc, interval Range[time.Duration]) *Ticker {
	return newTicker(tickFunc, interval, false)
}

// NewPausedTicker creates a stopped native timer for queued startup telemetry.
// The timer is stopped before its worker starts, so even a short interval cannot
// send initialization data before StartApp has prepared its payload.
// Deferred tickers keep their checkpoint callback; the client gates that flush.
func NewPausedTicker(tickFunc TickFunc, interval Range[time.Duration]) *Ticker {
	return newTicker(tickFunc, interval, true)
}

func newTicker(tickFunc TickFunc, interval Range[time.Duration], paused bool) *Ticker {
	if cidelivery.Enabled() {
		ticker := &Ticker{tickSpeed: interval.Max, interval: interval, tickFunc: tickFunc, paused: paused, lastTick: time.Now()}
		ticker.stopIdle = cidelivery.Register(ticker.tickIfDue)
		return ticker
	}
	ticker := &Ticker{
		ticker:    time.NewTicker(interval.Max),
		tickSpeed: interval.Max,
		interval:  interval,
		tickFunc:  tickFunc,
		stopChan:  make(chan struct{}),
		paused:    paused,
	}
	if paused {
		ticker.ticker.Stop()
	}

	go ticker.run()

	return ticker
}

// run is named so the goleak shim can identify this owned periodic worker.
func (t *Ticker) run() {
	for {
		select {
		case <-t.ticker.C:
			t.tickFunc()
		case <-t.stopChan:
			return
		}
	}
}

// tickIfDue runs at idle checkpoints in deferred mode. Like the periodic ticker,
// it ticks only after the current interval elapsed, rather than once per test,
// and not while paused for startup telemetry.
func (t *Ticker) tickIfDue() {
	t.tickSpeedMu.Lock()
	due := !t.paused && time.Since(t.lastTick) >= t.tickSpeed
	if due {
		t.lastTick = time.Now()
	}
	t.tickSpeedMu.Unlock()
	if due {
		t.tickFunc()
	}
}

func (t *Ticker) CanIncreaseSpeed() {
	t.tickSpeedMu.Lock()
	defer t.tickSpeedMu.Unlock()

	oldTickSpeed := t.tickSpeed
	t.tickSpeed = t.interval.Clamp(t.tickSpeed / 2)

	if oldTickSpeed == t.tickSpeed {
		return
	}

	log.Debug("telemetry: increasing flush speed to an interval of %s", t.tickSpeed)
	if t.ticker != nil && !t.paused && !t.stopped {
		t.ticker.Reset(t.tickSpeed)
	}
}

func (t *Ticker) CanDecreaseSpeed() {
	t.tickSpeedMu.Lock()
	defer t.tickSpeedMu.Unlock()

	oldTickSpeed := t.tickSpeed
	t.tickSpeed = t.interval.Clamp(t.tickSpeed * 2)

	if oldTickSpeed == t.tickSpeed {
		return
	}

	log.Debug("telemetry: decreasing flush speed to an interval of %s", t.tickSpeed)
	if t.ticker != nil && !t.paused && !t.stopped {
		t.ticker.Reset(t.tickSpeed)
	}
}

// Resume keeps the current interval, including adjustments while paused.
// Deferred tickers remain driven by checkpoints and have no native timer.
func (t *Ticker) Resume() {
	t.tickSpeedMu.Lock()
	defer t.tickSpeedMu.Unlock()
	t.paused = false
	if t.ticker != nil && !t.stopped {
		t.ticker.Reset(t.tickSpeed)
	}
}

func (t *Ticker) Stop() {
	t.tickSpeedMu.Lock()
	if t.stopped {
		t.tickSpeedMu.Unlock()
		return
	}
	t.stopped = true
	if t.ticker != nil {
		t.ticker.Stop()
	}
	t.tickSpeedMu.Unlock()
	if t.stopIdle != nil {
		t.stopIdle()
		return
	}
	t.stopChan <- struct{}{}
	close(t.stopChan)
}
