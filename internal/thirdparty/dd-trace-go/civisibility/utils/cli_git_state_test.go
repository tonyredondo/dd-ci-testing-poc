// Copyright 2026 Datadog, Inc. Licensed under the Apache License, Version 2.0.

package utils

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/goenv"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/civisibility/constants"
)

// git runs git in dir and returns its trimmed output.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	args = append([]string{"-c", "user.name=State", "-c", "user.email=state@example.invalid", "-c", "commit.gpgsign=false"}, args...)
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// The worker's record changes nothing in the repository before a test
// process uses the state: a CI head commit missing from a shallow clone is
// left to test processes, which fetch it as before.
func TestCollectCLIGitRecordDoesNotFetch(t *testing.T) {
	origin := t.TempDir()
	git(t, origin, "init", "--initial-branch=main")
	git(t, origin, "commit", "--allow-empty", "-m", "first")
	first := git(t, origin, "rev-parse", "HEAD")
	git(t, origin, "commit", "--allow-empty", "-m", "second")
	second := git(t, origin, "rev-parse", "HEAD")
	shallow := filepath.Join(t.TempDir(), "shallow")
	git(t, origin, "clone", "--depth=1", "file://"+filepath.ToSlash(origin), shallow)
	t.Chdir(shallow)

	original := getProviderTagsFunc
	t.Cleanup(func() { getProviderTagsFunc = original })
	for _, c := range []struct {
		head, want string
	}{{first, ""}, {second, second}} {
		getProviderTagsFunc = func() map[string]string { return map[string]string{constants.GitHeadCommit: c.head} }
		record := CollectCLIGitRecord()
		if record.HeadCommit != c.want || (c.want != "" && record.Head.Sha != c.want) || record.HeadError != "" {
			t.Fatalf("head %s: record %+v", c.head, record)
		}
	}
	if err := exec.Command("git", "-C", shallow, "cat-file", "-e", first+"^{commit}").Run(); err == nil {
		t.Fatal("the record fetched the missing head commit")
	}
}

// A test process uses the handed state only if it can mark it used: without
// the mark, the worker neither uploads nor starts the telemetry application
// that the process would wait for or report under.
func TestHandedCLIGitStateNeedsUsedMark(t *testing.T) {
	if runtime.GOOS == "windows" || os.Getuid() == 0 {
		t.Skip("needs a directory that the process cannot write")
	}
	repo := t.TempDir()
	git(t, repo, "init", "--initial-branch=main")
	git(t, repo, "commit", "--allow-empty", "-m", "state")
	t.Chdir(repo)
	t.Cleanup(ResetHandedCLIGitStateForTesting)
	state := t.TempDir()
	if err := WriteCLIGitStateFile(state, CLIGitRecordFile, CollectCLIGitRecord()); err != nil {
		t.Fatal(err)
	}
	hand := func() *CLIGitRecord {
		t.Helper()
		t.Setenv(goenv.GitStateHandoff, state)
		goenv.Receive()
		ResetHandedCLIGitStateForTesting()
		_, record := HandedCLIGitState()
		return record
	}
	t.Cleanup(goenv.Receive)

	if err := os.Chmod(state, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(state, 0o700) })
	if record := hand(); record != nil {
		t.Fatal("a state that could not be marked used was accepted")
	}
	if err := os.Chmod(state, 0o700); err != nil {
		t.Fatal(err)
	}
	if record := hand(); record == nil {
		t.Fatal("the state was not accepted")
	}
	if _, err := os.Stat(filepath.Join(state, CLIGitUsedFile)); err != nil {
		t.Fatalf("the state was not marked used: %v", err)
	}
}
