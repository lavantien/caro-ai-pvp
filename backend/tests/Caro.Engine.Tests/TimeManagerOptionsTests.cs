using Caro.Domain;
using Xunit;

namespace Caro.Engine.Tests;

public class TimeManagerOptionsTests
{
    [Fact]
    public void NullOptionsMatchCompiledDefaults()
    {
        TimeAllocation withNull = TimeManager.AllocateTime(60_000, 5_000, 10, null);
        TimeAllocation withDefaults = TimeManager.AllocateTime(60_000, 5_000, 10, new TimeManagementOptions());

        Assert.Equal(withDefaults, withNull);
    }

    [Fact]
    public void OptionsOverrideDivisorScalesOptimal()
    {
        // Halving the early divisor doubles the base spend.
        TimeManagementOptions options = new() { };
        options.PhaseDivisorEarly = Constants.TimeManagement.PhaseDivisorEarly / 2;

        TimeAllocation doubled = TimeManager.AllocateTime(60_000, 0, 1, options);
        TimeAllocation baseline = TimeManager.AllocateTime(60_000, 0, 1, null);

        Assert.Equal(baseline.OptimalMs * 2, doubled.OptimalMs);
    }

    [Fact]
    public void OptionsMaxFractionCapsOptimal()
    {
        // Base spend at 60s/25 is 2400, so a 2% cap is the binding limit.
        TimeManagementOptions options = new() { };
        options.MaxFraction = 0.02;

        TimeAllocation alloc = TimeManager.AllocateTime(60_000, 0, 1, options);

        Assert.Equal(1_200, alloc.OptimalMs);
    }

    [Fact]
    public void OptionsPhaseSwitchChangesDivisor()
    {
        TimeManagementOptions options = new() { };
        options.PhaseSwitchMove = 5;

        TimeAllocation early = TimeManager.AllocateTime(60_000, 0, 3, options);
        TimeAllocation late = TimeManager.AllocateTime(60_000, 0, 30, options);

        // Late divisor (30) is larger than early (25), so the same clock
        // spends less per move after the switch.
        Assert.True(early.OptimalMs > late.OptimalMs);
    }

    [Fact]
    public void OptionsReserveShrinksHardBound()
    {
        TimeManagementOptions options = new() { };
        options.ReserveMs = 20_000;

        TimeAllocation alloc = TimeManager.AllocateTime(60_000, 0, 1, options);

        Assert.True(alloc.HardBoundMs <= 60_000 - 20_000);
    }
}
