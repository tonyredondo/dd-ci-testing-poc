# Regenerating the CODEOWNERS reference

Go CI runs the saved .NET corpus. It needs no .NET SDK or NuGet dependencies.
Use this harness when updating the port or its expectations.

The frozen source revision and original file hashes are in
[`dotnet-tests.json`](../../internal/thirdparty/dd-trace-go/civisibility/codeownership/testdata/dotnet-tests.json).
The corpus includes every method in `CodeOwnersSpecTests.cs` and
`CodeOwnersTests.cs`: 72 methods and 137 test executions, including theory
parameters. It records owner order, parsing diagnostics, file loading and the
large stress inputs. Separate Go tests exercise concurrent readers, immutable
results and the same ten-second stress limit.

`CaptureCodeOwners.cs` forwards calls to the original parser. The test files
receive a type alias to that recorder; their assertions remain unchanged and
run with real Xunit and FluentAssertions assemblies. Logger and attribute
stubs have no effect on parsing or assertions. `EnvironmentTools` supplies the
fixture directory instead of requiring the complete .NET solution.

The generator checks original source hashes before compiling. It also exports
1,800 deterministic rulesets with 18,000 file queries, and Unicode properties
from the reference runtime. Corpus JSON is gzip-compressed because the original
tests include multi-megabyte strings. Decompress it with Python or `gzip -dc`
when inspecting a failure. Query records without `HasPath` check diagnostics
only; they are not file queries.

Obtain a dd-trace-dotnet archive at the recorded SHA, preserving repository
paths. Use an installed .NET 10 SDK, its reference assemblies, and existing
Xunit/FluentAssertions assemblies. The reference was validated with .NET
10.0.11, Xunit 2.9.3 and FluentAssertions 7.0.0 on Linux. Set the paths below for
your installation; the script does not download tools or packages.

```sh
python3 scripts/codeownership/generate.py \
  --source "$DOTNET_SOURCE" \
  --csc "$DOTNET_SDK/Roslyn/bincore/csc.dll" \
  --reference-dir "$DOTNET_REFERENCE/ref/net10.0" \
  --runtime-version 10.0.11 \
  --fluentassertions "$FLUENTASSERTIONS/lib/net6.0/FluentAssertions.dll" \
  --xunit-core "$XUNIT_CORE/lib/netstandard1.1/xunit.core.dll" \
  --xunit-assert "$XUNIT_ASSERT/lib/net6.0/xunit.assert.dll" \
  --temp-dir "$BENCHMARK_TEMP"

go test -race ./internal/thirdparty/dd-trace-go/civisibility/codeownership
python3 scripts/upstream.py verify
```

Review the regenerated expectations before committing. Changing the reference
runtime can change Unicode classifications or casing. `unicode.go` reconciles
the supported Go versions with the saved reference; compare new differences
before updating those tables. The Unicode test checks every UTF-16 char and
the supplementary case mappings in the corpus.

This harness covers the parser and matcher. .NET's repository-wide C# ownership
checks are checks of its own repository. Its CI source-path resolver uses .NET
compiler paths; Mini retains its Go source resolver, with separate discovery,
workspace-boundary and compiled-binary integration tests. Package-directory
queries for service names are an extension covered by `TestDirectorySemantics`.
