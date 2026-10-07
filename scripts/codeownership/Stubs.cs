
namespace Datadog.Trace.SourceGenerators { [System.AttributeUsage(System.AttributeTargets.All)] internal sealed class TestingAndPrivateOnlyAttribute : System.Attribute {} }
namespace Datadog.Trace.Logging {
 internal interface IDatadogLogger {
  void Warning<T>(string message,T value);
  void Warning<T1,T2>(string message,T1 first,T2 second);
  void Warning(System.Exception ex,string message,string value);
 }
 internal static class DatadogLogging { internal static IDatadogLogger GetLoggerFor<T>() => new NullLog(); }
 internal sealed class NullLog : IDatadogLogger {
  public void Warning<T>(string message,T value) {}
  public void Warning<T1,T2>(string message,T1 first,T2 second) {}
  public void Warning(System.Exception ex,string message,string value) {}
 }
}

namespace Datadog.Trace.TestHelpers { public static class EnvironmentTools { public static string GetSolutionDirectory() => System.Environment.GetEnvironmentVariable("CODEOWNERS_TEST_ROOT")!; } }
