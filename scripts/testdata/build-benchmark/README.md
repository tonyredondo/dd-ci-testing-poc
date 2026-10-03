# Frozen benchmark inputs

These inputs reproduce the module graphs and Testify fixtures from the
2026-10-03 compile matrix. They are benchmark data, not Mini runtime requirements.
The comparison graph intentionally includes both instrumentators and both
runtimes so Native, Orchestrion, SDK and Mini select identical dependency versions.

`../../build-benchmark.json` records the SDK version/commit, Orchestrion version,
Gin/Chi upstream versions and SHAs, expected binaries and edit signatures.
Gin/Chi source is obtained from the matching Go module during setup; only their
prepared `go.mod.template` and `go.sum` are stored here. The Testify fixtures are
POC-owned sources. Their graphs were derived from the same prepared Gin graph.

`@POC_ROOT@` is replaced with the current checkout's quoted absolute path.
The external fixture's `example.com/testkit` replacement stays relative to that
fixture. No other module version changes are made during setup. All four variants
share each prepared subject; temporary copies and overlays leave these files
unchanged.

See [the runner protocol](../../../docs/build-benchmarks.md) before updating
these inputs. Preserve dated results and use a new artifact directory when a
project, SDK pin, graph or fixture changes.
