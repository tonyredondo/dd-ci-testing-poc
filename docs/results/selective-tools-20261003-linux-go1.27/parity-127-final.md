# CI Visibility parity evidence

Testify parity passes for local and external-module callers. Complete product parity remains unverified.

SDK base: `96aedb31048c07e29e7a20a4333dc3b8d289c52d` (`v2.12.0-dev.3.0.20261002145613-96aedb31048c`).
Runner: `linux/amd64`, `go1.27.1`.

Counts below are sessions/modules/suites/tests/spans. Every matrix row
also compares CI attributes, exit status and hierarchy references;
coverage and side payloads are checked when that feature is selected.

Matrix: 65 passing scenarios; aggregate SDK = Mini `65/62/63/142/0`.

Child walltimes are seconds for one observation per variant per case.
They include startup, settings, execution/retries and shutdown/flush of
the prebuilt binary. Compilation and harness comparison are excluded.
Mini versus SDK is `(Mini / SDK - 1) * 100`; a negative value is shorter.
These functional fixtures have no timing pass/fail threshold.

| Scenario | Features | SDK events | Mini events | SDK wall (s) | Mini wall (s) | Mini vs SDK | Result |
| --- | --- | --- | --- | ---: | ---: | ---: | --- |
| pass | lifecycle | 1/1/1/1/0 | 1/1/1/1/0 | 0.025663 | 0.021035 | -18.0% | Passed |
| skip-reasons | skip | 1/1/1/3/0 | 1/1/1/3/0 | 0.025417 | 0.022006 | -13.4% | Passed |
| nested-cleanup-context | subtests, cleanup, context | 1/1/1/6/0 | 1/1/1/6/0 | 0.025285 | 0.023075 | -8.7% | Passed |
| parallel-count-shuffle | parallel, count, shuffle | 1/1/1/18/0 | 1/1/1/18/0 | 0.027835 | 0.024448 | -12.2% | Passed |
| multiple-suites | hierarchy, source | 1/1/2/2/0 | 1/1/2/2/0 | 0.026268 | 0.022998 | -12.4% | Passed |
| empty-selection | lifecycle, empty | 1/0/0/0/0 | 1/0/0/0/0 | 0.023790 | 0.020230 | -15.0% | Passed |
| list | lifecycle, list | 1/0/0/0/0 | 1/0/0/0/0 | 0.023131 | 0.019392 | -16.2% | Passed |
| examples-fuzz-seeds | examples, fuzz-seeds | 1/0/0/0/0 | 1/0/0/0/0 | 0.023505 | 0.020147 | -14.3% | Passed |
| settings-unavailable | settings, failure | 1/1/1/1/0 | 1/1/1/1/0 | 0.023802 | 0.022135 | -7.0% | Passed |
| tags-version-ci | metadata, version, ci | 1/1/1/1/0 | 1/1/1/1/0 | 0.024634 | 0.020594 | -16.4% | Passed |
| coverage-pass-skip | coverage, skip | 1/1/1/2/0 | 1/1/1/2/0 | 0.025946 | 0.023043 | -11.2% | Passed |
| coverage-parallel-shuffle | coverage, parallel, shuffle | 1/1/1/10/0 | 1/1/1/10/0 | 0.027420 | 0.025152 | -8.3% | Passed |
| coverage-report | coverage-report | 1/1/1/1/0 | 1/1/1/1/0 | 0.025914 | 0.021476 | -17.1% | Passed |
| logs | logs | 1/1/1/1/0 | 1/1/1/1/0 | 0.025037 | 0.022460 | -10.3% | Passed |
| error-Fail | errors | 1/1/1/2/0 | 1/1/1/2/0 | 0.024929 | 0.022429 | -10.0% | Passed |
| error-FailNow | errors | 1/1/1/2/0 | 1/1/1/2/0 | 0.025588 | 0.021815 | -14.7% | Passed |
| error-Error | errors | 1/1/1/2/0 | 1/1/1/2/0 | 0.025062 | 0.021822 | -12.9% | Passed |
| error-Errorf | errors | 1/1/1/2/0 | 1/1/1/2/0 | 0.025304 | 0.022202 | -12.3% | Passed |
| error-Fatal | errors | 1/1/1/2/0 | 1/1/1/2/0 | 0.024733 | 0.022221 | -10.2% | Passed |
| error-Fatalf | errors | 1/1/1/2/0 | 1/1/1/2/0 | 0.025208 | 0.021053 | -16.5% | Passed |
| atr-recovery-in_process | atr, in_process | 1/1/1/2/0 | 1/1/1/2/0 | 0.024179 | 0.022251 | -8.0% | Passed |
| atr-exhaustion-in_process | atr, in_process, failure | 1/1/1/3/0 | 1/1/1/3/0 | 0.025082 | 0.021354 | -14.9% | Passed |
| efd-new-in_process | efd, in_process | 1/1/1/3/0 | 1/1/1/3/0 | 0.025050 | 0.021822 | -12.9% | Passed |
| efd-atr-in_process | efd, atr, in_process | 1/1/1/3/0 | 1/1/1/3/0 | 0.025848 | 0.021832 | -15.5% | Passed |
| attempt-to-fix-in_process | attempt-to-fix, in_process | 1/1/1/2/0 | 1/1/1/2/0 | 0.025591 | 0.023025 | -10.0% | Passed |
| disabled-atf-quarantine-in_process | disabled, attempt-to-fix, quarantine, in_process | 1/1/1/2/0 | 1/1/1/2/0 | 0.025326 | 0.022494 | -11.2% | Passed |
| atr-coverage-in_process | atr, coverage, in_process | 1/1/1/2/0 | 1/1/1/2/0 | 0.025470 | 0.023499 | -7.7% | Passed |
| atr-recovery-process | atr, process | 1/1/1/2/0 | 1/1/1/2/0 | 0.036270 | 0.032677 | -9.9% | Passed |
| atr-exhaustion-process | atr, process, failure | 1/1/1/3/0 | 1/1/1/3/0 | 0.049371 | 0.043316 | -12.3% | Passed |
| efd-new-process | efd, process | 1/1/1/3/0 | 1/1/1/3/0 | 0.047899 | 0.042192 | -11.9% | Passed |
| efd-atr-process | efd, atr, process | 1/1/1/3/0 | 1/1/1/3/0 | 0.049019 | 0.043110 | -12.1% | Passed |
| attempt-to-fix-process | attempt-to-fix, process | 1/1/1/2/0 | 1/1/1/2/0 | 0.037930 | 0.032852 | -13.4% | Passed |
| disabled-atf-quarantine-process | disabled, attempt-to-fix, quarantine, process | 1/1/1/2/0 | 1/1/1/2/0 | 0.036200 | 0.034218 | -5.5% | Passed |
| atr-coverage-process | atr, coverage, process | 1/1/1/2/0 | 1/1/1/2/0 | 0.036908 | 0.032720 | -11.3% | Passed |
| atr-env-enables-remote-off | atr, override | 1/1/1/2/0 | 1/1/1/2/0 | 0.024598 | 0.021229 | -13.7% | Passed |
| atr-env-disables-remote-on | atr, override | 1/1/1/1/0 | 1/1/1/1/0 | 0.024119 | 0.021456 | -11.0% | Passed |
| efd-known | efd, known-tests | 1/1/1/1/0 | 1/1/1/1/0 | 0.025010 | 0.021406 | -14.4% | Passed |
| efd-cap-zero | efd, override | 1/1/1/1/0 | 1/1/1/1/0 | 0.024832 | 0.021937 | -11.7% | Passed |
| itr | itr | 1/1/1/1/0 | 1/1/1/1/0 | 0.023973 | 0.022517 | -6.1% | Passed |
| itr-atr-efd | itr, atr, efd | 1/1/1/1/0 | 1/1/1/1/0 | 0.025001 | 0.021661 | -13.4% | Passed |
| itr-disabled-quarantine | itr, disabled, quarantine | 1/1/1/1/0 | 1/1/1/1/0 | 0.025381 | 0.021942 | -13.5% | Passed |
| itr-coverage | itr, coverage | 1/1/1/2/0 | 1/1/1/2/0 | 0.027116 | 0.022705 | -16.3% | Passed |
| disabled-atr-efd | disabled, atr, efd | 1/1/1/1/0 | 1/1/1/1/0 | 0.025745 | 0.021642 | -15.9% | Passed |
| quarantine-atr-efd | quarantine, atr, efd | 1/1/1/1/0 | 1/1/1/1/0 | 0.024543 | 0.022029 | -10.2% | Passed |
| subtest-disabled | subtests, disabled | 1/1/1/2/0 | 1/1/1/2/0 | 0.025605 | 0.021005 | -18.0% | Passed |
| subtest-gate-off | subtests, disabled, override | 1/1/1/2/0 | 1/1/1/2/0 | 0.024813 | 0.021435 | -13.6% | Passed |
| subtest-quarantine | subtests, quarantine | 1/1/1/2/0 | 1/1/1/2/0 | 0.024036 | 0.021293 | -11.4% | Passed |
| management-env-off | disabled, override | 1/1/1/1/0 | 1/1/1/1/0 | 0.025199 | 0.021876 | -13.2% | Passed |
| benchmarks | benchmarks | 1/1/1/2/0 | 1/1/1/2/0 | 0.027956 | 0.024494 | -12.4% | Passed |
| impacted-known-efd | impacted-tests, known-tests, efd | 1/1/1/3/0 | 1/1/1/3/0 | 0.025461 | 0.022771 | -10.6% | Passed |
| impacted-env-off | impacted-tests, override | 1/1/1/1/0 | 1/1/1/1/0 | 0.024617 | 0.021685 | -11.9% | Passed |
| git-upload-require-git | git-upload, settings | 1/1/1/1/0 | 1/1/1/1/0 | 0.140418 | 0.110491 | -21.3% | Passed |
| logs-atr | logs, atr | 1/1/1/2/0 | 1/1/1/2/0 | 0.025510 | 0.021841 | -14.4% | Passed |
| report-itr-coverage | coverage-report, itr, coverage | 1/1/1/2/0 | 1/1/1/2/0 | 0.026859 | 0.023216 | -13.6% | Passed |
| efd-faulty-session | efd, faulty-session | 1/1/1/1/0 | 1/1/1/1/0 | 0.024943 | 0.022973 | -7.9% | Passed |
| itr-missing-line-coverage | itr, coverage, missing-line-coverage | 1/1/1/1/0 | 1/1/1/1/0 | 0.025889 | 0.022802 | -11.9% | Passed |
| itr-unskippable | itr, unskippable | 1/1/1/1/0 | 1/1/1/1/0 | 0.024634 | 0.022043 | -10.5% | Passed |
| itr-impacted-efd-known | itr, impacted-tests, efd, known-tests | 1/1/1/3/0 | 1/1/1/3/0 | 0.025676 | 0.022555 | -12.2% | Passed |
| itr-attempt-to-fix | itr, attempt-to-fix | 1/1/1/2/0 | 1/1/1/2/0 | 0.025510 | 0.021980 | -13.8% | Passed |
| atr-budget-zero | atr, budget | 1/1/1/1/0 | 1/1/1/1/0 | 0.023622 | 0.022002 | -6.9% | Passed |
| efd-parallel-execution | efd, parallel-execution | 1/1/1/3/0 | 1/1/1/3/0 | 0.025118 | 0.022025 | -12.3% | Passed |
| atr-parallel-in_process | atr, parallel, in_process | 1/1/1/2/0 | 1/1/1/2/0 | 0.024501 | 0.020862 | -14.9% | Passed |
| atr-parallel-process | atr, parallel, process | 1/1/1/2/0 | 1/1/1/2/0 | 0.036284 | 0.033228 | -8.4% | Passed |
| agent-coverage-itr | agent, coverage, itr | 1/1/1/2/0 | 1/1/1/2/0 | 0.025589 | 0.020879 | -18.4% | Passed |
| agent-retry | agent, atr | 1/1/1/2/0 | 1/1/1/2/0 | 0.024809 | 0.020857 | -15.9% | Passed |

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
| manual | 0.115305 | 0.013049 | -88.7% | prebuilt binary: startup, settings, execution and shutdown/flush |
| spans | 0.015129 | 0.011981 | -20.8% | prebuilt binary: startup, settings, execution and shutdown/flush |
| packages | 0.919103 | 0.390376 | -57.5% | CLI: preparation, compilation, execution and shutdown/flush |
| fuzz | 0.029551 | 0.021953 | -25.7% | prebuilt binary: startup, settings, execution and shutdown/flush |
| telemetry | 0.015806 | 0.011027 | -30.2% | prebuilt binary: startup, settings, execution and shutdown/flush |
| testify | 1.056785 | 1.041950 | -1.4% | prebuilt binary: startup, settings, execution and shutdown/flush; SDK column uses full Orchestrion reference |
| uds | 0.022498 | 0.017885 | -20.5% | prebuilt binary: startup, settings, execution and shutdown/flush |

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
| pass-skip | 1/1/2/3/0 | 1/1/2/3/0 | 1.056785 | 1.041950 | -1.4% |
| method-filter | 1/1/2/2/0 | 1/1/2/2/0 | 1.054977 | 1.041759 | -1.3% |
| count-shuffle | 1/1/2/4/0 | 1/1/2/4/0 | 1.060965 | 1.046321 | -1.4% |
| nested | 1/1/2/3/0 | 1/1/2/3/0 | 1.056406 | 1.042272 | -1.3% |
| assert-failure | 1/1/2/2/0 | 1/1/2/2/0 | 0.056266 | 0.040978 | -27.2% |
| require-failure | 1/1/2/2/0 | 1/1/2/2/0 | 0.057579 | 0.042717 | -25.8% |
| panic | 1/1/2/2/0 | 1/1/2/2/0 | 0.057390 | 0.040282 | -29.8% |
| lifecycle-stats | 1/1/2/4/0 | 1/1/2/4/0 | 1.057568 | 1.042457 | -1.4% |
| aliases-dot-helpers | 1/1/4/6/0 | 1/1/4/6/0 | 1.067927 | 1.053575 | -1.3% |
| external-module-helper | 1/1/2/2/0 | 1/1/2/2/0 | 1.055839 | 1.043168 | -1.2% |
| internal-package | 1/1/2/2/0 | 1/1/2/2/0 | 1.052680 | 1.040567 | -1.2% |
| custom-and-shadowed-run | 1/1/1/3/0 | 1/1/1/3/0 | 1.051653 | 1.038932 | -1.2% |
| parallel-suites | 1/1/2/4/0 | 1/1/2/4/0 | 1.057635 | 1.043061 | -1.4% |
| coverage-helpers | 1/1/2/2/0 | 1/1/2/2/0 | 1.058521 | 1.044324 | -1.3% |
| coverage | 1/1/2/4/0 | 1/1/2/4/0 | 1.059607 | 1.045831 | -1.3% |
| itr-parent | 1/1/1/1/0 | 1/1/1/1/0 | 1.050270 | 1.037224 | -1.2% |
| itr-method-sdk-limit | 1/1/2/2/0 | 1/1/2/2/0 | 0.056469 | 0.040161 | -28.9% |
| atr-in_process | 1/1/2/4/0 | 1/1/2/4/0 | 1.062330 | 1.047636 | -1.4% |
| atr-coverage-in_process | 1/1/2/4/0 | 1/1/2/4/0 | 1.063105 | 1.050717 | -1.2% |
| efd-in_process | 1/1/2/6/0 | 1/1/2/6/0 | 1.066311 | 1.051448 | -1.4% |
| atr-process | 1/1/2/3/0 | 1/1/2/3/0 | 2.086406 | 2.066190 | -1.0% |
| atr-coverage-process | 1/1/2/3/0 | 1/1/2/3/0 | 2.085690 | 2.067804 | -0.9% |
| efd-process | 1/1/2/4/0 | 1/1/2/4/0 | 3.111887 | 3.086170 | -0.8% |
| disabled | 1/1/2/2/0 | 1/1/2/2/0 | 1.056066 | 1.042298 | -1.3% |
| quarantined | 1/1/2/2/0 | 1/1/2/2/0 | 1.055493 | 1.042419 | -1.2% |
| attempt-to-fix | 1/1/2/2/0 | 1/1/2/2/0 | 0.056400 | 0.040862 | -27.6% |
