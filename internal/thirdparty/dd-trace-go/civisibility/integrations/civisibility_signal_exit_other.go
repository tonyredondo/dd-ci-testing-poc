//go:build !unix

package integrations

import "os"

// exitBySignal ends the process with exit status 1, as the SDK does: Windows
// cannot send a console event to this process alone.
func exitBySignal(os.Signal) {
	os.Exit(1)
}
