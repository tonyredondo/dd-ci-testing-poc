// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2026 Datadog, Inc.

package net

import (
	"context"
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
	require.Equal(t, readCacheSubdirectory(worker.readCacheScopeIdentity), entries[0].Name())
	require.NoError(t, RemoveCLIReadCache("/state"))
	_, err = os.Stat(filepath.Join(shared, entries[0].Name()))
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

// An invocation that ended without removing its entries, as a killed ddto
// does, leaves a directory that the next invocation removes once its entries
// expired; another invocation's recent directory stays.
func TestRemoveCLIReadCacheRemovesAbandonedDirectories(t *testing.T) {
	now := time.Now()
	root := t.TempDir()
	setReadCacheHooksForTest(t, root, &now, 111, 222)
	shared := filepath.Join(root, "dd-trace-go", "civisibility-read-cache")
	for _, name := range []string{"cli-abandoned", "cli-running", "unrelated"} {
		require.NoError(t, os.MkdirAll(filepath.Join(shared, name), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(shared, name, "entry.json"), nil, 0o600))
	}
	old := now.Add(-2 * time.Hour)
	for _, name := range []string{"cli-abandoned", "unrelated"} {
		require.NoError(t, os.Chtimes(filepath.Join(shared, name), old, old))
	}

	require.NoError(t, RemoveCLIReadCache("/state"))
	for name, kept := range map[string]bool{"cli-abandoned": false, "cli-running": true, "unrelated": true} {
		_, err := os.Stat(filepath.Join(shared, name))
		require.Equal(t, kept, err == nil, name)
	}
}
