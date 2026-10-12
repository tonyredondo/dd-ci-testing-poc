// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2026 Datadog, Inc.

package net

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	assert "github.com/tonyredondo/dd-ci-testing-poc/internal/testassert"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/testassert/require"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/civisibility/constants"
)

// A local run's read-cache scope is ddto's invocation when the process uses
// its state: the Git worker and the test processes, whose parents differ,
// share entries. CI scopes already include both and stay.
func TestCLIReadCacheScopeIdentity(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	setReadCacheHooksForTest(t, t.TempDir(), &now, 111, 222)
	local := newReadCacheScopeIdentity(map[string]string{})
	worker := cliReadCacheScopeIdentity(local, "/state")
	setReadCacheHooksForTest(t, t.TempDir(), &now, 333, 444)
	process := cliReadCacheScopeIdentity(newReadCacheScopeIdentity(map[string]string{}), "/state")

	require.Equal(t, readCacheScopeCLI, worker.Kind)
	require.Equal(t, time.Hour, worker.TTL)
	require.Equal(t, worker, process)
	assert.NotEqual(t, worker.Hash, cliReadCacheScopeIdentity(local, "/other").Hash)
	require.Equal(t, local, cliReadCacheScopeIdentity(local, ""))
	strong := newReadCacheScopeIdentity(map[string]string{constants.CIProviderName: "github", constants.CIJobID: "job"})
	require.Equal(t, strong, cliReadCacheScopeIdentity(strong, "/state"))
}

// The worker's copies of its client request as the test processes do,
// although the worker runs another toolchain: the read cache's scope and every
// field of their requests match a test process's client. There is one copy per
// service that the test processes report.
func TestCLITestProcessClients(t *testing.T) {
	origEnv := saveEnv()
	defer restoreEnv(origEnv)
	setCiVisibilityEnv(os.Getenv("PATH"), "https://custom.agentless.url")

	process, ok := NewClient().(*client)
	require.True(t, ok)
	worker, ok := NewClient().(*client)
	require.True(t, ok)
	worker.testConfigurations.RuntimeVersion = "go1.0-ddto"
	worker.testConfigurations.OsArchitecture = "ddto-arch"

	clients := CLITestProcessClients(worker, process.testConfigurations.RuntimeVersion, process.testConfigurations.OsArchitecture, nil)
	require.Len(t, clients, 1)
	copied, ok := clients[0].(*client)
	require.True(t, ok)
	request := func(c *client) client {
		fields := *c
		fields.id, fields.headers, fields.handler = "", nil, nil
		return fields
	}
	require.Equal(t, request(process), request(copied))
	require.Equal(t, process.readCacheBaseScope(), copied.readCacheBaseScope())
	require.Equal(t, "go1.0-ddto", worker.testConfigurations.RuntimeVersion)

	dirs := []string{t.TempDir(), t.TempDir()}
	clients = CLITestProcessClients(worker, "go1.99.0", "", dirs)
	require.Len(t, clients, 1)
	require.Equal(t, "dd-trace-go", clients[0].(*client).serviceName)
	require.Equal(t, "go1.99.0", clients[0].(*client).testConfigurations.RuntimeVersion)
	require.Equal(t, "ddto-arch", clients[0].(*client).testConfigurations.OsArchitecture)
	os.Setenv("DD_SERVICE", "named")
	clients = CLITestProcessClients(worker, "go1.99.0", "", dirs)
	require.Len(t, clients, 1)
	require.Equal(t, "named", clients[0].(*client).serviceName)
}

// The invocation's entries outlast the local TTL, as a long build does, and
// live in the invocation's own directory, which ddto removes afterwards.
func TestCLIReadCacheLastsTheInvocation(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	root := t.TempDir()
	setReadCacheHooksForTest(t, root, &now, 111, 222)
	// Without custom configurations, as clients have them, entries survive
	// their JSON encoding.
	worker := newReadCacheTestClient(map[string]string{})
	worker.testConfigurations.Custom = nil
	worker.readCacheScopeIdentity = cliReadCacheScopeIdentity(worker.readCacheScopeIdentity, "/state")
	setReadCacheHooksForTest(t, root, &now, 333, 444)
	process := newReadCacheTestClient(map[string]string{})
	process.testConfigurations.Custom = nil
	process.readCacheScopeIdentity = cliReadCacheScopeIdentity(process.readCacheScopeIdentity, "/state")
	request := map[string]string{"request": "ahead"}
	live := 0
	fetch := func(c *client) string {
		t.Helper()
		value, err := readThroughShortLivedCache(c, "unit", request, func() (readCacheLiveResult[string], error) {
			live++
			return readCacheLiveResult[string]{Value: "response", Cacheable: true}, nil
		}, nil)
		require.NoError(t, err)
		return value
	}

	require.Equal(t, "response", fetch(worker))
	now = now.Add(5 * time.Minute)
	require.Equal(t, "response", fetch(process))
	require.Equal(t, 1, live)

	shared := filepath.Join(root, "dd-trace-go", "civisibility-read-cache")
	entries, err := os.ReadDir(shared)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, readCacheCLIDirectory, entries[0].Name())
	own := filepath.Join(shared, readCacheSubdirectory(worker.readCacheScopeIdentity))
	_, err = os.Stat(own)
	require.NoError(t, err)
	require.NoError(t, RemoveCLIReadCache("/state"))
	_, err = os.Stat(own)
	require.True(t, os.IsNotExist(err))
}

