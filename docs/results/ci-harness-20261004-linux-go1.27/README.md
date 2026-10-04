# Sharing parity fixture builds

The normal and deferred matrices compile the same sources with the same flags.
This change builds their SDK, Mini and Orchestrion binaries once per harness
process, retaining the inputs until `TestMain` completes. The `testing` and
Testify sets remain separate: Testify requires race-enabled binaries. Every
scenario still gets its own receiver, child process and retry state.

Three alternating pairs compare the clean tree at
`c87af8e13f532206abb16227ccb94d66776cabab` with that tree plus this harness change.
Linux amd64 uses `go1.27.0-X:nodwarf5`, `GOMAXPROCS=8`, warm Go build/module
caches and no CPU affinity. No other builds or suites from this task ran during
the pairs; the host was not reserved. Each variant has a complete warmup,
excluded below, then a fresh harness process per measured run.

The clock includes fixture preparation, compilation, all 115 scenarios and
their comparisons. It excludes building the outer harness. All six runs retain
65 normal and 17 deferred `testing` cases, plus 26 normal and 7 deferred Testify
cases. Their names, features, event counts, outcomes and exit codes match the
baseline. The existing strict comparisons of metadata, hierarchy, errors and
coverage also pass. Comparison code and scenario definitions are unchanged.

| Scope | Before median | Shared builds median | Reduction | Before range | Shared range |
| --- | ---: | ---: | ---: | ---: | ---: |
| Four complete parity matrices | 110.257 s | 95.324 s | 13.5% | 109.357–110.644 s | 95.027–95.837 s |

These are local warm-cache walltimes. They do not predict the duration of a cold
GitHub runner or quantify CPU savings. For these four matrices, test-binary
compilations fall from twelve to six per process; all scenarios still execute.

## Raw measurements and reproduction

[`benchmark.json`](benchmark.json) retains every elapsed time, pair order,
command, binary hash and measured source hash. The adjacent `round-*.json`
files retain all per-case durations and outcomes for both variants. Full logs,
outer harness binaries and validation artifacts remain in the task directory
recorded by the manifest.

Build an outer harness from each revision, then run each from its own
`integration` directory. Keep the SDK and Orchestrion revisions fixed, run one
warmup per variant, and alternate baseline/candidate, candidate/baseline,
baseline/candidate. Do not clear the Go cache between these warm-cache runs.

```sh
go test -c -o /path/to/variant.integration.test ./integration
cd integration
GOMAXPROCS=8 ORCHESTRION_BIN=/path/to/frozen/orchestrion \
  PARITY_EXECUTION_ORDER=sdk-first PARITY_REPORT_PATH=/path/to/run.json \
  /path/to/variant.integration.test -test.v -test.count=1 -test.timeout=12m \
  '-test.run=^(TestCIVisibilityParityMatrix|TestDeferredDeliveryParityMatrix|TestCIVisibilityTestifyParity|TestDeferredDeliveryTestifyParity)$'
```

The SDK reference remains `96aedb31048c07e29e7a20a4333dc3b8d289c52d`;
Orchestrion remains `v1.13.2-0.20260917114356-5c24783fcd76`. Runtime code,
dependencies and incorporated sources are unchanged.

## Workflow changes

The workflow now runs on pull requests, pushes to `main` and manual dispatch.
A feature branch push with an open PR previously launched two matrices, each
with six complete suite executions. It now uses one PR matrix with those same
six executions.

Linux normal and race suites run in separate jobs for Go 1.26 and 1.27;
macOS and Windows each retain their normal Go 1.27 suite. Normal runs remain
SDK-first and race runs Mini-first. Artifacts include the mode in their names.
[`workflow-validation.json`](workflow-validation.json) records Actionlint,
matrix/event checks and execution of both shell branches with a local command
stub, including preservation of a failing exit through `tee`. The stub checks
shell behavior; it does not provide parity evidence.

At the time of these local measurements, the new workflow had not run on
GitHub. These measurements do not establish its elapsed time, runner queueing
or native macOS/Windows behavior.

## Local validation

The complete Go 1.26.8 suite passes in SDK-first order, and the complete
Go 1.27 development suite passes with `-race` in Mini-first order. Both retain
all 115 scenarios and supplemental evidence; the report renderer accepts both
sets. The rendered [Go 1.26 report](full-126.md) and
[Go 1.27 race report](full-race-127.md) sit beside their raw JSON files.

A deferred-only invocation with `-count=2 -shuffle=42` executes both deferred
matrices twice, compiling each fixture once. Subprocess tests verify that shared
inputs survive the first consumer and are removed after `TestMain`, and that a
builder's `Fatal` also fails the next consumer. An actual missing Orchestrion
executable makes both testing matrices fail: the first reports the compilation
error, the next reports the unavailable fixture. This negative control exits 1
as expected; it does not fall back to another reference.

All 21 Python maintenance/report tests, source and license manifests, and
`go vet ./...` pass. The integration harness also cross-compiles for Windows
and macOS amd64. Native execution remains a separate CI check.
[`validation.json`](validation.json) records commands, expected exit codes,
log hashes and source hashes from before publication.

The first published run at `c918fe17f037ac5821260baaf72c1ef4eb16e055` passed all
four Linux jobs and macOS. Windows passed the tests but failed the final
workspace removal because Orchestrion still held its stderr log open. The
follow-up restores the two-second Windows retry window that `testing.TempDir`
provides and keeps persistent errors fatal. Native tests reproduce a real open
handle, verify removal after closure, and require an error for a persistent
lock. The timing and source hashes above remain the original observations;
they have not been rewritten to describe the cleanup follow-up.

The next run at `4ce5e04304b529c913cd75632db4b63dfed93d00` passed both Windows
lock tests and shared workspace cleanup. It exposed a separate omission in the
independent plain-coverage Testify reference build: that case did not retain
`$WORK`, unlike the main matrix. Both builds now use one helper with the existing
Windows ownership rule. The case, coverage assertions and frozen reference are
retained.
