#!/usr/bin/env python3
"""Export the frozen .NET tests as a Go corpus using their real assertions.

The .NET parser is never rewritten. The original tests receive a type alias to
CaptureCodeOwners, which delegates to it and records inputs and results.
Go CI consumes the corpus and does not need .NET or NuGet dependencies.
"""

import argparse
import gzip
import hashlib
import json
import os
import random
from pathlib import Path
import shutil
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[2]
SUPPORT = Path(__file__).resolve().parent
PACKAGE = ROOT / "internal/thirdparty/dd-trace-go/civisibility/codeownership"
SHA = "843640c32bae5fe6dcdf790906f6431fe15f6973"
CORE = "tracer/src/Datadog.Trace/Ci/CodeOwnership/"
TESTS = "tracer/test/Datadog.Trace.Tests/Ci/"
FIXTURES = "tracer/test/Datadog.Trace.ClrProfiler.IntegrationTests/CI/Data/"


def edge_cases():
    cases = []
    patterns = ["?", "??", "[😀]", "[!😀]", "😀", "\\😀", "/", ".", "/a/**/z", "*", "**", "[a-z]"]
    paths = ["😀", "𐐀", "a", "z", "", ".", "a/z", "a/" + "x/" * 70000 + "z"]
    for dialect in ["GitHub", "GitLab"]:
        for pattern in patterns:
            cases.append(dict(Name="unicode-and-path:" + pattern, Dialect=dialect, Rules=pattern + " @owner", Paths=paths))
    for token in ["𐐀@team", "x́@@owner", "x‿@@owner", "(@@owner)foo", "(@@owner\u00a0suffix)", "😀" * 51 + "@x", "a@" + "𐐀" * 128, "a@😀", "foo@team\u00a0@@owner", "@@ownErS", "@@maİntainer"]:
        cases.append(dict(Name="owner:" + token[:20], Dialect="GitLab", Rules="* " + token, Paths=["/a"]))
    for first, second in [("k", "K"), ("s", "ſ"), ("ı", "I"), ("Σ", "ς"), ("𐐀", "𐐨")]:
        cases.append(dict(Name="section:" + first + second, Dialect="GitLab", Rules="[" + first + "]\n* @first\n[" + second + "]\n* @second", Paths=["/a"]))
    for header in ["[😀] @team", "[Docs] 𐐀@team", "[Docs][ 1 ] @team", "^[Docs][ 0 ] @team", "[Docs][0] @team", "[Docs] @team\u0085@@owner", "[Docs][١] @team", "[Docs][𝟙] @team", "[Docs][𞓰] @team"]:
        cases.append(dict(Name="header:" + header, Dialect="GitLab", Rules=header + "\n*.go", Paths=["/a.go"]))
    cases.append(dict(Name="file-line-endings", Dialect="GitHub", Rules="* @global\r*.go @go\r\n*.md @docs\n", Paths=["/a.go", "/a.md", "/a.txt"]))
    return cases


