# Preparation and runtime optimizations, 2026-10-03

The baseline is POC `95f8f32b7e8898d96cd72f7f4c89f6905031cd9e`. The candidate
contains the local hot-path changes documented in the [performance guide](../../performance.md)
and the [SDK adaptation record](../../../internal/thirdparty/dd-trace-go/ADAPTATIONS.md).
The SDK base stays at `96aedb31048c07e29e7a20a4333dc3b8d289c52d`.

These are targeted benchmarks, separate from the frozen Native/Orchestrion/SDK/Mini
compile matrix. They establish costs for these operations; runtime savings do not
reduce `go test -c` wall time.

## Preparation

Values are medians of three alternating before/after rounds, with ten timed
preparations per reported sample and `-p=4`. Each plan was removed outside the
timed section. Package/module caches were warm. Parent benchmark concurrency was
four; CPU affinity was not constrained. Go subprocesses inherited the same host
environment in both variants.

| Fixture | Before | After | Change | Before B/op | After B/op |
| --- | ---: | ---: | ---: | ---: | ---: |
| Gin | 85.93 ms | 76.61 ms | -10.8% | 3,177,401 | 2,350,011 |
| Chi | 60.89 ms | 59.11 ms | -2.9% | 2,505,165 | 2,202,062 |
| Testify Direct | 44.95 ms | 44.40 ms | -1.2% | 2,414,653 | 2,005,174 |
| Testify External | 57.94 ms | 56.89 ms | -1.8% | 2,689,637 | 2,285,823 |

Gin saves about 9 ms per preparation in this control. Smaller time differences
need the retained ranges: three rounds do not establish a stable improvement for
every fixture. All four reduce parent-process allocated bytes; those bytes exclude
Go subprocess allocations and are not resident-memory peaks.

The actual Go 1.27 testing transform, measured separately, changes as follows:

| Transform | Before | After | Change | Before B/op | After B/op |
| --- | ---: | ---: | ---: | ---: | ---: |
| `testing` source rewrite | 2.319 ms | 2.185 ms | -5.7% | 1,743,464 | 1,513,584 |

The prepared testing sources, suite entry, hooks and Testify fingerprint match
the baseline byte for byte in Gin, Chi, direct Testify and external-helper Testify.
The manifest records hashes of the canonical generated-content maps.

## Counter updates

Each sample ran for 500 ms. Values are medians of the same three alternating
rounds. A parallel ns/op is elapsed time divided by aggregate operations across
all workers; it is not the latency of an individual worker or CPU time.

The CI operation calls `EventCreated`, `EventFinished` and the enqueue counter,
flushing the real telemetry client every 1,024 operations through an in-memory
HTTP 200 transport. The benchmark argument `testing` maps to the SDK unknown
framework tag. Functional checks cover both that mapping and
`golang.org/pkg/testing`, plus feature tags and disabled telemetry.

| Operation | Workers | Before | After | Change | Before → after B/op | Before → after allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Single counter | 1 | 56.18 ns/op | 41.45 ns/op | -26.2% | 32 → 0 | 1 → 0 |
| Single counter | 8 | 142.60 ns/op | 47.44 ns/op | -66.7% | 32 → 0 | 1 → 0 |
| Three CI updates + amortized flush | 1 | 477.30 ns/op | 157.40 ns/op | -67.0% | 259 → 3 | 7 → 0 |
| Three CI updates + amortized flush | 8 | 132.00 ns/op | 76.22 ns/op | -42.3% | 266 → 3 | 7 → 0 |

Direct count submission now allocates no point. The CI operation retains about
3 B/op of amortized collection/delivery allocations; Go rounds its allocation
count to zero per operation. That does not mean flushing allocates nothing.
Additional retry/EFD/quarantine tags keep the general lookup path.

## Evidence and reproduction

[`manifest.json`](manifest.json) retains both binaries' hashes, source hashes and
generated-content equality hashes. [`benchmark-statistics.json`](benchmark-statistics.json)
retains sample counts, medians and ranges. [`alternating-runs.json`](alternating-runs.json)
lists every measured command; the matching `.log` files keep every observation.

[`harness_test.go.txt`](harness_test.go.txt) is the measured harness. To repeat it,
create a task-local module named
`github.com/tonyredondo/dd-ci-testing-poc/diagnostic` with a `replace` to the desired
POC worktree, save the harness as `bench_test.go`, and update its four fixture
paths to modules with the same pinned inputs. Build one test executable per
revision before timing. Its selected fixtures must already require the POC
module. Keep the toolchain, module graph, flags and cache policy identical.

Run the executables in alternating order with instrumentation telemetry enabled:

```sh
DD_INSTRUMENTATION_TELEMETRY_ENABLED=true ./before.test -test.run='^$' \
  -test.bench='Benchmark(Count|CIMetrics)$' -test.benchtime=500ms -test.cpu=1,8
DD_INSTRUMENTATION_TELEMETRY_ENABLED=true ./after.test -test.run='^$' \
  -test.bench='Benchmark(Count|CIMetrics)$' -test.benchtime=500ms -test.cpu=1,8
./before.test -test.run='^$' -test.bench='BenchmarkPrepare$' -test.benchtime=10x -test.cpu=4
./after.test -test.run='^$' -test.bench='BenchmarkPrepare$' -test.benchtime=10x -test.cpu=4
```

Use a dedicated `TMPDIR` on disk. `TestPreparedOutput` requires `OUTPUT_DIR` and
writes canonical content maps for equality checks; it is not part of timed runs.
Do not compare these warm preparation figures to a cold whole-build table.

## Compatibility

The full `go test ./...` suites passed on Linux with Go 1.26.8 and Go 1.27.1,
with the frozen Orchestrion reference enabled. Each differential report retains
65 matrix scenarios plus supplemental feature evidence. Go 1.27 internal and
integration suites also passed with `-race`; `go vet ./...` passed. The explicit
disabled-counter subprocess passed under `-race`.

[`validation.json`](validation.json) links the retained logs. The three
`parity-*.json` reports retain feature/event counts and SDK/Mini comparisons.
The CLI and Mini runtime cross-compiled for Windows amd64 and macOS arm64.
Runtime parity on those hosts still requires their native workflows.
No compression-level, common-metadata, timeout or retry-code refactoring experiment
was integrated.
