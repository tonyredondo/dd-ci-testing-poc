// Copyright 2026 Datadog, Inc. Licensed under the Apache License, Version 2.0.

// Package cligit is the worker that ddto's CLI starts once per invocation to do
// the Git work that every test process of a checkout would otherwise repeat.
// It runs in the invocation's directory, with the test processes' environment,
// so the SDK code it calls sees what a test process would see. It writes its
// results to a state directory that the CLI hands to the test processes; see
// utils.HandedCLIGitState. It also owns the invocation's telemetry
// application, which those test processes report under; see
// utils.HandedCLITelemetry.
package cligit

import (
	"bufio"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	infra "github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/civisibility/constants"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/civisibility/utils"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/civisibility/utils/gitupload"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/civisibility/utils/impactedtests"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/civisibility/utils/net"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/log"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/telemetry"
)

// Proceed starts the line that the CLI writes to the worker's input once test
// processes will receive the state, followed by a space and their Go version;
// end of input without it stops the worker. After it, the CLI keeps the input
// open until go test exits.
const Proceed = "proceed"

// client is the part of the CI Visibility client that the worker uses.
type client interface {
	gitupload.Client
	GetSettings() (*net.SettingsResponseData, error)
}

// appTelemetry is the invocation's telemetry application, which the worker
// owns: see net.CLITelemetry.
type appTelemetry interface {
	Start(languageVersion string)
	Stop()
}

// newClient creates the CI Visibility client and the invocation's telemetry
// application in the state directory dir; tests replace it.
var newClient = func(dir, runtimeID string) (client, appTelemetry) {
	c, t := net.NewCLIClient(dir, runtimeID)
	if c == nil {
		return nil, nil
	}
	if t == nil {
		return c, nil
	}
	return c, t
}

// Run writes the checkout's Git record to dir, then waits for the CLI. Once
// told to proceed, it compares the local commits with the backend's while go
// test builds, a read-only request. Once a test process uses the state, it
// starts the invocation's telemetry application, which the test processes
// that use the state report under, and uploads the missing commits, as each
// test process would, and records the end of the upload; meanwhile it
// requests the settings, and if they enable impacted tests it records the
// base commit and modified files. It closes the telemetry application after
// go test exits. When go test exits without running a test process that uses
// the state, as when every result comes from its cache, the worker exits
// without changing the repository, uploading or sending telemetry.
func Run(dir, runtimeID string, input io.Reader) error {
	if infra.BoolEnv("DD_TRACE_DEBUG", false) {
		log.SetLevel(log.LevelDebug)
	}
	record := utils.CollectCLIGitRecord()
	utils.UseCLIGitState(dir, record)
	// The telemetry application exists before the record, which ddto waits
	// for before it hands the state over.
	c, app := newClient(dir, runtimeID)
	if err := utils.WriteCLIGitStateFile(dir, utils.CLIGitRecordFile, record); err != nil {
		return err
	}
	reader := bufio.NewReader(input)
	line, _ := reader.ReadString('\n')
	verb, goVersion, _ := strings.Cut(strings.TrimSuffix(line, "\n"), " ")
	if !strings.HasSuffix(line, "\n") || verb != Proceed {
		return nil
	}
	uploadEnabled := infra.BoolEnv(constants.CIVisibilityGitUploadEnabledEnvironmentVariable, true)
	search := searchAhead(c, uploadEnabled)
	exited := drain(reader)
	if !waitUsed(dir, exited) {
		return nil
	}
	if app != nil {
		started := make(chan struct{})
		go func() {
			defer close(started)
			app.Start(goVersion)
		}()
		defer func() {
			<-started
			reportTestProcessFailures(dir)
			app.Stop()
		}()
	}
	// Test processes request the settings while they upload.
	settings := make(chan settingsResult, 1)
	if c != nil {
		go func() {
			response, err := c.GetSettings()
			settings <- settingsResult{response, err}
		}()
	}
	uploaded := upload(c, uploadEnabled, search)
	if err := writeState(dir, utils.CLIGitUploadFile, uploaded); err != nil {
		return err
	}
	if err := writeState(dir, utils.CLIGitImpactedFile, impacted(c, uploaded, settings)); err != nil {
		return err
	}
	// Test processes send their telemetry before go test exits, and the
	// application closes after them, in one request with the worker's data.
	<-exited
	return nil
}

