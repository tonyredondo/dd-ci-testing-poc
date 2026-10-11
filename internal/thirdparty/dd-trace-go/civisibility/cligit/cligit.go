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
	"io"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/civisibility/utils"
)

// Proceed is the line the CLI writes to the worker's input once test
// processes will receive the state; end of input without it stops the worker.
const Proceed = "proceed"

// Run writes the checkout's Git record to dir, then waits for the CLI.
func Run(dir string, input io.Reader) error {
	if err := utils.WriteCLIGitStateFile(dir, utils.CLIGitRecordFile, utils.CollectCLIGitRecord()); err != nil {
		return err
	}
	line, _ := bufio.NewReader(input).ReadString('\n')
	if line != Proceed+"\n" {
		return nil
	}
	return nil
}
