using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using FluentAssertions;

namespace Caro.Core.Tests.GameLogic;

public sealed class DifficultySearchOptionsMappingTests
{
    [Theory]
    [InlineData(1)]
    [InlineData(2)]
    [InlineData(3)]
    [InlineData(4)]
    [InlineData(5)]
    public void BuildSearchOptions_AllLevels_ProducesValidOptions(int level)
    {
        var opts = BuildFromDifficulty(level, 420_000, 5, 10);
        opts.TimeFraction.Should().BeApproximately(DifficultyProfile.GetTimeFraction(level), 0.001);
        opts.UseVCF.Should().Be(DifficultyProfile.GetUseVCF(level));
        opts.PonderingEnabled.Should().Be(DifficultyProfile.GetPonderingEnabled(level));
        opts.ParallelSearchEnabled.Should().Be(DifficultyProfile.GetParallelSearchEnabled(level));
        opts.ThreadCount.Should().Be(DifficultyProfile.GetThreadCount(level));
        opts.TimeRemainingMs.Should().Be(420_000);
        opts.IncrementSeconds.Should().Be(5);
        opts.MoveNumber.Should().Be(10);
    }

    [Fact]
    public void BuildSearchOptions_NullDifficulty_DefaultsToFullStrength()
    {
        var opts = BuildFromDifficulty(null, 420_000, 5, 10);
        opts.TimeFraction.Should().BeApproximately(1.0, 0.001);
        opts.UseVCF.Should().BeTrue();
        opts.PonderingEnabled.Should().BeTrue();
        opts.ParallelSearchEnabled.Should().BeTrue();
        opts.ThreadCount.Should().BeNull();
    }

    [Fact]
    public void ResolveDifficulty_RedPlayer_ReturnsRedDifficulty()
    {
        var (redDiff, blueDiff) = (5, 1);
        ResolveForPlayer(Player.Red, redDiff, blueDiff).Should().Be(5);
    }

    [Fact]
    public void ResolveDifficulty_BluePlayer_ReturnsBlueDifficulty()
    {
        var (redDiff, blueDiff) = (5, 1);
        ResolveForPlayer(Player.Blue, redDiff, blueDiff).Should().Be(1);
    }

    [Fact]
    public void ResolveDifficulty_RedNullBlueSet_ReturnsBlueForBlue()
    {
        int? redDiff = null;
        int? blueDiff = 3;
        ResolveForPlayer(Player.Blue, redDiff, blueDiff).Should().Be(3);
        ResolveForPlayer(Player.Red, redDiff, blueDiff).Should().BeNull();
    }

    private static int? ResolveForPlayer(Player player, int? redDifficulty, int? blueDifficulty)
    {
        return player == Player.Red ? redDifficulty : blueDifficulty;
    }

    private static SearchOptions BuildFromDifficulty(int? difficulty, long timeRemainingMs, int incrementSeconds, int moveNumber)
    {
        if (difficulty is int level and >= 1 and <= 5)
        {
            return new SearchOptions
            {
                TimeRemainingMs = timeRemainingMs, IncrementSeconds = incrementSeconds,
                MoveNumber = moveNumber, ThreadCount = DifficultyProfile.GetThreadCount(level),
                PonderingEnabled = DifficultyProfile.GetPonderingEnabled(level),
                ParallelSearchEnabled = DifficultyProfile.GetParallelSearchEnabled(level),
                TimeFraction = DifficultyProfile.GetTimeFraction(level),
                UseVCF = DifficultyProfile.GetUseVCF(level),
            };
        }
        return new SearchOptions
        {
            TimeRemainingMs = timeRemainingMs, IncrementSeconds = incrementSeconds,
            MoveNumber = moveNumber, PonderingEnabled = true, ParallelSearchEnabled = true,
        };
    }
}
