package integrations

import (
	"os/signal"
	"testing"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/testassert/require"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/civisibility"
)

// os.Exit during the tests closes CI Visibility through ddto's exit hook,
// running the close actions and stopping the signal handler.
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
	assertSignalHandlerDone(t, handler)
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
