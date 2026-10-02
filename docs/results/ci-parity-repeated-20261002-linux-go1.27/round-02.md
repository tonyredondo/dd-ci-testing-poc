# CI Visibility parity evidence

Complete feature parity: **no**. The Testify instrumentation gap remains.

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

Execution order: mini then sdk.

| Entire 65-scenario block | SDK wall (s) | Mini wall (s) | Mini vs SDK |
| --- | ---: | ---: | ---: |
| Continuous walltime | 1.773130 | 1.553498 | -12.4% |

Measured scope: 65-scenario continuous block: receiver setup, process execution and shutdown/flush; excludes compilation and comparisons.

| Scenario | Features | SDK events | Mini events | SDK wall (s) | Mini wall (s) | Mini vs SDK | Result |
| --- | --- | --- | --- | ---: | ---: | ---: | --- |
| pass | lifecycle | 1/1/1/1/0 | 1/1/1/1/0 | 0.022817 | 0.020553 | -9.9% | Passed |
| skip-reasons | skip | 1/1/1/3/0 | 1/1/1/3/0 | 0.022335 | 0.020294 | -9.1% | Passed |
| nested-cleanup-context | subtests, cleanup, context | 1/1/1/6/0 | 1/1/1/6/0 | 0.022703 | 0.019097 | -15.9% | Passed |
| parallel-count-shuffle | parallel, count, shuffle | 1/1/1/18/0 | 1/1/1/18/0 | 0.023862 | 0.020980 | -12.1% | Passed |
| multiple-suites | hierarchy, source | 1/1/2/2/0 | 1/1/2/2/0 | 0.022515 | 0.019810 | -12.0% | Passed |
| empty-selection | lifecycle, empty | 1/0/0/0/0 | 1/0/0/0/0 | 0.022267 | 0.017812 | -20.0% | Passed |
| list | lifecycle, list | 1/0/0/0/0 | 1/0/0/0/0 | 0.021712 | 0.017545 | -19.2% | Passed |
| examples-fuzz-seeds | examples, fuzz-seeds | 1/0/0/0/0 | 1/0/0/0/0 | 0.021806 | 0.018020 | -17.4% | Passed |
| settings-unavailable | settings, failure | 1/1/1/1/0 | 1/1/1/1/0 | 0.022905 | 0.018510 | -19.2% | Passed |
| tags-version-ci | metadata, version, ci | 1/1/1/1/0 | 1/1/1/1/0 | 0.022972 | 0.020003 | -12.9% | Passed |
| coverage-pass-skip | coverage, skip | 1/1/1/2/0 | 1/1/1/2/0 | 0.024531 | 0.020708 | -15.6% | Passed |
| coverage-parallel-shuffle | coverage, parallel, shuffle | 1/1/1/10/0 | 1/1/1/10/0 | 0.026409 | 0.021453 | -18.8% | Passed |
| coverage-report | coverage-report | 1/1/1/1/0 | 1/1/1/1/0 | 0.023289 | 0.019956 | -14.3% | Passed |
| logs | logs | 1/1/1/1/0 | 1/1/1/1/0 | 0.022710 | 0.019062 | -16.1% | Passed |
| error-Fail | errors | 1/1/1/2/0 | 1/1/1/2/0 | 0.023605 | 0.020100 | -14.8% | Passed |
| error-FailNow | errors | 1/1/1/2/0 | 1/1/1/2/0 | 0.022530 | 0.018817 | -16.5% | Passed |
| error-Error | errors | 1/1/1/2/0 | 1/1/1/2/0 | 0.022611 | 0.019417 | -14.1% | Passed |
| error-Errorf | errors | 1/1/1/2/0 | 1/1/1/2/0 | 0.022386 | 0.019839 | -11.4% | Passed |
| error-Fatal | errors | 1/1/1/2/0 | 1/1/1/2/0 | 0.023110 | 0.018981 | -17.9% | Passed |
| error-Fatalf | errors | 1/1/1/2/0 | 1/1/1/2/0 | 0.022466 | 0.019285 | -14.2% | Passed |
| atr-recovery-in_process | atr, in_process | 1/1/1/2/0 | 1/1/1/2/0 | 0.022227 | 0.019174 | -13.7% | Passed |
| atr-exhaustion-in_process | atr, in_process, failure | 1/1/1/3/0 | 1/1/1/3/0 | 0.022504 | 0.019724 | -12.4% | Passed |
| efd-new-in_process | efd, in_process | 1/1/1/3/0 | 1/1/1/3/0 | 0.023622 | 0.019595 | -17.0% | Passed |
| efd-atr-in_process | efd, atr, in_process | 1/1/1/3/0 | 1/1/1/3/0 | 0.022616 | 0.020595 | -8.9% | Passed |
| attempt-to-fix-in_process | attempt-to-fix, in_process | 1/1/1/2/0 | 1/1/1/2/0 | 0.022433 | 0.019994 | -10.9% | Passed |
| disabled-atf-quarantine-in_process | disabled, attempt-to-fix, quarantine, in_process | 1/1/1/2/0 | 1/1/1/2/0 | 0.023763 | 0.019278 | -18.9% | Passed |
| atr-coverage-in_process | atr, coverage, in_process | 1/1/1/2/0 | 1/1/1/2/0 | 0.023519 | 0.020017 | -14.9% | Passed |
| atr-recovery-process | atr, process | 1/1/1/2/0 | 1/1/1/2/0 | 0.034474 | 0.029935 | -13.2% | Passed |
| atr-exhaustion-process | atr, process, failure | 1/1/1/3/0 | 1/1/1/3/0 | 0.045076 | 0.040621 | -9.9% | Passed |
| efd-new-process | efd, process | 1/1/1/3/0 | 1/1/1/3/0 | 0.045658 | 0.040144 | -12.1% | Passed |
| efd-atr-process | efd, atr, process | 1/1/1/3/0 | 1/1/1/3/0 | 0.046151 | 0.040007 | -13.3% | Passed |
| attempt-to-fix-process | attempt-to-fix, process | 1/1/1/2/0 | 1/1/1/2/0 | 0.035076 | 0.029939 | -14.6% | Passed |
| disabled-atf-quarantine-process | disabled, attempt-to-fix, quarantine, process | 1/1/1/2/0 | 1/1/1/2/0 | 0.034314 | 0.029633 | -13.6% | Passed |
| atr-coverage-process | atr, coverage, process | 1/1/1/2/0 | 1/1/1/2/0 | 0.034982 | 0.031704 | -9.4% | Passed |
| atr-env-enables-remote-off | atr, override | 1/1/1/2/0 | 1/1/1/2/0 | 0.023042 | 0.018945 | -17.8% | Passed |
| atr-env-disables-remote-on | atr, override | 1/1/1/1/0 | 1/1/1/1/0 | 0.022299 | 0.019273 | -13.6% | Passed |
| efd-known | efd, known-tests | 1/1/1/1/0 | 1/1/1/1/0 | 0.022631 | 0.019810 | -12.5% | Passed |
| efd-cap-zero | efd, override | 1/1/1/1/0 | 1/1/1/1/0 | 0.022656 | 0.019618 | -13.4% | Passed |
| itr | itr | 1/1/1/1/0 | 1/1/1/1/0 | 0.023737 | 0.019134 | -19.4% | Passed |
| itr-atr-efd | itr, atr, efd | 1/1/1/1/0 | 1/1/1/1/0 | 0.022737 | 0.019779 | -13.0% | Passed |
| itr-disabled-quarantine | itr, disabled, quarantine | 1/1/1/1/0 | 1/1/1/1/0 | 0.022757 | 0.019044 | -16.3% | Passed |
| itr-coverage | itr, coverage | 1/1/1/2/0 | 1/1/1/2/0 | 0.024096 | 0.020796 | -13.7% | Passed |
| disabled-atr-efd | disabled, atr, efd | 1/1/1/1/0 | 1/1/1/1/0 | 0.022296 | 0.019407 | -13.0% | Passed |
| quarantine-atr-efd | quarantine, atr, efd | 1/1/1/1/0 | 1/1/1/1/0 | 0.022773 | 0.019699 | -13.5% | Passed |
| subtest-disabled | subtests, disabled | 1/1/1/2/0 | 1/1/1/2/0 | 0.022853 | 0.019364 | -15.3% | Passed |
| subtest-gate-off | subtests, disabled, override | 1/1/1/2/0 | 1/1/1/2/0 | 0.022431 | 0.019399 | -13.5% | Passed |
| subtest-quarantine | subtests, quarantine | 1/1/1/2/0 | 1/1/1/2/0 | 0.023229 | 0.019019 | -18.1% | Passed |
| management-env-off | disabled, override | 1/1/1/1/0 | 1/1/1/1/0 | 0.021580 | 0.018855 | -12.6% | Passed |
| benchmarks | benchmarks | 1/1/1/2/0 | 1/1/1/2/0 | 0.025816 | 0.021459 | -16.9% | Passed |
| impacted-known-efd | impacted-tests, known-tests, efd | 1/1/1/3/0 | 1/1/1/3/0 | 0.023607 | 0.020036 | -15.1% | Passed |
| impacted-env-off | impacted-tests, override | 1/1/1/1/0 | 1/1/1/1/0 | 0.022557 | 0.019290 | -14.5% | Passed |
| git-upload-require-git | git-upload, settings | 1/1/1/1/0 | 1/1/1/1/0 | 0.154150 | 0.168821 | +9.5% | Passed |
| logs-atr | logs, atr | 1/1/1/2/0 | 1/1/1/2/0 | 0.023212 | 0.020558 | -11.4% | Passed |
| report-itr-coverage | coverage-report, itr, coverage | 1/1/1/2/0 | 1/1/1/2/0 | 0.023214 | 0.021019 | -9.5% | Passed |
| efd-faulty-session | efd, faulty-session | 1/1/1/1/0 | 1/1/1/1/0 | 0.022686 | 0.018825 | -17.0% | Passed |
| itr-missing-line-coverage | itr, coverage, missing-line-coverage | 1/1/1/1/0 | 1/1/1/1/0 | 0.023087 | 0.019010 | -17.7% | Passed |
| itr-unskippable | itr, unskippable | 1/1/1/1/0 | 1/1/1/1/0 | 0.022246 | 0.019388 | -12.8% | Passed |
| itr-impacted-efd-known | itr, impacted-tests, efd, known-tests | 1/1/1/3/0 | 1/1/1/3/0 | 0.023190 | 0.019680 | -15.1% | Passed |
| itr-attempt-to-fix | itr, attempt-to-fix | 1/1/1/2/0 | 1/1/1/2/0 | 0.023291 | 0.018808 | -19.2% | Passed |
| atr-budget-zero | atr, budget | 1/1/1/1/0 | 1/1/1/1/0 | 0.022156 | 0.018332 | -17.3% | Passed |
| efd-parallel-execution | efd, parallel-execution | 1/1/1/3/0 | 1/1/1/3/0 | 0.022940 | 0.019973 | -12.9% | Passed |
| atr-parallel-in_process | atr, parallel, in_process | 1/1/1/2/0 | 1/1/1/2/0 | 0.023240 | 0.018126 | -22.0% | Passed |
| atr-parallel-process | atr, parallel, process | 1/1/1/2/0 | 1/1/1/2/0 | 0.034615 | 0.029117 | -15.9% | Passed |
| agent-coverage-itr | agent, coverage, itr | 1/1/1/2/0 | 1/1/1/2/0 | 0.023527 | 0.019339 | -17.8% | Passed |
| agent-retry | agent, atr | 1/1/1/2/0 | 1/1/1/2/0 | 0.022845 | 0.017718 | -22.4% | Passed |

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
The manual fixture injects a settings delay; Testify retains a known grouping gap.
Use the scope recorded in each JSON observation when comparing runs.