// writeState writes a result that test processes wait for. If it fails, the
// worker exits without waiting for go test, so that ddto records the result.
func writeState(dir, name string, value any) error {
	err := utils.WriteCLIGitStateFile(dir, name, value)
	if err != nil {
		logError("civisibility: CLI state not written", "file:"+name)
	}
	return err
}

// logError reports a failure of the worker's work as a telemetry log.
func logError(message string, tags ...string) {
	telemetry.Log(telemetry.NewRecord(telemetry.LogError, message), telemetry.WithTags(tags))
}

// reportTestProcessFailures reports, under the invocation's application, the
// requests that test processes could not deliver under it.
func reportTestProcessFailures(dir string) {
	for code, count := range utils.CLITelemetryFailures(dir) {
		tag := "status_code:" + strconv.Itoa(code)
		if code == 0 {
			tag = "type:network"
		}
		for range count {
			logError("civisibility: test process telemetry request failed", tag)
		}
	}
}

type settingsResult struct {
	response *net.SettingsResponseData
	err      error
}

// searchAhead starts the upload's first commit search; the returned function
// waits for it.
func searchAhead(c client, uploadEnabled bool) func() (*gitupload.SearchCommitsResponse, error) {
	if c == nil || !uploadEnabled {
		return nil
	}
	var response *gitupload.SearchCommitsResponse
	var err error
	done := make(chan struct{})
	go func() {
		defer close(done)
		response, err = gitupload.SearchCommits(c)
	}()
	return func() (*gitupload.SearchCommitsResponse, error) {
		<-done
		return response, err
	}
}

// upload runs the test processes' repository upload, starting from the
// commit search made ahead. Test processes that disable it do not wait for
// this result.
func upload(c client, uploadEnabled bool, search func() (*gitupload.SearchCommitsResponse, error)) utils.CLIGitUpload {
	if !uploadEnabled {
		return utils.CLIGitUpload{Error: "repository upload disabled"}
	}
	if c == nil {
		return utils.CLIGitUpload{Error: errors.New("no CI Visibility client").Error()}
	}
	hooks := gitupload.ClientHooks(c)
	repeat := hooks.GetSearchCommits
	hooks.GetSearchCommits = func() (*gitupload.SearchCommitsResponse, error) {
		hooks.GetSearchCommits = repeat
		return search()
	}
	bytes, err := gitupload.Upload(hooks)
	if err != nil {
		log.Error("civisibility: error uploading repository changes: %s", err.Error())
		logError("civisibility: CLI repository upload failed")
		return utils.CLIGitUpload{Bytes: bytes, Error: err.Error()}
	}
	log.Debug("civisibility: uploaded %d bytes in pack files", bytes)
	return utils.CLIGitUpload{Bytes: bytes}
}

// impacted computes the impacted-tests state when the settings enable it, as
// a test process does after the upload: the base branch discovery and the diff
// then need the history that the upload may have fetched.
func impacted(c client, uploaded utils.CLIGitUpload, settings <-chan settingsResult) impactedtests.CLIState {
	if c == nil {
		return impactedtests.CLIState{Reason: "no CI Visibility client"}
	}
	result := <-settings
	response, err := result.response, result.err
	if err == nil && response != nil && response.RequireGit && uploaded.Error == "" {
		response, err = c.GetSettings()
	}
	switch {
	case err != nil:
		logError("civisibility: CLI settings request failed")
		return impactedtests.CLIState{Reason: "settings: " + err.Error()}
	case response == nil || !response.ImpactedTestsEnabled:
		return impactedtests.CLIState{Reason: "impacted tests disabled by the settings"}
	case !infra.BoolEnv(constants.CIVisibilityImpactedTestsDetectionEnabled, true):
		return impactedtests.CLIState{Reason: "impacted tests disabled by the environment"}
	}
	state := impactedtests.ComputeCLIState()
	if !state.Computed {
		logError("civisibility: CLI impacted tests state not computed")
	}
	return state
}

// drain reads the CLI's input to its end; the returned channel is closed
// then, because go test has exited.
func drain(input io.Reader) <-chan struct{} {
	exited := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, input)
		close(exited)
	}()
	return exited
}

// waitUsed waits until a test process marks the state as used, or go test
// exits.
func waitUsed(dir string, exited <-chan struct{}) bool {
	used := filepath.Join(dir, utils.CLIGitUsedFile)
	delay := time.Millisecond
	for {
		if _, err := os.Stat(used); err == nil {
			return true
		}
		select {
		case <-exited:
			_, err := os.Stat(used)
			return err == nil
		case <-time.After(delay):
			delay = min(2*delay, 5*time.Millisecond)
		}
	}
}
