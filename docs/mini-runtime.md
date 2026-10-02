# Native CI runtime

The experimental `mini` backend keeps the testing hooks and CI policies extracted
from `dd-trace-go/v2@v2.11.0-rc.1`, replacing its general tracer with a native CI
event client. The SDK backend remains the default and uses the original module.
Neither the original SDK nor target sources are edited.

## Use the local POC

Build the driver from this checkout:

```sh
go build -o bin/ddtest ./cmd/ddtest
```

In the target module, explicitly add the local runtime before invoking the CLI:

```sh
go mod edit -replace=github.com/tonyredondo/dd-ci-testing-poc=/path/to/this/checkout
go get github.com/tonyredondo/dd-ci-testing-poc/testopt@v0.0.0
/path/to/ddtest test --runtime=mini -count=1 ./...
```

`--runtime` must immediately follow `test`. Omitting it selects `sdk`. The mini
backend does not require `dd-trace-go`. Existing application imports of that SDK
still link it normally; switching the instrumentator does not remove those imports.
Both backends retain the CLI's `DD_CIVISIBILITY_ENABLED=parent` default when absent.
Native Go controls compilation, test-result caching, flags and exit status.

## Portable test context

```go
ctx := testopt.Context(t)
identity, ok := propagation.FromContext(ctx)
if ok {
    headers := http.Header{}
    if err := propagation.Inject(identity, headers, propagation.W3C); err != nil {
        t.Fatal(err)
    }
    // Attach headers to an integration-test request.
}
```

Import `testopt` and `propagation` from this module. `testopt.Context` retains the
native testing cancellation lifetime. Subtests have their own event identity.
An uninstrumented test has no identity. Isolated retry children also have no local
identity: their events are reconstructed by the controlling parent, following the
original SDK contract.

Both W3C `traceparent`/`tracestate` and Datadog headers carry the complete 128-bit
trace ID and active span ID. W3C vendor state is preserved. Sampling priority is
propagation metadata; the mini client records every CI event. Invalid identifiers
return an error. This is trace identity propagation, without an OpenTelemetry SDK,
baggage API, or an APM adapter.

A private `context.Context` value is not automatically visible to another tracer.
The receiving tracer must explicitly extract a carrier. The integration suite
checks extraction by the original `dd-trace-go` propagator. A future dd-trace-go
helper can bridge the portable value in the same process; that shim is outside
this implementation.

## Native events and delivery

The client writes the CI test-cycle MessagePack protocol directly: envelope
version 1, test events version 2, and session/module/suite events version 1. CI
hierarchy IDs are separate from distributed trace identity. The runtime has its
own experimental version, `0.0.0-ci-mini`.

Agentless delivery uses gzip and `/api/v2/citestcycle`. Agent delivery uses the EVP
v2 proxy and its intake header. Coverage continues through the extracted native
CI coverage client. No APM tracer, profiler, security engine, remote configuration
client, OTLP stack, or agent statistics pipeline is imported by the mini runtime.
Existing CI configuration, diagnostics and CI telemetry helpers are retained.
`DD_VERSION` sets the service `version` tag on CI events. Session naming reuses
the SDK algorithm: an explicitly defined `DD_TEST_SESSION_NAME` wins, including
an empty value; otherwise use `<CI job name>-<test command>`, or the test command
when no job name is available. The name is sent in envelope metadata.

Test-cycle batches use the CI writer's 5 MiB uncompressed MessagePack limit and
2.5 MiB flush threshold. Accounting includes the envelope and uses schema
`Msgsize()` upper bounds, so batching is conservative. A single oversized event
is rejected explicitly. Successful gzip compression never bypasses the byte
limit. The event-count bound remains in force, failed batches retain their byte
accounting, and request failures, serialization and drops report CI telemetry.
Delivery retains bounded retries, permanent-error handling and cancellation.

