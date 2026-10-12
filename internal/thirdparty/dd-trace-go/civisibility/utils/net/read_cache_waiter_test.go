// Copyright 2026 Datadog, Inc. Licensed under the Apache License, Version 2.0.

package net

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/testassert/require"
)

// waiterTest holds a lock for request as the process ownerPID, as another
// process would, and runs the read cache as the process 111.
type waiterTest struct {
	t       *testing.T
	now     time.Time
	start   time.Time
	c       *client
	request map[string]string
	paths   readCachePaths
	key     string
	owner   *readCacheLockOwner
	alive   map[int]bool
	// onSleep runs after each poll's sleep, with the time waited so far.
	onSleep  func(waited time.Duration)
	maxSleep time.Duration
}

func newWaiterTest(t *testing.T, ownerPID int, lockAge time.Duration) *waiterTest {
	w := &waiterTest{t: t, now: time.Unix(1_700_000_000, 0), c: newReadCacheTestClient(map[string]string{}), request: map[string]string{"request": t.Name()}, alive: map[int]bool{}}
	// An empty custom map does not survive the entry's JSON, as in
	// TestReadThroughShortLivedCacheWritesSanitizedEntryAndHits.
	w.c.testConfigurations.Custom = map[string]string{"Region": "us1"}
	root := t.TempDir()
	pid := ownerPID
	SetReadCacheHooksForTesting(root, func() time.Time { return w.now }, func() int { return pid }, func() int { return 222 }, func(d time.Duration) {
		w.maxSleep = max(w.maxSleep, d)
		w.now = w.now.Add(d)
		if w.onSleep != nil {
			w.onSleep(w.now.Sub(w.start))
		}
	})
	t.Cleanup(ResetReadCacheHooksForTesting)
	setReadCacheProcessAliveForTesting(func(pid int) (bool, bool) {
		alive, known := w.alive[pid]
		return alive, known
	})
	w.key, w.paths = readCacheTestKeyAndPaths(t, w.c, "unit", w.request)
	w.now = w.now.Add(-lockAge)
	var status readCacheLockAcquireStatus
	w.owner, status = acquireReadCacheLock(w.paths, w.key)
	require.Equal(t, readCacheLockAcquired, status)
	w.now = w.now.Add(lockAge)
	w.start = w.now
	pid = 111
	w.alive[111] = true
	return w
}

// publish writes the owner's entry and releases its lock, as the owner does.
func (w *waiterTest) publish(value string) {
	requestHash, err := readCacheHashJSON(w.request)
	require.NoError(w.t, err)
	endpointScope := readCacheEndpointScope{Endpoint: "unit", EndpointVersion: readCacheEndpointVersionFor("unit"), RequestHash: requestHash}
	writeReadCacheEntry(w.paths, w.owner, w.key, w.c.readCacheBaseScope(), endpointScope, w.c.readCacheScopeIdentity.TTL, value)
	releaseReadCacheLock(w.owner)
	require.FileExists(w.t, w.paths.CacheFile)
}

// read reads through the cache and returns the value, how many requests the
// process made and how long it waited.
func (w *waiterTest) read() (string, int, time.Duration) {
	requests := 0
	value, err := readThroughShortLivedCache(w.c, "unit", w.request, func() (readCacheLiveResult[string], error) {
		requests++
		return readCacheLiveResult[string]{Value: "own request", Cacheable: true}, nil
	}, nil)
	require.NoError(w.t, err)
	return value, requests, w.now.Sub(w.start)
}

// A waiter keeps waiting past readCacheWaiterTimeout while the lock's owner
// runs, and receives its entry instead of sending the request again. It polls
// at least every 20 ms.
func TestReadCacheWaiterWaitsForLiveOwner(t *testing.T) {
	w := newWaiterTest(t, 333, 0)
	w.alive[333] = true
	w.onSleep = func(waited time.Duration) {
		if waited >= 5*time.Second {
			w.publish("owner's response")
			w.onSleep = nil
		}
	}
	value, requests, waited := w.read()
	require.Equal(t, "owner's response", value)
	require.Equal(t, 0, requests)
	require.LessOrEqual(t, waited, 5*time.Second+readCacheWaiterMaxBackoff)
	require.LessOrEqual(t, w.maxSleep, readCacheWaiterMaxBackoff)
	require.Equal(t, 20*time.Millisecond, readCacheWaiterMaxBackoff)
}

// The wait for a running owner ends readCacheWaiterLiveOwnerTimeout after the
// lock's creation, and a lock that old gets the usual two seconds.
func TestReadCacheWaiterBoundsTheWaitForLiveOwner(t *testing.T) {
	for _, tc := range []struct {
		lockAge, waited time.Duration
	}{
		{0, readCacheWaiterLiveOwnerTimeout},
		{time.Minute, readCacheWaiterTimeout},
	} {
		w := newWaiterTest(t, 333, tc.lockAge)
		w.alive[333] = true
		value, requests, waited := w.read()
		require.Equal(t, "own request", value)
		require.Equal(t, 1, requests)
		require.GreaterOrEqual(t, waited, tc.waited)
		require.LessOrEqual(t, waited, tc.waited+readCacheWaiterMaxBackoff)
		_, err := os.Stat(w.paths.CacheFile)
		require.True(t, os.IsNotExist(err), "a waiter that falls back writes no entry")
	}
}

// A lock whose owner's process ended is stale: the next process takes it at
// once and publishes the entry, before it waits or when its owner ends during
// the wait.
func TestReadCacheTakesOverDeadOwnersLock(t *testing.T) {
	for _, diesAfter := range []time.Duration{0, time.Second} {
		w := newWaiterTest(t, 333, 0)
		w.alive[333] = diesAfter != 0
		w.onSleep = func(waited time.Duration) {
			if waited >= diesAfter {
				w.alive[333] = false
			}
		}
		value, requests, waited := w.read()
		require.Equal(t, "own request", value)
		require.Equal(t, 1, requests)
		require.LessOrEqual(t, waited, diesAfter+readCacheWaiterMaxBackoff)
		require.FileExists(t, w.paths.CacheFile, "the process that took over publishes the entry")
		_, err := os.Stat(w.paths.LockFile)
		require.True(t, os.IsNotExist(err))
	}
}

// processAlive tells a running process from one that ended.
func TestProcessAlive(t *testing.T) {
	alive, known := processAlive(os.Getpid())
	require.True(t, known)
	require.True(t, alive)

	cmd := exec.Command(os.Args[0], "-test.run=^$")
	require.NoError(t, cmd.Run())
	alive, known = processAlive(cmd.Process.Pid)
	require.True(t, known)
	require.False(t, alive)

	_, known = processAlive(0)
	require.False(t, known)
}