// A stopped worker does not wait for another process's request: the read
// cache's waiter stops with the client's context.
func TestReadCacheWaiterStopsWithContext(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	root := t.TempDir()
	setReadCacheHooksForTest(t, root, &now, 111, 222)
	c := newReadCacheTestClient(map[string]string{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.handler = &RequestHandler{Context: ctx}
	request := map[string]string{"request": "stopped"}
	cacheKey, paths := readCacheTestKeyAndPaths(t, c, "unit", request)
	owner, status := acquireReadCacheLock(paths, cacheKey)
	require.Equal(t, readCacheLockAcquired, status)
	defer releaseReadCacheLock(owner)
	sleeps := 0
	SetReadCacheHooksForTesting(root, func() time.Time { return now }, func() int { return 111 }, func() int { return 222 }, func(d time.Duration) {
		sleeps++
		cancel()
		now = now.Add(d)
	})

	_, err := readThroughShortLivedCache(c, "unit", request, func() (readCacheLiveResult[string], error) {
		return readCacheLiveResult[string]{}, ctx.Err()
	}, nil)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, sleeps)
}

// writeCLIEntry writes, in the invocation directory dir, an entry named name
// created at createdAt that lasts ttl.
func writeCLIEntry(t *testing.T, dir, name string, createdAt time.Time, ttl time.Duration) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o700))
	entry := fmt.Sprintf(`{"created_at_unix_nano":%d,"ttl_seconds":%d}`, createdAt.UnixNano(), int64(ttl.Seconds()))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(entry), 0o600))
}

// A kill or a reboot leaves an invocation's directory. A later invocation
// removes its expired entries, as the shared directory's cleanup does, and
// then the emptied directory. A running invocation's entries, which have not
// expired, stay, and so does a directory where a process holds a lock.
func TestRemoveCLIReadCacheRemovesExpiredEntries(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	root := t.TempDir()
	setReadCacheHooksForTest(t, root, &now, 111, 222)
	shared := filepath.Join(root, "dd-trace-go", "civisibility-read-cache")
	cli := filepath.Join(shared, readCacheCLIDirectory)
	writeCLIEntry(t, filepath.Join(cli, "ended"), "ended.json", now.Add(-2*time.Hour), time.Hour)
	writeCLIEntry(t, filepath.Join(cli, "running"), "running.json", now.Add(-time.Minute), time.Hour)
	writeCLIEntry(t, filepath.Join(cli, "writing"), "writing.lock", now.Add(-time.Minute), time.Hour)
	writeCLIEntry(t, shared, "shared.json", now.Add(-2*time.Hour), time.Hour)

	require.NoError(t, RemoveCLIReadCache("/state"))
	for name, kept := range map[string]bool{
		filepath.Join(readCacheCLIDirectory, "ended"):                   false,
		filepath.Join(readCacheCLIDirectory, "running", "running.json"): true,
		filepath.Join(readCacheCLIDirectory, "writing", "writing.lock"): true,
		"shared.json": true,
	} {
		_, err := os.Stat(filepath.Join(shared, name))
		require.Equal(t, kept, err == nil, name)
	}
}

// Each invocation examines a bounded number of directories. Directories that
// stay, whatever their order, hide the others only until their entries expire.
func TestRemoveCLIReadCacheReachesEveryDirectory(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	root := t.TempDir()
	setReadCacheHooksForTest(t, root, &now, 111, 222)
	cli := filepath.Join(root, "dd-trace-go", "civisibility-read-cache", readCacheCLIDirectory)
	for i := range readCacheCLICleanupMaxDirectories {
		writeCLIEntry(t, filepath.Join(cli, fmt.Sprint("fresh-", i)), "entry.json", now.Add(-time.Minute), time.Hour)
	}
	for i := range 2 {
		writeCLIEntry(t, filepath.Join(cli, fmt.Sprint("expired-", i)), "entry.json", now.Add(-2*time.Hour), time.Hour)
	}
	count := func() int {
		entries, err := os.ReadDir(cli)
		require.NoError(t, err)
		return len(entries)
	}

	require.NoError(t, RemoveCLIReadCache("/state"))
	require.True(t, count() >= readCacheCLICleanupMaxDirectories, "a fresh entry was removed")
	now = now.Add(2 * time.Hour)
	require.NoError(t, RemoveCLIReadCache("/state"))
	// Every entry has expired: a call removes the bounded number of
	// directories, and the next one the rest.
	require.Equal(t, 2, count())
	require.NoError(t, RemoveCLIReadCache("/state"))
	require.Equal(t, 0, count())
}

// The cleanup can remove an invocation's empty directory after a process
// created it for a request: the process creates it again for its lock, and the
// directory, which then holds the lock, cannot go while the process writes.
func TestReadCacheLockRecreatesRemovedDirectory(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	setReadCacheHooksForTest(t, t.TempDir(), &now, 111, 222)
	identity := cliReadCacheScopeIdentity(newReadCacheScopeIdentity(map[string]string{}), "/state")
	paths, err := readCachePathsIn(readCacheSubdirectory(identity), "key")
	require.NoError(t, err)
	require.NoError(t, os.Remove(paths.Dir))

	owner, status := acquireReadCacheLock(paths, "key")
	require.Equal(t, readCacheLockAcquired, status)
	require.Error(t, os.Remove(paths.Dir))
	releaseReadCacheLock(owner)
}
