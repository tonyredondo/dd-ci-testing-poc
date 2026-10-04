# CI Visibility parity evidence

Testify parity passes for local and external-module callers. Complete product parity remains unverified.

SDK base: `96aedb31048c07e29e7a20a4333dc3b8d289c52d` (`v2.12.0-dev.3.0.20261002145613-96aedb31048c`).
Runner: `linux/amd64`, `go1.26.8`.

Counts below are sessions/modules/suites/tests/spans. Every matrix row
also compares CI attributes, exit status and hierarchy references;
coverage and side payloads are checked when that feature is selected.

Matrix: 65 passing scenarios; aggregate SDK = Mini `65/62/63/142/0`.

Child walltimes are seconds for one observation per variant per case.
They include startup, settings, execution/retries and shutdown/flush of
the prebuilt binary. Compilation and harness comparison are excluded.
Mini versus SDK is `(Mini / SDK - 1) * 100`; a negative value is shorter.
These functional fixtures have no timing pass/fail threshold.

Execution order: sdk then mini.

| Entire 65-scenario block | SDK wall (s) | Mini wall (s) | Mini vs SDK |
| --- | ---: | ---: | ---: |
| Continuous walltime | 3.831027 | 3.632851 | -5.2% |

Measured scope: 65-scenario continuous block: receiver setup, process execution and shutdown/flush; excludes compilation and comparisons.

