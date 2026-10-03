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

## Cold compilation

| Project | Flags | CPUs | Native | Orchestrion | POC SDK | POC Mini |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| Gin | `none` | 4 | 17.984 s | 46.941 s | 27.791 s (-40.8%; +54.5%) | 19.314 s (-58.9%; +7.4%) |
| Gin | `none` | 32 | 11.155 s | 23.795 s | 14.796 s (-37.8%; +32.6%) | 11.701 s (-50.8%; +4.9%) |
| Chi | `none` | 4 | 7.758 s | 28.672 s | 18.189 s (-36.6%; +134.5%) | 9.539 s (-66.7%; +23.0%) |
| Chi | `none` | 32 | 4.615 s | 16.810 s | 9.348 s (-44.4%; +102.5%) | 5.901 s (-64.9%; +27.9%) |
| Gin | `-race` | 4 | 27.080 s | 54.044 s | 34.249 s (-36.6%; +26.5%) | 27.760 s (-48.6%; +2.5%) |
| Gin | `-race` | 32 | 22.866 s | 33.764 s | 26.537 s (-21.4%; +16.1%) | 23.717 s (-29.8%; +3.7%) |
| Gin | `-cover` | 4 | 18.276 s | 66.570 s | 27.279 s (-59.0%; +49.3%) | 19.677 s (-70.4%; +7.7%) |
| Gin | `-cover` | 32 | 10.750 s | 31.493 s | 14.616 s (-53.6%; +36.0%) | 11.520 s (-63.4%; +7.2%) |
| Gin | `-coverpkg=./... -covermode=atomic` | 4 | 18.432 s | 55.382 s | 28.481 s (-48.6%; +54.5%) | 19.944 s (-64.0%; +8.2%) |
| Gin | `-coverpkg=./... -covermode=atomic` | 32 | 11.090 s | 27.029 s | 16.742 s (-38.1%; +51.0%) | 11.930 s (-55.9%; +7.6%) |
| Gin | `-race -coverpkg=./... -covermode=atomic` | 4 | 28.931 s | 63.578 s | 38.028 s (-40.2%; +31.4%) | 29.740 s (-53.2%; +2.8%) |
| Gin | `-race -coverpkg=./... -covermode=atomic` | 32 | 24.106 s | 37.571 s | 27.536 s (-26.7%; +14.2%) | 24.423 s (-35.0%; +1.3%) |
| Chi | `-race` | 4 | 11.393 s | 37.118 s | 23.489 s (-36.7%; +106.2%) | 14.343 s (-61.4%; +25.9%) |
| Chi | `-race` | 32 | 9.334 s | 27.500 s | 16.957 s (-38.3%; +81.7%) | 12.292 s (-55.3%; +31.7%) |
| Chi | `-cover` | 4 | 7.984 s | 36.111 s | 18.211 s (-49.6%; +128.1%) | 9.727 s (-73.1%; +21.8%) |
| Chi | `-cover` | 32 | 4.709 s | 19.579 s | 9.498 s (-51.5%; +101.7%) | 5.924 s (-69.7%; +25.8%) |
| Chi | `-coverpkg=./... -covermode=atomic` | 4 | 7.979 s | 33.063 s | 18.549 s (-43.9%; +132.5%) | 9.760 s (-70.5%; +22.3%) |
| Chi | `-coverpkg=./... -covermode=atomic` | 32 | 5.032 s | 18.378 s | 9.427 s (-48.7%; +87.3%) | 5.951 s (-67.6%; +18.3%) |
| Chi | `-race -coverpkg=./... -covermode=atomic` | 4 | 11.625 s | 40.849 s | 23.661 s (-42.1%; +103.5%) | 14.459 s (-64.6%; +24.4%) |
| Chi | `-race -coverpkg=./... -covermode=atomic` | 32 | 9.552 s | 28.938 s | 19.140 s (-33.9%; +100.4%) | 12.262 s (-57.6%; +28.4%) |
| Testify Direct | `none` | 4 | 8.068 s | 28.251 s | 17.765 s (-37.1%; +120.2%) | 9.942 s (-64.8%; +23.2%) |
| Testify Direct | `none` | 32 | 4.706 s | 17.741 s | 9.400 s (-47.0%; +99.7%) | 6.861 s (-61.3%; +45.8%) |
| Testify Direct | `-race` | 4 | 12.300 s | 36.149 s | 23.381 s (-35.3%; +90.1%) | 15.139 s (-58.1%; +23.1%) |
| Testify Direct | `-race` | 32 | 9.625 s | 26.081 s | 16.833 s (-35.5%; +74.9%) | 12.140 s (-53.5%; +26.1%) |
| Testify Direct | `-cover` | 4 | 8.169 s | 33.107 s | 17.840 s (-46.1%; +118.4%) | 10.020 s (-69.7%; +22.7%) |
| Testify Direct | `-cover` | 32 | 4.796 s | 19.399 s | 9.678 s (-50.1%; +101.8%) | 5.975 s (-69.2%; +24.6%) |
| Testify Direct | `-coverpkg=./... -covermode=atomic` | 4 | 8.207 s | 29.215 s | 17.851 s (-38.9%; +117.5%) | 9.971 s (-65.9%; +21.5%) |
| Testify Direct | `-coverpkg=./... -covermode=atomic` | 32 | 4.711 s | 17.488 s | 9.569 s (-45.3%; +103.1%) | 6.004 s (-65.7%; +27.5%) |
| Testify Direct | `-race -coverpkg=./... -covermode=atomic` | 4 | 12.414 s | 37.249 s | 23.612 s (-36.6%; +90.2%) | 15.315 s (-58.9%; +23.4%) |
| Testify Direct | `-race -coverpkg=./... -covermode=atomic` | 32 | 9.722 s | 26.852 s | 17.028 s (-36.6%; +75.2%) | 12.273 s (-54.3%; +26.2%) |
| Testify Direct | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 8.225 s | 28.934 s | 17.871 s (-38.2%; +117.3%) | 10.150 s (-64.9%; +23.4%) |
| Testify Direct | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 4.730 s | 17.190 s | 9.649 s (-43.9%; +104.0%) | 5.990 s (-65.2%; +26.6%) |
| Testify Direct | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 12.492 s | 40.515 s | 23.498 s (-42.0%; +88.1%) | 15.378 s (-62.0%; +23.1%) |
| Testify Direct | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 9.677 s | 26.267 s | 17.013 s (-35.2%; +75.8%) | 12.235 s (-53.4%; +26.4%) |
| Testify External | `none` | 4 | 8.072 s | 28.404 s | 17.837 s (-37.2%; +121.0%) | 9.883 s (-65.2%; +22.4%) |
| Testify External | `none` | 32 | 4.673 s | 17.106 s | 9.504 s (-44.4%; +103.4%) | 6.053 s (-64.6%; +29.5%) |
| Testify External | `-race` | 4 | 12.312 s | 36.359 s | 23.330 s (-35.8%; +89.5%) | 15.129 s (-58.4%; +22.9%) |
| Testify External | `-race` | 32 | 9.629 s | 26.388 s | 16.942 s (-35.8%; +75.9%) | 12.328 s (-53.3%; +28.0%) |
| Testify External | `-cover` | 4 | 8.212 s | 35.387 s | 17.821 s (-49.6%; +117.0%) | 10.027 s (-71.7%; +22.1%) |
| Testify External | `-cover` | 32 | 4.801 s | 18.694 s | 9.674 s (-48.3%; +101.5%) | 6.015 s (-67.8%; +25.3%) |
| Testify External | `-coverpkg=./... -covermode=atomic` | 4 | 8.251 s | 29.469 s | 17.917 s (-39.2%; +117.2%) | 10.072 s (-65.8%; +22.1%) |
| Testify External | `-coverpkg=./... -covermode=atomic` | 32 | 4.791 s | 16.785 s | 9.580 s (-42.9%; +100.0%) | 6.051 s (-64.0%; +26.3%) |
| Testify External | `-race -coverpkg=./... -covermode=atomic` | 4 | 12.431 s | 37.709 s | 23.423 s (-37.9%; +88.4%) | 15.445 s (-59.0%; +24.2%) |
| Testify External | `-race -coverpkg=./... -covermode=atomic` | 32 | 9.682 s | 27.106 s | 17.066 s (-37.0%; +76.3%) | 12.233 s (-54.9%; +26.3%) |
| Testify External | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 8.228 s | 29.804 s | 19.557 s (-34.4%; +137.7%) | 10.154 s (-65.9%; +23.4%) |
| Testify External | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 4.740 s | 16.570 s | 9.525 s (-42.5%; +100.9%) | 6.383 s (-61.5%; +34.6%) |
| Testify External | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 12.511 s | 37.381 s | 23.747 s (-36.5%; +89.8%) | 15.347 s (-58.9%; +22.7%) |
| Testify External | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 9.653 s | 26.233 s | 17.040 s (-35.0%; +76.5%) | 12.337 s (-53.0%; +27.8%) |

