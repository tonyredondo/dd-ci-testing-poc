# Shared CI metadata measurements

This compares the local runtime/deferred/goleak implementation with that same
implementation plus common CI tag sharing. Both start from branch
`feat/ci-minitracer`, HEAD `580db827b34c441c55090549df70e1767f4652ae`; the baseline
already includes the changes described in the
[delivery follow-up](../runtime-20261004-linux-go1.27/README.md).
This is an isolated follow-up comparison, not a comparison against that commit's
clean tree. The SDK base remains `96aedb31048c07e29e7a20a4333dc3b8d289c52d`.

Linux amd64, Ryzen 9 5950X, `go1.27.0-X:nodwarf5`. Five alternating pairs use the
same benchmark body and 500 ms calibration. The fixture adds 24 synthetic static
CI fields to the usual SDK-derived Git/OS/runtime tags. Its in-memory HTTP
transport consumes complete request bodies; there is no real network latency.
Both GOMAXPROCS settings use one serial event loop. No other builds or suites
from this task ran during these pairs; the host was not reserved.

The measured path includes `fillCommonTags`, span creation, `Finish`, batching,
MessagePack encoding and delivery, with gzip off/on. Results are medians; the raw
logs and JSON retain every sample and the summary retains timing ranges.

| Gzip | GOMAXPROCS | Before | After | Time reduction | Allocations/event | Allocated bytes/event | Wire bytes/event |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Off | 1 | 8.746 µs | 3.924 µs | 55.1% | 151 → 25 | 7,582 → 2,155 | 2265 → 481.1 |
| Off | 8 | 6.829 µs | 3.247 µs | 52.5% | 151 → 25 | 7,588 → 2,160 | 2265 → 481.1 |
| On | 1 | 11.786 µs | 4.608 µs | 60.9% | 151 → 25 | 7,804 → 2,165 | 201.4 → 48.42 |
| On | 8 | 9.814 µs | 3.945 µs | 59.8% | 151 → 25 | 8,909 → 2,650 | 201.5 → 48.36 |

Uncompressed bytes fall 78.8%; gzip wire bytes fall about 76%. Tag sharing avoids
rebuilding most common tag options and inserting their strings into every span
map. A homogeneous event kind writes those values once in its metadata entry.
Mixed snapshots and text/numeric overrides use the documented fallback instead of
forcing values into defaults. Snapshot content comparison remains on the start
path to preserve the SDK's mutable cached-map API.

These numbers measure a synthetic CI-tag workload. Fewer tags, smaller batches,
frequent tag changes or many numeric overrides will change the gain. They do
not quantify build-time savings or a complete application's wall time.

## Reproduce and inspect

The benchmark is part of the repository:

```sh
go test ./internal/thirdparty/dd-trace-go/civisibility/integrations \
  -run='^$' -bench=BenchmarkCommonMetadataLifecycle \
  -benchmem -benchtime=500ms -count=5 -cpu=1,8
```

The alternating run used separately compiled before/after test binaries with
the same flags and basename, so the generated `test.command` tag also matches.
`inputs.json` records their hashes, the measured source hashes
and the baseline diff manifest in the task artifact directory. The raw
`alternating-common.txt`, `observations.json` and `summary.json` are retained here.
Per-span identifiers remain random in both runs, as in the real runtime.

The [delivery contract](../../delivery.md#payload-level-common-metadata) covers
getters, precedence, byte accounting and Bazel. The
[SDK adaptation record](../../../internal/thirdparty/dd-trace-go/ADAPTATIONS.md)
identifies the changed upstream paths and required checks for a future update.
The SDK/Mini comparator resolves declared shared CI keys before comparing their
values, counts and hierarchy, while raw decoded requests still verify placement.

## Validation of the final implementation

On Linux amd64, the complete Go 1.26.8 suite and complete Go 1.27 suite with
`-race` passed against the frozen Orchestrion/SDK reference. Earlier ordinary
Go 1.27 and integration-only race runs also passed; the final runs supersede
their byte-accounting inputs. No production source changed after the final
benchmark or those final suites.

Focused checks include before/after-Finish getters, earlier and later options,
text/metric transitions, late snapshot changes, retries after a failed batch,
Unicode truncation, generic child spans and Bazel filtering. Actual Agent and
agentless requests verify session metadata placement. A large overridden
default reproduces a false rejection in the initial implementation; the final
test delivers both text and metric overrides without consuming that unused
string's byte budget.

The four source manifests verify at their unchanged upstream revisions; all
21 Python maintenance contracts pass. Mini still imports zero external runtime
packages. All 14 current diagrams render without clipped labels. The existing
platform workflow includes these test packages, but these local changes have
not been published. Native macOS/Windows execution and deployed Agent/intake
acceptance are not established by these Linux loopback checks.

`validation.json` records commands, log hashes and the artifact directory. This
comparison has no commit or push of its own.