| Scenario | Features | SDK events | Mini events | SDK wall (s) | Mini wall (s) | Mini vs SDK | Result |
| --- | --- | --- | --- | ---: | ---: | ---: | --- |
| pass | lifecycle | 1/1/1/1/0 | 1/1/1/1/0 | 0.047740 | 0.043083 | -9.8% | Passed |
| skip-reasons | skip | 1/1/1/3/0 | 1/1/1/3/0 | 0.047383 | 0.043888 | -7.4% | Passed |
| nested-cleanup-context | subtests, cleanup, context | 1/1/1/6/0 | 1/1/1/6/0 | 0.046038 | 0.043740 | -5.0% | Passed |
| parallel-count-shuffle | parallel, count, shuffle | 1/1/1/18/0 | 1/1/1/18/0 | 0.048661 | 0.041509 | -14.7% | Passed |
| multiple-suites | hierarchy, source | 1/1/2/2/0 | 1/1/2/2/0 | 0.047634 | 0.042205 | -11.4% | Passed |
| empty-selection | lifecycle, empty | 1/0/0/0/0 | 1/0/0/0/0 | 0.045229 | 0.041503 | -8.2% | Passed |
| list | lifecycle, list | 1/0/0/0/0 | 1/0/0/0/0 | 0.045847 | 0.041811 | -8.8% | Passed |
| examples-fuzz-seeds | examples, fuzz-seeds | 1/0/0/0/0 | 1/0/0/0/0 | 0.045079 | 0.041834 | -7.2% | Passed |
| settings-unavailable | settings, failure | 1/1/1/1/0 | 1/1/1/1/0 | 0.045878 | 0.041527 | -9.5% | Passed |
| tags-version-ci | metadata, version, ci | 1/1/1/1/0 | 1/1/1/1/0 | 0.047026 | 0.042814 | -9.0% | Passed |
| coverage-pass-skip | coverage, skip | 1/1/1/2/0 | 1/1/1/2/0 | 0.048651 | 0.043917 | -9.7% | Passed |
| coverage-parallel-shuffle | coverage, parallel, shuffle | 1/1/1/10/0 | 1/1/1/10/0 | 0.048033 | 0.044422 | -7.5% | Passed |
| coverage-report | coverage-report | 1/1/1/1/0 | 1/1/1/1/0 | 0.046236 | 0.042640 | -7.8% | Passed |
| logs | logs | 1/1/1/1/0 | 1/1/1/1/0 | 0.046928 | 0.042862 | -8.7% | Passed |
| error-Fail | errors | 1/1/1/2/0 | 1/1/1/2/0 | 0.047105 | 0.042322 | -10.2% | Passed |
| error-FailNow | errors | 1/1/1/2/0 | 1/1/1/2/0 | 0.046446 | 0.042949 | -7.5% | Passed |
| error-Error | errors | 1/1/1/2/0 | 1/1/1/2/0 | 0.046695 | 0.043489 | -6.9% | Passed |
| error-Errorf | errors | 1/1/1/2/0 | 1/1/1/2/0 | 0.046847 | 0.042186 | -10.0% | Passed |
| error-Fatal | errors | 1/1/1/2/0 | 1/1/1/2/0 | 0.047442 | 0.042892 | -9.6% | Passed |
| error-Fatalf | errors | 1/1/1/2/0 | 1/1/1/2/0 | 0.047149 | 0.041634 | -11.7% | Passed |
| atr-recovery-in_process | atr, in_process | 1/1/1/2/0 | 1/1/1/2/0 | 0.045903 | 0.041655 | -9.3% | Passed |
| atr-exhaustion-in_process | atr, in_process, failure | 1/1/1/3/0 | 1/1/1/3/0 | 0.044864 | 0.041324 | -7.9% | Passed |
| efd-new-in_process | efd, in_process | 1/1/1/3/0 | 1/1/1/3/0 | 0.048977 | 0.042040 | -14.2% | Passed |
| efd-atr-in_process | efd, atr, in_process | 1/1/1/3/0 | 1/1/1/3/0 | 0.048027 | 0.043525 | -9.4% | Passed |
| attempt-to-fix-in_process | attempt-to-fix, in_process | 1/1/1/2/0 | 1/1/1/2/0 | 0.047105 | 0.042450 | -9.9% | Passed |
| disabled-atf-quarantine-in_process | disabled, attempt-to-fix, quarantine, in_process | 1/1/1/2/0 | 1/1/1/2/0 | 0.046288 | 0.042790 | -7.6% | Passed |
| atr-coverage-in_process | atr, coverage, in_process | 1/1/1/2/0 | 1/1/1/2/0 | 0.048163 | 0.043069 | -10.6% | Passed |
| atr-recovery-process | atr, process | 1/1/1/2/0 | 1/1/1/2/0 | 0.058353 | 0.052395 | -10.2% | Passed |
| atr-exhaustion-process | atr, process, failure | 1/1/1/3/0 | 1/1/1/3/0 | 0.068591 | 0.063281 | -7.7% | Passed |
| efd-new-process | efd, process | 1/1/1/3/0 | 1/1/1/3/0 | 0.070662 | 0.063593 | -10.0% | Passed |
| efd-atr-process | efd, atr, process | 1/1/1/3/0 | 1/1/1/3/0 | 0.070683 | 0.063501 | -10.2% | Passed |
| attempt-to-fix-process | attempt-to-fix, process | 1/1/1/2/0 | 1/1/1/2/0 | 0.059305 | 0.052463 | -11.5% | Passed |
| disabled-atf-quarantine-process | disabled, attempt-to-fix, quarantine, process | 1/1/1/2/0 | 1/1/1/2/0 | 0.059747 | 0.052794 | -11.6% | Passed |
| atr-coverage-process | atr, coverage, process | 1/1/1/2/0 | 1/1/1/2/0 | 0.060167 | 0.054573 | -9.3% | Passed |
| atr-env-enables-remote-off | atr, override | 1/1/1/2/0 | 1/1/1/2/0 | 0.046422 | 0.042228 | -9.0% | Passed |
| atr-env-disables-remote-on | atr, override | 1/1/1/1/0 | 1/1/1/1/0 | 0.046872 | 0.043112 | -8.0% | Passed |
| efd-known | efd, known-tests | 1/1/1/1/0 | 1/1/1/1/0 | 0.047641 | 0.042614 | -10.6% | Passed |
| efd-cap-zero | efd, override | 1/1/1/1/0 | 1/1/1/1/0 | 0.047677 | 0.043400 | -9.0% | Passed |
| itr | itr | 1/1/1/1/0 | 1/1/1/1/0 | 0.047312 | 0.042920 | -9.3% | Passed |
| itr-atr-efd | itr, atr, efd | 1/1/1/1/0 | 1/1/1/1/0 | 0.047334 | 0.043782 | -7.5% | Passed |
| itr-disabled-quarantine | itr, disabled, quarantine | 1/1/1/1/0 | 1/1/1/1/0 | 0.046535 | 0.042860 | -7.9% | Passed |
| itr-coverage | itr, coverage | 1/1/1/2/0 | 1/1/1/2/0 | 0.047883 | 0.043627 | -8.9% | Passed |
| disabled-atr-efd | disabled, atr, efd | 1/1/1/1/0 | 1/1/1/1/0 | 0.047241 | 0.044078 | -6.7% | Passed |
| quarantine-atr-efd | quarantine, atr, efd | 1/1/1/1/0 | 1/1/1/1/0 | 0.046644 | 0.042545 | -8.8% | Passed |
| subtest-disabled | subtests, disabled | 1/1/1/2/0 | 1/1/1/2/0 | 0.046309 | 0.043270 | -6.6% | Passed |
| subtest-gate-off | subtests, disabled, override | 1/1/1/2/0 | 1/1/1/2/0 | 0.045957 | 0.042764 | -6.9% | Passed |
| subtest-quarantine | subtests, quarantine | 1/1/1/2/0 | 1/1/1/2/0 | 0.048125 | 0.042594 | -11.5% | Passed |
| management-env-off | disabled, override | 1/1/1/1/0 | 1/1/1/1/0 | 0.045826 | 0.042104 | -8.1% | Passed |
| benchmarks | benchmarks | 1/1/1/2/0 | 1/1/1/2/0 | 0.048152 | 0.043906 | -8.8% | Passed |
| impacted-known-efd | impacted-tests, known-tests, efd | 1/1/1/3/0 | 1/1/1/3/0 | 0.046678 | 0.042815 | -8.3% | Passed |
| impacted-env-off | impacted-tests, override | 1/1/1/1/0 | 1/1/1/1/0 | 0.045644 | 0.041712 | -8.6% | Passed |
| git-upload-require-git | git-upload, settings | 1/1/1/1/0 | 1/1/1/1/0 | 0.221632 | 0.320725 | +44.7% | Passed |
| logs-atr | logs, atr | 1/1/1/2/0 | 1/1/1/2/0 | 0.047440 | 0.044131 | -7.0% | Passed |
| report-itr-coverage | coverage-report, itr, coverage | 1/1/1/2/0 | 1/1/1/2/0 | 0.048197 | 0.043566 | -9.6% | Passed |
| efd-faulty-session | efd, faulty-session | 1/1/1/1/0 | 1/1/1/1/0 | 0.046078 | 0.041725 | -9.4% | Passed |
| itr-missing-line-coverage | itr, coverage, missing-line-coverage | 1/1/1/1/0 | 1/1/1/1/0 | 0.047207 | 0.043402 | -8.1% | Passed |
| itr-unskippable | itr, unskippable | 1/1/1/1/0 | 1/1/1/1/0 | 0.045848 | 0.042628 | -7.0% | Passed |
| itr-impacted-efd-known | itr, impacted-tests, efd, known-tests | 1/1/1/3/0 | 1/1/1/3/0 | 0.047924 | 0.042580 | -11.2% | Passed |
| itr-attempt-to-fix | itr, attempt-to-fix | 1/1/1/2/0 | 1/1/1/2/0 | 0.047420 | 0.042704 | -9.9% | Passed |
| atr-budget-zero | atr, budget | 1/1/1/1/0 | 1/1/1/1/0 | 0.047189 | 0.042441 | -10.1% | Passed |
| efd-parallel-execution | efd, parallel-execution | 1/1/1/3/0 | 1/1/1/3/0 | 0.049179 | 0.043542 | -11.5% | Passed |
| atr-parallel-in_process | atr, parallel, in_process | 1/1/1/2/0 | 1/1/1/2/0 | 0.048002 | 0.041853 | -12.8% | Passed |
| atr-parallel-process | atr, parallel, process | 1/1/1/2/0 | 1/1/1/2/0 | 0.058329 | 0.053005 | -9.1% | Passed |
| agent-coverage-itr | agent, coverage, itr | 1/1/1/2/0 | 1/1/1/2/0 | 0.048203 | 0.042192 | -12.5% | Passed |
| agent-retry | agent, atr | 1/1/1/2/0 | 1/1/1/2/0 | 0.046924 | 0.041881 | -10.7% | Passed |

