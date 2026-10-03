# Compile-only comparison: Native, Orchestrion, POC SDK and POC Mini

POC `95f8f32b7e8898d96cd72f7f4c89f6905031cd9e`, Go `go version go1.27.1 linux/amd64`, SDK `v2.12.0-dev.3.0.20261002145613-96aedb31048c` (`96aedb31048c07e29e7a20a4333dc3b8d289c52d`), Orchestrion `v1.13.2-0.20260917114356-5c24783fcd76`.

Every observation compiles all selected test binaries with `-c -o <output-directory>/ -ldflags=-w ./...`. Test binaries are never executed. SDK startup, test execution and event delivery are excluded. Orchestrion uses the testing-only rules from the pinned SDK, including its Testify aspect. POC SDK uses `ddtest test --runtime=sdk`; POC Mini uses `ddtest test --runtime=mini`.

CPU affinity, `GOMAXPROCS` and `-p` agree. Four CPUs are four physical cores; 32 CPUs include 16 physical cores and their SMT siblings. Builds run serially with rotating variant order. Every cold observation has an independent empty Go build cache and no existing output. Module downloads are disabled; module contents and the OS page cache are warm. All four variants use identical subject sources and the same effective module graph.

Gin v1.12.0 produces six test binaries, Chi v5.3.2 two. Testify v1.12.1 uses small fixtures producing one binary each: a direct suite call, and a call through an external replacement module. These fixtures measure instrumentation/build costs rather than a representative large application. They include a normal Go source file for coverage. Coverage of `testing` and `testify/suite` exercises the POC coverage bridge.

**Values are medians in seconds. Each POC cell shows the signed change against total Orchestrion wall time first, followed by the change against Native.** Percentages use unrounded medians. For example, `(-50%; +20%)` means half Orchestrion wall time and 20% more time than Native.

| Series | Cold | Unchanged | Forced link | Real test-body edit | Unused-constant diagnostic |
| --- | ---: | ---: | ---: | ---: | ---: |
| Gin/Chi without additional flags | 5 | 10 | 20 | 10 | 3 |
| Testify/coverage/race combinations | 3 | 10 | 10 | 5 | 3 |

An 80-run Native link control met the declared median-convergence rule: median 1.140 s, bootstrap 95% median interval 1.049–1.192 s. Individual runs still ranged from 0.783 to 2.561 s. Some matrix cells also contain I/O-correlated slow runs, which remain included. This convergence result applies to the control, not every cell. Small differences should be read with the ranges, sample counts and uncertainty in the summaries.

The original expanded runner stopped at its initial three-hour boundary. One incomplete forced-link attempt is retained in `extra/censored`; its timing could not be collected. The authorized five-hour continuation kept completed rows and warm caches and retried that missing forced link with a fresh build ID. Completed observations were not discarded or repeated.

Regenerate these tables from the checked-in CSV with:

```sh
python3 scripts/build_benchmark.py report \
  --input docs/results/compile-matrix-20261003-linux-go1.27 --update-readme
```

[Run the complete matrix or a smaller selection](../../build-benchmarks.md).

