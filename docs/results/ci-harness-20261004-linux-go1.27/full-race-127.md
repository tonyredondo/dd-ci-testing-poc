# CI Visibility parity evidence

Testify parity passes for local and external-module callers. Complete product parity remains unverified.

SDK base: `96aedb31048c07e29e7a20a4333dc3b8d289c52d` (`v2.12.0-dev.3.0.20261002145613-96aedb31048c`).
Runner: `linux/amd64`, `go1.27.0-X:nodwarf5`.

Counts below are sessions/modules/suites/tests/spans. Every matrix row
also compares CI attributes, exit status and hierarchy references;
coverage and side payloads are checked when that feature is selected.

Matrix: 65 passing scenarios; aggregate SDK = Mini `65/62/63/142/0`.

Child walltimes are seconds for one observation per variant per case.
They include startup, settings, execution/retries and shutdown/flush of
the prebuilt binary. Compilation and harness comparison are excluded.
Mini versus SDK is `(Mini / SDK - 1) * 100`; a negative value is shorter.
These functional fixtures have no timing pass/fail threshold.

Execution order: mini then sdk.

| Entire 65-scenario block | SDK wall (s) | Mini wall (s) | Mini vs SDK |
| --- | ---: | ---: | ---: |
| Continuous walltime | 4.245022 | 3.767365 | -11.3% |

Measured scope: 65-scenario continuous block: receiver setup, process execution and shutdown/flush; excludes compilation and comparisons.

