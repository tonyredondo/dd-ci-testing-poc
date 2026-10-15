package integrations

import (
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/testassert/require"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/civisibility"
)

// os.Exit during the tests closes CI Visibility through ddto's exit hook,
// running the close actions. The signal handler is told to stop but not
// joined: the process is ending.
func TestForcedExitClosesCIVisibility(t *testing.T) {
	replaceSignalNotifyForTesting(t, signal.Notify)
	resetCIVisibilityBootstrapStateForTesting()
	disableAdditionalFeaturesForBootstrapTest()
	t.Cleanup(restoreCIVisibilityBootstrapForTesting)

	initializeCIVisibilityLifecycleForTesting()
	handler := currentCIVisibilitySignalHandlerForTesting()
	require.NotNil(t, handler)
	closed := false
	PushCiVisibilityCloseAction(func() { closed = true })

	forcedExit()

	require.True(t, closed)
	require.True(t, handler.stopping.Load())
	require.Equal(t, civisibility.StateExited, civisibility.GetState())

	// The exit after the tests finds CI Visibility closed.
	closed = false
	forcedExit()
	require.False(t, closed)
}

// A panic in an exit hook makes the runtime end the process with a fatal
// error instead of os.Exit's status, so forcedExit recovers it.
func TestForcedExitRecoversPanics(t *testing.T) {
	resetCIVisibilityBootstrapStateForTesting()
	disableAdditionalFeaturesForBootstrapTest()
	t.Cleanup(restoreCIVisibilityBootstrapForTesting)

	initializeCIVisibilityLifecycleForTesting()
	PushCiVisibilityCloseAction(func() { panic("close action") })

	forcedExit()
}

// On Windows the signal handler's goroutine ends the process with os.Exit,
// which runs the exit hook on that goroutine. forcedExit must not wait for
// the handler to finish.
func TestForcedExitFromTheSignalHandlerReturns(t *testing.T) {
	replaceSignalNotifyForTesting(t, signal.Notify)
	resetCIVisibilityBootstrapStateForTesting()
	disableAdditionalFeaturesForBootstrapTest()
	t.Cleanup(restoreCIVisibilityBootstrapForTesting)

	exited := make(chan struct{})
	original := ciVisibilitySignalExitFunc
	ciVisibilitySignalExitFunc = func(os.Signal) {
		forcedExit() // what os.Exit's exit hook runs
		close(exited)
	}
	t.Cleanup(func() { ciVisibilitySignalExitFunc = original })

	initializeCIVisibilityLifecycleForTesting()
	handler := currentCIVisibilitySignalHandlerForTesting()
	require.NotNil(t, handler)
	handler.signals <- syscall.SIGTERM

	select {
	case <-exited:
	case <-time.After(5 * time.Second):
		t.Fatal("the exit hook waits for the signal handler that runs it")
	}
	assertSignalHandlerDone(t, handler)
}
