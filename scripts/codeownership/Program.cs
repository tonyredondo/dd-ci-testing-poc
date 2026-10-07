using System.Linq;
using System.Reflection;
using System.Text.Json;
using Xunit;
using Datadog.Trace.Tests.Ci;

var rows = new List<object>();
var testTypes = args.Length > 3 && args[3] == "differential"
                    ? Array.Empty<Type>()
                    : new[] { typeof(CodeOwnersSpecTests), typeof(CodeOwnersTests) };

foreach (var type in testTypes)
{
    foreach (var method in type.GetMethods().Where(m => m.DeclaringType == type))
    {
        var inputs = method.GetCustomAttributes<InlineDataAttribute>()
                           .Select(a => a.GetData(method).Single()).ToArray();
        if (inputs.Length == 0)
        {
            inputs = [[]];
        }

        foreach (var input in inputs)
        {
            try
            {
                CaptureCodeOwners.CurrentTest = method.Name;
                var suite = Activator.CreateInstance(type)!;
                method.Invoke(suite, input);
                rows.Add(new { name = method.Name, status = "passed" });
            }
            catch (Exception ex)
            {
                Console.Error.WriteLine(method.Name + ": " + ex);
                Environment.Exit(1);
            }
        }
    }
}

var options = new JsonSerializerOptions { PropertyNameCaseInsensitive = true };
foreach (var extra in JsonSerializer.Deserialize<ExtraCase[]>(File.ReadAllText(args[2]), options)!)
{
    CaptureCodeOwners.CurrentTest = extra.Name;
    var filename = Path.Combine(Path.GetTempPath(), Guid.NewGuid() + ".CODEOWNERS");
    try
    {
        File.WriteAllText(filename, extra.Rules);
        var owners = new CaptureCodeOwners(filename, Enum.Parse<CaptureCodeOwners.Dialect>(extra.Dialect));
        foreach (var path in extra.Paths)
        {
            owners.Match(path);
        }
    }
    finally
    {
        File.Delete(filename);
    }
}

File.WriteAllText(args[0], JsonSerializer.Serialize(new
{
    runs = rows,
    inputs = CaptureCodeOwners.Inputs,
    queries = CaptureCodeOwners.Queries,
}));
Console.WriteLine(rows.Count + " original test executions passed");

record ExtraCase(string Name, string Dialect, string Rules, string[] Paths);
