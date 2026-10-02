# Repeated CI parity execution

Runner: `linux/amd64`, `go1.27.1`.
SDK: `96aedb31048c07e29e7a20a4333dc3b8d289c52d`; instrumentation: `orchestrion`.

6 measured rounds, with equal numbers of SDK-first and Mini-first rounds.
Every round compares all 65 matrix scenarios and all additional fixtures.
The primary measurement is one continuous 65-scenario block per variant,
including receiver setup, child startup, settings, execution/retries and shutdown/flush.
Compilation and the differential comparisons are outside that block.
The seven additional fixtures have separate raw timing records and are not included in the block.

| Scope | SDK median (s) | Mini median (s) | Mini vs SDK |
| --- | ---: | ---: | ---: |
| Entire matrix execution | 1.776579 | 1.548271 | -12.9% |

| Variant | Mean (s) | Min (s) | Max (s) | CV |
| --- | ---: | ---: | ---: | ---: |
| sdk | 1.814102 | 1.760366 | 2.021734 | 5.14% |
| mini | 1.556968 | 1.524502 | 1.626494 | 2.23% |

CV is the standard deviation divided by the mean; lower means less spread.
Negative Mini-versus-SDK percentages indicate shorter duration.
Median of the paired per-round differences: -12.9%.

| Round | Order | SDK block (s) | Mini block (s) | Mini vs SDK |
| --- | --- | ---: | ---: | ---: |
| 1 | sdk then mini | 1.780028 | 1.525107 | -14.3% |
| 2 | mini then sdk | 1.773130 | 1.553498 | -12.4% |
| 3 | sdk then mini | 1.762912 | 1.543044 | -12.5% |
| 4 | mini then sdk | 2.021734 | 1.569163 | -22.4% |
| 5 | sdk then mini | 1.760366 | 1.524502 | -13.4% |
| 6 | mini then sdk | 1.786440 | 1.626494 | -9.0% |

Individual reports retain event counts, CI-attribute comparisons, case durations
and the known Testify gap. Compilation time and per-fixture clocks are not interchangeable
with the continuous execution clock. These are local loopback results, not real-intake measurements.