def differential_cases():
    rng = random.Random(20261007)
    patterns = ["*", "**", "?", "??", "[ab]", "[^ab]", "[z-a]", "[!😀]", "[😀]", "a**b", "a/**/b", "a\\ b", "foo\\/bar", "[", "foo/", "/foo/*", "/foo/**", "/**/a", "/", ".", "\\#x", "\\*", "trailing\\", "*.go"]
    parts = ["a", "b", "foo", "bar", "😀", "𐐀", ".", "x", "a b", "[", "*", "é"]
    owners = ["@team", "@team/sub", "@team-", "@@owner", "@@ownErS", "@@maİntainer", "bad", "x́@@owner", "x‿@@owner", "𐐀@team", "(@@owner\u00a0tail)", "a@example.com", "foo@team\u0085@bar", "a@😀", "@.hidden", "@group/nested/", "owner#@foo", "@@unknown", "@@owner\u000b@@developer"]
    headers = ["[Docs]", "[DOCS] @default", "^[Docs][0] @default", "[Docs][ 1 ] @team", "[Docs][] @owner", "[Docs][x] @leaked", "[Docs]@@owner", "[Docs] @owner! @leaked", "[ ] @owner", "[𐐀] @team", "[𐐨] @team", "[Broken", "[x][١] @team", "[x] 𐐀@team", "[x] @team\u001c@tail"]
    cases = []
    for index in range(1800):
        dialect = rng.choice(["GitHub", "GitLab"])
        rules = []
        for _ in range(rng.randint(1, 8)):
            if rng.randrange(6) == 0:
                rules.append(rng.choice(headers))
            else:
                rules.append(rng.choice(patterns) + rng.choice([" ", "\t", "  "]) + " ".join(rng.sample(owners, rng.randint(0, 3))))
        paths = ["/" + "/".join(rng.choices(parts, k=rng.randint(1, 5))) for _ in range(6)] + ["", ".", "/foo", "/foo/a"]
        cases.append(dict(Name=f"differential-seed-20261007-{index}", Dialect=dialect, Rules="\n".join(rules), Paths=paths))
    return cases


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", type=Path, required=True, help="dd-trace-dotnet archive at the recorded SHA")
    parser.add_argument("--csc", type=Path, required=True, help="Roslyn csc.dll from an installed .NET SDK")
    parser.add_argument("--reference-dir", type=Path, required=True, help="Microsoft.NETCore.App.Ref ref/net10.0 directory")
    parser.add_argument("--runtime-version", default="10.0.11")
    parser.add_argument("--fluentassertions", type=Path, required=True)
    parser.add_argument("--xunit-core", type=Path, required=True)
    parser.add_argument("--xunit-assert", type=Path, required=True)
    parser.add_argument("--temp-dir", type=Path, required=True, help="Owned disk-backed directory for temporary output")
    args = parser.parse_args()
    inventory = json.loads((PACKAGE / "testdata/dotnet-tests.json").read_text())
    if inventory["commit"] != SHA:
        raise ValueError("Review and update the generator's pinned SHA before changing the oracle")
    for entry in inventory["source_files"]:
        data = (args.source / entry["path"]).read_bytes()
        if hashlib.sha256(data).hexdigest() != entry["sha256"]:
            raise ValueError("Frozen upstream source differs: " + entry["path"])
    with tempfile.TemporaryDirectory(prefix="dotnet-codeownership-", dir=args.temp_dir) as temporary:
        root = Path(temporary)
        for name in ["CodeOwners.cs", "CodeOwners.GitHub.cs", "CodeOwners.GitLab.cs"]:
            shutil.copy(args.source / CORE / name, root / name)
        for name in ["CodeOwnersSpecTests.cs", "CodeOwnersTests.cs"]:
            text = (args.source / TESTS / name).read_text()
            text = text.replace("using Datadog.Trace.Ci.CodeOwnership;", "using Datadog.Trace.Ci.CodeOwnership;\nusing CodeOwners = CaptureCodeOwners;")
            (root / name).write_text(text)
        for file in SUPPORT.glob("*.cs"):
            if file.name == "UnicodeFacts.cs":
                continue
            shutil.copy(file, root / file.name)
        (root / "GlobalUsings.cs").write_text("global using System;\nglobal using System.IO;\nglobal using System.Collections.Generic;\n")
        for name in ["CODEOWNERS_GITHUB", "CODEOWNERS_GITLAB"]:
            target = root / FIXTURES / name
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy(args.source / FIXTURES / name, target)
        libraries = [args.fluentassertions, args.xunit_core, args.xunit_assert]
        for library in libraries:
            shutil.copy(library, root / library.name)
        response = ["-nologo", "-target:exe", "-nullable:enable", "-langversion:preview", "-out:" + str(root / "Tests.dll")]
        response += ["-r:" + str(p) for p in sorted(args.reference_dir.glob("*.dll"))]
        response += ["-r:" + str(p) for p in libraries]
        response += [str(p) for p in sorted(root.glob("*.cs"))]
        (root / "refs.rsp").write_text("\n".join('"' + line + '"' for line in response))
        config = {"runtimeOptions": {"tfm": "net10.0", "framework": {"name": "Microsoft.NETCore.App", "version": args.runtime_version}}}
        (root / "Tests.runtimeconfig.json").write_text(json.dumps(config))
        (root / "edge-cases.json").write_text(json.dumps(edge_cases(), ensure_ascii=False))
        subprocess.run(["dotnet", str(args.csc), "@" + str(root / "refs.rsp")], check=True)
        environment = dict(os.environ, CODEOWNERS_TEST_ROOT=str(root))
        subprocess.run(["dotnet", str(root / "Tests.dll"), str(root / "corpus.json"), str(root), str(root / "edge-cases.json")], check=True, env=environment)
        corpus = json.loads((root / "corpus.json").read_text())
        actual = {run["name"] for run in corpus["runs"]}
        if actual != set(inventory["upstream_methods"]):
            raise ValueError("The upstream method inventory is incomplete")
        def save_corpus(name, content):
            data = json.dumps(content, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode()
            (PACKAGE / "testdata" / name).write_bytes(gzip.compress(data, mtime=0))
        save_corpus("dotnet-spec.json.gz", corpus)
        (root / "edge-cases.json").write_text(json.dumps(differential_cases(), ensure_ascii=False))
        subprocess.run(["dotnet", str(root / "Tests.dll"), str(root / "differential.json"), str(root), str(root / "edge-cases.json"), "differential"], check=True, env=environment)
        differential = json.loads((root / "differential.json").read_text())
        save_corpus("dotnet-differential.json.gz", differential)
        unicode_response = ["-nologo", "-target:exe", "-out:" + str(root / "UnicodeFacts.dll")]
        unicode_response += ["-r:" + str(p) for p in sorted(args.reference_dir.glob("*.dll"))]
        unicode_response += [str(SUPPORT / "UnicodeFacts.cs")]
        (root / "unicode.rsp").write_text("\n".join('"' + line + '"' for line in unicode_response))
        shutil.copy(root / "Tests.runtimeconfig.json", root / "UnicodeFacts.runtimeconfig.json")
        subprocess.run(["dotnet", str(args.csc), "@" + str(root / "unicode.rsp")], check=True)
        result = subprocess.run(["dotnet", str(root / "UnicodeFacts.dll")], check=True, capture_output=True, text=True)
        save_corpus("dotnet-unicode.json.gz", json.loads(result.stdout))
        print(f"{len(corpus['runs'])} original test runs; {len(corpus['queries'])} recorded queries; {len(differential['queries'])} differential queries")



if __name__ == "__main__":
    main()
