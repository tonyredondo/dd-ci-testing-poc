package runner

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/civisibility/utils"
)

// TestGitWorkerHelperProcess is a worker that reads its go-ahead, closes its
// results without writing them, and runs until go test exits.
func TestGitWorkerHelperProcess(t *testing.T) {
	if os.Getenv("DDTO_GIT_WORKER_HELPER") != "1" {
		t.Skip("helper process")
	}
	input := bufio.NewReader(os.Stdin)
	_, _ = input.ReadString('\n')
	_ = os.Stdout.Close()
	_, _ = io.Copy(io.Discard, input)
	os.Exit(0)
}

// A worker that closes its results without writing them, as after a failed
// write, still runs until go test exits; ddto records the results at once,
// so test processes stop waiting for them.
func TestGitWorkerRecordsResultsWhenOutputCloses(t *testing.T) {
	state := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestGitWorkerHelperProcess$")
	cmd.Env = append(os.Environ(), "DDTO_GIT_WORKER_HELPER=1")
	w, err := launchGitWorker(cmd, state, nil)
	if err != nil {
		t.Fatal(err)
	}
	w.proceeded.Store(true)
	if _, err := fmt.Fprintln(w.input, "proceed"); err != nil {
		t.Fatal(err)
	}
	var upload utils.CLIGitUpload
	if err := utils.WaitCLIGitStateFile(state, utils.CLIGitUploadFile, &upload, time.Minute); err != nil || upload.Error == "" {
		t.Fatalf("upload %+v, %v", upload, err)
	}
	select {
	case <-w.exited:
		t.Fatal("the worker exited before go test")
	default:
	}
	_ = w.input.Close()
	select {
	case <-w.exited:
	case <-time.After(time.Minute):
		t.Fatal("the worker did not exit after go test")
	}
}
