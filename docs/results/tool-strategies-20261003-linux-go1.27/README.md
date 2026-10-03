# Testify tool strategy experiment

The selected change uses `go list -find` when the first package query already proves that Testify suites are reachable. Unknown test imports still use `-deps`; targets without suites or covered rewritten sources keep `-toolexec` disabled. The source transformation, selected-version/API validation and native compiler/linker identities are unchanged.

All measurements compile a Mini test binary with `go test -c -o`, `-ldflags=-w` and no test execution. Go 1.27.1, Linux/amd64, AMD Ryzen 9 5950X. `GOMAXPROCS`, `-p` and process affinity are set to 4 or 32 CPUs. Modules and filesystem caches are warm. Every cold observation starts with an empty `GOCACHE` and no output binary; the harness requires actual compiler and linker invocations.

660 observations: three cold samples and seven unchanged/edit samples per scenario, CPU count and variant, plus four cache-switch checks. Orders rotate across variants. Source edits are unique and must compile. All observations are kept. Cold measurements have only three samples; small differences must be read with their ranges.

## Alternatives

| Variant | Preparation | Tool activation |
| --- | --- | --- |
| Current | Existing package query, then resolve test imports as needed | Exact suite/coverage detection |
| Find (selected) | Use `-find` for a known suite; preserve `-deps` for unknown imports | Same as Current |
| One query (prototype) | `go list -deps -test`, filter client roots and their reachable dependencies | Exact suite/coverage detection |

The one-query prototype passes the Testify compatibility cases and shares the current cache. It saves more time on assert-only targets and external helpers, but adds about 8 ms to unchanged plain tests and needs extra handling for test variants and auxiliary packages. It remains an experiment.

## Wall time

Values are medians in milliseconds. Parenthesized percentages compare each alternative with Current; a negative value means a shorter command.

### Cold

| Scenario | CPUs | Current | Find | One query |
| --- | ---: | ---: | ---: | ---: |
| plain | 4 | 9046.34 | 9107.23 (+0.7%) | 9061.61 (+0.2%) |
| plain | 32 | 5708.60 | 5676.70 (-0.6%) | 5715.50 (+0.1%) |
| assert-only | 4 | 9574.39 | 9581.25 (+0.1%) | 9559.11 (-0.2%) |
| assert-only | 32 | 5930.44 | 5974.18 (+0.7%) | 5907.04 (-0.4%) |
| suite-direct | 4 | 9925.38 | 9886.05 (-0.4%) | 9870.96 (-0.5%) |
| suite-direct | 32 | 6351.86 | 6315.21 (-0.6%) | 6326.20 (-0.4%) |
| suite-external-helper | 4 | 9889.19 | 9918.01 (+0.3%) | 9870.99 (-0.2%) |
| suite-external-helper | 32 | 5998.04 | 6060.03 (+1.0%) | 5960.05 (-0.6%) |
| suite-client-coverage | 4 | 10043.93 | 10017.42 (-0.3%) | 9968.11 (-0.8%) |
| suite-client-coverage | 32 | 6060.89 | 6041.24 (-0.3%) | 6063.69 (+0.0%) |
| suite-testing-coverage | 4 | 10076.90 | 10086.27 (+0.1%) | 10112.23 (+0.4%) |
| suite-testing-coverage | 32 | 6292.68 | 6117.91 (-2.8%) | 6087.48 (-3.3%) |

### Unchanged

| Scenario | CPUs | Current | Find | One query |
| --- | ---: | ---: | ---: | ---: |
| plain | 4 | 89.15 | 88.39 (-0.9%) | 97.16 (+9.0%) |
| plain | 32 | 100.80 | 101.30 (+0.5%) | 109.34 (+8.5%) |
| assert-only | 4 | 115.00 | 113.71 (-1.1%) | 101.02 (-12.2%) |
| assert-only | 32 | 130.83 | 129.60 (-0.9%) | 111.53 (-14.8%) |
| suite-direct | 4 | 120.31 | 110.34 (-8.3%) | 108.15 (-10.1%) |
| suite-direct | 32 | 133.74 | 120.24 (-10.1%) | 115.97 (-13.3%) |
| suite-external-helper | 4 | 123.25 | 122.02 (-1.0%) | 107.99 (-12.4%) |
| suite-external-helper | 32 | 135.31 | 133.78 (-1.1%) | 116.58 (-13.8%) |
| suite-client-coverage | 4 | 125.45 | 113.35 (-9.6%) | 111.89 (-10.8%) |
| suite-client-coverage | 32 | 139.18 | 124.67 (-10.4%) | 121.50 (-12.7%) |
| suite-testing-coverage | 4 | 129.40 | 117.18 (-9.5%) | 115.68 (-10.6%) |
| suite-testing-coverage | 32 | 139.73 | 125.74 (-10.0%) | 124.03 (-11.2%) |

### After editing the tested package