## Cached compilation — unchanged output reused

| Project | Flags | CPUs | Native | Orchestrion | POC SDK | POC Mini |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| Gin | `none` | 4 | 0.096 s | 0.369 s | 0.240 s (-35.0%; +149.5%) | 0.179 s (-51.6%; +85.7%) |
| Gin | `none` | 32 | 0.098 s | 0.411 s | 0.253 s (-38.5%; +158.6%) | 0.199 s (-51.7%; +103.2%) |
| Chi | `none` | 4 | 0.063 s | 0.327 s | 0.189 s (-42.3%; +200.9%) | 0.123 s (-62.4%; +96.3%) |
| Chi | `none` | 32 | 0.067 s | 0.382 s | 0.199 s (-47.9%; +195.0%) | 0.141 s (-63.2%; +108.8%) |
| Gin | `-race` | 4 | 0.098 s | 0.368 s | 0.241 s (-34.5%; +145.8%) | 0.182 s (-50.6%; +85.3%) |
| Gin | `-race` | 32 | 0.098 s | 0.415 s | 0.255 s (-38.6%; +159.7%) | 0.198 s (-52.2%; +102.1%) |
| Gin | `-cover` | 4 | 0.128 s | 0.515 s | 0.270 s (-47.5%; +111.6%) | 0.211 s (-59.0%; +65.3%) |
| Gin | `-cover` | 32 | 0.127 s | 0.571 s | 0.275 s (-51.8%; +117.0%) | 0.225 s (-60.7%; +77.3%) |
| Gin | `-coverpkg=./... -covermode=atomic` | 4 | 0.131 s | 0.435 s | 0.280 s (-35.7%; +114.0%) | 0.216 s (-50.5%; +64.9%) |
| Gin | `-coverpkg=./... -covermode=atomic` | 32 | 0.127 s | 0.470 s | 0.286 s (-39.2%; +125.6%) | 0.227 s (-51.8%; +78.7%) |
| Gin | `-race -coverpkg=./... -covermode=atomic` | 4 | 0.133 s | 0.440 s | 0.281 s (-36.1%; +112.2%) | 0.218 s (-50.5%; +64.3%) |
| Gin | `-race -coverpkg=./... -covermode=atomic` | 32 | 0.130 s | 0.479 s | 0.286 s (-40.2%; +121.0%) | 0.228 s (-52.4%; +76.0%) |
| Chi | `-race` | 4 | 0.062 s | 0.330 s | 0.190 s (-42.4%; +204.6%) | 0.123 s (-62.7%; +97.2%) |
| Chi | `-race` | 32 | 0.066 s | 0.383 s | 0.200 s (-47.9%; +200.8%) | 0.140 s (-63.4%; +111.6%) |
| Chi | `-cover` | 4 | 0.067 s | 0.427 s | 0.194 s (-54.6%; +190.8%) | 0.128 s (-70.0%; +92.3%) |
| Chi | `-cover` | 32 | 0.071 s | 0.493 s | 0.201 s (-59.3%; +182.8%) | 0.143 s (-71.0%; +101.2%) |
| Chi | `-coverpkg=./... -covermode=atomic` | 4 | 0.068 s | 0.373 s | 0.202 s (-45.8%; +197.5%) | 0.130 s (-65.2%; +90.8%) |
| Chi | `-coverpkg=./... -covermode=atomic` | 32 | 0.072 s | 0.431 s | 0.207 s (-51.9%; +186.5%) | 0.141 s (-67.3%; +94.7%) |
| Chi | `-race -coverpkg=./... -covermode=atomic` | 4 | 0.068 s | 0.374 s | 0.203 s (-45.8%; +199.2%) | 0.130 s (-65.2%; +92.3%) |
| Chi | `-race -coverpkg=./... -covermode=atomic` | 32 | 0.072 s | 0.435 s | 0.207 s (-52.5%; +189.2%) | 0.143 s (-67.2%; +99.9%) |
| Testify Direct | `none` | 4 | 0.058 s | 0.314 s | 0.172 s (-45.3%; +195.8%) | 0.110 s (-65.0%; +89.4%) |
| Testify Direct | `none` | 32 | 0.059 s | 0.358 s | 0.177 s (-50.6%; +199.5%) | 0.121 s (-66.3%; +104.4%) |
| Testify Direct | `-race` | 4 | 0.057 s | 0.312 s | 0.172 s (-44.8%; +201.0%) | 0.108 s (-65.3%; +89.4%) |
| Testify Direct | `-race` | 32 | 0.059 s | 0.356 s | 0.178 s (-50.0%; +199.3%) | 0.120 s (-66.3%; +102.0%) |
| Testify Direct | `-cover` | 4 | 0.064 s | 0.415 s | 0.183 s (-55.8%; +186.8%) | 0.116 s (-72.2%; +80.5%) |
| Testify Direct | `-cover` | 32 | 0.062 s | 0.432 s | 0.181 s (-58.1%; +190.2%) | 0.122 s (-71.7%; +95.8%) |
| Testify Direct | `-coverpkg=./... -covermode=atomic` | 4 | 0.063 s | 0.336 s | 0.184 s (-45.1%; +194.9%) | 0.115 s (-65.8%; +83.4%) |
| Testify Direct | `-coverpkg=./... -covermode=atomic` | 32 | 0.064 s | 0.383 s | 0.186 s (-51.4%; +190.8%) | 0.124 s (-67.5%; +94.3%) |
| Testify Direct | `-race -coverpkg=./... -covermode=atomic` | 4 | 0.061 s | 0.333 s | 0.181 s (-45.5%; +198.5%) | 0.112 s (-66.3%; +84.6%) |
| Testify Direct | `-race -coverpkg=./... -covermode=atomic` | 32 | 0.064 s | 0.380 s | 0.186 s (-51.1%; +188.5%) | 0.123 s (-67.5%; +91.6%) |
| Testify Direct | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 0.064 s | 0.324 s | 0.188 s (-42.0%; +191.6%) | 0.119 s (-63.3%; +84.9%) |
| Testify Direct | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 0.064 s | 0.360 s | 0.191 s (-46.9%; +198.3%) | 0.126 s (-65.0%; +96.5%) |
| Testify Direct | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 0.063 s | 0.319 s | 0.188 s (-40.9%; +197.9%) | 0.117 s (-63.4%; +84.5%) |
| Testify Direct | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 0.065 s | 0.364 s | 0.195 s (-46.5%; +197.5%) | 0.127 s (-65.0%; +94.7%) |
| Testify External | `none` | 4 | 0.059 s | 0.315 s | 0.184 s (-41.6%; +212.0%) | 0.121 s (-61.6%; +105.2%) |
| Testify External | `none` | 32 | 0.060 s | 0.361 s | 0.191 s (-47.2%; +215.9%) | 0.133 s (-63.1%; +121.0%) |
| Testify External | `-race` | 4 | 0.057 s | 0.313 s | 0.183 s (-41.5%; +221.4%) | 0.120 s (-61.6%; +110.9%) |
| Testify External | `-race` | 32 | 0.060 s | 0.357 s | 0.189 s (-47.0%; +217.2%) | 0.134 s (-62.4%; +124.7%) |
| Testify External | `-cover` | 4 | 0.063 s | 0.378 s | 0.186 s (-50.8%; +197.1%) | 0.124 s (-67.2%; +98.3%) |
| Testify External | `-cover` | 32 | 0.063 s | 0.432 s | 0.192 s (-55.6%; +203.2%) | 0.135 s (-68.8%; +112.8%) |
| Testify External | `-coverpkg=./... -covermode=atomic` | 4 | 0.063 s | 0.337 s | 0.195 s (-42.2%; +209.3%) | 0.128 s (-62.0%; +103.2%) |
| Testify External | `-coverpkg=./... -covermode=atomic` | 32 | 0.064 s | 0.382 s | 0.200 s (-47.6%; +210.6%) | 0.138 s (-63.9%; +114.1%) |
| Testify External | `-race -coverpkg=./... -covermode=atomic` | 4 | 0.062 s | 0.335 s | 0.195 s (-41.8%; +216.0%) | 0.125 s (-62.7%; +102.7%) |
| Testify External | `-race -coverpkg=./... -covermode=atomic` | 32 | 0.064 s | 0.380 s | 0.199 s (-47.6%; +210.8%) | 0.138 s (-63.6%; +115.7%) |
| Testify External | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 0.065 s | 0.322 s | 0.200 s (-37.8%; +209.8%) | 0.130 s (-59.7%; +100.7%) |
| Testify External | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 0.065 s | 0.361 s | 0.203 s (-44.0%; +213.1%) | 0.139 s (-61.5%; +115.2%) |
| Testify External | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 0.063 s | 0.321 s | 0.200 s (-37.6%; +216.1%) | 0.128 s (-60.3%; +101.3%) |
| Testify External | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 0.066 s | 0.366 s | 0.204 s (-44.2%; +210.7%) | 0.140 s (-61.8%; +112.9%) |