| Scenario | Features | SDK events | Mini events | SDK wall (s) | Mini wall (s) | Mini vs SDK | Result |
| --- | --- | --- | --- | ---: | ---: | ---: | --- |
| pass | lifecycle | 1/1/1/1/0 | 1/1/1/1/0 | 0.048234 | 0.044032 | -8.7% | Passed |
| skip-reasons | skip | 1/1/1/3/0 | 1/1/1/3/0 | 0.048049 | 0.044500 | -7.4% | Passed |
| nested-cleanup-context | subtests, cleanup, context | 1/1/1/6/0 | 1/1/1/6/0 | 0.048805 | 0.044169 | -9.5% | Passed |
| parallel-count-shuffle | parallel, count, shuffle | 1/1/1/18/0 | 1/1/1/18/0 | 0.050951 | 0.045342 | -11.0% | Passed |
| multiple-suites | hierarchy, source | 1/1/2/2/0 | 1/1/2/2/0 | 0.047775 | 0.044247 | -7.4% | Passed |
| empty-selection | lifecycle, empty | 1/0/0/0/0 | 1/0/0/0/0 | 0.047031 | 0.042826 | -8.9% | Passed |
| list | lifecycle, list | 1/0/0/0/0 | 1/0/0/0/0 | 0.047255 | 0.041983 | -11.2% | Passed |
| examples-fuzz-seeds | examples, fuzz-seeds | 1/0/0/0/0 | 1/0/0/0/0 | 0.047301 | 0.043558 | -7.9% | Passed |
| settings-unavailable | settings, failure | 1/1/1/1/0 | 1/1/1/1/0 | 0.048433 | 0.043390 | -10.4% | Passed |
| tags-version-ci | metadata, version, ci | 1/1/1/1/0 | 1/1/1/1/0 | 0.047778 | 0.044730 | -6.4% | Passed |
| coverage-pass-skip | coverage, skip | 1/1/1/2/0 | 1/1/1/2/0 | 0.049952 | 0.044594 | -10.7% | Passed |
| coverage-parallel-shuffle | coverage, parallel, shuffle | 1/1/1/10/0 | 1/1/1/10/0 | 0.051074 | 0.047243 | -7.5% | Passed |
| coverage-report | coverage-report | 1/1/1/1/0 | 1/1/1/1/0 | 0.047877 | 0.043564 | -9.0% | Passed |
| logs | logs | 1/1/1/1/0 | 1/1/1/1/0 | 0.047580 | 0.043937 | -7.7% | Passed |
| error-Fail | errors | 1/1/1/2/0 | 1/1/1/2/0 | 0.047690 | 0.043864 | -8.0% | Passed |
| error-FailNow | errors | 1/1/1/2/0 | 1/1/1/2/0 | 0.047910 | 0.044562 | -7.0% | Passed |
| error-Error | errors | 1/1/1/2/0 | 1/1/1/2/0 | 0.048148 | 0.043293 | -10.1% | Passed |
| error-Errorf | errors | 1/1/1/2/0 | 1/1/1/2/0 | 0.048195 | 0.043273 | -10.2% | Passed |
| error-Fatal | errors | 1/1/1/2/0 | 1/1/1/2/0 | 0.048413 | 0.044461 | -8.2% | Passed |
| error-Fatalf | errors | 1/1/1/2/0 | 1/1/1/2/0 | 0.048771 | 0.044571 | -8.6% | Passed |
| atr-recovery-in_process | atr, in_process | 1/1/1/2/0 | 1/1/1/2/0 | 0.049054 | 0.044427 | -9.4% | Passed |
| atr-exhaustion-in_process | atr, in_process, failure | 1/1/1/3/0 | 1/1/1/3/0 | 0.048231 | 0.045165 | -6.4% | Passed |
| efd-new-in_process | efd, in_process | 1/1/1/3/0 | 1/1/1/3/0 | 0.048897 | 0.044859 | -8.3% | Passed |
| efd-atr-in_process | efd, atr, in_process | 1/1/1/3/0 | 1/1/1/3/0 | 0.048928 | 0.044753 | -8.5% | Passed |
| attempt-to-fix-in_process | attempt-to-fix, in_process | 1/1/1/2/0 | 1/1/1/2/0 | 0.048975 | 0.044928 | -8.3% | Passed |
| disabled-atf-quarantine-in_process | disabled, attempt-to-fix, quarantine, in_process | 1/1/1/2/0 | 1/1/1/2/0 | 0.047850 | 0.043978 | -8.1% | Passed |
| atr-coverage-in_process | atr, coverage, in_process | 1/1/1/2/0 | 1/1/1/2/0 | 0.048738 | 0.044537 | -8.6% | Passed |
| atr-recovery-process | atr, process | 1/1/1/2/0 | 1/1/1/2/0 | 0.059705 | 0.055086 | -7.7% | Passed |
| atr-exhaustion-process | atr, process, failure | 1/1/1/3/0 | 1/1/1/3/0 | 0.071413 | 0.064621 | -9.5% | Passed |
| efd-new-process | efd, process | 1/1/1/3/0 | 1/1/1/3/0 | 0.072044 | 0.062948 | -12.6% | Passed |
| efd-atr-process | efd, atr, process | 1/1/1/3/0 | 1/1/1/3/0 | 0.070770 | 0.064800 | -8.4% | Passed |
| attempt-to-fix-process | attempt-to-fix, process | 1/1/1/2/0 | 1/1/1/2/0 | 0.058944 | 0.053405 | -9.4% | Passed |
| disabled-atf-quarantine-process | disabled, attempt-to-fix, quarantine, process | 1/1/1/2/0 | 1/1/1/2/0 | 0.060230 | 0.052953 | -12.1% | Passed |
| atr-coverage-process | atr, coverage, process | 1/1/1/2/0 | 1/1/1/2/0 | 0.060406 | 0.055654 | -7.9% | Passed |
| atr-env-enables-remote-off | atr, override | 1/1/1/2/0 | 1/1/1/2/0 | 0.048381 | 0.044339 | -8.4% | Passed |
| atr-env-disables-remote-on | atr, override | 1/1/1/1/0 | 1/1/1/1/0 | 0.046847 | 0.044564 | -4.9% | Passed |
| efd-known | efd, known-tests | 1/1/1/1/0 | 1/1/1/1/0 | 0.047951 | 0.043832 | -8.6% | Passed |
| efd-cap-zero | efd, override | 1/1/1/1/0 | 1/1/1/1/0 | 0.048073 | 0.044205 | -8.0% | Passed |
| itr | itr | 1/1/1/1/0 | 1/1/1/1/0 | 0.048631 | 0.044481 | -8.5% | Passed |
| itr-atr-efd | itr, atr, efd | 1/1/1/1/0 | 1/1/1/1/0 | 0.049140 | 0.044446 | -9.6% | Passed |
| itr-disabled-quarantine | itr, disabled, quarantine | 1/1/1/1/0 | 1/1/1/1/0 | 0.049597 | 0.044129 | -11.0% | Passed |
| itr-coverage | itr, coverage | 1/1/1/2/0 | 1/1/1/2/0 | 0.049500 | 0.045080 | -8.9% | Passed |
| disabled-atr-efd | disabled, atr, efd | 1/1/1/1/0 | 1/1/1/1/0 | 0.048397 | 0.044555 | -7.9% | Passed |
| quarantine-atr-efd | quarantine, atr, efd | 1/1/1/1/0 | 1/1/1/1/0 | 0.048316 | 0.043928 | -9.1% | Passed |
| subtest-disabled | subtests, disabled | 1/1/1/2/0 | 1/1/1/2/0 | 0.048895 | 0.044075 | -9.9% | Passed |
| subtest-gate-off | subtests, disabled, override | 1/1/1/2/0 | 1/1/1/2/0 | 0.048040 | 0.045621 | -5.0% | Passed |
| subtest-quarantine | subtests, quarantine | 1/1/1/2/0 | 1/1/1/2/0 | 0.048864 | 0.044114 | -9.7% | Passed |
| management-env-off | disabled, override | 1/1/1/1/0 | 1/1/1/1/0 | 0.049976 | 0.044561 | -10.8% | Passed |
| benchmarks | benchmarks | 1/1/1/2/0 | 1/1/1/2/0 | 0.052171 | 0.047065 | -9.8% | Passed |
| impacted-known-efd | impacted-tests, known-tests, efd | 1/1/1/3/0 | 1/1/1/3/0 | 0.048349 | 0.044784 | -7.4% | Passed |
| impacted-env-off | impacted-tests, override | 1/1/1/1/0 | 1/1/1/1/0 | 0.048727 | 0.044701 | -8.3% | Passed |
| git-upload-require-git | git-upload, settings | 1/1/1/1/0 | 1/1/1/1/0 | 0.406901 | 0.216756 | -46.7% | Passed |
| logs-atr | logs, atr | 1/1/1/2/0 | 1/1/1/2/0 | 0.048103 | 0.044207 | -8.1% | Passed |
| report-itr-coverage | coverage-report, itr, coverage | 1/1/1/2/0 | 1/1/1/2/0 | 0.049170 | 0.046832 | -4.8% | Passed |
| efd-faulty-session | efd, faulty-session | 1/1/1/1/0 | 1/1/1/1/0 | 0.048676 | 0.043454 | -10.7% | Passed |
| itr-missing-line-coverage | itr, coverage, missing-line-coverage | 1/1/1/1/0 | 1/1/1/1/0 | 0.048923 | 0.044541 | -9.0% | Passed |
| itr-unskippable | itr, unskippable | 1/1/1/1/0 | 1/1/1/1/0 | 0.048165 | 0.043591 | -9.5% | Passed |
| itr-impacted-efd-known | itr, impacted-tests, efd, known-tests | 1/1/1/3/0 | 1/1/1/3/0 | 0.049729 | 0.046391 | -6.7% | Passed |
| itr-attempt-to-fix | itr, attempt-to-fix | 1/1/1/2/0 | 1/1/1/2/0 | 0.047935 | 0.043408 | -9.4% | Passed |
| atr-budget-zero | atr, budget | 1/1/1/1/0 | 1/1/1/1/0 | 0.047720 | 0.042918 | -10.1% | Passed |
| efd-parallel-execution | efd, parallel-execution | 1/1/1/3/0 | 1/1/1/3/0 | 0.048955 | 0.045065 | -7.9% | Passed |
| atr-parallel-in_process | atr, parallel, in_process | 1/1/1/2/0 | 1/1/1/2/0 | 0.048169 | 0.044259 | -8.1% | Passed |
| atr-parallel-process | atr, parallel, process | 1/1/1/2/0 | 1/1/1/2/0 | 0.058495 | 0.053429 | -8.7% | Passed |
| agent-coverage-itr | agent, coverage, itr | 1/1/1/2/0 | 1/1/1/2/0 | 0.047843 | 0.045396 | -5.1% | Passed |
| agent-retry | agent, atr | 1/1/1/2/0 | 1/1/1/2/0 | 0.047612 | 0.042576 | -10.6% | Passed |

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
| manual | 0.115764 | 0.012668 | -89.1% | prebuilt binary: startup, settings, execution and shutdown/flush |
| spans | 0.015027 | 0.011034 | -26.6% | prebuilt binary: startup, settings, execution and shutdown/flush |
| packages | 9.138622 | 5.875170 | -35.7% | CLI: preparation, compilation, execution and shutdown/flush |
| fuzz | 0.029375 | 0.020285 | -30.9% | prebuilt binary: startup, settings, execution and shutdown/flush |
| telemetry | 0.015627 | 0.010407 | -33.4% | prebuilt binary: startup, settings, execution and shutdown/flush |
| testify | 1.081001 | 1.058198 | -2.1% | prebuilt binary: startup, settings, execution and shutdown/flush; SDK column uses full Orchestrion reference |
| uds | 0.044969 | 0.040277 | -10.4% | prebuilt binary: startup, settings, execution and shutdown/flush |

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
| pass-skip | 1/1/2/3/0 | 1/1/2/3/0 | 1.081001 | 1.058198 | -2.1% |
| method-filter | 1/1/2/2/0 | 1/1/2/2/0 | 1.077888 | 1.059013 | -1.8% |
| count-shuffle | 1/1/2/4/0 | 1/1/2/4/0 | 1.082946 | 1.061699 | -2.0% |
| nested | 1/1/2/3/0 | 1/1/2/3/0 | 1.078378 | 1.059598 | -1.7% |
| assert-failure | 1/1/2/2/0 | 1/1/2/2/0 | 0.078990 | 0.059701 | -24.4% |
| require-failure | 1/1/2/2/0 | 1/1/2/2/0 | 0.080570 | 0.059934 | -25.6% |
| panic | 1/1/2/2/0 | 1/1/2/2/0 | 0.078916 | 0.058131 | -26.3% |
| lifecycle-stats | 1/1/2/4/0 | 1/1/2/4/0 | 1.077293 | 1.058762 | -1.7% |
| aliases-dot-helpers | 1/1/4/6/0 | 1/1/4/6/0 | 1.090536 | 1.067089 | -2.2% |
| external-module-helper | 1/1/2/2/0 | 1/1/2/2/0 | 1.077940 | 1.058257 | -1.8% |
| internal-package | 1/1/2/2/0 | 1/1/2/2/0 | 1.075797 | 1.056411 | -1.8% |
| custom-and-shadowed-run | 1/1/1/3/0 | 1/1/1/3/0 | 1.074279 | 1.056542 | -1.7% |
| parallel-suites | 1/1/2/4/0 | 1/1/2/4/0 | 1.079054 | 1.060903 | -1.7% |
| coverage-helpers | 1/1/2/2/0 | 1/1/2/2/0 | 1.078839 | 1.061903 | -1.6% |
| coverage | 1/1/2/4/0 | 1/1/2/4/0 | 1.082099 | 1.062817 | -1.8% |
| itr-parent | 1/1/1/1/0 | 1/1/1/1/0 | 1.071979 | 1.056681 | -1.4% |
| itr-method-sdk-limit | 1/1/2/2/0 | 1/1/2/2/0 | 0.077146 | 0.056975 | -26.1% |
| atr-in_process | 1/1/2/4/0 | 1/1/2/4/0 | 1.083318 | 1.063145 | -1.9% |
| atr-coverage-in_process | 1/1/2/4/0 | 1/1/2/4/0 | 1.085001 | 1.064326 | -1.9% |
| efd-in_process | 1/1/2/6/0 | 1/1/2/6/0 | 1.088851 | 1.064276 | -2.3% |
| atr-process | 1/1/2/3/0 | 1/1/2/3/0 | 2.104531 | 2.082795 | -1.0% |
| atr-coverage-process | 1/1/2/3/0 | 1/1/2/3/0 | 2.107827 | 2.086665 | -1.0% |
| efd-process | 1/1/2/4/0 | 1/1/2/4/0 | 3.131300 | 3.100283 | -1.0% |
| disabled | 1/1/2/2/0 | 1/1/2/2/0 | 1.081295 | 1.058191 | -2.1% |
| quarantined | 1/1/2/2/0 | 1/1/2/2/0 | 1.076638 | 1.056645 | -1.9% |
| attempt-to-fix | 1/1/2/2/0 | 1/1/2/2/0 | 0.077669 | 0.058792 | -24.3% |
