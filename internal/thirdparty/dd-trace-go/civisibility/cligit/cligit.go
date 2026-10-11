// Copyright 2026 Datadog, Inc. Licensed under the Apache License, Version 2.0.

// Package cligit is the worker that ddto's CLI starts once per invocation to do
// the Git work that every test process of a checkout would otherwise repeat.
// It runs in the invocation's directory, with the test processes' environment,
// so the SDK code it calls sees what a test process would see. It writes its
// results to a state directory that the CLI hands to the test processes; see
// utils.HandedCLIGitState.
package cligit

import (
	"bufio"
	"errors"
	"io"

	infra "github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/civisibility/constants"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/civisibility/utils"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/civisibility/utils/gitupload"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/civisibility/utils/impactedtests"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/civisibility/utils/net"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/log"
)

// Proceed is the line the CLI writes to the worker's input once test
// processes will receive the state; end of input without it stops the worker.
const Proceed = "proceed"

// client is the part of the CI Visibility client that the worker uses.
type client interface {
	gitupload.Client
	GetSettings() (*net.SettingsResponseData, error)
}

// newClient creates the CI Visibility client; tests replace it.
var newClient = func() client {
	c := net.NewClientWithoutTelemetry("")
	if c == nil {
		return nil
	}
	return c
}

// Run writes the checkout's Git record to dir, then waits for the CLI. Once
// told to proceed, it uploads the repository's missing commits, as each test
// process would, and records the end of the upload. Then, if the settings
// enable impacted tests, it records the base commit and modified files.
func Run(dir string, input io.Reader) error {
	if infra.BoolEnv("DD_TRACE_DEBUG", false) {
		log.SetLevel(log.LevelDebug)
	}
	record := utils.CollectCLIGitRecord()
	if err := utils.WriteCLIGitStateFile(dir, utils.CLIGitRecordFile, record); err != nil {
		return err
	}
	utils.UseCLIGitState(dir, record)
	line, _ := bufio.NewReader(input).ReadString('\n')
	if line != Proceed+"\n" {
		return nil
	}
	c := newClient()
	uploaded := upload(c)
	if err := utils.WriteCLIGitStateFile(dir, utils.CLIGitUploadFile, uploaded); err != nil {
		return err
	}
	return utils.WriteCLIGitStateFile(dir, utils.CLIGitImpactedFile, impacted(c, uploaded))
}

// upload runs the test processes' repository upload. Test processes that
// disable it do not wait for this result.
func upload(c client) utils.CLIGitUpload {
	if !infra.BoolEnv(constants.CIVisibilityGitUploadEnabledEnvironmentVariable, true) {
		return utils.CLIGitUpload{Error: "repository upload disabled"}
	}
	if c == nil {
		return utils.CLIGitUpload{Error: errors.New("no CI Visibility client").Error()}
	}
	bytes, err := gitupload.Upload(gitupload.ClientHooks(c))
	if err != nil {
		log.Error("civisibility: error uploading repository changes: %s", err.Error())
		return utils.CLIGitUpload{Bytes: bytes, Error: err.Error()}
	}
	log.Debug("civisibility: uploaded %d bytes in pack files", bytes)
	return utils.CLIGitUpload{Bytes: bytes}
}

// impacted computes the impacted-tests state when the settings enable it, as
// a test process does after the upload: the base branch discovery and the diff
// then need the history that the upload may have fetched.
func impacted(c client, uploaded utils.CLIGitUpload) impactedtests.CLIState {
	if c == nil {
		return impactedtests.CLIState{Reason: "no CI Visibility client"}
	}
	settings, err := c.GetSettings()
	if err == nil && settings != nil && settings.RequireGit && uploaded.Error == "" {
		settings, err = c.GetSettings()
	}
	switch {
	case err != nil:
		return impactedtests.CLIState{Reason: "settings: " + err.Error()}
	case settings == nil || !settings.ImpactedTestsEnabled:
		return impactedtests.CLIState{Reason: "impacted tests disabled by the settings"}
	case !infra.BoolEnv(constants.CIVisibilityImpactedTestsDetectionEnabled, true):
		return impactedtests.CLIState{Reason: "impacted tests disabled by the environment"}
	}
	return impactedtests.ComputeCLIState()
}