## Warm dependencies — forced fresh link

| Project | Flags | CPUs | Native | Orchestrion | POC SDK | POC Mini |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| Gin | `none` | 4 | 1.248 s | 5.105 s | 4.334 s (-15.1%; +247.2%) | 1.658 s (-67.5%; +32.8%) |
| Gin | `none` | 32 | 1.204 s | 5.118 s | 4.357 s (-14.9%; +261.9%) | 1.750 s (-65.8%; +45.4%) |
| Chi | `none` | 4 | 0.262 s | 1.808 s | 1.051 s (-41.9%; +300.8%) | 0.390 s (-78.5%; +48.6%) |
| Chi | `none` | 32 | 0.269 s | 1.768 s | 0.986 s (-44.2%; +266.6%) | 0.406 s (-77.0%; +51.0%) |
| Gin | `-race` | 4 | 1.619 s | 7.070 s | 6.412 s (-9.3%; +296.1%) | 2.682 s (-62.1%; +65.7%) |
| Gin | `-race` | 32 | 1.602 s | 8.310 s | 6.831 s (-17.8%; +326.5%) | 2.131 s (-74.4%; +33.1%) |
| Gin | `-cover` | 4 | 1.356 s | 6.477 s | 4.276 s (-34.0%; +215.3%) | 1.612 s (-75.1%; +18.9%) |
| Gin | `-cover` | 32 | 1.203 s | 5.344 s | 4.351 s (-18.6%; +261.7%) | 1.725 s (-67.7%; +43.4%) |
| Gin | `-coverpkg=./... -covermode=atomic` | 4 | 1.430 s | 5.501 s | 3.974 s (-27.8%; +178.0%) | 2.041 s (-62.9%; +42.8%) |
| Gin | `-coverpkg=./... -covermode=atomic` | 32 | 1.141 s | 4.768 s | 4.811 s (+0.9%; +321.8%) | 1.743 s (-63.4%; +52.8%) |
| Gin | `-race -coverpkg=./... -covermode=atomic` | 4 | 1.799 s | 7.708 s | 6.160 s (-20.1%; +242.5%) | 2.527 s (-67.2%; +40.5%) |
| Gin | `-race -coverpkg=./... -covermode=atomic` | 32 | 1.905 s | 7.816 s | 6.239 s (-20.2%; +227.6%) | 2.553 s (-67.3%; +34.1%) |
| Chi | `-race` | 4 | 0.362 s | 2.122 s | 1.344 s (-36.7%; +271.2%) | 0.579 s (-72.7%; +59.8%) |
| Chi | `-race` | 32 | 0.367 s | 2.026 s | 1.220 s (-39.8%; +232.7%) | 0.639 s (-68.5%; +74.2%) |
| Chi | `-cover` | 4 | 0.283 s | 2.062 s | 1.052 s (-49.0%; +272.0%) | 0.400 s (-80.6%; +41.4%) |
| Chi | `-cover` | 32 | 0.289 s | 2.037 s | 1.075 s (-47.2%; +272.4%) | 0.429 s (-78.9%; +48.7%) |
| Chi | `-coverpkg=./... -covermode=atomic` | 4 | 0.282 s | 2.024 s | 1.061 s (-47.6%; +276.0%) | 0.405 s (-80.0%; +43.4%) |
| Chi | `-coverpkg=./... -covermode=atomic` | 32 | 0.288 s | 1.910 s | 1.068 s (-44.1%; +270.4%) | 0.420 s (-78.0%; +45.7%) |
| Chi | `-race -coverpkg=./... -covermode=atomic` | 4 | 0.391 s | 2.166 s | 1.330 s (-38.6%; +239.9%) | 0.671 s (-69.0%; +71.4%) |
| Chi | `-race -coverpkg=./... -covermode=atomic` | 32 | 0.390 s | 2.101 s | 1.306 s (-37.8%; +235.2%) | 0.685 s (-67.4%; +75.8%) |
| Testify Direct | `none` | 4 | 0.244 s | 1.632 s | 0.932 s (-42.9%; +281.4%) | 0.355 s (-78.2%; +45.4%) |
| Testify Direct | `none` | 32 | 0.252 s | 1.819 s | 0.938 s (-48.4%; +272.4%) | 0.377 s (-79.2%; +49.8%) |
| Testify Direct | `-race` | 4 | 0.332 s | 1.829 s | 1.045 s (-42.9%; +214.3%) | 0.485 s (-73.5%; +45.9%) |
| Testify Direct | `-race` | 32 | 0.350 s | 1.923 s | 1.024 s (-46.7%; +192.6%) | 0.542 s (-71.8%; +54.8%) |
| Testify Direct | `-cover` | 4 | 0.261 s | 1.791 s | 0.881 s (-50.8%; +237.7%) | 0.369 s (-79.4%; +41.3%) |
| Testify Direct | `-cover` | 32 | 0.261 s | 1.873 s | 0.993 s (-47.0%; +280.2%) | 0.386 s (-79.4%; +48.0%) |
| Testify Direct | `-coverpkg=./... -covermode=atomic` | 4 | 0.255 s | 1.733 s | 0.908 s (-47.6%; +256.5%) | 0.368 s (-78.8%; +44.5%) |
| Testify Direct | `-coverpkg=./... -covermode=atomic` | 32 | 0.263 s | 1.872 s | 0.997 s (-46.7%; +278.6%) | 0.391 s (-79.1%; +48.4%) |
| Testify Direct | `-race -coverpkg=./... -covermode=atomic` | 4 | 0.350 s | 1.818 s | 1.045 s (-42.5%; +198.4%) | 0.582 s (-68.0%; +66.0%) |
| Testify Direct | `-race -coverpkg=./... -covermode=atomic` | 32 | 0.359 s | 1.928 s | 1.078 s (-44.1%; +200.4%) | 0.538 s (-72.1%; +49.9%) |
| Testify Direct | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 0.267 s | 1.737 s | 1.009 s (-41.9%; +278.0%) | 0.379 s (-78.2%; +41.8%) |
| Testify Direct | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 0.269 s | 1.853 s | 0.960 s (-48.2%; +256.7%) | 0.395 s (-78.7%; +46.8%) |
| Testify Direct | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 0.347 s | 1.813 s | 1.023 s (-43.6%; +195.0%) | 0.515 s (-71.6%; +48.4%) |
| Testify Direct | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 0.360 s | 1.947 s | 1.110 s (-43.0%; +208.8%) | 0.553 s (-71.6%; +53.7%) |
| Testify External | `none` | 4 | 0.253 s | 1.717 s | 0.979 s (-43.0%; +287.1%) | 0.374 s (-78.2%; +47.9%) |
| Testify External | `none` | 32 | 0.257 s | 1.753 s | 1.003 s (-42.8%; +290.2%) | 0.396 s (-77.4%; +54.0%) |
| Testify External | `-race` | 4 | 0.324 s | 1.797 s | 1.033 s (-42.5%; +218.8%) | 0.502 s (-72.1%; +54.9%) |
| Testify External | `-race` | 32 | 0.343 s | 1.908 s | 1.097 s (-42.5%; +220.3%) | 0.537 s (-71.8%; +56.8%) |
| Testify External | `-cover` | 4 | 0.254 s | 1.772 s | 0.938 s (-47.0%; +269.8%) | 0.381 s (-78.5%; +50.1%) |
| Testify External | `-cover` | 32 | 0.265 s | 1.809 s | 0.972 s (-46.2%; +266.8%) | 0.405 s (-77.6%; +52.6%) |
| Testify External | `-coverpkg=./... -covermode=atomic` | 4 | 0.253 s | 1.751 s | 0.968 s (-44.7%; +282.0%) | 0.382 s (-78.2%; +50.6%) |
| Testify External | `-coverpkg=./... -covermode=atomic` | 32 | 0.279 s | 1.864 s | 1.003 s (-46.2%; +259.0%) | 0.406 s (-78.2%; +45.5%) |
| Testify External | `-race -coverpkg=./... -covermode=atomic` | 4 | 0.340 s | 1.850 s | 1.041 s (-43.7%; +206.1%) | 0.516 s (-72.1%; +51.7%) |
| Testify External | `-race -coverpkg=./... -covermode=atomic` | 32 | 0.360 s | 1.981 s | 1.086 s (-45.2%; +201.6%) | 0.571 s (-71.2%; +58.6%) |
| Testify External | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 0.254 s | 1.756 s | 0.986 s (-43.8%; +287.7%) | 0.384 s (-78.1%; +51.0%) |
| Testify External | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 0.270 s | 1.812 s | 0.960 s (-47.0%; +255.4%) | 0.410 s (-77.4%; +51.8%) |
| Testify External | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 0.347 s | 1.845 s | 1.077 s (-41.6%; +210.7%) | 0.563 s (-69.5%; +62.3%) |
| Testify External | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 0.359 s | 1.945 s | 1.086 s (-44.2%; +202.1%) | 0.554 s (-71.5%; +54.0%) |

