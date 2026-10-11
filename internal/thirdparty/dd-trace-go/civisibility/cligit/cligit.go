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
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/civisibility/utils/net"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/log"
)

// Proceed is the line the CLI writes to the worker's input once test
// processes will receive the state; end of input without it stops the worker.
const Proceed = "proceed"

// newClient creates the CI Visibility client; tests replace it.
var newClient = func() gitupload.Client {
	client := net.NewClientWithoutTelemetry("")
	if client == nil {
		return nil
	}
	return client
}

// Run writes the checkout's Git record to dir, then waits for the CLI. Once
// told to proceed, it uploads the repository's missing commits, as each test
// process would, and records the end of the upload.
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
	return utils.WriteCLIGitStateFile(dir, utils.CLIGitUploadFile, upload())
}

// upload runs the test processes' repository upload. Test processes that
// disable it do not wait for this result.
func upload() utils.CLIGitUpload {
	if !infra.BoolEnv(constants.CIVisibilityGitUploadEnabledEnvironmentVariable, true) {
		return utils.CLIGitUpload{Error: "repository upload disabled"}
	}
	client := newClient()
	if client == nil {
		return utils.CLIGitUpload{Error: errors.New("no CI Visibility client").Error()}
	}
	bytes, err := gitupload.Upload(gitupload.ClientHooks(client))
	if err != nil {
		log.Error("civisibility: error uploading repository changes: %s", err.Error())
		return utils.CLIGitUpload{Bytes: bytes, Error: err.Error()}
	}
	log.Debug("civisibility: uploaded %d bytes in pack files", bytes)
	return utils.CLIGitUpload{Bytes: bytes}
}
