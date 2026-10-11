// Copyright 2026 Datadog, Inc. Licensed under the Apache License, Version 2.0.

package utils

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/log"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/telemetry"
)

// ddto's Git worker owns the invocation's telemetry application. The test
// processes that use its state report their telemetry under that
// application, numbering their requests with the worker's in one sequence.
const (
	// CLITelemetryFile holds the invocation's telemetry.SharedApp.
	CLITelemetryFile = "telemetry.json"
	// cliTelemetrySeqDir holds one file for each request number taken.
	cliTelemetrySeqDir = "seq"
	// cliTelemetryFailurePrefix names a test process's undelivered request.
	cliTelemetryFailurePrefix = "telemetry-failure-"
)

// CLITelemetrySequence numbers the requests of the processes that report the
// invocation's telemetry application. A number is taken by creating its file
// exclusively, so no two requests share one. Each process creates the number
// after the highest that exists, so the numbers taken are 1 to n without
// gaps, and a request numbered after another, in any process, has a higher
// number.
type CLITelemetrySequence struct {
	dir      string
	mu       sync.Mutex
	last     int64 // a number that exists
	reserved int64 // the owner's first number, taken before any other
	failed   bool
}

// cliTelemetryMaxSteps bounds the search for a free number under contention.
const cliTelemetryMaxSteps = 1 << 10

// Next returns the next request number. If the sequence's directory cannot
// be used, it continues this process's numbers, which may then repeat
// another process's.
func (s *CLITelemetrySequence) Next() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reserved != 0 {
		n := s.reserved
		s.reserved = 0
		s.last = max(s.last, n)
		return n
	}
	var err error
	for range cliTelemetryMaxSteps {
		n := s.highest() + 1
		var file *os.File
		file, err = os.OpenFile(s.path(n), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_ = file.Close()
			s.last = n
			return n
		}
		if !errors.Is(err, fs.ErrExist) {
			break
		}
		s.last = n
	}
	s.last++
	if !s.failed {
		s.failed = true
		reason := "contention"
		if err != nil && !errors.Is(err, fs.ErrExist) {
			reason = "file_system"
			log.Debug("civisibility: CLI telemetry sequence unavailable: %s", err.Error())
		}
		telemetry.Log(telemetry.NewRecord(telemetry.LogError, "civisibility: CLI telemetry sequence unavailable"), telemetry.WithTags([]string{"reason:" + reason}))
	}
	return s.last
}

// highest finds the highest number taken, at least s.last: it doubles its
// step from s.last while numbers exist, then bisects the last step.
func (s *CLITelemetrySequence) highest() int64 {
	low, step := s.last, int64(1)
	for step < 1<<40 && s.exists(low+step) {
		low += step
		step *= 2
	}
	high := low + step
	for high-low > 1 {
		middle := low + (high-low)/2
		if s.exists(middle) {
			low = middle
		} else {
			high = middle
		}
	}
	s.last = low
	return low
}

func (s *CLITelemetrySequence) exists(n int64) bool {
	_, err := os.Lstat(s.path(n))
	return err == nil
}

func (s *CLITelemetrySequence) path(n int64) string {
	return filepath.Join(s.dir, strconv.FormatInt(n, 10))
}

// WriteCLITelemetry records app as the invocation's telemetry application in
// dir and returns its owner's sequence, whose first number, for app-started,
// precedes every test process's.
func WriteCLITelemetry(dir string, app telemetry.SharedApp) (*CLITelemetrySequence, error) {
	seq := &CLITelemetrySequence{dir: filepath.Join(dir, cliTelemetrySeqDir), reserved: 1}
	if err := os.Mkdir(seq.dir, 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(seq.path(1), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	_ = file.Close()
	if err := WriteCLIGitStateFile(dir, CLITelemetryFile, app); err != nil {
		return nil, err
	}
	return seq, nil
}

// CLITelemetry is the invocation's telemetry application as a test process
// reports it.
type CLITelemetry struct {
	App      telemetry.SharedApp
	Sequence *CLITelemetrySequence
	// Failed records an undelivered request for the owner to report.
	Failed func(statusCode int)
}

// HandedCLITelemetry returns the invocation's telemetry application when
// ddto handed its state to this process (see HandedCLIGitState). Otherwise
// the process reports its own application.
func HandedCLITelemetry() (CLITelemetry, bool) {
	dir, record := HandedCLIGitState()
	if record == nil {
		return CLITelemetry{}, false
	}
	var app telemetry.SharedApp
	if err := ReadCLIGitStateFile(dir, CLITelemetryFile, &app); err != nil || app.RuntimeID == "" {
		reason := "invalid"
		if err != nil {
			log.Debug("civisibility: CLI telemetry application unavailable: %s", err.Error())
			reason = "unreadable"
			if errors.Is(err, fs.ErrNotExist) {
				reason = "missing"
			}
		}
		telemetry.Log(telemetry.NewRecord(telemetry.LogError, "civisibility: CLI telemetry application unavailable"), telemetry.WithTags([]string{"reason:" + reason}))
		return CLITelemetry{}, false
	}
	return CLITelemetry{
		App:      app,
		Sequence: &CLITelemetrySequence{dir: filepath.Join(dir, cliTelemetrySeqDir)},
		Failed:   func(statusCode int) { recordCLITelemetryFailure(dir, statusCode) },
	}, true
}

// recordCLITelemetryFailure records a test process's undelivered request.
func recordCLITelemetryFailure(dir string, statusCode int) {
	file, err := os.CreateTemp(dir, cliTelemetryFailurePrefix+"*")
	if err != nil {
		return
	}
	_, _ = file.WriteString(strconv.Itoa(statusCode))
	_ = file.Close()
}

// CLITelemetryFailures counts the requests that test processes could not
// deliver under the invocation's application, by status code, 0 for a
// network error.
func CLITelemetryFailures(dir string) map[int]int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var failures map[int]int
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), cliTelemetryFailurePrefix) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		code, err := strconv.Atoi(string(data))
		if err != nil {
			continue
		}
		if failures == nil {
			failures = map[int]int{}
		}
		failures[code]++
	}
	return failures
}