## Incremental compilation — reachable test-body edit

| Project | Flags | CPUs | Native | Orchestrion | POC SDK | POC Mini |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| Gin | `none` | 4 | 1.289 s | 3.064 s | 2.197 s (-28.3%; +70.5%) | 1.355 s (-55.8%; +5.2%) |
| Gin | `none` | 32 | 1.150 s | 2.599 s | 2.108 s (-18.9%; +83.3%) | 1.268 s (-51.2%; +10.3%) |
| Chi | `none` | 4 | 0.419 s | 1.818 s | 1.035 s (-43.0%; +146.9%) | 0.532 s (-70.7%; +26.9%) |
| Chi | `none` | 32 | 0.403 s | 1.953 s | 1.053 s (-46.1%; +161.4%) | 0.529 s (-72.9%; +31.4%) |
| Gin | `-race` | 4 | 2.109 s | 3.718 s | 2.939 s (-21.0%; +39.4%) | 2.293 s (-38.3%; +8.7%) |
| Gin | `-race` | 32 | 2.158 s | 4.475 s | 3.278 s (-26.8%; +51.9%) | 2.438 s (-45.5%; +13.0%) |
| Gin | `-cover` | 4 | 1.424 s | 3.515 s | 2.168 s (-38.3%; +52.3%) | 1.567 s (-55.4%; +10.1%) |
| Gin | `-cover` | 32 | 1.262 s | 3.685 s | 2.111 s (-42.7%; +67.2%) | 1.486 s (-59.7%; +17.7%) |
| Gin | `-coverpkg=./... -covermode=atomic` | 4 | 1.543 s | 3.445 s | 2.157 s (-37.4%; +39.7%) | 1.603 s (-53.5%; +3.8%) |
| Gin | `-coverpkg=./... -covermode=atomic` | 32 | 1.361 s | 3.514 s | 2.325 s (-33.8%; +70.8%) | 1.461 s (-58.4%; +7.3%) |
| Gin | `-race -coverpkg=./... -covermode=atomic` | 4 | 2.458 s | 4.442 s | 3.045 s (-31.4%; +23.9%) | 2.569 s (-42.2%; +4.5%) |
| Gin | `-race -coverpkg=./... -covermode=atomic` | 32 | 2.529 s | 4.310 s | 3.264 s (-24.3%; +29.1%) | 2.759 s (-36.0%; +9.1%) |
| Chi | `-race` | 4 | 0.723 s | 2.576 s | 1.440 s (-44.1%; +99.1%) | 0.863 s (-66.5%; +19.3%) |
| Chi | `-race` | 32 | 0.751 s | 2.221 s | 1.505 s (-32.2%; +100.3%) | 0.891 s (-59.9%; +18.6%) |
| Chi | `-cover` | 4 | 0.469 s | 1.869 s | 1.064 s (-43.1%; +126.7%) | 0.576 s (-69.2%; +22.6%) |
| Chi | `-cover` | 32 | 0.478 s | 1.982 s | 1.093 s (-44.9%; +128.6%) | 0.590 s (-70.2%; +23.4%) |
| Chi | `-coverpkg=./... -covermode=atomic` | 4 | 0.489 s | 1.936 s | 1.109 s (-42.7%; +126.5%) | 0.581 s (-70.0%; +18.7%) |
| Chi | `-coverpkg=./... -covermode=atomic` | 32 | 0.472 s | 1.984 s | 1.066 s (-46.3%; +126.0%) | 0.596 s (-70.0%; +26.3%) |
| Chi | `-race -coverpkg=./... -covermode=atomic` | 4 | 0.820 s | 2.172 s | 1.448 s (-33.3%; +76.6%) | 0.941 s (-56.7%; +14.8%) |
| Chi | `-race -coverpkg=./... -covermode=atomic` | 32 | 0.850 s | 2.317 s | 1.426 s (-38.5%; +67.7%) | 0.978 s (-57.8%; +15.0%) |
| Testify Direct | `none` | 4 | 0.280 s | 1.790 s | 0.945 s (-47.2%; +237.8%) | 0.392 s (-78.1%; +39.9%) |
| Testify Direct | `none` | 32 | 0.294 s | 1.895 s | 1.007 s (-46.9%; +242.9%) | 0.425 s (-77.6%; +44.9%) |
| Testify Direct | `-race` | 4 | 0.424 s | 2.244 s | 1.376 s (-38.7%; +224.6%) | 0.581 s (-74.1%; +37.0%) |
| Testify Direct | `-race` | 32 | 0.430 s | 2.246 s | 1.428 s (-36.4%; +232.3%) | 0.605 s (-73.0%; +40.9%) |
| Testify Direct | `-cover` | 4 | 0.287 s | 1.970 s | 1.207 s (-38.8%; +320.3%) | 0.404 s (-79.5%; +40.9%) |
| Testify Direct | `-cover` | 32 | 0.297 s | 1.948 s | 1.120 s (-42.5%; +277.4%) | 0.427 s (-78.1%; +43.7%) |
| Testify Direct | `-coverpkg=./... -covermode=atomic` | 4 | 0.300 s | 1.877 s | 1.255 s (-33.2%; +318.0%) | 0.406 s (-78.4%; +35.3%) |
| Testify Direct | `-coverpkg=./... -covermode=atomic` | 32 | 0.299 s | 2.026 s | 1.042 s (-48.6%; +248.3%) | 0.429 s (-78.8%; +43.5%) |
| Testify Direct | `-race -coverpkg=./... -covermode=atomic` | 4 | 0.428 s | 1.990 s | 1.178 s (-40.8%; +175.6%) | 0.601 s (-69.8%; +40.5%) |
| Testify Direct | `-race -coverpkg=./... -covermode=atomic` | 32 | 0.446 s | 2.135 s | 1.230 s (-42.4%; +176.0%) | 0.694 s (-67.5%; +55.7%) |
| Testify Direct | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 0.295 s | 1.904 s | 1.045 s (-45.1%; +254.5%) | 0.410 s (-78.5%; +39.0%) |
| Testify Direct | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 0.305 s | 1.925 s | 1.026 s (-46.7%; +236.3%) | 0.434 s (-77.4%; +42.4%) |
| Testify Direct | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 0.439 s | 1.989 s | 1.170 s (-41.2%; +166.6%) | 0.613 s (-69.2%; +39.8%) |
| Testify Direct | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 0.449 s | 2.132 s | 1.219 s (-42.8%; +171.3%) | 0.620 s (-70.9%; +37.9%) |
| Testify External | `none` | 4 | 0.266 s | 1.744 s | 0.861 s (-50.6%; +224.0%) | 0.387 s (-77.8%; +45.7%) |
| Testify External | `none` | 32 | 0.267 s | 1.892 s | 1.026 s (-45.8%; +285.0%) | 0.412 s (-78.2%; +54.5%) |
| Testify External | `-race` | 4 | 0.341 s | 1.872 s | 1.131 s (-39.6%; +232.1%) | 0.500 s (-73.3%; +46.9%) |
| Testify External | `-race` | 32 | 0.362 s | 1.984 s | 1.100 s (-44.5%; +204.2%) | 0.536 s (-73.0%; +48.2%) |
| Testify External | `-cover` | 4 | 0.264 s | 1.865 s | 0.966 s (-48.2%; +266.6%) | 0.392 s (-79.0%; +48.8%) |
| Testify External | `-cover` | 32 | 0.280 s | 1.987 s | 1.024 s (-48.5%; +266.1%) | 0.425 s (-78.6%; +51.9%) |
| Testify External | `-coverpkg=./... -covermode=atomic` | 4 | 0.268 s | 1.787 s | 0.963 s (-46.1%; +259.5%) | 0.405 s (-77.4%; +51.1%) |
| Testify External | `-coverpkg=./... -covermode=atomic` | 32 | 0.282 s | 1.823 s | 1.017 s (-44.2%; +260.1%) | 0.423 s (-76.8%; +49.9%) |
| Testify External | `-race -coverpkg=./... -covermode=atomic` | 4 | 0.355 s | 1.879 s | 1.064 s (-43.4%; +199.8%) | 0.560 s (-70.2%; +57.8%) |
| Testify External | `-race -coverpkg=./... -covermode=atomic` | 32 | 0.372 s | 2.002 s | 1.061 s (-47.0%; +185.7%) | 0.573 s (-71.4%; +54.2%) |
| Testify External | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 0.269 s | 1.799 s | 0.869 s (-51.7%; +223.2%) | 0.397 s (-77.9%; +47.8%) |
| Testify External | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 0.279 s | 1.966 s | 1.020 s (-48.1%; +265.6%) | 0.428 s (-78.2%; +53.3%) |
| Testify External | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 0.354 s | 1.894 s | 1.059 s (-44.1%; +199.1%) | 0.547 s (-71.1%; +54.4%) |
| Testify External | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 0.375 s | 2.007 s | 1.111 s (-44.7%; +196.5%) | 0.578 s (-71.2%; +54.4%) |

