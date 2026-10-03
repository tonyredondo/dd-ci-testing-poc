# dd-ci-testing-poc

A testing-only CI Visibility tool built on native Go build overlays.
The driver uses the Go standard library only. The default `sdk` backend uses the
**unmodified `dd-trace-go` SDK** at the exact revision pinned in
[`internal/version`](internal/version/version.go). The experimental `mini` backend
uses the SDK-derived CI logic and a native event client with a smaller dependency
graph. See [native runtime usage and contracts](docs/mini-runtime.md).

For maintainers, start with the [documentation guide](docs/README.md):
[architecture and diagrams](docs/architecture.md),
[source updates](docs/maintenance.md) and
[performance and profiling](docs/performance.md), and
[CI feature parity, combinations and remaining gaps](docs/ci-parity.md).

```sh
go build -o bin/ddtest ./cmd/ddtest
# Run from a target module which already requires the supported SDK:
/path/to/ddtest test -count=1 -race ./...
```

When `DD_CIVISIBILITY_ENABLED` is absent, the CLI sets it to `parent`. The SDK
activates CI Visibility for each test process and disables it for ordinary child
processes. Explicit values, including `false` and an empty value, are retained by
the CLI; runtime normalization remains the SDK's responsibility.

The tool prepares the SDK's nine `testing` aspects, injects an external test
file importing `dd-trace-go/v2/civisibility`, then calls native `go test` with an
overlay. Original project sources, GOROOT and the SDK are not modified on
disk. Temporary sources are removed after Go finishes. Go owns compilation and
cache invalidation.
The exact SDK ownership marker and linkname ABI are retained, including process
retry control and abnormal finalization.

This is an experimental POC, not a replacement for supported Orchestrion releases.
It supports package-mode tests with the pinned SDK, build tags, test selection,
count/shuffle, JSON, benchmarks, race and coverage. It forwards native flags and
preserves the user's result-cache choice; use `-count=1` for fresh CI events.
Existing overlays are merged. Missing or ambiguous hooks and conflicting `-toolexec`
configuration fail before compilation. Explicit `.go` file mode, `-C`, SDK
replacements and standard-library test targets are outside this POC.
[Testify suite support](docs/testify.md) covers v1.11.1 and newer v1 releases,
including callers in external dependencies. A selective `-toolexec` hook is
activated only for reachable Testify suites or covered rewritten `testing`
sources. Other builds use the overlay directly. Preparation validates the selected
Testify version and API even when Go can reuse a cached archive. Version fixtures
cover v1.11.1 and v1.12.1. Run from the desired module directory. Runtime
configuration and retry/skip/quarantine behavior remain in the selected runtime.

## Reproduce verification

```sh
go install github.com/DataDog/orchestrion@v1.13.2-0.20260917114356-5c24783fcd76
ORCHESTRION_BIN="$(go env GOPATH)/bin/orchestrion" go test -v ./...
```

With `ORCHESTRION_BIN`, the suite compares **both instruments using the same
temporary module graph, SDK version, fixture sources and binary basename**.
Orchestrion loads the SDK's actual `gotesting/orchestrion.yml`; other APM
integrations are outside the comparison. Its required tool dependency is added
only to the temporary comparison fixture, never the driver module or SDK.
Without that variable, tests still verify the overlay against native Go and the
real SDK, but the Orchestrion differential check has not run.

Tests send actual SDK MessagePack payloads to a loopback HTTP server with a
synthetic API key. They compare test/session/module/suite events, statuses,
skip reasons, error messages/stacks and source locations. Generated IDs,
timestamps/durations and invocation names are excluded from the semantic
comparison. Native output checks retain all lines and their multiplicity while
allowing native parallel completion order and elapsed times to differ.

GitHub Actions is configured to run the differential suite on Go 1.26/1.27 Linux and Go 1.27
macOS/Windows. A green Go version is compatibility evidence for that tested
version; it does not imply support for every future toolchain or SDK.

The [results index](docs/results.md) links each experiment with its source inputs.
The [full compile matrix](docs/results/compile-matrix-20261003-linux-go1.27/README.md)
includes all four variants at 4/32 CPUs, including Testify, race and coverage. The
[Testify strategy experiment](docs/results/tool-strategies-20261003-linux-go1.27/README.md)
retains all 660 compile-only observations, cache checks and the selected `-find`
optimization. Those small Mini fixtures are separate from the Gin/Chi comparison.
The latest summaries below are followed by the earlier SDK-backend measurement.

<!-- build-benchmark-summary:start -->
## Latest four-variant compilation comparison

The [complete matrix](docs/results/compile-matrix-20261003-linux-go1.27/README.md) retains 7,113 completed
command timings across the selected CPU configurations and five build scenarios.
Measured POC: `95f8f32b7e8898d96cd72f7f4c89f6905031cd9e`; `go version go1.27.1 linux/amd64`.
SDK: `v2.12.0-dev.3.0.20261002145613-96aedb31048c`. Test binaries were compiled with `-ldflags=-w`
and never executed. These excerpts show the cases without extra flags.