| Scenario | CPUs | Current | Find | One query |
| --- | ---: | ---: | ---: | ---: |
| plain | 4 | 308.32 | 305.59 (-0.9%) | 309.49 (+0.4%) |
| plain | 32 | 317.70 | 323.00 (+1.7%) | 329.13 (+3.6%) |
| assert-only | 4 | 337.54 | 335.85 (-0.5%) | 318.80 (-5.6%) |
| assert-only | 32 | 386.56 | 382.24 (-1.1%) | 366.10 (-5.3%) |
| suite-direct | 4 | 405.56 | 395.75 (-2.4%) | 393.33 (-3.0%) |
| suite-direct | 32 | 454.35 | 433.20 (-4.7%) | 437.10 (-3.8%) |
| suite-external-helper | 4 | 391.26 | 385.15 (-1.6%) | 364.55 (-6.8%) |
| suite-external-helper | 32 | 402.99 | 412.05 (+2.2%) | 385.73 (-4.3%) |
| suite-client-coverage | 4 | 426.26 | 414.95 (-2.7%) | 418.93 (-1.7%) |
| suite-client-coverage | 32 | 443.21 | 425.87 (-3.9%) | 430.06 (-3.0%) |
| suite-testing-coverage | 4 | 421.79 | 407.13 (-3.5%) | 407.49 (-3.4%) |
| suite-testing-coverage | 32 | 449.31 | 444.64 (-1.0%) | 432.39 (-3.8%) |

## CPU time

CPU time sums the CLI and the descendants it waits for. These candidates have no detached build daemon. The JSON and CSV preserve CPU time, wall ranges, tool counts, source hashes and commands for every observation. The `wrapper` field records wrapper commands visible in `-x`; cached version probes are not printed, so `false` does not prove that the wrapper was disabled.

## Cache and error behavior

All 252 unchanged samples avoid compilation and linking. After the initial cache-switch output is created, all 36 switches between safe variants avoid both operations. The variants keep the same prepared source fingerprint; this optimization does not create a new cache namespace.

Two initial prototypes moved Testify resolution into the compile wrapper: always enabled, and enabled only for known/uncertain imports. Their shared static marker reused correctly instrumented objects across modes, but both failed the selected-version guard with a warm cache. The fixture changed a consistent vendored replacement from v1.11.1 to v1.10.0 while keeping source bytes identical. Current rejected it before compilation; both deferred prototypes accepted it without recompiling Testify. They were rejected before the performance matrix. See [the counterexample](cache-gate-vendor.json).

A plain requirement change was insufficient for that reproduction because Go minimum-version selection kept v1.11.1. Replacing files under the module cache was also rejected by Go. Those setup probes remain in the task artifacts; the final counterexample uses a normal vendor tree and matching metadata.

## Compatibility

Both safe prototypes pass the existing selected-version, native-output, aliases, overlays, coverage, GOFLAGS, workspace, bypass and cache tests. Each passes all 26 Testify feature combinations against the full SDK/Orchestrion reference, including external helpers and both retry modes. The reports keep event counts and per-case timings: [Find](parity-find-testify.json), [One query](parity-one-pass-testify.json). Those runtime timings qualify compatibility and are separate from this compile-only benchmark.

The selected production source retains the preflight guard, with a new regression test for warm vendored sources. The selected source passes the complete suite on Go 1.26.8 and the complete `-race` suite on Go 1.27.1. Both include the new warm-vendor regression and all 26 Testify cases against SDK/Orchestrion: [Go 1.26 report](final-parity-126-testify.json), [Go 1.27 race report](final-parity-127-race-testify.json). `go vet`, all 14 Python tests and source/license manifest verification pass. The CLI cross-compiles for Windows/amd64 and macOS/arm64; native execution remains pending publication and platform CI.

## Evidence and reproduction

- Shared source base: `15727c1c469c7faf21551bf638424420125d234b`, including the prior uncommitted selective-tool implementation.
- Incorporated SDK/reference: `96aedb31048c07e29e7a20a4333dc3b8d289c52d`.
- [Raw measurements](benchmark-results.json), [CSV](runs.csv), [harness](benchmark.py), [input hashes](input-hashes.json), [safe cache checks](cache-gate-safe.json).

The harness describes the workstation paths for the three frozen CLI binaries and the POC module. Change those paths for another machine. It stores full `-x` logs in its task directory. Those logs and all fixture files are retained at `/var/tmp/dd-ci-testing-poc-tool-strategies-20261003`.

The preliminary series used an empty Go cache but preserved output binaries. Go skipped linking on repeat cold samples. It was stopped, retained as diagnostic evidence and excluded; the final matrix above removes the output and verifies linking.

The measured Find CLI and final CLI differ by comments/formatting in the same lookup branch and the added regression test; the transformation contract and logic are unchanged. [Final source hashes](final-source-hashes.json) identify the applied files. The final CLI SHA-256 is `3c1b833b4e41914ffe9f87fb50e208c4f133e3dca3c2cf0ea145925effa788f7`.