Bazel payload-file mode converts native test-cycle MessagePack to JSON before
writing to `TEST_UNDECLARED_OUTPUTS_DIR/payloads/tests/`. The extracted coverage
and CI telemetry writers use their corresponding directories. Manifest mode
uses the original offline read-cache rules. File mode does not require HTTP
credentials for the native event client and does not send event HTTP requests.
The SDK-derived settings loader still requires a nonempty API key to consume
a settings cache; the offline differential fixture supplies a synthetic key.
The test verifies zero HTTP requests with all three payload types present.
The mini runtime imports 266 packages including the standard library, versus 511
for the original SDK public runtime. Six direct utility modules and one indirect
module remain in the runtime graph. Test-only `testify` dependencies support
the ported SDK assertions and do not enter `go list -deps ./testopt`. The CLI
has no external package imports.
[Dependency and module integrity proof](mini-dependency-proof.json) retains the
package lists and successful module verification output.

`testopt.New` also creates an explicit client without reading API credentials
from the environment. Its configuration controls endpoint, tags, batch capacity
and timeout. `Finish` seals an event once; `Flush` and `Close` report delivery
errors. Failed batches stay queued for a later flush. A full queue applies
backpressure and rejects incoming events if delivery fails; `DroppedEvents`
reports those rejections, including events finished after closure. Hook-driven
shutdown logs delivery failures and rejected counts without changing test results.
An ambiguous HTTP failure can lead to a repeated delivery; this is not a durable
or exactly-once delivery mechanism.

## Verification and boundaries

The local suite checks real loopback payloads against the SDK, preserving test
attributes, statuses, error messages, stack frames and source lines. Only the
relocated library namespace and its source root are canonicalized in mini stack
comparisons; application frames are retained. The expanded comparator keeps CI
metadata, metrics, service/resource/type, custom tags, capability tags and ITR
correlation. It explicitly excludes APM sampling/profiling/process enrichment
and process-local identity/order. Duplicate SDK Git aliases and hierarchy IDs
are checked for agreement before exclusion. Native MessagePack hierarchy IDs
and complete envelope metadata are tested separately. Benchmark timing metrics
are observational and retain their existing semantic benchmark checks. Synthetic policy servers and their read caches are isolated so port reuse cannot reuse another policy response.

Coverage checks link each payload to its test event and verify the executed
application line belongs to `TestPass`, with no attribution to `TestSkip`.
Race builds exercise parallel and nested tests. A second race/atomic-coverage
fixture executes distinct functions in concurrent tests with shuffle/count,
then retries a failing test against another distinct function. Exact coverage
bitmaps are checked against the SDK. Each parallel test owns only its function
blocks. Retry coverage follows the SDK policy: the initial attempt is uploaded,
and subsequent attempts must not contaminate its bitmap.
Additional checks cover CLI
activation and inheritance, TestMain, all failure/skip methods, retries, EFD,
ITR, quarantine, attempt-to-fix, benchmarks, examples, fuzz seeds, panic, Goexit,
timeouts, overlays, JSON, build tags, multiple packages and Go's result cache.