Values are medians in seconds. Each POC cell lists its signed change against total
Orchestrion wall time first, then against Native. For example, `(-50%; +20%)` means
half Orchestrion time and 20% more than Native. Every sample, including slow runs,
remains included; small differences need the ranges and uncertainty in the report.

### Cold compilation

| Project | Flags | CPUs | Native | Orchestrion | POC SDK | POC Mini |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| Gin | `none` | 4 | 17.984 s | 46.941 s | 27.791 s (-40.8%; +54.5%) | 19.314 s (-58.9%; +7.4%) |
| Gin | `none` | 32 | 11.155 s | 23.795 s | 14.796 s (-37.8%; +32.6%) | 11.701 s (-50.8%; +4.9%) |
| Chi | `none` | 4 | 7.758 s | 28.672 s | 18.189 s (-36.6%; +134.5%) | 9.539 s (-66.7%; +23.0%) |
| Chi | `none` | 32 | 4.615 s | 16.810 s | 9.348 s (-44.4%; +102.5%) | 5.901 s (-64.9%; +27.9%) |
| Testify Direct | `none` | 4 | 8.068 s | 28.251 s | 17.765 s (-37.1%; +120.2%) | 9.942 s (-64.8%; +23.2%) |
| Testify Direct | `none` | 32 | 4.706 s | 17.741 s | 9.400 s (-47.0%; +99.7%) | 6.861 s (-61.3%; +45.8%) |
| Testify External | `none` | 4 | 8.072 s | 28.404 s | 17.837 s (-37.2%; +121.0%) | 9.883 s (-65.2%; +22.4%) |
| Testify External | `none` | 32 | 4.673 s | 17.106 s | 9.504 s (-44.4%; +103.4%) | 6.053 s (-64.6%; +29.5%) |

### Cached compilation — unchanged output reused

| Project | Flags | CPUs | Native | Orchestrion | POC SDK | POC Mini |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| Gin | `none` | 4 | 0.096 s | 0.369 s | 0.240 s (-35.0%; +149.5%) | 0.179 s (-51.6%; +85.7%) |
| Gin | `none` | 32 | 0.098 s | 0.411 s | 0.253 s (-38.5%; +158.6%) | 0.199 s (-51.7%; +103.2%) |
| Chi | `none` | 4 | 0.063 s | 0.327 s | 0.189 s (-42.3%; +200.9%) | 0.123 s (-62.4%; +96.3%) |
| Chi | `none` | 32 | 0.067 s | 0.382 s | 0.199 s (-47.9%; +195.0%) | 0.141 s (-63.2%; +108.8%) |
| Testify Direct | `none` | 4 | 0.058 s | 0.314 s | 0.172 s (-45.3%; +195.8%) | 0.110 s (-65.0%; +89.4%) |
| Testify Direct | `none` | 32 | 0.059 s | 0.358 s | 0.177 s (-50.6%; +199.5%) | 0.121 s (-66.3%; +104.4%) |
| Testify External | `none` | 4 | 0.059 s | 0.315 s | 0.184 s (-41.6%; +212.0%) | 0.121 s (-61.6%; +105.2%) |
| Testify External | `none` | 32 | 0.060 s | 0.361 s | 0.191 s (-47.2%; +215.9%) | 0.133 s (-63.1%; +121.0%) |

[Run the matrix or regenerate its tables](docs/build-benchmarks.md) with
`scripts/build_benchmark.py`. The older measurements below preserve their original
inputs and are separate experiments.
<!-- build-benchmark-summary:end -->

## Compilation performance

The following compile-only measurements use **`go test -c -o <directory>/ -ldflags=-w ./...`**. No test binaries are executed, so SDK runtime and network flushing are excluded. Times are wall-clock medians in seconds. The percentage in the POC column is the reduction relative to Orchestrion, calculated from unrounded medians as `100 × (1 − POC / Orchestrion)`.

Measured on an AMD Ryzen 9 5950X running Arch Linux amd64 and the development toolchain `go1.27.0-X:nodwarf5`. Each configuration uses the first N physical cores (no SMT), `GOMAXPROCS=N`, and `-p=N`. Gin v1.12.0 produces six test binaries; Chi v5.3.2 produces two. All variants use the same project sources and module resolution graph. Both instrumentators use the unmodified `dd-trace-go/v2@v2.11.0-rc.1` SDK; Orchestrion `v1.13.2-0.20260917114356-5c24783fcd76` loads only its `gotesting` aspects. The measured POC source is commit `4e56e8657949aaa1209afbaa19ef9767bbf6da90`.

### Cold build

Two runs per variant with independent, empty Go build caches and no existing output binaries. Downloaded modules and the OS page cache remain warm.

