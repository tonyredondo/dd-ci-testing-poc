# Validation contract

The default SDK runtime is the unchanged public dd-trace-go SDK, pinned to
[main at 96aedb31048c](https://github.com/DataDog/dd-trace-go/tree/96aedb31048c07e29e7a20a4333dc3b8d289c52d).
The reference is Orchestrion commit
[5c24783fcd76](https://github.com/DataDog/orchestrion/commit/5c24783fcd76f00cd1ff21c418a6662785d6c811),
installed as `v1.13.2-0.20260917114356-5c24783fcd76`.

## Architecture and dependency boundary

The driver imports only the Go standard library and its own packages. A targeted
`go list` supplies native `testing`, runtime and selected client-package metadata.
Preparation parses `testing` with `go/parser` and writes an overlay containing
changed sources, private hook declarations and an external runtime import for
each selected test package. Native Go performs the build.

Testify and goleak share library discovery. Known reachability without unknown
test imports uses `go list -find`; unknown imports require `-deps` to discover
libraries through helpers. Version and API validation happen before the native
build-cache lookup. A selective compiler wrapper transforms `testify/suite`
and, in Mini, reachable `go.uber.org/goleak`; a coverage bridge handles rewritten
`testing` sources when coverage includes them. Other builds omit `-toolexec`. The POC has
no configuration engine, build daemon or nested dependency build.

The nine SDK aspects are retained: M.Run, T.Run, B.Run, Fail, FailNow, formatted
errors, formatted skips, SkipNow and Parallel. Formatting wraps the already
formatted result, so String methods run once. The SDK's ownership marker retains
its existing name. Runtime retries, skip policies and finalization are owned by
the selected runtime. This POC depends on its private hook ABI and is intentionally version-pinned.

Virtual external test files anchor the public SDK import without editing project
sources. Temporary plans are invocation-local and removed after Go exits. There
is no additional persistent instrumentation cache. Test-result caching remains
Go's choice: explicitly select packages for result caching, and use `-count=1`
when fresh CI events are required. `-work` preserves Go's own work directory, but
the POC overlay is still removed; this POC does not provide retained overlay debugging.

## CI feature parity

The [feature inventory and differential matrix](ci-parity.md) compare Mini against
the full SDK/Orchestrion testing configuration. CI exports per-scenario counts
and supplemental hierarchy/span/telemetry evidence. [Testify validation](testify.md)
adds 26 policy/lifecycle cases plus selected-version, workspace, covered-library,
fast-bypass and cache-invalidation tests. Both local and external-module callers
reach the same original runner.

## Runtime checks

The tests compile actual native, POC and Orchestrion binaries using the same
fixture and temporary module graph. Adding Orchestrion may upgrade transitive
modules through Go MVS; that graph is used by both instrumented variants, and
an assertion verifies the SDK remains the pinned release. The reference YAML is
read directly from the installed SDK. The SDK backend reads the original module; the mini backend contains adapted CI source. See [mini runtime validation](mini-runtime.md).

| Contract | Proof |
| --- | --- |
| CLI activation | Unset enables parent-only mode; explicit true/false/parent/empty/custom values, real child processes and SDK-managed retries checked |
| TestMain, success, logs and exit codes | Native output with CI disabled; real SDK events versus Orchestrion when enabled |
| Subtests, nested tests, parallel children, cleanup and Context | Fixture asserts completion, callback order and cancellation before cleanup |
| Fail, FailNow, Error, Errorf, Fatal, Fatalf and Helper | Negative cases compare exit codes, error messages, stacks and source locations; formatting occurs once |
| Skip, Skipf, SkipNow | Native output and SDK skip semantics |
| Count, shuffle, list, JSON, tags and multiple packages | Native flags and selected test execution |
| Test-result cache | Second package-mode run is cached; count=1 runs again |
| Existing overlay | Both command flag and GOFLAGS preserve an intentional test failure |
| Race and atomic coverage | Actual instrumented fixture builds/runs; SDK events compared to Orchestrion |
| Benchmark, Example and fuzz seed | Native execution and reference SDK event semantics; one-iteration campaign in the parity inventory |
| Process retries | First attempt fails, second passes; reference events and process exit |
| EFD, ITR, disabled, quarantine, attempt-to-fix | Real SDK requests against loopback policy responses and reference event equivalence |
| Panic, Goexit and timeout | Abnormal exit and diagnostic marker; enabled/disabled reference event equivalence |
| Unsupported input | Missing/ambiguous hooks, malformed source, double instrumentation, conflicting toolexec and ambiguous arguments rejected |

The backend responses are synthetic, but hooks, retry processes, serialization
and network requests come from the real SDK. Fixtures inherit a small whitelist
of tool/path environment variables; real API keys and CI credentials are excluded.
Events are captured over loopback using a synthetic key.

The comparison retains event type, name, resource, error flag, semantic test
attributes, error stacks and source positions. It excludes generated IDs,
durations, timestamps, execution order and invocation names. It does not prove
identity of binaries or generated source bytes. The added parity matrix validates
the event ID hierarchy and additional CI span parentage independently.
Parallel child completion order and elapsed times may differ in native output;
line content and multiplicity are retained.

## Scope limits

The default backend supports module packages with the exact unreplaced SDK.
The optional mini backend requires this module instead. Standard
library test targets, explicit Go file mode, `-C` and custom flags before `-args`
are unsupported. Testify callers in client and external modules use the original
selected runner; dedicated version fixtures cover v1.11.1 and v1.12.1.
Other APM integrations remain outside the POC.
The AST transformer validates hook presence and ambiguity and selected shape
constraints; future Go source/ABI changes still require a new compatibility run.
Go 1.26 and 1.27 are selected from the current [official releases](https://go.dev/dl/).

The workflow is the source of evidence for the actual Linux/macOS/Windows and Go
versions it tests. This document describes its coverage, not an unconditional
compatibility guarantee. The local mini implementation was checked with development Go 1.27 and stable
Go 1.27.1. See the mini validation report for compile-only measurements. Full
fuzzing and arbitrary downstream modules remain unverified.

## Mini runtime follow-up

The current mini/SDK wire contract compares CI attributes and metrics while
explicitly excluding APM sampling, profiling and process enrichment. It also
checks complete envelope metadata, native hierarchy IDs, duplicate aliases,
service version and all session-name fallback cases. The unmodified SDK remains
the oracle; fewer APM fields is intentional.

| Added contract | Proof |
| --- | --- |
| Service version and session name | `TestMiniCIConfigurationWireParity`: explicit `DD_VERSION`, custom tags, automatic command/job name, explicitly empty session name |
| CI payload byte limits and gzip | `TestCIByteBatchingAndCompression`: multiple real decoded batches, all events delivered below 5 MiB in both agent/agentless modes |
| Oversized events and delivery failures | `TestLargeBatchFailureRetainedAndOversizedEventRejected`: single-event rejection, failed batch retention and recovery |
| Bazel output and offline mode | `TestMiniBazelOfflineAndPayloadFiles`: real manifest/cache, test/coverage/telemetry JSON files versus SDK, zero HTTP requests; native file writer error propagation tested separately |
| Parallel and retry coverage attribution | `TestMiniParallelAndRetryCoverageAttribution`: both runtimes compiled with `-race -covermode=atomic`, exact distinct-function bitmaps and initial-attempt-only retry policy |
| CI product metadata | Expanded pass/error/policy comparison retains capability tags and ITR correlation; delayed session enrichment is checked |
| Original CI assertions | Ported SDK tests, including retry runtime/parallel ownership, coverage writer/profile, ITR backfill, source metadata and lifecycle; [exact provenance](../internal/thirdparty/dd-trace-go/TESTS.json) |

Actual Bazel compiler invocation and real Datadog intake/UI acceptance remain
unverified. Native platform evidence is recorded below and is tied to its tested
revisions.

Before the SDK refresh, local verification passed on development Go 1.27:
the full normal suite and full `-race` suite, followed by focused race checks after strengthening the
parallel/retry CI assertions. Go 1.27.1 with `-trimpath` passed the new
configuration, byte-batching, Bazel, coverage-attribution and source-metadata
contracts. `go vet ./...` and both modules' `go mod verify` passed. The dependency
check confirms 266 mini runtime packages versus 511 SDK packages and excludes
`testify` from runtime imports. These checks do not establish real intake
acceptance or full upstream SDK-suite/platform parity.

## Verified CI revision

On 2026-10-02, [branch CI](https://github.com/tonyredondo/dd-ci-testing-poc/actions/runs/37035170311)
passed all four jobs for commit `fa7657f2f11e5d4f06cf006d7fb3e2f3a442e4e1`.
[PR CI](https://github.com/tonyredondo/dd-ci-testing-poc/actions/runs/37035175247)
also passed all four jobs for merge revision
`913f359cdddd00736f13e5a36b78f3b659fa58dc`, combining that head with
`main` at `e46c2dc1c94ff1f24038415726401c55f8d5cb2c`. Job checkout logs confirmed
those inputs.

The matrix ran the full compatibility suite on Linux with Go 1.26 and 1.27,
and on macOS and Windows with Go 1.27. Both Linux jobs also passed the full
`-race` suite. Source/license audits, their Python contract tests, module
verification and `go vet` passed. This closes the previously pending native
platform checks for that implementation, not for future code or SDK changes.

See the [maintenance guide](maintenance.md#verification-before-publication)
for checks to repeat after an update. Historical performance artifacts remain
unchanged; these CI runs did not repeat the build benchmark matrix.

## Selective-tool validation

The final selected implementation passed the complete Linux suite on Go 1.26.8
and the complete `-race` suite on Go 1.27.1. Each run included all 26 Testify
cases against the full SDK/Orchestrion reference. The [tool-strategy report](results/tool-strategies-20261003-linux-go1.27/README.md)
retains these event reports, source hashes and full local logs. The CLI also
cross-compiled for Windows/amd64 and macOS/arm64. Those local checks do not
establish native execution on either platform; publication must be followed
through the compatibility workflow.

[`TestTestifyVersionGuardWithWarmVendoredSources`](../integration/vendor_cache_test.go)
warms the build cache with supported vendored sources, then changes consistent
replacement metadata to an unsupported release while keeping source bytes
identical. Preparation must reject it even if Go would reuse the suite archive.
Other [selective-tool tests](../integration/selective_tools_test.go) check native
compiler/linker identities, bypass exit status, Unix process replacement,
coverage, workspaces and fingerprint invalidation. The compile-only matrix
records unchanged cache hits and cross-strategy reuse separately from runtime
feature comparisons.
