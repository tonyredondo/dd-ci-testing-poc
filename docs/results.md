# Results index

## Later experiments

Each report names the source inputs it measured. Results from different dates,
projects or flag sets cannot isolate the effect of one optimization.

| Report | What it measures | Evidence retained in the repository |
| --- | --- | --- |
| [Shared CI metadata, 2026-10-04](results/common-metadata-20261004-linux-go1.27/README.md) | Common tag options, span creation, MessagePack batching and gzip delivery | Five alternating before/after pairs, every observation, source/binary hashes, allocation and wire-byte measurements |
| [Delivery follow-up, 2026-10-04](results/runtime-20261004-linux-go1.27/README.md) | Lazy enqueue deadlines, gzip level and preparation with automatic goleak detection | Five alternating pairs, source hashes, raw logs and Go 1.26/1.27 compatibility proof; predates shared CI metadata |
| [Preparation and runtime hot paths, 2026-10-03](results/hotpaths-20261003-linux-go1.27/README.md) | Local query/JSON/AST preparation changes and ordinary CI counter updates | Three alternating before/after rounds, allocation counts, ranges, source/binary hashes, generated-source equality and Go 1.26/1.27 parity/race evidence |
| [Full compile matrix, 2026-10-03](results/compile-matrix-20261003-linux-go1.27/README.md) | Native, Orchestrion, POC SDK and Mini; Gin/Chi and direct/external Testify, race/coverage; 4/32 CPUs, five scenarios | 7,113 completed command timings, five 48-row tables, ranges, uncertainty, input/binary hashes, traces and one interrupted attempt; [reusable runner](build-benchmarks.md) |
| [Tool strategies, 2026-10-03](results/tool-strategies-20261003-linux-go1.27/README.md) | Current, selected `-find` and one-query Mini preparation; six fixtures, 4/32 CPUs, cold/cache/edit scenarios | 660 raw observations, commands, CPU/wall times, cache counters, hashes and compatibility reports |
| [Selective tools, 2026-10-03](results/selective-tools-20261003-linux-go1.27/README.md) | Caller overlay versus original suite-entry instrumentation; wrapper startup and allocation-free dispatch | Every observation, reproduction script and 26-case SDK comparisons |
| [Coverage adapter, 2026-10-03](results/testify-coverage-adapter-20261003-linux-go1.27/README.md) | Historical coverage wrapper process cost | Balanced observations, exploratory controls and input manifest; architecture superseded |
| [Four-variant compile, 2026-10-02](results/compile-20261002-linux-go1.27/README.md) | Native, Orchestrion, POC SDK and Mini on Gin/Chi; 4/32 CPUs, five build scenarios | Published summary tables and conditions; per-command dataset is not part of this snapshot |
| [Repeated CI matrix, 2026-10-02](results/ci-parity-repeated-20261002-linux-go1.27/README.md) | Continuous execution time of the complete 65-case SDK/Mini block, six rounds | All reports, counts and raw block clocks; predates Testify support |

Build measurements use `go test -c` and never execute the test binary. CI
compatibility clocks measure real runtime initialization, test policies and
delivery. Keep those two kinds of observations separate. The current Testify
contract and remaining parity boundaries live in [Testify support](testify.md)
and [CI feature parity](ci-parity.md).

## Initial local results

Local checks ran on Arch Linux/amd64 with `go1.27.0-X:nodwarf5`, the unchanged
SDK `v2.11.0-rc.1`, and frozen Orchestrion commit `5c24783fcd76`. The differential
suite passed, including actual SDK policy/retry behavior, race and coverage builds,
result caching, and user overlay composition. The workflow separately tests official
Go releases on Linux, macOS and Windows; consult its run for current CI status.

## Compilation measurements

Every timing is retained in [benchmark-local.json](benchmark-local.json).
These are wall-clock times for `go test -c` on the small fixture, with
`GOMAXPROCS=8`, identical dependency graphs and alternating variant order.
Runtime and SDK network flushing are excluded. Each cold variant has its own
fresh Go build cache. Warm variants reuse their corresponding cache.

| Scenario | Native median [range], seconds | POC median [range], seconds | Orchestrion median [range], seconds | Runs per variant |
| --- | --- | --- | --- | --- |
| Cold compile | 17.746 [10.432–25.059] | 13.368 [10.635–16.100] | 25.853 [21.418–30.288] | 2 |
| Unchanged warm compile | 0.089 [0.086–3.302] | 1.748 [1.629–1.948] | 2.367 [0.347–3.791] | 5 |
| Comment-only edit | 1.262 [0.091–1.276] | 1.681 [1.614–1.779] | 2.333 [0.542–2.494] | 5 |

The cold POC observations are lower than Orchestrion's two observations. The
native and warm results vary substantially; these measurements do not establish
a stable percentage improvement, nor a production-application performance claim.
The initial warm comparison also shared its output file between variants.
Overwriting another variant binary forced additional links; the corrected
incremental run below is the relevant warm measurement. These original data
remain published rather than being discarded.

The initial edit benchmark changed a comment. Those rows are explicitly retained
as `comment-only`; they do not measure a developer's function-body edit. The current
script uses body edits for future runs. No full-process CPU or aggregate memory
measurement was collected, and binary size is not semantic-equivalence proof.

The initial instrumented fixture binaries were approximately 26.87 MB with both
tools, versus 18.31 MB native. Runtime dependency size remains because both tools
link the same SDK. Compilation work avoided by the specialized front-end does not
remove that runtime dependency graph.

## Corrected incremental comparison

[benchmark-incremental.json](benchmark-incremental.json) retains all 30 measurements
from a follow-up run. Each variant has a separate output directory and the same
binary basename. The run reuses its corresponding previous Go cache, updates all
variants to the same fixture sources, warms them once, then alternates order for
five unchanged runs and five identical function-body edits. SDK and Orchestrion
versions and GOMAXPROCS remain unchanged. There are no additional cold measurements.

| Scenario | Native median [range], seconds | POC median [range], seconds | Orchestrion median [range], seconds |
| --- | --- | --- | --- |
| Unchanged warm compile | 0.086 [0.085–0.089] | 0.123 [0.122–0.127] | 0.341 [0.339–0.342] |
| Function-body edit | 1.288 [1.260–1.562] | 1.774 [1.640–1.787] | 2.613 [2.490–2.684] |

In this fixture the POC adds approximately 37 ms over native unchanged compilation,
compared with 255 ms for Orchestrion. Editing and recompiling has lower observed
POC wall time than Orchestrion in all five pairs. These are promising local
observations, with a small sample and no production-application or cross-machine
performance claim. Runtime event compatibility is established separately by the
SDK differential tests, not by these timings. The driver still queries packages
and validates native testing sources each invocation; that front-end cost remains.

## Decision supported by this POC

The SDK's existing testing hooks can be driven by a small standard-library-only
overlay tool without changing the SDK. The differential tests provide bounded
compatibility evidence for the exercised scenarios. The next performance decision
needs repeated measurements on a representative application with stable controls,
including unchanged runs and real edits. Broad instrumentation and future Go/SDK
versions are outside the present evidence.