| Project | Cores | Native | Orchestrion | POC |
| --- | ---: | ---: | ---: | ---: |
| Gin | 1 | 60.94 s | 141.76 s | 95.63 s (**−32.5%**) |
| Gin | 4 | 17.74 s | 45.96 s | 27.72 s (**−39.7%**) |
| Gin | 8 | 12.92 s | 28.32 s | 18.25 s (**−35.6%**) |
| Gin | 16 | 11.34 s | 26.82 s | 17.33 s (**−35.4%**) |
| Chi | 1 | 26.61 s | 94.27 s | 65.38 s (**−30.6%**) |
| Chi | 4 | 7.81 s | 29.41 s | 18.10 s (**−38.5%**) |
| Chi | 8 | 5.49 s | 19.61 s | 11.72 s (**−40.2%**) |
| Chi | 16 | 4.64 s | 17.11 s | 10.76 s (**−37.1%**) |

### Cached dependencies with forced linking

Five runs per variant with compiled dependencies in cache. Every run adds a unique `-buildid` alongside `-w` to force fresh links. These timings include CLI preparation, Go build planning, and any required generated-main compilation; they do not isolate the linker or measure unchanged-binary reuse.

| Project | Cores | Native | Orchestrion | POC |
| --- | ---: | ---: | ---: | ---: |
| Gin | 1 | 2.49 s | 10.96 s | 6.00 s (**−45.2%**) |
| Gin | 4 | 1.08 s | 4.98 s | 2.96 s (**−40.7%**) |
| Gin | 8 | 1.06 s | 5.43 s | 4.23 s (**−22.2%**) |
| Gin | 16 | 0.99 s | 4.97 s | 4.12 s (**−17.2%**) |
| Chi | 1 | 0.56 s | 3.87 s | 1.68 s (**−56.7%**) |
| Chi | 4 | 0.26 s | 1.80 s | 1.21 s (**−32.7%**) |
| Chi | 8 | 0.25 s | 1.61 s | 1.03 s (**−36.3%**) |
| Chi | 16 | 0.27 s | 1.73 s | 0.97 s (**−44.2%**) |

[Complete measurements](docs/benchmark-compile-no-dwarf.json) retain all 400 commands: 360 performance samples, 16 A/A controls, and 24 trace validations, with ranges and paired comparisons. All commands succeeded; all 96 output binaries were checked without DWARF; the SDK instrumentation hook was present only in the POC and Orchestrion outputs. CPU and peak memory accounting include the Orchestrion daemon and nested builds.

These are local, exploratory results. Two cold runs do not establish stability, and the physical cores were not reserved from other workloads. Results should not be extrapolated to other applications, platforms, or toolchains. The earlier series that retained DWARF was measured at a different time; its noisy Gin eight-core cold results do not isolate the effect of `-w`. The JSON also preserves unused-constant edit measurements as diagnostics, not evidence for real test-body edit performance.

The commands below illustrate the eight-core configuration from each prepared target module. Use separate Go build caches and output directories for each variant; for the cached-dependency scenario, append a different `-buildid` value to `-ldflags` on each run. CPU affinity and cache cleanup were managed outside the timed commands.

```sh
GOMAXPROCS=8 go test -p=8 -c -o ./out/native/ -ldflags=-w ./...
GOMAXPROCS=8 /path/to/ddtest test -p=8 -c -o ./out/poc/ -ldflags=-w ./...
GOMAXPROCS=8 go test -p=8 -c -o ./out/orchestrion/ \
  -toolexec="/path/to/orchestrion toolexec" -ldflags=-w ./...
```

## Compare compilation

```sh
python3 scripts/benchmark.py --ddtest /path/to/ddtest \
  --orchestrion /path/to/orchestrion --output /var/tmp/dd-ci-benchmark
```

The bounded script keeps every timing, alternates variant order, isolates cold
Go build caches, and checks warm compilation and identical package edits. It
uses `go test -c`, so runtime/network flushing is excluded. It reports wall time,
not incomplete CPU accounting. Two cold rounds and five warm/edit rounds are
exploratory data; this small fixture does not establish savings for your application.

See [validation scope](docs/validation.md) and [results](docs/results.md).

## Source maintenance

Incorporated sources live in [`internal/thirdparty`](internal/thirdparty/README.md).
Every origin records its repository, exact upstream SHA, licenses and file hashes.
The current SDK extraction base is `dd-trace-go/main` at
`96aedb31048c07e29e7a20a4333dc3b8d289c52d`; differential fixtures use the same
version. The native client/transport remain separate from the upstream subsets.
Run `python3 scripts/upstream.py verify` to audit the source record offline.
The [maintenance guide](docs/maintenance.md) covers three-way SDK updates, codec
regeneration, platform changes and the required checks. The benchmark tables
above describe their explicitly recorded historical revisions.
