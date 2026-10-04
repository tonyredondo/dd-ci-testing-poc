# Runtime delivery follow-up

Baseline: `580db827b34c441c55090549df70e1767f4652ae`. After: local changes on
`feat/ci-minitracer`, identified by the measured source hashes in `inputs.json`. A final goleak options
copy refinement is also recorded there; it does not execute in these benchmark
cases. The SDK
base remains `96aedb31048c07e29e7a20a4333dc3b8d289c52d`.

Linux amd64, Ryzen 9 5950X, `go1.27.0-X:nodwarf5`. Five alternating before/after
pairs retain every observation. Event lifecycle uses an in-memory HTTP transport;
it measures serial creation, Finish, batching and compression, not real network
latency. Both GOMAXPROCS settings still execute one serial benchmark loop.
The first samples overlapped platform cross-compilation; the host was not reserved.
Times are observations, not a predicted saving for an application or build.

## Event lifecycle

| Gzip | GOMAXPROCS | Before | After | Reduction | Allocations | Allocated bytes |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Off | 1 | 3.095 µs | 2.558 µs | 17.4% | 16 → 11 | 2313 → 1928 B |
| Off | 8 | 2.674 µs | 2.229 µs | 16.6% | 16 → 11 | 2322 → 1939 B |
| On | 1 | 5.579 µs | 3.767 µs | 32.5% | 16 → 11 | 2330 → 1938 B |
| On | 8 | 5.425 µs | 3.513 µs | 35.2% | 16 → 11 | 3124 → 2579 B |

The unused enqueue timeout no longer allocates a context/timer. The original
absolute deadline is reused if the operation later waits or flushes. Wire byte
limits, rejection semantics and failure retention remain covered by tests.

## Preparation

| Case | Before | After | Allocated bytes before | Allocated bytes after |
| --- | ---: | ---: | ---: | ---: |
| TestingTransform | 2.208 ms | 2.195 ms | 1,513,084 | 1,421,315 |
| Prepare/gin | 63.150 ms | 63.330 ms | 2,343,967 | 2,253,777 |
| Prepare/chi | 48.759 ms | 48.382 ms | 2,195,756 | 2,104,183 |
| Prepare/testify-direct | 36.375 ms | 46.365 ms | 2,009,399 | 2,187,848 |
| Prepare/testify-external | 48.000 ms | 46.990 ms | 2,278,148 | 2,188,932 |

Removing comment AST construction saves about 90 KiB for Gin and Chi;
it produces no material total Prepare wall-time improvement in these samples.
Direct Testify preparation now costs about 10 ms more. Shared library discovery
walks test-only imports to detect goleak instead of always taking the old
suite-only `-find` shortcut. External helpers remain included. No concurrent
preparation pipeline or separate persistent cache was added.

## Gzip level

Five repeated observations compare pooled default compression with BestSpeed
on each identical input. CI400 is a native MessagePack payload with 400 synthetic
CI-shaped events. The probe also verifies exact decompression for each shape.

| Shape | Default | BestSpeed | Time reduction | Default wire | BestSpeed wire |
| --- | ---: | ---: | ---: | ---: | ---: |
| CI400 | 499.562 µs | 239.226 µs | 52.1% | 16,312 B | 17,984 B |
| Small | 0.110 µs | 0.103 µs | 6.5% | 38 B | 38 B |
| Repeated64KiB | 77.667 µs | 13.215 µs | 83.0% | 106 B | 107 B |
| Random64KiB | 9.988 µs | 6.859 µs | 31.3% | 65,566 B | 65,566 B |

CI400 wire size grows 10.3%. This is standard gzip; uncompressed payload limits
do not change. The level applies to Mini test-cycle delivery, not SDK-derived
coverage/log compression. These synthetic shapes do not establish a production
compression ratio.

## Profile and validation

The final uncompressed lifecycle allocation profile assigns 80.6% of allocated
bytes directly to `newSpan`; event acceptance is now about 1.2% including callees.
This is cumulative allocation, not resident memory. The serial CPU profile
shows map iteration, span creation and encoding as remaining work. Additional
payload-level common metadata lifting is investigated but not implemented.

Local Linux checks passed the complete Go 1.26.8/1.27 suites with the pinned
Orchestrion reference, the Go 1.27 integration suite with `-race`, and focused
race checks after the connection-ownership refinement. Go 1.26 repeated the
final goleak/cache cases with `-race`. Deferred parity covers 17 native testing
and seven Testify combinations. Real test/HTTP leaks remain failures; unchanged
goleak archives reuse cache; changing prepared inputs rebuilds goleak while
standard controls remain cached. An unsupported warm vendored version is rejected.

The runtime graph contains 258 packages including the standard library and
zero external runtime module packages. Windows amd64 and Darwin arm64 builds
passed; their native execution and deployed Agent/intake acceptance are unverified.
All 13 maintainer diagrams rendered without clipped labels. CI workflows already
run these test packages, but this local change has not been published.

Raw logs, commands, summaries and probe source are retained here. Full test
logs and profiles remain in the task artifact directory identified in `inputs.json`.
The [delivery guide](../../delivery.md) documents deferred queue memory and
connection ownership; [SDK adaptations](../../../internal/thirdparty/dd-trace-go/ADAPTATIONS.md)
record the port changes that must survive upstream synchronization.