## Unused-constant edit — diagnostic

| Project | Flags | CPUs | Native | Orchestrion | POC SDK | POC Mini |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| Gin | `none` | 4 | 0.760 s | 1.192 s | 0.886 s (-25.7%; +16.7%) | 0.833 s (-30.2%; +9.6%) |
| Gin | `none` | 32 | 0.654 s | 1.129 s | 0.819 s (-27.5%; +25.1%) | 0.746 s (-33.9%; +14.0%) |
| Chi | `none` | 4 | 0.309 s | 0.735 s | 0.437 s (-40.5%; +41.5%) | 0.389 s (-47.1%; +25.8%) |
| Chi | `none` | 32 | 0.267 s | 0.713 s | 0.399 s (-44.0%; +49.5%) | 0.344 s (-51.7%; +29.0%) |
| Gin | `-race` | 4 | 1.366 s | 1.768 s | 1.492 s (-15.6%; +9.2%) | 1.457 s (-17.6%; +6.7%) |
| Gin | `-race` | 32 | 1.408 s | 1.854 s | 1.568 s (-15.4%; +11.4%) | 1.495 s (-19.4%; +6.2%) |
| Gin | `-cover` | 4 | 0.938 s | 1.555 s | 1.068 s (-31.3%; +13.8%) | 1.018 s (-34.6%; +8.5%) |
| Gin | `-cover` | 32 | 0.796 s | 1.486 s | 0.978 s (-34.2%; +22.9%) | 0.909 s (-38.8%; +14.2%) |
| Gin | `-coverpkg=./... -covermode=atomic` | 4 | 0.993 s | 1.600 s | 1.129 s (-29.4%; +13.6%) | 1.064 s (-33.5%; +7.1%) |
| Gin | `-coverpkg=./... -covermode=atomic` | 32 | 0.867 s | 1.495 s | 1.023 s (-31.5%; +18.0%) | 0.972 s (-35.0%; +12.1%) |
| Gin | `-race -coverpkg=./... -covermode=atomic` | 4 | 1.690 s | 2.260 s | 1.842 s (-18.5%; +9.0%) | 1.756 s (-22.3%; +3.9%) |
| Gin | `-race -coverpkg=./... -covermode=atomic` | 32 | 1.702 s | 2.366 s | 1.855 s (-21.6%; +9.0%) | 1.801 s (-23.9%; +5.8%) |
| Chi | `-race` | 4 | 0.492 s | 0.886 s | 0.610 s (-31.1%; +24.0%) | 0.551 s (-37.8%; +12.0%) |
| Chi | `-race` | 32 | 0.511 s | 0.952 s | 0.644 s (-32.3%; +26.1%) | 0.581 s (-39.0%; +13.6%) |
| Chi | `-cover` | 4 | 0.319 s | 0.766 s | 0.433 s (-43.4%; +35.7%) | 0.383 s (-49.9%; +20.0%) |
| Chi | `-cover` | 32 | 0.307 s | 0.803 s | 0.440 s (-45.2%; +43.2%) | 0.377 s (-53.1%; +22.7%) |
| Chi | `-coverpkg=./... -covermode=atomic` | 4 | 0.333 s | 0.744 s | 0.445 s (-40.2%; +33.8%) | 0.394 s (-47.0%; +18.6%) |
| Chi | `-coverpkg=./... -covermode=atomic` | 32 | 0.312 s | 0.777 s | 0.463 s (-40.4%; +48.1%) | 0.402 s (-48.2%; +28.9%) |
| Chi | `-race -coverpkg=./... -covermode=atomic` | 4 | 0.575 s | 0.998 s | 0.713 s (-28.5%; +24.1%) | 1.530 s (+53.3%; +166.2%) |
| Chi | `-race -coverpkg=./... -covermode=atomic` | 32 | 0.607 s | 1.063 s | 0.736 s (-30.7%; +21.3%) | 0.681 s (-35.9%; +12.3%) |
| Testify Direct | `none` | 4 | 0.101 s | 0.577 s | 0.206 s (-64.4%; +104.0%) | 0.154 s (-73.4%; +52.7%) |
| Testify Direct | `none` | 32 | 0.100 s | 0.624 s | 0.212 s (-66.1%; +111.3%) | 0.156 s (-74.9%; +56.2%) |
| Testify Direct | `-race` | 4 | 0.145 s | 0.679 s | 0.246 s (-63.7%; +70.5%) | 0.197 s (-71.1%; +36.0%) |
| Testify Direct | `-race` | 32 | 0.153 s | 0.678 s | 0.273 s (-59.7%; +78.4%) | 0.211 s (-68.8%; +38.0%) |
| Testify Direct | `-cover` | 4 | 0.109 s | 0.637 s | 0.218 s (-65.8%; +99.3%) | 0.164 s (-74.3%; +50.1%) |
| Testify Direct | `-cover` | 32 | 0.105 s | 0.703 s | 0.224 s (-68.1%; +114.0%) | 0.169 s (-76.0%; +60.9%) |
| Testify Direct | `-coverpkg=./... -covermode=atomic` | 4 | 0.107 s | 0.597 s | 0.225 s (-62.4%; +110.3%) | 0.164 s (-72.6%; +53.2%) |
| Testify Direct | `-coverpkg=./... -covermode=atomic` | 32 | 0.107 s | 1.018 s | 0.231 s (-77.3%; +115.1%) | 0.167 s (-83.6%; +55.7%) |
| Testify Direct | `-race -coverpkg=./... -covermode=atomic` | 4 | 0.152 s | 0.690 s | 0.269 s (-61.1%; +76.4%) | 0.205 s (-70.3%; +34.7%) |
| Testify Direct | `-race -coverpkg=./... -covermode=atomic` | 32 | 0.163 s | 0.707 s | 0.285 s (-59.7%; +74.9%) | 0.224 s (-68.3%; +37.6%) |
| Testify Direct | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 0.108 s | 0.578 s | 0.223 s (-61.4%; +106.3%) | 0.159 s (-72.5%; +47.0%) |
| Testify Direct | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 0.104 s | 0.625 s | 0.228 s (-63.6%; +118.6%) | 0.164 s (-73.8%; +57.4%) |
| Testify Direct | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 0.150 s | 0.617 s | 0.263 s (-57.3%; +75.5%) | 0.203 s (-67.1%; +35.3%) |
| Testify Direct | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 0.159 s | 0.695 s | 0.285 s (-59.0%; +79.0%) | 0.223 s (-68.0%; +39.9%) |
| Testify External | `none` | 4 | 0.079 s | 0.551 s | 0.194 s (-64.8%; +144.6%) | 0.141 s (-74.3%; +78.6%) |
| Testify External | `none` | 32 | 0.078 s | 0.604 s | 0.207 s (-65.6%; +165.7%) | 0.154 s (-74.4%; +98.0%) |
| Testify External | `-race` | 4 | 0.080 s | 0.547 s | 0.195 s (-64.4%; +143.2%) | 0.141 s (-74.3%; +75.6%) |
| Testify External | `-race` | 32 | 0.083 s | 0.603 s | 0.219 s (-63.6%; +164.6%) | 0.158 s (-73.8%; +90.5%) |
| Testify External | `-cover` | 4 | 0.085 s | 0.617 s | 0.204 s (-66.9%; +140.5%) | 0.148 s (-76.1%; +74.0%) |
| Testify External | `-cover` | 32 | 0.085 s | 0.684 s | 0.217 s (-68.2%; +155.8%) | 0.160 s (-76.6%; +88.7%) |
| Testify External | `-coverpkg=./... -covermode=atomic` | 4 | 0.087 s | 0.576 s | 0.210 s (-63.6%; +140.2%) | 0.153 s (-73.4%; +75.8%) |
| Testify External | `-coverpkg=./... -covermode=atomic` | 32 | 0.089 s | 0.634 s | 0.222 s (-65.0%; +148.3%) | 0.163 s (-74.3%; +82.3%) |
| Testify External | `-race -coverpkg=./... -covermode=atomic` | 4 | 0.087 s | 0.576 s | 0.209 s (-63.6%; +141.6%) | 0.151 s (-73.9%; +73.6%) |
| Testify External | `-race -coverpkg=./... -covermode=atomic` | 32 | 0.091 s | 0.634 s | 0.223 s (-64.8%; +145.0%) | 0.164 s (-74.2%; +79.7%) |
| Testify External | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 0.085 s | 0.556 s | 0.213 s (-61.8%; +148.9%) | 0.151 s (-72.8%; +76.9%) |
| Testify External | `-coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 0.084 s | 0.611 s | 0.225 s (-63.2%; +166.1%) | 0.158 s (-74.1%; +87.2%) |
| Testify External | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 4 | 0.085 s | 0.556 s | 0.217 s (-61.0%; +154.5%) | 0.148 s (-73.4%; +73.7%) |
| Testify External | `-race -coverpkg=testing,github.com/stretchr/testify/suite -covermode=atomic` | 32 | 0.087 s | 0.611 s | 0.225 s (-63.2%; +158.0%) | 0.164 s (-73.2%; +87.9%) |

## Noisy diagnostic cell

The unused-constant diagnostic for Chi, four CPUs, `-race -coverpkg=./... -covermode=atomic` reverses the wall comparison: Mini median 1.530 s, Orchestrion 0.998 s (`+53.3%` vs Orchestrion). Mini observations span 0.631–4.541 s with only three repetitions. Median CPU is lower for Mini (0.942 vs 1.747 CPU-seconds). Both measurements remain reported; this cell does not establish a build-time regression.

## Evidence

The two series contain **6,224 measured observations** and **889 separate controls/trace checks**: 7,113 completed driver commands. All 432 retained test binaries have checked instrumentation hooks and no DWARF sections. The final source/tool hashes match the frozen input manifests. No repository source, index, commit or remote state was changed.

- [Every individual timing, combined CSV](observations.csv).
- [Baseline summary, raw ranges, CPU, memory and uncertainty](baseline/summary.json).
- [Expanded summary, raw ranges, CPU, memory and uncertainty](expanded/summary.json).
- [Baseline verification](baseline/verification.json) and [expanded verification](expanded/verification.json).
- [Baseline final binaries](baseline/final-binaries.json) and [expanded final binaries](expanded/final-binaries.json).
- [Native convergence control](convergence.json).
- [Interrupted-at-limit attempt](censored/testify-direct--cover-testing-cpu4-warm-link-5-orchestrion.interruption.json) and [scope boundary proof](interruption-boundary.json).

The CSV preserves every completed command and timing. [Input provenance](manifest.json), final binary hash manifests and per-cell tool traces are checked in. Full stdout/stderr logs, binaries and build caches remain in the original local artifact directory; they are not included in Git. Absolute paths in the evidence identify that historical run and are not required for table generation.

These are measurements of the recorded historical revision. Previous published runs used a different SDK pin and Go toolchain, so differences against those historical tables cannot be attributed solely to POC optimizations.
