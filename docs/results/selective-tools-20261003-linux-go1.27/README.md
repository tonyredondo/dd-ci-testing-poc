# Selective tool measurements

This is the entry-instrumentation baseline before the known-suite `-find`
optimization. [The later strategy experiment](../tool-strategies-20261003-linux-go1.27/README.md)
compares that branch with the baseline; these measurements remain unchanged.

Go 1.27.1 on Linux/amd64, AMD Ryzen 9 5950X, `GOMAXPROCS=4`. Each command compiles a test binary with `go test -c -o`; no tests or SDK runtime code execute.

Before is the previously validated caller-overlay prototype. After intercepts the original Testify entry and bypasses unrelated tools. Both use Mini on the same fixture and incorporated SDK base. The before prototype does not cover external callers; that additional support is part of the after variant.

Each variant has a stable output filename. Orders alternate after warmup. The series keeps nine observations per variant without changes, seven after edits, and three with `-a`. Edit constants are unique across series, and the harness rejects any edit that does not actually compile. `-a` rebuilds the graph with downloaded modules and warm filesystem caches; it is not a fresh empty-cache benchmark.

## Wall time

| Scenario | State | Before (ms) | Selective (ms) | Difference (ms) |
| --- | --- | ---: | ---: | ---: |
| No Testify | No changes | 86.08 | 90.49 | +4.41 |
| No Testify | Source edit | 388.46 | 401.04 | +12.58 |
| No Testify | Forced rebuild (-a) | 6780.20 | 6747.84 | -32.36 |
| Testify | No changes | 112.33 | 121.44 | +9.11 |
| Testify | Source edit | 530.54 | 551.84 | +21.30 |
| Testify | Forced rebuild (-a) | 7382.18 | 7801.39 | +419.21 |
| Client coverage | No changes | 96.31 | 92.52 | -3.79 |
| Client coverage | Source edit | 419.87 | 411.29 | -8.59 |
| Client coverage | Forced rebuild (-a) | 7015.54 | 6869.83 | -145.71 |
| Testify + client coverage | No changes | 125.06 | 125.84 | +0.79 |
| Testify + client coverage | Source edit | 559.33 | 561.21 | +1.88 |
| Testify + client coverage | Forced rebuild (-a) | 7682.10 | 7789.88 | +107.77 |

These are medians on a small fixture, not a forecast for Gin, Chi or another application. Forced rebuilds have only three observations per variant; inspect the ranges and CPU measurements in [the raw results](performance.json) before interpreting small differences. The new entry hook closes an instrumentation gap but does not make every Testify build faster.

## Isolated bypass

41 observations per variant invoke the real `compile -V=full`. The former helper delegates through a child; the selective helper replaces itself on Unix. This isolates startup/delegation from package preparation and linking.

| Invocation | Wall median (ms) | CPU median (ms) |
| --- | ---: | ---: | ---: |
| Native tool | 2.532 | 2.489 |
| Former helper | 3.878 | 4.049 |
| Selective bypass | 3.638 | 3.692 |

The dispatch microbenchmark takes 12.40–12.56 ns/op with 0 B/op and 0 allocations/op across five runs. That measures only the branch decision. Process startup remains visible in the table above. See [the benchmark output](dispatch-benchmark.log).

## Correctness evidence

Linux Go 1.26.8 passes the complete suite; the final unique-contract cache assertion was also rerun separately. Linux Go 1.27.1 passes the complete `-race` suite. Both compare 26 Testify cases against the full SDK/Orchestrion reference. The external helper matches one session, one module, two suites and two tests.

| Evidence | Report |
| --- | --- |
| Go 1.26.8 | [Feature combinations and event counts](parity-126-final.md) |
| Go 1.27.1 with race harness | [Feature combinations and event counts](parity-127-final.md) |

Both reports preserve per-case durations; the suites ran alongside each other and those durations are compatibility observations, not an isolated timing comparison. The CLI timing series ran after the suites stopped.

The actual CLI and a Mini + Testify fixture with covered `testing` and `testify/suite` cross-compile for Windows/amd64 and macOS/arm64. Those binaries were not executed. Native platform CI remains required after publication. Ten seconds of fuzzing completed 133,363 executions. Source/license manifests and `go vet` passed.

## Inputs and reproduction

- Shared source base: `15727c1c469c7faf21551bf638424420125d234b`, with the uncommitted Testify prototype before this change.
- SDK extraction/reference: `96aedb31048c07e29e7a20a4333dc3b8d289c52d`.
- Before CLI SHA-256: `8d9712940c2c1b6b4c72d25f01b38a09784c4061475872f5f921c0c04905b9ed`.
- Selective CLI SHA-256: `b404521803df95a3d8b296c3a6c1bc8d6787318210ce36107de0e2880766d741`.
- [Final source hashes](final-source-hashes.json) identify the tested files; documentation and evidence files are outside that set.

[The measurement script](measure.py) records commands, order, wall/CPU time, compiler/cover/link counts and wrapper activation for every invocation. Its absolute toolchain and artifact paths describe this workstation; adjust those paths for another machine. It keeps all observations rather than removing outliers.
