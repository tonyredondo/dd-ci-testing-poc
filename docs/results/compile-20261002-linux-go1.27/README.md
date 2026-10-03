# Refreshed four-variant compilation comparison

This report preserves the 2026-10-02 comparison published in
[PR #2](https://github.com/tonyredondo/dd-ci-testing-poc/pull/2).
It predates the Testify entry instrumentation and discovery optimization.
The table values retain the measured revision below; updating this document
does not remeasure them. This snapshot contains summary medians, not the
per-command dataset. The later [tool-strategy report](../tool-strategies-20261003-linux-go1.27/README.md)
includes every observation for that separate experiment.

Measured code: `8030a883d51a3e19e972dd4c7e5039c5b25ea8b9`. Native, Orchestrion, POC SDK and POC Mini use identical frozen Gin/Chi sources and module resolution. `go test -c -o <directory>/ -ldflags=-w ./...`; test binaries were never executed. The same Go development toolchain and frozen Orchestrion reference were retained.

Wall-clock medians in seconds. In each POC cell, the first percentage is the signed time change relative to Orchestrion (negative means faster); the second is the added time relative to Native (positive means slower). Both use unrounded medians. Five independent empty-cache cold rounds, five cached/forced-link rounds and three edit rounds. Variant order rotates; every observation is retained. Modules and OS page cache remain warm. Four logical CPUs use four physical cores; 32 logical CPUs use all 16 physical cores and SMT. Affinity, `GOMAXPROCS` and `-p` match each configuration. Aggregate CPU and peak memory use exclusive cgroups and include detached daemons and nested builds.

The idle-host native same-code controls passed the predeclared thresholds: cold wall variation 2.47%, CPU 0.39%; forced-link wall variation 2.86%. All 430 commands passed, with 64 qualified output binaries.

## Cold compile (empty Go build cache)

| Project | CPUs | Native | Orchestrion | POC SDK | POC Mini |
| --- | ---: | ---: | ---: | ---: | ---: |
| Gin | 4 | 17.888 s | 46.691 s | 27.332 s (−41.5% vs Orchestrion; +52.8% vs Native) | 18.951 s (−59.4% vs Orchestrion; +5.9% vs Native) |
| Gin | 32 | 10.666 s | 24.340 s | 14.821 s (−39.1% vs Orchestrion; +39.0% vs Native) | 11.716 s (−51.9% vs Orchestrion; +9.8% vs Native) |
| Chi | 4 | 7.794 s | 28.940 s | 18.267 s (−36.9% vs Orchestrion; +134.4% vs Native) | 9.504 s (−67.2% vs Orchestrion; +21.9% vs Native) |
| Chi | 32 | 4.701 s | 18.351 s | 9.963 s (−45.7% vs Orchestrion; +112.0% vs Native) | 5.935 s (−67.7% vs Orchestrion; +26.3% vs Native) |

## Cached compile (no changes, binary reused)

| Project | CPUs | Native | Orchestrion | POC SDK | POC Mini |
| --- | ---: | ---: | ---: | ---: | ---: |
| Gin | 4 | 0.096 s | 0.358 s | 0.194 s (−45.8% vs Orchestrion; +103.3% vs Native) | 0.141 s (−60.6% vs Orchestrion; +47.7% vs Native) |
| Gin | 32 | 0.099 s | 0.399 s | 0.194 s (−51.3% vs Orchestrion; +95.5% vs Native) | 0.152 s (−61.9% vs Orchestrion; +52.9% vs Native) |
| Chi | 4 | 0.060 s | 0.322 s | 0.156 s (−51.4% vs Orchestrion; +158.7% vs Native) | 0.097 s (−70.0% vs Orchestrion; +59.9% vs Native) |
| Chi | 32 | 0.065 s | 0.376 s | 0.157 s (−58.3% vs Orchestrion; +139.8% vs Native) | 0.106 s (−71.9% vs Orchestrion; +61.7% vs Native) |

## Warm packages, forced fresh link

| Project | CPUs | Native | Orchestrion | POC SDK | POC Mini |
| --- | ---: | ---: | ---: | ---: | ---: |
| Gin | 4 | 1.196 s | 5.521 s | 3.398 s (−38.5% vs Orchestrion; +184.2% vs Native) | 1.492 s (−73.0% vs Orchestrion; +24.7% vs Native) |
| Gin | 32 | 1.173 s | 4.521 s | 4.015 s (−11.2% vs Orchestrion; +242.2% vs Native) | 1.863 s (−58.8% vs Orchestrion; +58.8% vs Native) |
| Chi | 4 | 0.268 s | 1.840 s | 0.981 s (−46.7% vs Orchestrion; +266.7% vs Native) | 0.378 s (−79.4% vs Orchestrion; +41.3% vs Native) |
| Chi | 32 | 0.279 s | 1.749 s | 0.997 s (−43.0% vs Orchestrion; +257.1% vs Native) | 0.390 s (−77.7% vs Orchestrion; +39.7% vs Native) |

## Legacy unused-constant edit (diagnostic)

| Project | CPUs | Native | Orchestrion | POC SDK | POC Mini |
| --- | ---: | ---: | ---: | ---: | ---: |
| Gin | 4 | 0.771 s | 1.162 s | 0.853 s (−26.6% vs Orchestrion; +10.6% vs Native) | 0.810 s (−30.3% vs Orchestrion; +5.0% vs Native) |
| Gin | 32 | 0.646 s | 1.087 s | 0.742 s (−31.8% vs Orchestrion; +14.8% vs Native) | 0.718 s (−33.9% vs Orchestrion; +11.1% vs Native) |
| Chi | 4 | 0.270 s | 0.651 s | 0.348 s (−46.5% vs Orchestrion; +29.0% vs Native) | 0.301 s (−53.7% vs Orchestrion; +11.4% vs Native) |
| Chi | 32 | 0.259 s | 0.680 s | 0.334 s (−51.0% vs Orchestrion; +28.9% vs Native) | 0.304 s (−55.3% vs Orchestrion; +17.4% vs Native) |

## Incremental after a real test-body edit

| Project | CPUs | Native | Orchestrion | POC SDK | POC Mini |
| --- | ---: | ---: | ---: | ---: | ---: |
| Gin | 4 | 1.428 s | 2.670 s | 1.839 s (−31.1% vs Orchestrion; +28.8% vs Native) | 1.449 s (−45.7% vs Orchestrion; +1.5% vs Native) |
| Gin | 32 | 1.087 s | 3.022 s | 1.915 s (−36.6% vs Orchestrion; +76.2% vs Native) | 1.281 s (−57.6% vs Orchestrion; +17.9% vs Native) |
| Chi | 4 | 0.409 s | 1.639 s | 0.919 s (−44.0% vs Orchestrion; +124.6% vs Native) | 0.500 s (−69.5% vs Orchestrion; +22.2% vs Native) |
| Chi | 32 | 0.409 s | 1.723 s | 0.957 s (−44.5% vs Orchestrion; +133.8% vs Native) | 0.507 s (−70.6% vs Orchestrion; +23.8% vs Native) |


Remaining variation: Gin/32 cold Mini ranges 11.210–14.360 s, Orchestrion 21.319–29.105 s and native 10.463–11.171 s. Two additional consecutive Mini/32 cold controls gave 11.133 and 11.860 s (6.53% wall variation), separately from the medians above. The unused-constant edit is a diagnostic and can reuse executable code; the real test-body edit compiles and links a reachable `t.Log` change. These local observations do not prove universal performance gains or identify the cause of residual variation.
