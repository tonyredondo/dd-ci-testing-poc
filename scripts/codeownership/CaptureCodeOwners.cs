using System;
using System.IO;
using System.Collections.Generic;
using Original = Datadog.Trace.Ci.CodeOwnership.CodeOwners;

// Record calls while the original parser and its assertions stay unchanged.
internal sealed class CaptureCodeOwners
{
    internal enum Dialect { GitHub, GitLab }

    internal static string CurrentTest = "";
    internal static readonly List<Input> Inputs = new();
    internal static readonly List<Query> Queries = new();

    private readonly Original _original;
    private readonly int _input;

    internal sealed record Input(string Name, string Dialect, string Mode, string Rules);
    internal sealed record Query(int Input, string? Path, string[] Owners, int Diagnostics, bool HasPath);

    private CaptureCodeOwners(Original original, string rules, Dialect dialect, string mode)
    {
        _original = original;
        _input = Inputs.Count;
        Inputs.Add(new(CurrentTest, dialect.ToString(), mode, rules));
        Queries.Add(new(_input, null, [], original.ParsingDiagnosticsCount, false));
    }

    internal CaptureCodeOwners(string path, Dialect dialect)
        : this(new Original(path, (Original.Dialect)dialect), File.ReadAllText(path), dialect, "load")
    {
    }

    internal static CaptureCodeOwners Parse(IEnumerable<string> lines, Dialect dialect)
    {
        var rules = string.Join("\n", lines);
        var original = Original.Parse(rules.Split('\n'), (Original.Dialect)dialect);
        return new(original, rules, dialect, "parse");
    }

    internal string[] Match(string? path)
    {
        var owners = _original.Match(path!);
        lock (Queries)
        {
            Queries.Add(new(_input, path, owners, _original.ParsingDiagnosticsCount, true));
        }

        return owners;
    }

    internal int ParsingDiagnosticsCount => _original.ParsingDiagnosticsCount;
    internal const long GitHubMaximumFileSizeBytes = Original.GitHubMaximumFileSizeBytes;

    internal static bool TryLoad(string path, Dialect dialect, out CaptureCodeOwners? value)
    {
        if (!Original.TryLoad(path, (Original.Dialect)dialect, out var original))
        {
            value = null;
            return false;
        }

        value = new(original, File.ReadAllText(path), dialect, "load");
        return true;
    }
}
