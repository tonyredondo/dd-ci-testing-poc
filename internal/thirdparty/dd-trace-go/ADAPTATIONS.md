# Performance adaptations to the SDK port

Base: [`96aedb31048c07e29e7a20a4333dc3b8d289c52d`](https://github.com/DataDog/dd-trace-go/commit/96aedb31048c07e29e7a20a4333dc3b8d289c52d).
`SOURCE.json` retains each original path/hash alongside the local hash. This
record explains changes that an upstream synchronization must review manually.
The base revision and the supported CI behavior remain unchanged.

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
Timestamps are captured before competing for the lock, matching the previous
submission path. They need not be monotonic between concurrent callers.

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

## POC-owned code outside this source subset

`internal/minitracer/span.go` encodes the high eight trace-ID bytes directly with
`hex.EncodeToString`. This retains the same 16-character, zero-padded lowercase
`_dd.p.tid` value as the prior big-endian integer formatter. It is a Mini change,
not an upstream source-file adaptation; propagation/event tests cover it.

The front-end also prunes known dependency queries, decodes package JSON from
stdout, reserves rewritten-source buffer capacity and reuses the validated
Testify AST within a preparation. See the
[performance guide](../../../docs/performance.md) for those POC-owned paths and
their separate build-time measurements.

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

Queue timeouts, common-span metadata storage, compression level and retry-code
refactoring are outside this change. They need their own behavior and delivery
checks before adoption.