This is a POC. Real Datadog intake acceptance and UI behavior are unverified.
Agent tests use a loopback EVP implementation, not a deployed agent. Bazel
manifest/payload-file contracts are checked against the SDK using a real offline
manifest, read cache and output files; an actual Bazel build/toolchain invocation
has not been run. Full fuzz campaigns and `testify/suite` integration remain
outside the instrumentator's verified scope. Cross-platform runtime proof
requires the existing GitHub Actions matrix after publication.
The exact local checks and their limits are recorded in
[the validation contract](validation.md#mini-runtime-follow-up).

Go rejects overlays targeting a toolchain beneath `GOMODCACHE`. This pre-existing
limitation affects both backends. Stable Go 1.27.1 was verified using an identical
toolchain copy outside that cache. The CLI does not relocate toolchains.

## Source ownership

Extracted CI code and helper files retain their upstream Apache 2.0 copyright
headers. Adaptations change internal import paths, bind span calls to the native
client, replace private metadata linknames with direct access, and remove APM
mock, security, profiling and process-tag dependencies. The hook execution and
CI policy logic are retained. See `NOTICE` and the generated MessagePack schema
in `internal/minitracer/events.go`.

## Initial compile-only comparison

Chi v5.3.2, eight physical cores, `go test -c -o <directory>/ -ldflags=-w ./...`.
No test binaries executed. Both SDK instruments use the same unmodified SDK; all
four variants use the same project inputs and module resolution graph. Values are
wall-time medians; percentages compare mini with the POC SDK backend.

| Scenario | Native | Orchestrion | POC SDK | POC Mini |
| --- | ---: | ---: | ---: | ---: |
| Cold (n=2) | 6.68 s | 30.95 s | 14.66 s | 8.37 s (-42.9%) |
| Cached dependencies, forced link (n=5) | 0.28 s | 1.75 s | 1.13 s | 0.53 s (-53.3%) |

CPU accounting includes all descendants in an exclusive cgroup. Cold median CPU
falls from 73.74 s (POC SDK) to 38.58 s (mini); native uses 29.85 s and Orchestrion
112.51 s. Warm-link median CPU is 1.80 s (SDK), 0.86 s (mini), 0.62 s (native), and
4.44 s (Orchestrion). Every build produced two checked binaries without DWARF;
only mini binaries contain the mini hooks, and they do not link dd-trace-go.

These measurements predate the compatibility follow-up (service version,
session metadata, byte accounting, Bazel transport and CI telemetry) and have
not been rerun for the current source. These results are exploratory.
The native A/A control ranges from 0.255 to 1.052 s
with almost unchanged CPU consumption, showing wall-time noise. Warm-link ranges
overlap: SDK 0.793-1.169 s; mini 0.393-1.536 s. A stable warm wall-time benefit is
not established by this experiment. No samples were discarded. Two cold runs do
not establish stability or results for other projects.

[All 32 commands, input hashes, ranges and binary checks](benchmark-mini-chi.json)
are retained. The earlier README matrix measures the SDK backend, not mini.

## Reused upstream CI tests

59 complete CI test files from the pinned SDK are ported, along with
Bazel mode and telemetry-recorder tests and selected concurrency assertions.
They cover configuration, CI providers and session naming, source/trimpath and
CODEOWNERS metadata, Git fixtures, impacted tests, networking, coverage writers
and profiles, retry runtime ownership/lifecycle, feature selection, ITR
backfill, offline cache, HTTP lifecycle, signals and CI telemetry.
[Source hashes and exact adaptations](ci-test-provenance.json) distinguish
complete files from selected tests/helpers and list what is not fully ported.
The port does not claim to reproduce every SDK test: APM mock/span-count and
Orchestrion-specific harnesses have different prerequisites. Actual native
spans and wire integration tests supply the corresponding runtime evidence.

### Compatibility harness portability

Fixture preparation accepts LF and CRLF checkouts. Parallel/retry coverage
checks use the current Go toolchain's independent `go tool cover` ranges rather
than Go 1.27-specific byte constants, retaining exact per-test bitmap checks.
The copied backend-count/read-cache test runs its complete assertions in a
fresh process so mock bootstrap workers from preceding tests cannot reuse its
package globals.

The SDK starts feature discovery asynchronously. Its initial session event can
omit capabilities or ITR correlation while its test events contain them. Mini
retains these session attributes. The comparator fills only missing session
values from consistent attributes of that session's actual test events; it
never drops a CI attribute, changes a present value, crosses session identities
or fills a missing test attribute. Unit checks reject conflicts and omissions.
This is semantic inheritance parity, not byte-identical event placement.

Listing tests emits a session without test events. That case validates Mini's
complete capabilities against the frozen SDK declarations and permits only
missing SDK capabilities. Wrong values, unknown capabilities, test events and
all other CI attribute differences remain failures.
