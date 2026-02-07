using Xunit;

namespace Caro.Core.Tests;

/// <summary>
/// Marks a test as slow and excludes it from default test runs.
/// Use this for tests that involve AI search and take significant time.
/// </summary>
public sealed class SlowFactAttribute : FactAttribute
{
    public SlowFactAttribute()
    {
        Skip = "Slow test - run with explicit filter Category=Slow to enable";
    }
}

/// <summary>
/// Marks a theory as slow and excludes it from default test runs.
/// </summary>
public sealed class SlowTheoryAttribute : TheoryAttribute
{
    public SlowTheoryAttribute()
    {
        Skip = "Slow test - run with explicit filter Category=Slow to enable";
    }
}

/// <summary>
/// Marks a test as debug-only and excludes it from default test runs.
/// </summary>
public sealed class DebugFactAttribute : FactAttribute
{
    public DebugFactAttribute()
    {
        Skip = "Debug test - run with explicit filter Category=Debug to enable";
    }
}

/// <summary>
/// Marks a test as stress/integration and excludes it from default test runs.
/// </summary>
public sealed class StressFactAttribute : FactAttribute
{
    public StressFactAttribute()
    {
        Skip = "Stress test - run with explicit filter Category=Stress to enable";
    }
}
