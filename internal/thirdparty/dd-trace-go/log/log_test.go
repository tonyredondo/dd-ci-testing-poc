// Copyright 2026 Datadog, Inc. Licensed under the Apache License, Version 2.0.

package log

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// DD_LOGGING_RATE applies when the first error is reported, and an invalid
// value warns then, not at initialization.
func TestLoggingRateAppliesAtFirstError(t *testing.T) {
	reset := func() {
		Flush()
		loggingRateOnce = sync.Once{}
		errrate = time.Minute
	}
	reset()
	t.Cleanup(reset)
	logs := new(RecordLogger)
	t.Cleanup(UseLogger(logs))

	t.Setenv("DD_LOGGING_RATE", "0")
	Error("reported %d", 1)
	if got := logs.Logs(); len(got) != 1 || !strings.Contains(got[0], "reported 1") {
		t.Fatalf("with DD_LOGGING_RATE=0, logs %q; want the error at once", got)
	}

	reset()
	logs.Reset()
	t.Setenv("DD_LOGGING_RATE", "bad")
	if got := logs.Logs(); len(got) != 0 {
		t.Fatalf("logs %q before any error", got)
	}
	Error("aggregated")
	if got := logs.Logs(); len(got) != 1 || !strings.Contains(got[0], "Invalid value for DD_LOGGING_RATE") || errrate != time.Minute {
		t.Fatalf("with DD_LOGGING_RATE=bad, logs %q and rate %v; want one warning and the default rate", got, errrate)
	}
}
