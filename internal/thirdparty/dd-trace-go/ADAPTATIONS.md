# Performance adaptations to the SDK port

Base: [`96aedb31048c07e29e7a20a4333dc3b8d289c52d`](https://github.com/DataDog/dd-trace-go/commit/96aedb31048c07e29e7a20a4333dc3b8d289c52d).
`SOURCE.json` retains each original path/hash alongside the local hash. This
record explains changes that an upstream synchronization must review manually.

## Retained metric handles

[`civisibility/utils/telemetry/telemetry_count.go`](civisibility/utils/telemetry/telemetry_count.go)
binds counters for the ordinary test/suite/module/session tags and the enqueue
counter. Each binding lazily registers through the existing global registry;
tags are sorted and the swappable handle is created once. Steady-state calls
submit directly, without rebuilding tag slices or joining a metric key.

The set is bounded: two framework classifications, four hierarchy types and two
lifecycle counters. Additional feature tags use the general path. Retry, EFD,
quarantine, disabled/attempt-to-fix and benchmark tags must never be discarded
just to select a bound handle. Values of the SDK's mutable tag slices are checked
on each call; their slice identity is not a cache key.

[`telemetry/metrichandle.go`](telemetry/metrichandle.go) owns a copy of bound tags.
Bindings preserve disabled telemetry, calls before `StartApp`, startup replay and
`SwapClient`. `MockClient` clears global registrations, so
[`telemetry/globalclient.go`](telemetry/globalclient.go) advances a generation at
that reset. A binding registers again on its next use; ordinary swaps still use
the existing swappable pointer. If upstream changes recorder limits, client
swapping, disablement or test resets, review this lifetime as well.

Checks: `TestBoundCountReplaysSwapsAndResets`,
`TestGlobalRegistrationReplaysAndSwapsOnce` and
`TestEventCountersKeepCanonicalAndFeatureTagsAcrossClients` and
`TestEventCountersDisabled`, plus the HTTP
telemetry parity matrix. Verify the feature-tag fallback and disabled behavior,
not just the ordinary three-counter benchmark.

## Counter points kept inline

[`telemetry/metrics.go`](telemetry/metrics.go) keeps a metric's value, timestamp
and presence flag together under a short `sync.Mutex`. Upstream allocated an
immutable point on every count/gauge submission and replaced an atomic pointer.
The port allocates no point on submission. Collection detaches and resets the
whole point under the same lock, then constructs the wire payload after release.

Every concurrent increment belongs to exactly one collection. `Get` returns
`NaN` until a submission and after collection, while a submitted zero is a real
point. Fractional/negative values and gauge replacement keep their semantics.
Timestamps are captured before competing for the lock. They need not be
monotonic between concurrent callers.

Rate metrics retain their existing atomic interval start and short-interval
rule. A short interval does not consume the accumulated count; an eligible
collection detaches the count and divides it by that interval. Distributions,
wire fields and interval calculation have not been redesigned.

Checks: `TestMetricPointLifecycle`,
`TestConcurrentMetricCollectionPreservesValues`, telemetry HTTP parity and
`-race`. Compare both serial updates and contended updates when changing this
lock; an allocation reduction alone does not establish faster throughput.

## Source metadata parsing

[`civisibility/integrations/manual_api_sourcecache.go`](civisibility/integrations/manual_api_sourcecache.go)
adds `parser.SkipObjectResolution`. The cache reads declarations, source ranges,
function literals and ITR comments; it does not use identifier objects, scopes
or unresolved-identifier lists. `ParseComments` and `AllErrors` remain enabled.

If upstream adds semantic identifier inspection, reassess this flag. Preserve
the source-cache tests for named functions, adjacent literals, missing/invalid
files and ITR comments, and the CI matrix's source/coverage comparisons.

## Testify method names

[`civisibility/integrations/gotesting/testify.go`](civisibility/integrations/gotesting/testify.go)
uses `strings.HasPrefix(name, "Test")` where upstream compiled `^Test` on every
method. Both match the same literal prefix, including a method named `Test`.
The SDK's other method-registration and suite-grouping rules stay in place.
Run Testify parity with method filtering, lifecycle hooks, helpers, retries and
coverage whenever upstream changes this advice.

## Lazy stack classification

[`stacktrace/stacktrace.go`](stacktrace/stacktrace.go) constructs its immutable
prefix tries through independent `sync.OnceValue` initializers. Internal-frame
filtering does not construct the large third-party table. Raw stack capture
constructs neither table; third-party classification builds its trie on first
use, using the same generated library list and `golang.org/` prefix.

No redaction or matching rule changes. This moves table construction from package
startup to the first lookup that needs it. `stacktrace/trie.go` documents the
publication boundary: initialize completely, publish once, then only read.
Update the generated library list with upstream and retain
`TestConcurrentStackClassification` under `-race`, plus error-stack parity.

## Deferred delivery and goleak checkpoints

The POC's `internal/cidelivery` owns the optional
`DD_CIVISIBILITY_DEFERRED_DELIVERY` coordinator. The port integrates with it in
`integrations/civisibility.go`, `civisibility_features.go`, `gotesting/testing.go`,
the coverage/log writers and telemetry startup/ticker. Test activity begins
inside the existing instrumented closure, not an outer wrapper: the SDK uses
that closure's identity to recognize already instrumented tests. The first
registered cleanup runs last, covering user cleanups and parallel descendants.
Preserve this placement when syncing retry or testing lifecycle changes.

In deferred mode, settings and repository upload are synchronous, coverage/log
batches transfer ownership to an idle queue, and telemetry flushes at checkpoints
instead of starting a periodic worker. Coverage acquires its delivery concurrency
permit when the queued work runs, not while a parallel test is buffering it.
Normal mode retains the asynchronous SDK paths. Terminal shutdown force-drains
pending work before writer barriers. Memory may grow across a parallel group;
outgoing payload bounds remain unchanged. See
[delivery checkpoints](../../../docs/delivery.md).

The automatic goleak shim applies in both Mini delivery modes. CI HTTP paths in
`utils/net/http.go` and `telemetry/internal/writer.go` bracket requests with the
send gate. A goleak check waits for active sends, pauses new sends and closes
owned idle CI connections before taking snapshots. Named telemetry, coverage and
log worker functions permit exact filters without ignoring `net/http` or a user
goroutine snapshot. Preserve worker names together with
`internal/instrument/goleak.go` when moving these functions.

Checks: `TestDeferredDeliveryParityMatrix` (17 policy combinations),
`TestDeferredDeliveryTestifyParity` (seven suite combinations),
`TestMiniGoleakIntegration` (normal/deferred, covered library, race, external
helper and real leak controls), coordinator/transport tests and `-race`.
The error-stack comparator maps only the relocated wrapper's known line 844
to the pinned SDK's line 838; application frames and other library lines remain
strict. A source move must update that explicit mapping with proof, not erase
stack locations globally.

## Shared CI string tags

`civisibility/utils/environmentTags.go` adds `GetCITagsSnapshot`, an owned,
read-only snapshot with a revision. Current contents are checked with `maps.Equal`
under the existing mutex. This retains the SDK's sequential direct cached-map
edits as well as `AddCITags`, `AddCITagsMap` and resets. Published snapshots never
change. Do not replace this content check with pointer identity or update-only
invalidation while the original `GetCITags` map remains mutable.

`civisibility/integrations/manual_api_common.go` caches truncated string options
per snapshot revision and Bazel mode. CI/Git/OS/runtime strings and
`_dd.ci.env_vars` bind one Mini `CommonTags` option; other strings keep ordinary
tag options, and numeric CI metrics remain fresh per call. Keep the original
option order: common tags replace earlier options, and event-specific tags and
metrics applied afterward can replace them. Session name stays in its original
envelope entry. The UTF-8 character limit still applies before sharing.

Bazel filtering precedes snapshot binding. CI tags must not reappear through
envelope metadata in payload-file mode. New CI metrics or special tag handling
in upstream require review of this boundary.

Mini's `internal/minitracer/common_tags.go` owns getters and wire projection.
Homogeneous event kinds share strings in metadata; text/numeric overrides or a
mixed snapshot fall back to local strings without mutating sealed maps. Child
spans do not receive CI defaults. Accounting excludes replaced default values
and includes envelope overhead conservatively, even when the wire payload
shrinks. A large masked default must never reject a smaller valid event.

Checks: `TestCITagsSnapshotUpdatesAndRetainsOldValues`,
`TestCommonTagOptionsKeepUpdatesTruncationAndBazelFiltering`, the native shared-tag
wire/concurrency/bounds tests and the SDK/Mini parity matrices. The differential
capture expands only the declared shared CI keys before semantic comparison;
its negative controls retain missing/wrong values, overrides and numeric
collisions. Keep the raw-payload placement assertions too.

## POC-owned code outside this source subset

`internal/minitracer/span.go` encodes the high eight trace-ID bytes directly with
`hex.EncodeToString`. This retains the same 16-character, zero-padded lowercase
`_dd.p.tid` value. It is owned by Mini; propagation and event tests cover it.

The front-end also prunes known dependency queries, decodes package JSON from
stdout, reserves rewritten-source buffer capacity and reuses the validated
Testify AST within a preparation. See the
[performance guide](../../../docs/performance.md) for those POC-owned paths and
their separate build-time measurements.

`internal/minitracer/batch.go` creates a timeout context only on a waiting or
flushing enqueue, retaining the deadline captured at entry. Deferred batches
split by the original intake thresholds and retain only unsent chunks after
failure. `internal/citransport/compression.go` pools `gzip.BestSpeed` writers;
this changes compression ratio, not content or protocol. The source rewriter
omits comment AST construction while preserving original comment bytes.

## Update checklist

1. Produce `scripts/upstream.py patch` against the recorded SDK snapshot and
   compare the affected paths with the new default-branch snapshot.
2. Review changes to tags, metric lifetimes, timestamps, source metadata and
   stack redaction against the invariants above. Preserve improvements explicitly;
   do not overwrite them with an import-path-only copy.
3. Run the focused checks and SDK/Mini differential suite on the same SDK base.
   Run concurrency cases with `-race` and the supported platform workflows.
4. Refresh `TESTS.json` for adapted assertions and `SOURCE.json` using the shared
   [maintenance procedure](../../../docs/maintenance.md). Rerun comparable
   benchmarks if upstream changes a hot path.
