using Caro.Api;
using Caro.Domain;
using Xunit;

namespace Caro.Api.Tests;

/// <summary>
/// Time-control resolution. The canonical keys must match the frontend
/// select values exactly; the bullet/blitz/classical aliases are the Go
/// engine's legacy inputs and keep resolving.
/// </summary>
public class TimeControlsTests
{
    [Theory]
    [InlineData("1+0", "1+0", 60_000, 0)]
    [InlineData("3+2", "3+2", 180_000, 2)]
    [InlineData("3+0", "3+0", 180_000, 0)]
    [InlineData("7+5", "7+5", 420_000, 5)]
    [InlineData("10+0", "10+0", 600_000, 0)]
    [InlineData("15+10", "15+10", 900_000, 10)]
    public void CanonicalKeys(string key, string canonical, long initialMs, int increment)
    {
        (string c, long ms, int inc) = TimeControls.Resolve(key);
        Assert.Equal(canonical, c);
        Assert.Equal(initialMs, ms);
        Assert.Equal(increment, inc);
    }

    [Theory]
    [InlineData("bullet", "1+0", 60_000, 0)]
    [InlineData("blitz", "3+2", 180_000, 2)]
    [InlineData("classical", "15+10", 900_000, 10)]
    public void LegacyAliases(string alias, string canonical, long initialMs, int increment)
    {
        (string c, long ms, int inc) = TimeControls.Resolve(alias);
        Assert.Equal(canonical, c);
        Assert.Equal(initialMs, ms);
        Assert.Equal(increment, inc);
    }

    [Theory]
    [InlineData(null)]
    [InlineData("")]
    [InlineData("2+1")]
    [InlineData("rapid")]
    public void UnknownFallsBackToDefault(string? requested)
    {
        (string c, long ms, int inc) = TimeControls.Resolve(requested);
        Assert.Equal(Constants.TimeControl.Default, c);
        Assert.Equal(Constants.TimeControl.DefaultInitialTimeMs, ms);
        Assert.Equal(Constants.TimeControl.DefaultIncrementSeconds, inc);
    }
}