## Additional fixtures

| Fixture | SDK events | Mini events | Scope |
| --- | --- | --- | --- |
| manual | 1/2/4/12/1 | 1/2/4/12/1 | internal hierarchy API: repeated lookup/close, statuses, custom tags/metrics, errors, logs and child span |
| spans | 1/1/1/1/2 | 1/1/1/1/2 | root/child spans attached to the active test context; external APM integration uses propagation |
| packages | 2/2/2/2/0 | 2/2/2/2/0 | one session per package binary; packages without tests emit no events |
| fuzz | 2/0/0/0/0 | 2/0/0/0/0 | one-iteration campaign; SDK does not emit seed/case test events |
| uds | 1/1/1/1/0 | 1/1/1/1/0 | Unix Agent: settings and test-cycle delivery over a real socket |

### Additional fixture walltimes

Scopes differ: the packages fixture also prepares and compiles via the CLI.
The manual fixture injects a settings delay; historical Testify records may retain a grouping gap.
Use the scope recorded in each JSON observation when comparing runs.

| Fixture | SDK wall (s) | Mini wall (s) | Mini vs SDK | Measured scope |
| --- | ---: | ---: | ---: | --- |
| manual | 0.113483 | 0.009644 | -91.5% | prebuilt binary: startup, settings, execution and shutdown/flush |
| spans | 0.012874 | 0.009854 | -23.5% | prebuilt binary: startup, settings, execution and shutdown/flush |
| packages | 9.204079 | 5.675119 | -38.3% | CLI: preparation, compilation, execution and shutdown/flush |
| fuzz | 0.028255 | 0.018876 | -33.2% | prebuilt binary: startup, settings, execution and shutdown/flush |
| telemetry | 0.015103 | 0.009258 | -38.7% | prebuilt binary: startup, settings, execution and shutdown/flush |
| testify | 1.079677 | 1.060118 | -1.8% | prebuilt binary: startup, settings, execution and shutdown/flush; SDK column uses full Orchestrion reference |
| uds | 0.044186 | 0.039519 | -10.6% | prebuilt binary: startup, settings, execution and shutdown/flush |

