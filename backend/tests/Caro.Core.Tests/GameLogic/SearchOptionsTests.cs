using Caro.Core.Domain.Configuration;
using FluentAssertions;
using Xunit;

namespace Caro.Core.Tests.GameLogic;

public sealed class SearchOptionsTests
{
    [Fact]
    public void SearchOptions_Default_TimeFractionIsOneAndUseVCFIsTrue()
    {
        var opts = SearchOptions.Default;
        opts.TimeFraction.Should().BeApproximately(1.0, 0.001);
        opts.UseVCF.Should().BeTrue();
    }

    [Fact]
    public void SearchOptions_CanSetTimeFractionAndUseVCF()
    {
        var opts = new SearchOptions { TimeFraction = 0.15, UseVCF = false };
        opts.TimeFraction.Should().BeApproximately(0.15, 0.001);
        opts.UseVCF.Should().BeFalse();
    }

    [Fact]
    public void SearchOptions_TimeFractionAboveOne_Throws()
    {
        ((Action)(() => new SearchOptions { TimeFraction = 1.5 })).Should()
            .Throw<ArgumentOutOfRangeException>();
    }

    [Fact]
    public void SearchOptions_TimeFractionNegative_Throws()
    {
        ((Action)(() => new SearchOptions { TimeFraction = -0.1 })).Should()
            .Throw<ArgumentOutOfRangeException>();
    }

    [Fact]
    public void SearchOptions_TimeFractionZero_IsAllowed()
    {
        var opts = new SearchOptions { TimeFraction = 0.0 };
        opts.TimeFraction.Should().BeApproximately(0.0, 0.001);
    }
}
