# CODEOWNERS CPU and allocation measurements

These benchmarks measure the parser and matcher in `codeownership`. They
compare the initial .NET port with the optimized implementation in this change.
The baseline is neither the older parser on `main` nor a .NET performance run.
They measure elapsed nanoseconds per operation and allocations; hardware cycle
counters were not available on this host.

The test executables were built with Go 1.27.1 on Linux amd64, on an AMD Ryzen
9 5950X. Each variant ran five times with `GOMAXPROCS=1` and `4`, for 200 ms
per benchmark. Their order alternated within each pair. No other build or test
from this task ran during the final measurements. Tables show medians; the
[raw samples](results/codeownership-20261007.json) retain every repetition,
source hashes and executable hashes.

`BenchmarkParse` generates 50 or 2,000 rooted package rules with two owners per
rule. `BenchmarkLookup` queries the last rule, the first rule and a missing
path in the 2,000-rule input. The GitLab input has one section and no exclusions.
The mixed-wildcard benchmark uses three rules. The union benchmark has four
matching sections; it distinguishes additional owners from duplicate-only
sections. Setup and parsing are outside lookup measurements.

Time is in **microseconds per operation**. Bytes are cumulative allocations
per operation, not resident memory. Each cell shows `baseline → optimized`.

## 1 CPU

| Operation | Time (µs/op) | Time change | Bytes/op | Allocations/op |
| --- | ---: | ---: | ---: | ---: |
| Parse GitHub, 50 rules | 87.829 → 52.976 | -39.68% | 113,923 → 60,742 | 1,492 → 562 |
| Parse GitHub, 2,000 rules | 4204.859 → 2244.348 | -46.62% | 4,646,969 → 2,537,668 | 59,999 → 22,020 |
| Parse GitLab, 50 rules | 108.640 → 57.808 | -46.79% | 127,718 → 66,075 | 1,939 → 619 |
| Parse GitLab, 2,000 rules | 5244.170 → 2370.397 | -54.80% | 5,236,136 → 2,775,188 | 78,002 → 24,033 |
| GitHub: winning last rule | 0.080 → 0.018 | -77.98% | 0 → 0 | 0 → 0 |
| GitHub: winning first rule | 130.690 → 10.447 | -92.01% | 0 → 0 | 0 → 0 |
| GitHub: no match | 27.062 → 6.475 | -76.07% | 0 → 0 | 0 → 0 |
| GitLab: winning last rule, no exclusions | 136.741 → 0.058 | -99.96% | 0 → 0 | 0 → 0 |
| GitLab: winning first rule | 132.299 → 11.370 | -91.41% | 0 → 0 | 0 → 0 |
| GitLab: no match | 27.942 → 7.289 | -73.91% | 0 → 0 | 0 → 0 |
| GitLab: four sections, five distinct owners | 1.087 → 0.875 | -19.48% | 672 → 384 | 10 → 7 |
| GitLab: four sections, same two owners | 0.867 → 0.354 | -59.13% | 432 → 0 | 8 → 0 |
| GitHub: mixed wildcard rules | 0.093 → 0.070 | -25.33% | 0 → 0 | 0 → 0 |
| GitLab: mixed wildcard rules | 0.193 → 0.073 | -61.95% | 0 → 0 | 0 → 0 |

## 4 CPUs

