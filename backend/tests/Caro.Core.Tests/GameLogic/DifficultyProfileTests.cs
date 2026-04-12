using Caro.Core.GameLogic;
using FluentAssertions;

namespace Caro.Core.Tests.GameLogic;

public sealed class DifficultyProfileTests
{
    [Theory]
    [InlineData(1, "Novice")]
    [InlineData(2, "Beginner")]
    [InlineData(3, "Intermediate")]
    [InlineData(4, "Advanced")]
    [InlineData(5, "Grandmaster")]
    public void GetName_ReturnsCorrectName(int level, string expectedName)
    {
        DifficultyProfile.GetName(level).Should().Be(expectedName);
    }

    [Fact]
    public void GetName_InvalidLevel_Throws()
    {
        ((Action)(() => DifficultyProfile.GetName(0))).Should().Throw<ArgumentOutOfRangeException>();
        ((Action)(() => DifficultyProfile.GetName(6))).Should().Throw<ArgumentOutOfRangeException>();
    }

    [Theory]
    [InlineData(1, 1)]
    [InlineData(2, 1)]
    [InlineData(3, 2)]
    public void GetThreadCount_LowLevels_FixedValue(int level, int expected)
    {
        DifficultyProfile.GetThreadCount(level).Should().Be(expected);
    }

    [Fact]
    public void GetThreadCount_L4_Max2HalfProcessors()
    {
        DifficultyProfile.GetThreadCount(4).Should().Be(Math.Max(2, Environment.ProcessorCount / 2));
    }

    [Fact]
    public void GetThreadCount_L5_PowerOf2()
    {
        var result = DifficultyProfile.GetThreadCount(5);
        result.Should().BeGreaterThanOrEqualTo(1);
        (result & (result - 1)).Should().Be(0); // power of 2
    }

    [Theory]
    [InlineData(1, 0.05)]
    [InlineData(2, 0.15)]
    [InlineData(3, 0.40)]
    [InlineData(4, 0.70)]
    [InlineData(5, 1.00)]
    public void GetTimeFraction_ReturnsCorrectValue(int level, double expected)
    {
        DifficultyProfile.GetTimeFraction(level).Should().BeApproximately(expected, 0.001);
    }

    [Theory]
    [InlineData(1, false)]
    [InlineData(2, false)]
    [InlineData(3, false)]
    [InlineData(4, false)]
    [InlineData(5, true)]
    public void GetPonderingEnabled(int level, bool expected)
    {
        DifficultyProfile.GetPonderingEnabled(level).Should().Be(expected);
    }

    [Theory]
    [InlineData(1, false)]
    [InlineData(2, false)]
    [InlineData(3, true)]
    [InlineData(4, true)]
    [InlineData(5, true)]
    public void GetUseVCF(int level, bool expected)
    {
        DifficultyProfile.GetUseVCF(level).Should().Be(expected);
    }

    [Theory]
    [InlineData(1, false)]
    [InlineData(2, false)]
    [InlineData(3, true)]
    [InlineData(4, true)]
    [InlineData(5, true)]
    public void GetParallelSearchEnabled(int level, bool expected)
    {
        DifficultyProfile.GetParallelSearchEnabled(level).Should().Be(expected);
    }
}
