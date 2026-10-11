// Copyright 2026 Datadog, Inc. Licensed under the Apache License, Version 2.0.

package utils

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sync"
	"testing"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/telemetry"
)

// Processes that report the CLI's application take distinct, gap-free
// request numbers; the owner's first one is 1, and a number taken after
// others is higher than all of them.
func TestCLITelemetrySequenceAcrossProcesses(t *testing.T) {
	dir := t.TempDir()
	app := telemetry.NewSharedApp("cli-runtime", "service", "env", "")
	owner, err := WriteCLITelemetry(dir, app)
	if err != nil {
		t.Fatal(err)
	}
	var handed telemetry.SharedApp
	if err := ReadCLIGitStateFile(dir, CLITelemetryFile, &handed); err != nil || !reflect.DeepEqual(handed, app) {
		t.Fatalf("recorded application %+v, %v", handed, err)
	}

	const processes, requests = 8, 40
	taken := make([][]int64, processes)
	var wg sync.WaitGroup
	for p := range processes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Each test process has its own sequence over the same directory.
			seq := &CLITelemetrySequence{dir: filepath.Join(dir, cliTelemetrySeqDir)}
			for range requests {
				taken[p] = append(taken[p], seq.Next())
			}
		}()
	}
	wg.Wait()
	if first := owner.Next(); first != 1 {
		t.Fatalf("app-started took %d", first)
	}
	var all []int64
	for p, numbers := range taken {
		if !slices.IsSorted(numbers) {
			t.Errorf("process %d took decreasing numbers: %v", p, numbers)
		}
		all = append(all, numbers...)
	}
	slices.Sort(all)
	for i, n := range all {
		if n != int64(i+2) {
			t.Fatalf("numbers are not 2..%d without repeats: %v", processes*requests+1, all)
		}
	}
	if last := owner.Next(); last != processes*requests+2 {
		t.Fatalf("app-closing took %d after %d numbers", last, processes*requests+1)
	}
	if owner.failed {
		t.Fatal("the sequence fell back to local numbers")
	}

	// Without its directory, a process keeps numbering its own requests.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if next := owner.Next(); next != processes*requests+3 || !owner.failed {
		t.Fatalf("without the directory: %d, failed %t", next, owner.failed)
	}
}

func TestCLITelemetryFailures(t *testing.T) {
	dir := t.TempDir()
	for _, tag := range []string{"status_code:403", cliTelemetryNetworkFailure, "status_code:403", CLITelemetrySequenceFailure} {
		recordCLITelemetryFailure(dir, tag)
	}
	if err := os.WriteFile(filepath.Join(dir, CLITelemetryFile), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"status_code:403": 2, cliTelemetryNetworkFailure: 1, CLITelemetrySequenceFailure: 1}
	if got := CLITelemetryFailures(dir); !reflect.DeepEqual(got, want) {
		t.Fatalf("failures %v, want %v", got, want)
	}
}
