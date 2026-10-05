# Compile-only comparison: Native, Orchestrion, POC SDK and POC Mini

POC `9d4786fbd27f573bb68c5516bf23b336e8d7bdaa`; `go version go1.27.1 linux/amd64`; SDK `v2.12.0-dev.3.0.20261002145613-96aedb31048c`; Orchestrion `v1.13.2-0.20260917114356-5c24783fcd76`.

Values are medians in seconds. POC percentages show the signed change against total Orchestrion wall time first, then against Native; both use unrounded medians.

All variants compile with `go test -c -o <directory>/ -ldflags=-w ./...`. Test binaries are never run. The same prepared source and module graph is used for all four variants. Orchestrion loads only the pinned SDK's testing aspects.

Builds run serially in rotating order. Each cold run has an empty Go build cache and no existing output. Downloads are disabled during timing; modules and OS page cache stay warm. Affinity, `GOMAXPROCS` and `-p` match the selected CPU count; manifest.json records logical CPU IDs and physical core topology.

Unchanged output reuse, forced linking and reachable edits are checked with tool traces. Final binaries are checked for the expected testing/Testify hooks and absence of DWARF. CPU and memory include daemons and nested builds through exclusive cgroups. Wall time ends at the top-level command's exit; drain wait is recorded separately.

7153 completed build commands, 432 qualified binaries. Native control repetitions: 160. A skipped control in a smoke run provides no stability evidence.

[Every observation](observations.csv), [ranges and uncertainty](statistics.json), [inputs](manifest.json) and [binary qualification](final-binaries.json) remain alongside this report. No slow samples are discarded. Fixture results are not a general application performance claim.
