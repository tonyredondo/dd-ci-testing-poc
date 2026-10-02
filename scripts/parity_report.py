#!/usr/bin/env python3
"""Render differential evidence without turning a known gap into a parity claim."""

import argparse
import json
from pathlib import Path


COUNT_FIELDS = ("sessions", "modules", "suites", "tests", "spans")


def counts(value):
    return "/".join(str(value[key]) for key in COUNT_FIELDS)


def cell(value):
    return str(value).replace("|", "\\|").replace("\n", " ")


def render(path):
    report = json.loads(path.read_text())
    rows = report["scenarios"]
    if report["sdk_instrumentation"] != "orchestrion":
        raise ValueError("full SDK/Orchestrion reference was not executed")
    if len(rows) < 65 or len({row["scenario"] for row in rows}) != len(rows):
        raise ValueError("missing or duplicated matrix scenarios")
    for row in rows:
        if row["status"] != "passed" or row["sdk"] != row["mini"] or row["sdk_exit"] != row["mini_exit"]:
            raise ValueError("failed scenario: " + row["scenario"])

    evidence = {}
    names = ["manual", "spans", "packages", "fuzz", "telemetry", "testify"]
    if report["os"] in ("linux", "darwin"):
        names.append("uds")
    for name in names:
        evidence[name] = json.loads(path.with_name(path.stem + "-" + name + ".json").read_text())
    for name, item in evidence.items():
        if name == "telemetry":
            if not item["semantic_counts_equal"] or not item["request_counts_match_http"]:
                raise ValueError("CI telemetry comparison was not verified")
        elif name != "testify" and (item["status"] != "passed" or item["sdk"] != item["mini"]):
            raise ValueError("failed additional fixture: " + name)
    testify = evidence["testify"]
    if testify["status"] != "gap":
        raise ValueError("Testify status changed; review and update the parity contract")

    totals = {key: sum(row["sdk"][key] for row in rows) for key in COUNT_FIELDS}
    lines = [
        "# CI Visibility parity evidence", "",
        "Complete feature parity: **no**. The Testify instrumentation gap remains.", "",
        f"SDK base: `{report['sdk_commit']}` (`{report['sdk_version']}`).",
        f"Runner: `{report['os']}/{report['architecture']}`, `{report['go']}`.", "",
        "Counts below are sessions/modules/suites/tests/spans. Every matrix row",
        "also compares CI attributes, exit status and hierarchy references;",
        "coverage and side payloads are checked when that feature is selected.", "",
        f"Matrix: {len(rows)} passing scenarios; aggregate SDK = Mini `{counts(totals)}`.", "",
        "| Scenario | Features | SDK | Mini | Result |",
        "| --- | --- | --- | --- | --- |",
    ]
    for row in rows:
        lines.append(f"| {cell(row['scenario'])} | {cell(', '.join(row['features']))} | {counts(row['sdk'])} | {counts(row['mini'])} | Passed |")
    lines += ["", "## Additional fixtures", "",
              "| Fixture | SDK | Mini | Scope |", "| --- | --- | --- | --- |"]
    for name in names:
        if name in ("telemetry", "testify"):
            continue
        item = evidence[name]
        lines.append(f"| {name} | {counts(item['sdk'])} | {counts(item['mini'])} | {cell(item['scope'])} |")
    lines += ["", "CI telemetry: semantic count/rate metrics match; request counts are",
              "checked against each sender's actual HTTP requests. Batch sizes and",
              "timings may differ. Distributions are outside this counter fixture.", "",
              f"Testify: full SDK `{counts(testify['sdk_with_orchestrion'])}`, POC SDK `{counts(testify['sdk_with_poc'])}`, Mini `{counts(testify['mini'])}`.",
              f"Known gap: {cell(testify['reason'])}.", "",
              "This is loopback protocol evidence. Real intake/UI acceptance, an actual",
              "Bazel toolchain run and an external APM shim are unverified. Read",
              "`docs/ci-parity.md` for the exclusions and remaining feature coverage.", ""]
    return "\n".join(lines)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("report", type=Path)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    text = render(args.report)
    args.output.write_text(text)


if __name__ == "__main__":
    main()
