// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2026 Datadog, Inc.

package net

import (
	"os"
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
	require.Equal(t, local.TTL, worker.TTL)
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
