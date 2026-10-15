//go:build unix

package integrations

import (
	"os"
	"syscall"
)

// exitBySignal ends the process with sig, as Go's runtime does for a signal
// that no channel wants, so go test reports it as without CI Visibility. If
// another channel registered for sig in the meantime, that channel receives
// it and the process goes on.
func exitBySignal(sig os.Signal) {
	if s, ok := sig.(syscall.Signal); ok {
		_ = syscall.Kill(syscall.Getpid(), s)
	}
}
