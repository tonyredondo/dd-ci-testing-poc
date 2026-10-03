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
