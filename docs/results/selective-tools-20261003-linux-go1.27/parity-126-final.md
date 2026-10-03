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

| Scenario | Features | SDK events | Mini events | SDK wall (s) | Mini wall (s) | Mini vs SDK | Result |
| --- | --- | --- | --- | ---: | ---: | ---: | --- |
| pass | lifecycle | 1/1/1/1/0 | 1/1/1/1/0 | 0.023632 | 0.020752 | -12.2% | Passed |
| skip-reasons | skip | 1/1/1/3/0 | 1/1/1/3/0 | 0.024506 | 0.021058 | -14.1% | Passed |
| nested-cleanup-context | subtests, cleanup, context | 1/1/1/6/0 | 1/1/1/6/0 | 0.023331 | 0.020084 | -13.9% | Passed |
| parallel-count-shuffle | parallel, count, shuffle | 1/1/1/18/0 | 1/1/1/18/0 | 0.025825 | 0.022762 | -11.9% | Passed |
| multiple-suites | hierarchy, source | 1/1/2/2/0 | 1/1/2/2/0 | 0.023851 | 0.020123 | -15.6% | Passed |
| empty-selection | lifecycle, empty | 1/0/0/0/0 | 1/0/0/0/0 | 0.022606 | 0.018447 | -18.4% | Passed |
| list | lifecycle, list | 1/0/0/0/0 | 1/0/0/0/0 | 0.022649 | 0.018891 | -16.6% | Passed |
| examples-fuzz-seeds | examples, fuzz-seeds | 1/0/0/0/0 | 1/0/0/0/0 | 0.021991 | 0.019290 | -12.3% | Passed |
| settings-unavailable | settings, failure | 1/1/1/1/0 | 1/1/1/1/0 | 0.022501 | 0.019713 | -12.4% | Passed |
| tags-version-ci | metadata, version, ci | 1/1/1/1/0 | 1/1/1/1/0 | 0.022571 | 0.019282 | -14.6% | Passed |
| coverage-pass-skip | coverage, skip | 1/1/1/2/0 | 1/1/1/2/0 | 0.024078 | 0.021709 | -9.8% | Passed |
| coverage-parallel-shuffle | coverage, parallel, shuffle | 1/1/1/10/0 | 1/1/1/10/0 | 0.024421 | 0.022899 | -6.2% | Passed |
| coverage-report | coverage-report | 1/1/1/1/0 | 1/1/1/1/0 | 0.023567 | 0.020370 | -13.6% | Passed |
| logs | logs | 1/1/1/1/0 | 1/1/1/1/0 | 0.024137 | 0.019671 | -18.5% | Passed |
| error-Fail | errors | 1/1/1/2/0 | 1/1/1/2/0 | 0.022708 | 0.021524 | -5.2% | Passed |
| error-FailNow | errors | 1/1/1/2/0 | 1/1/1/2/0 | 0.025924 | 0.021096 | -18.6% | Passed |
| error-Error | errors | 1/1/1/2/0 | 1/1/1/2/0 | 0.023912 | 0.021259 | -11.1% | Passed |
| error-Errorf | errors | 1/1/1/2/0 | 1/1/1/2/0 | 0.024519 | 0.021259 | -13.3% | Passed |
| error-Fatal | errors | 1/1/1/2/0 | 1/1/1/2/0 | 0.024725 | 0.021574 | -12.7% | Passed |
| error-Fatalf | errors | 1/1/1/2/0 | 1/1/1/2/0 | 0.023746 | 0.020737 | -12.7% | Passed |
| atr-recovery-in_process | atr, in_process | 1/1/1/2/0 | 1/1/1/2/0 | 0.025817 | 0.021497 | -16.7% | Passed |
| atr-exhaustion-in_process | atr, in_process, failure | 1/1/1/3/0 | 1/1/1/3/0 | 0.024150 | 0.020200 | -16.4% | Passed |
| efd-new-in_process | efd, in_process | 1/1/1/3/0 | 1/1/1/3/0 | 0.024612 | 0.022029 | -10.5% | Passed |
| efd-atr-in_process | efd, atr, in_process | 1/1/1/3/0 | 1/1/1/3/0 | 0.026567 | 0.021886 | -17.6% | Passed |
| attempt-to-fix-in_process | attempt-to-fix, in_process | 1/1/1/2/0 | 1/1/1/2/0 | 0.024945 | 0.021504 | -13.8% | Passed |
| disabled-atf-quarantine-in_process | disabled, attempt-to-fix, quarantine, in_process | 1/1/1/2/0 | 1/1/1/2/0 | 0.023753 | 0.021932 | -7.7% | Passed |
| atr-coverage-in_process | atr, coverage, in_process | 1/1/1/2/0 | 1/1/1/2/0 | 0.025667 | 0.022391 | -12.8% | Passed |
| atr-recovery-process | atr, process | 1/1/1/2/0 | 1/1/1/2/0 | 0.036354 | 0.033100 | -9.0% | Passed |
| atr-exhaustion-process | atr, process, failure | 1/1/1/3/0 | 1/1/1/3/0 | 0.049093 | 0.044090 | -10.2% | Passed |
| efd-new-process | efd, process | 1/1/1/3/0 | 1/1/1/3/0 | 0.048117 | 0.042788 | -11.1% | Passed |
| efd-atr-process | efd, atr, process | 1/1/1/3/0 | 1/1/1/3/0 | 0.048736 | 0.042937 | -11.9% | Passed |
| attempt-to-fix-process | attempt-to-fix, process | 1/1/1/2/0 | 1/1/1/2/0 | 0.037640 | 0.032566 | -13.5% | Passed |
| disabled-atf-quarantine-process | disabled, attempt-to-fix, quarantine, process | 1/1/1/2/0 | 1/1/1/2/0 | 0.038062 | 0.032802 | -13.8% | Passed |
| atr-coverage-process | atr, coverage, process | 1/1/1/2/0 | 1/1/1/2/0 | 0.037391 | 0.033200 | -11.2% | Passed |
| atr-env-enables-remote-off | atr, override | 1/1/1/2/0 | 1/1/1/2/0 | 0.023988 | 0.021567 | -10.1% | Passed |
| atr-env-disables-remote-on | atr, override | 1/1/1/1/0 | 1/1/1/1/0 | 0.024011 | 0.021797 | -9.2% | Passed |
| efd-known | efd, known-tests | 1/1/1/1/0 | 1/1/1/1/0 | 0.024702 | 0.021096 | -14.6% | Passed |
| efd-cap-zero | efd, override | 1/1/1/1/0 | 1/1/1/1/0 | 0.024514 | 0.021470 | -12.4% | Passed |
| itr | itr | 1/1/1/1/0 | 1/1/1/1/0 | 0.024166 | 0.021159 | -12.4% | Passed |
| itr-atr-efd | itr, atr, efd | 1/1/1/1/0 | 1/1/1/1/0 | 0.025067 | 0.022670 | -9.6% | Passed |
| itr-disabled-quarantine | itr, disabled, quarantine | 1/1/1/1/0 | 1/1/1/1/0 | 0.024936 | 0.021520 | -13.7% | Passed |
| itr-coverage | itr, coverage | 1/1/1/2/0 | 1/1/1/2/0 | 0.025904 | 0.022445 | -13.4% | Passed |
| disabled-atr-efd | disabled, atr, efd | 1/1/1/1/0 | 1/1/1/1/0 | 0.025069 | 0.022020 | -12.2% | Passed |
| quarantine-atr-efd | quarantine, atr, efd | 1/1/1/1/0 | 1/1/1/1/0 | 0.024833 | 0.021076 | -15.1% | Passed |
| subtest-disabled | subtests, disabled | 1/1/1/2/0 | 1/1/1/2/0 | 0.023396 | 0.021710 | -7.2% | Passed |
| subtest-gate-off | subtests, disabled, override | 1/1/1/2/0 | 1/1/1/2/0 | 0.025289 | 0.020918 | -17.3% | Passed |
| subtest-quarantine | subtests, quarantine | 1/1/1/2/0 | 1/1/1/2/0 | 0.024786 | 0.021347 | -13.9% | Passed |
| management-env-off | disabled, override | 1/1/1/1/0 | 1/1/1/1/0 | 0.023234 | 0.021372 | -8.0% | Passed |
| benchmarks | benchmarks | 1/1/1/2/0 | 1/1/1/2/0 | 0.027273 | 0.023207 | -14.9% | Passed |
| impacted-known-efd | impacted-tests, known-tests, efd | 1/1/1/3/0 | 1/1/1/3/0 | 0.024656 | 0.021288 | -13.7% | Passed |
| impacted-env-off | impacted-tests, override | 1/1/1/1/0 | 1/1/1/1/0 | 0.024224 | 0.019956 | -17.6% | Passed |
| git-upload-require-git | git-upload, settings | 1/1/1/1/0 | 1/1/1/1/0 | 0.325048 | 0.178043 | -45.2% | Passed |
| logs-atr | logs, atr | 1/1/1/2/0 | 1/1/1/2/0 | 0.026380 | 0.020744 | -21.4% | Passed |
| report-itr-coverage | coverage-report, itr, coverage | 1/1/1/2/0 | 1/1/1/2/0 | 0.025736 | 0.022911 | -11.0% | Passed |
| efd-faulty-session | efd, faulty-session | 1/1/1/1/0 | 1/1/1/1/0 | 0.025326 | 0.020949 | -17.3% | Passed |
| itr-missing-line-coverage | itr, coverage, missing-line-coverage | 1/1/1/1/0 | 1/1/1/1/0 | 0.024208 | 0.022450 | -7.3% | Passed |
| itr-unskippable | itr, unskippable | 1/1/1/1/0 | 1/1/1/1/0 | 0.024026 | 0.020691 | -13.9% | Passed |
| itr-impacted-efd-known | itr, impacted-tests, efd, known-tests | 1/1/1/3/0 | 1/1/1/3/0 | 0.025834 | 0.022434 | -13.2% | Passed |
| itr-attempt-to-fix | itr, attempt-to-fix | 1/1/1/2/0 | 1/1/1/2/0 | 0.024815 | 0.022268 | -10.3% | Passed |
| atr-budget-zero | atr, budget | 1/1/1/1/0 | 1/1/1/1/0 | 0.026532 | 0.020288 | -23.5% | Passed |
| efd-parallel-execution | efd, parallel-execution | 1/1/1/3/0 | 1/1/1/3/0 | 0.024410 | 0.021058 | -13.7% | Passed |
| atr-parallel-in_process | atr, parallel, in_process | 1/1/1/2/0 | 1/1/1/2/0 | 0.024071 | 0.019896 | -17.3% | Passed |
| atr-parallel-process | atr, parallel, process | 1/1/1/2/0 | 1/1/1/2/0 | 0.034590 | 0.030658 | -11.4% | Passed |
| agent-coverage-itr | agent, coverage, itr | 1/1/1/2/0 | 1/1/1/2/0 | 0.023984 | 0.019925 | -16.9% | Passed |
| agent-retry | agent, atr | 1/1/1/2/0 | 1/1/1/2/0 | 0.023682 | 0.019017 | -19.7% | Passed |

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
| manual | 0.112539 | 0.011323 | -89.9% | prebuilt binary: startup, settings, execution and shutdown/flush |
| spans | 0.015447 | 0.010499 | -32.0% | prebuilt binary: startup, settings, execution and shutdown/flush |
| packages | 1.145873 | 0.450207 | -60.7% | CLI: preparation, compilation, execution and shutdown/flush |
| fuzz | 0.028738 | 0.021847 | -24.0% | prebuilt binary: startup, settings, execution and shutdown/flush |
| telemetry | 0.016707 | 0.010667 | -36.2% | prebuilt binary: startup, settings, execution and shutdown/flush |
| testify | 1.055859 | 1.042093 | -1.3% | prebuilt binary: startup, settings, execution and shutdown/flush; SDK column uses full Orchestrion reference |
| uds | 0.025030 | 0.016970 | -32.2% | prebuilt binary: startup, settings, execution and shutdown/flush |

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
| pass-skip | 1/1/2/3/0 | 1/1/2/3/0 | 1.055859 | 1.042093 | -1.3% |
| method-filter | 1/1/2/2/0 | 1/1/2/2/0 | 1.054634 | 1.040881 | -1.3% |
| count-shuffle | 1/1/2/4/0 | 1/1/2/4/0 | 1.059505 | 1.046223 | -1.3% |
| nested | 1/1/2/3/0 | 1/1/2/3/0 | 1.054421 | 1.041782 | -1.2% |
| assert-failure | 1/1/2/2/0 | 1/1/2/2/0 | 0.057366 | 0.039034 | -32.0% |
| require-failure | 1/1/2/2/0 | 1/1/2/2/0 | 0.055207 | 0.038495 | -30.3% |
| panic | 1/1/2/2/0 | 1/1/2/2/0 | 0.055271 | 0.040223 | -27.2% |
| lifecycle-stats | 1/1/2/4/0 | 1/1/2/4/0 | 1.054741 | 1.041116 | -1.3% |
| aliases-dot-helpers | 1/1/4/6/0 | 1/1/4/6/0 | 1.065771 | 1.053258 | -1.2% |
| external-module-helper | 1/1/2/2/0 | 1/1/2/2/0 | 1.054680 | 1.040373 | -1.4% |
| internal-package | 1/1/2/2/0 | 1/1/2/2/0 | 1.053839 | 1.039646 | -1.3% |
| custom-and-shadowed-run | 1/1/1/3/0 | 1/1/1/3/0 | 1.050932 | 1.036094 | -1.4% |
| parallel-suites | 1/1/2/4/0 | 1/1/2/4/0 | 1.057374 | 1.042106 | -1.4% |
| coverage-helpers | 1/1/2/2/0 | 1/1/2/2/0 | 1.057273 | 1.043300 | -1.3% |
| coverage | 1/1/2/4/0 | 1/1/2/4/0 | 1.058205 | 1.043885 | -1.4% |
| itr-parent | 1/1/1/1/0 | 1/1/1/1/0 | 1.048949 | 1.036057 | -1.2% |
| itr-method-sdk-limit | 1/1/2/2/0 | 1/1/2/2/0 | 0.054241 | 0.038944 | -28.2% |
| atr-in_process | 1/1/2/4/0 | 1/1/2/4/0 | 1.059711 | 1.046288 | -1.3% |
| atr-coverage-in_process | 1/1/2/4/0 | 1/1/2/4/0 | 1.063191 | 1.047358 | -1.5% |
| efd-in_process | 1/1/2/6/0 | 1/1/2/6/0 | 1.063654 | 1.052593 | -1.0% |
| atr-process | 1/1/2/3/0 | 1/1/2/3/0 | 2.085321 | 2.062830 | -1.1% |
| atr-coverage-process | 1/1/2/3/0 | 1/1/2/3/0 | 2.090234 | 2.073915 | -0.8% |
| efd-process | 1/1/2/4/0 | 1/1/2/4/0 | 3.115455 | 3.088422 | -0.9% |
| disabled | 1/1/2/2/0 | 1/1/2/2/0 | 1.054652 | 1.047678 | -0.7% |
| quarantined | 1/1/2/2/0 | 1/1/2/2/0 | 1.056162 | 1.044689 | -1.1% |
| attempt-to-fix | 1/1/2/2/0 | 1/1/2/2/0 | 0.059208 | 0.044772 | -24.4% |