| Fixture | SDK wall (s) | Mini wall (s) | Mini vs SDK | Measured scope |
| --- | ---: | ---: | ---: | --- |
| manual | 0.113688 | 0.011039 | -90.3% | prebuilt binary: startup, settings, execution and shutdown/flush |
| spans | 0.013621 | 0.010847 | -20.4% | prebuilt binary: startup, settings, execution and shutdown/flush |
| packages | 0.865429 | 0.324412 | -62.5% | CLI: preparation, compilation, execution and shutdown/flush |
| fuzz | 0.028107 | 0.020015 | -28.8% | prebuilt binary: startup, settings, execution and shutdown/flush |
| telemetry | 0.015327 | 0.009074 | -40.8% | prebuilt binary: startup, settings, execution and shutdown/flush |
| testify | 0.014063 | 0.010901 | -22.5% | prebuilt binary: startup, settings, execution and shutdown/flush; SDK column uses full Orchestrion reference |
| uds | 0.021273 | 0.016391 | -22.9% | prebuilt binary: startup, settings, execution and shutdown/flush |

CI telemetry: semantic count/rate metrics match; request counts are
checked against each sender's actual HTTP requests. Batch sizes and
timings may differ. Distributions are outside this counter fixture.

Testify: full SDK `1/1/2/3/0`, POC SDK `1/2/3/3/0`, Mini `1/2/3/3/0`.
Known gap: missing testify.suite.Run advice; method suite/source metadata differ.

This is loopback protocol evidence. Real intake/UI acceptance, an actual
Bazel toolchain run and an external APM shim are unverified. Read
`docs/ci-parity.md` for the exclusions and remaining feature coverage.
