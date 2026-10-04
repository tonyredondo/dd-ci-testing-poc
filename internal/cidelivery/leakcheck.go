package cidelivery

import "sync"

var sendGate sync.RWMutex
var connections struct {
	sync.Mutex
	closers map[*connectionCloser]struct{}
}

type connectionCloser struct{ close func() }

// BeginSend and EndSend bracket only CI network delivery. A leak check waits
// for existing sends, then pauses new ones until goleak has taken its snapshots.
// The named waiting frame lets the shim distinguish our senders from user work.
func BeginSend() { sendGate.RLock() }
func EndSend()   { sendGate.RUnlock() }

func RegisterConnectionCloser(close func()) func() {
	entry := &connectionCloser{close: close}
	connections.Lock()
	if connections.closers == nil {
		connections.closers = make(map[*connectionCloser]struct{})
	}
	connections.closers[entry] = struct{}{}
	connections.Unlock()
	return func() {
		connections.Lock()
		delete(connections.closers, entry)
		connections.Unlock()
	}
}

// PrepareLeakCheck leaves goroutine discovery to goleak. Closing only owned CI
// connections allows its normal retry loop to observe HTTP workers terminating;
// no net/http function or goroutine snapshot is added to an ignore filter.
func PrepareLeakCheck() func() {
	sendGate.Lock()
	connections.Lock()
	closers := make([]func(), 0, len(connections.closers))
	for entry := range connections.closers {
		closers = append(closers, entry.close)
	}
	connections.Unlock()
	for _, close := range closers {
		close()
	}
	return sendGate.Unlock
}