CI telemetry: semantic count/rate metrics match; request counts are
checked against each sender's actual HTTP requests. Batch sizes and
timings may differ. Distributions are outside this counter fixture.

Testify: full SDK `1/1/2/3/0`, POC SDK `1/1/2/3/0`, Mini `1/1/2/3/0`.
Testify: 26 passing scenarios; external-module callers covered.

This is loopback protocol evidence. Real intake/UI acceptance, an actual
Bazel toolchain run and an external APM shim are unverified. Read
`docs/ci-parity.md` for the exclusions and remaining feature coverage.

## Testify combinations

| Scenario | SDK events | Mini events | SDK wall (s) | Mini wall (s) | Mini vs SDK |
| --- | --- | --- | ---: | ---: | ---: |
| pass-skip | 1/1/2/3/0 | 1/1/2/3/0 | 1.079677 | 1.060118 | -1.8% |
| method-filter | 1/1/2/2/0 | 1/1/2/2/0 | 1.076392 | 1.058382 | -1.7% |
| count-shuffle | 1/1/2/4/0 | 1/1/2/4/0 | 1.079466 | 1.061043 | -1.7% |
| nested | 1/1/2/3/0 | 1/1/2/3/0 | 1.077954 | 1.058551 | -1.8% |
| assert-failure | 1/1/2/2/0 | 1/1/2/2/0 | 0.077571 | 0.057682 | -25.6% |
| require-failure | 1/1/2/2/0 | 1/1/2/2/0 | 0.077896 | 0.058060 | -25.5% |
| panic | 1/1/2/2/0 | 1/1/2/2/0 | 0.076983 | 0.057967 | -24.7% |
| lifecycle-stats | 1/1/2/4/0 | 1/1/2/4/0 | 1.079888 | 1.057837 | -2.0% |
| aliases-dot-helpers | 1/1/4/6/0 | 1/1/4/6/0 | 1.087183 | 1.064017 | -2.1% |
| external-module-helper | 1/1/2/2/0 | 1/1/2/2/0 | 1.078326 | 1.059476 | -1.7% |
| internal-package | 1/1/2/2/0 | 1/1/2/2/0 | 1.074962 | 1.057712 | -1.6% |
| custom-and-shadowed-run | 1/1/1/3/0 | 1/1/1/3/0 | 1.073678 | 1.056242 | -1.6% |
| parallel-suites | 1/1/2/4/0 | 1/1/2/4/0 | 1.080732 | 1.060243 | -1.9% |
| coverage-helpers | 1/1/2/2/0 | 1/1/2/2/0 | 1.079467 | 1.059785 | -1.8% |
| coverage | 1/1/2/4/0 | 1/1/2/4/0 | 1.079655 | 1.061256 | -1.7% |
| itr-parent | 1/1/1/1/0 | 1/1/1/1/0 | 1.072319 | 1.055867 | -1.5% |
| itr-method-sdk-limit | 1/1/2/2/0 | 1/1/2/2/0 | 0.077268 | 0.056836 | -26.4% |
| atr-in_process | 1/1/2/4/0 | 1/1/2/4/0 | 1.081563 | 1.062762 | -1.7% |
| atr-coverage-in_process | 1/1/2/4/0 | 1/1/2/4/0 | 1.082931 | 1.064400 | -1.7% |
| efd-in_process | 1/1/2/6/0 | 1/1/2/6/0 | 1.085952 | 1.063964 | -2.0% |
| atr-process | 1/1/2/3/0 | 1/1/2/3/0 | 2.105681 | 2.082189 | -1.1% |
| atr-coverage-process | 1/1/2/3/0 | 1/1/2/3/0 | 2.106591 | 2.084271 | -1.1% |
| efd-process | 1/1/2/4/0 | 1/1/2/4/0 | 3.132110 | 3.101413 | -1.0% |
| disabled | 1/1/2/2/0 | 1/1/2/2/0 | 1.077317 | 1.058230 | -1.8% |
| quarantined | 1/1/2/2/0 | 1/1/2/2/0 | 1.075799 | 1.058617 | -1.6% |
| attempt-to-fix | 1/1/2/2/0 | 1/1/2/2/0 | 0.077528 | 0.057156 | -26.3% |
