# Coverage adapter process overhead

This report measures the earlier caller-overlay coverage adapter. Its caller
rewriting and helper exclusions were superseded by entry instrumentation. See
[the current Testify contract](../../testify.md) and
[selective-tool results](../selective-tools-20261003-linux-go1.27/README.md).
The values below retain their original inputs.

This local Linux/amd64 run uses Go 1.27.1 and `GOMAXPROCS=4`. Both variants
compile the same prepared Mini overlay with native coverage. The fixture has no
normal Testify callers, so the control can compile without the adapter. Forcing
it in the second variant isolates its process and version-probe overhead.
Preparation, test execution and SDK runtime work are excluded. This is not a
Native/POC/Orchestrion comparison or a measurement of all Testify rewriting work.

The final run uses separate stable binary outputs per variant. It preserves six
balanced pairs for unchanged and edited builds, and four pairs with `-a`.
The edit adds an unused exported constant with equal-length values, forcing the
same compiler/linker work without changing test behavior. CPU seconds include
all waited descendant processes via `RUSAGE_CHILDREN`; this adapter starts no
detached daemon. The raw JSON keeps commands, nanosecond timings and tool counts.

| Scenario | Direct wall (s) | Adapter wall (s) | Added wall | Direct CPU (s) | Adapter CPU (s) |
| --- | ---: | ---: | ---: | ---: | ---: |
| Unchanged | 0.0678 | 0.0749 | +10.5% | 0.1792 | 0.1957 |
| Source edit | 0.4590 | 0.4756 | +3.6% | 0.7141 | 0.7364 |
| Forced rebuild (`-a`) | 7.5669 | 7.7916 | +3.0% | 34.3590 | 35.7984 |

These are medians. Unchanged builds have zero compile/cover/link calls in both
variants. Edited builds have three compiles, two covers and one link; forced
rebuilds have 279 compiles, two covers and one link. `-a` means recompilation with
warm filesystem/module state; it is not an empty-cache cold build.

The edit control contains a 1.592 s outlier; its other values and all observations
remain in the JSON. The full-rebuild wall ranges are 7.421–7.708 s for the control
and 7.743–7.800 s with the adapter. The measured process cost is about 7 ms for a
cache hit and 225 ms for this forced rebuild. Covered Testify helpers also do
source-path translation and generated-file exclusion, which this control does
not separately time. The adapter remains off when no covered normal source was
rewritten.

## Earlier observations

`adapter-overhead-exploratory.json` overlapped local validation and is not used
for conclusions. `adapter-overhead-shared-output.json` contains another setup
error: switching variants overwrote one output binary and caused extra links.
Its tool counts distinguish cache hits from relinks. The final run corrects that
confound; none of the earlier observations were deleted.

## Reproducing the measurement

`measure-adapter.py` records the exact local paths and all commands. It requires
a neutral copy of `testdata/fixture` and one Mini overlay produced by
`runner.PrepareRuntime` with `-coverpkg=.`. The fixture needs the POC module in its
module graph and must not blank-import the full SDK. Build the current `ddtest`,
adjust the task artifact/fixture/plan paths, warm both identities, then run the
script while no other validation command is active. `prepare-plan.go.txt` is
the exact small helper used to create the retained plan. Build it inside this
repository (for example, temporarily as `.adapter-plan/main.go`) so Go permits
its `internal/runner` import. It prints the plan JSON that the measurement script
reads. Remove the helper and plan only after all builds finish. The measurement
never runs the test binary. Functional coverage/cache parity lives in the integration tests rather
than this timing control.

The manifest records the uncommitted source base and input hashes. This run
cannot establish the wall cost of another project, CPU count or operating system.