| Operation | Time (µs/op) | Time change | Bytes/op | Allocations/op |
| --- | ---: | ---: | ---: | ---: |
| Parse GitHub, 50 rules | 83.115 → 48.112 | -42.11% | 113,947 → 60,754 | 1,492 → 562 |
| Parse GitHub, 2,000 rules | 3843.143 → 1977.363 | -48.55% | 4,648,431 → 2,538,346 | 60,006 → 22,023 |
| Parse GitLab, 50 rules | 98.909 → 50.322 | -49.12% | 127,742 → 66,086 | 1,939 → 619 |
| Parse GitLab, 2,000 rules | 4537.023 → 2031.527 | -55.22% | 5,237,601 → 2,775,812 | 78,009 → 24,035 |
| GitHub: winning last rule | 0.081 → 0.018 | -77.91% | 0 → 0 | 0 → 0 |
| GitHub: winning first rule | 130.599 → 10.486 | -91.97% | 0 → 0 | 0 → 0 |
| GitHub: no match | 27.132 → 6.488 | -76.09% | 0 → 0 | 0 → 0 |
| GitLab: winning last rule, no exclusions | 137.603 → 0.059 | -99.96% | 0 → 0 | 0 → 0 |
| GitLab: winning first rule | 131.396 → 11.365 | -91.35% | 0 → 0 | 0 → 0 |
| GitLab: no match | 27.796 → 7.334 | -73.61% | 0 → 0 | 0 → 0 |
| GitLab: four sections, five distinct owners | 1.001 → 0.807 | -19.41% | 672 → 384 | 10 → 7 |
| GitLab: four sections, same two owners | 0.810 → 0.352 | -56.50% | 432 → 0 | 8 → 0 |
| GitHub: mixed wildcard rules | 0.093 → 0.068 | -26.55% | 0 → 0 | 0 → 0 |
| GitLab: mixed wildcard rules | 0.194 → 0.072 | -62.94% | 0 → 0 | 0 → 0 |

## What changed

The baseline allocation profile put most parsing bytes in compiled segment
arrays and the two glob compilations. CPU profiles also identified UTF-16
conversion and repeated segment matching. The implementation now:

- Compiles each segment once and shares its immutable tokens between file
  and directory targets. Their globstar requirements stay separate.
- Compares ASCII literals and rooted literal prefixes directly. Prefix checks
  include the separator boundary; wildcards, escapes and Unicode retain the
  UTF-16 matcher and its per-segment work bound.
- Uses stack storage for short UTF-16 conversions, keeps valid owner substrings,
  and avoids temporary role builders and unchanged-key copies.
- Stops a GitLab section at its winning rule when it contains no exclusions.
  A section with exclusions retains the scan needed for their sticky behavior.
- Allocates an owner union only when a section adds an owner. Lists below 16
  use a linear search; larger lists build an index to avoid quadratic work.

Literal prefixes add fields to each compiled rule. Their storage cost is included
in the tables: parsing still allocates fewer bytes than the baseline, while
first-rule queries and misses avoid repeated segment traversal.

The large improvement for a winning last GitLab rule is specific to the input
without exclusions: an entire section scan becomes an early return. Worst-case
first-rule queries and misses still inspect many rules. Actual source ownership
is cached once per file, while parsing occurs once per test process. These
figures therefore do not predict a similar reduction in full test-command time.

The original .NET specification, differential and Unicode corpora pass after
all optimizations. Separate checks cover directory targets, large owner unions,
concurrent readers, HTTP event tags and both delivery modes.

## Reproduce

From the repository root:

```sh
go test -run '^$' -bench 'Benchmark(Parse|Lookup|SectionUnion|Match)$' \
  -benchmem -benchtime=200ms -count=5 -cpu=1,4 \
  ./internal/thirdparty/dd-trace-go/civisibility/codeownership
```

To compare another change, build a test executable from each worktree, then run
the same benchmark command against those executables in alternating order.
Do not run the compatibility suite or another compilation at the same time.
Retain the output and the source/executable hashes. Passing tests from a different
source tree is not correctness evidence for a measured executable.

For CPU and allocation profiles:

```sh
go test -run '^$' -bench 'BenchmarkParse/1/2000$' -benchtime=2s \
  -cpuprofile=codeowners-cpu.pprof -memprofile=codeowners-memory.pprof \
  ./internal/thirdparty/dd-trace-go/civisibility/codeownership
go tool pprof -top codeowners-cpu.pprof
go tool pprof -top -alloc_space codeowners-memory.pprof
```

Keep the default allocation sampling for CPU profiles. Sampling every allocation
adds stack-recording work that can overwhelm the code being profiled. For exact
allocation-site counts, make a separate run with `-memprofilerate=1`.
