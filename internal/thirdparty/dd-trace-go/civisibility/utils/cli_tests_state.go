// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2026 Datadog, Inc.

package utils

import (
	"path/filepath"
	"sync"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/log"
)

// CLITestsFile describes the test processes of ddto's invocation. ddto
// writes it before it tells the worker to proceed.
const CLITestsFile = "tests.json"

// CLITests describes the test processes that ddto hands its state to, as
// ddto's own go env and go list report them.
type CLITests struct {
	// GOARCH is the test binaries' architecture.
	GOARCH string `json:"goarch,omitempty"`
	// Packages are the packages that have tests.
	Packages []CLITestPackage `json:"packages"`
}

// CLITestPackage is a test package's directory and the path and directory of
// its module.
type CLITestPackage struct {
	Dir        string `json:"dir"`
	ModulePath string `json:"module_path,omitempty"`
	ModuleDir  string `json:"module_dir,omitempty"`
}

var (
	cliTestsOnce sync.Once
	cliTests     *CLITests
)

// HandedCLITests returns the description of the test processes in ddto's
// state, or nil without it.
func HandedCLITests() *CLITests {
	cliTestsOnce.Do(func() {
		dir, record := HandedCLIGitState()
		if record == nil {
			return
		}
		var tests CLITests
		if err := ReadCLIGitStateFile(dir, CLITestsFile, &tests); err != nil {
			log.Debug("civisibility: CLI test description unavailable: %s", err.Error())
			return
		}
		cliTests = &tests
	})
	return cliTests
}

// CLITestPackageModule returns the path and directory of the module of the
// test package in dir, as ddto's state describes it, so that a test process
// does not start go list for them.
func CLITestPackageModule(dir string) (path, moduleDir string, ok bool) {
	tests := HandedCLITests()
	if tests == nil || dir == "" {
		return "", "", false
	}
	dir = filepath.Clean(dir)
	for _, p := range tests.Packages {
		if p.ModulePath != "" && p.ModuleDir != "" && filepath.Clean(p.Dir) == dir {
			return p.ModulePath, p.ModuleDir, true
		}
	}
	return "", "", false
}
