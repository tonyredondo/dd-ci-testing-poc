# CODEOWNERS parser

This package is a Go port of DataDog/dd-trace-dotnet's
[CodeOwnership](https://github.com/DataDog/dd-trace-dotnet/tree/843640c32bae5fe6dcdf790906f6431fe15f6973/tracer/src/Datadog.Trace/Ci/CodeOwnership)
at master commit `843640c32bae5fe6dcdf790906f6431fe15f6973`.
Original sources are Copyright 2017 Datadog, Inc. The port retains the upstream
Apache-2.0 [license](LICENSE). Paths and hashes are recorded in the parent SDK
[SOURCE.json](../../SOURCE.json), under the separate `codeownership-dotnet`
feature port. That record does not change the dd-trace-go base.

| Upstream file | Go implementation |
| --- | --- |
| `CodeOwners.cs` | `codeowners.go`, `glob.go`, `file.go` |
| `CodeOwners.GitHub.cs` | `github.go` |
| `CodeOwners.GitLab.cs` | `gitlab.go` |
| `CodeOwnersFileLocator.cs` | `locator.go` |
| `CodeOwnersSpecTests.cs` | `codeowners_test.go`, `testdata/dotnet-spec.json.gz` |
| `CodeOwnersTests.cs` | Same corpus; original GitHub/GitLab fixtures retained |
| `CodeOwnersFallbackTests.cs` | Relevant discovery cases in `locator_test.go` |

The package has no CI globals or external imports. `utils.GetCodeOwners`
supplies repository context and caches discovery. The source cache and retry
integration call `Match`; service selection calls `MatchDirectory`.

The [maintenance guide](../../../../../docs/codeownership.md) explains the
dialects, immutable results, directory semantics, bounds and update checks.

The complete parser test inventory is in [dotnet-tests.json](testdata/dotnet-tests.json).
The [reference harness](../../../../../scripts/codeownership/README.md) executes
the original .NET assertions before exporting expectations. Go CI runs those
expectations, a deterministic differential corpus, and Unicode property checks.
No .NET tools or external modules are needed by Go consumers or Go CI.

Performance adaptations are documented in the [measurement guide](../../../../../docs/codeownership-performance.md)
and the parent [ADAPTATIONS.md](../../ADAPTATIONS.md). The benchmarks use
synthetic rules and contain no customer CODEOWNERS content.
