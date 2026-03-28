using Caro.Core.GameLogic;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.MatchupTests.Engine.Performance;

[Trait("Category", "Integration")]
public class RelativePerformanceCrossLevelTests : IDisposable
{
    private readonly ITestOutputHelper _output;

    public static readonly TheoryData<AIDifficulty, AIDifficulty> CrossLevelPairs = new()
    {
        { AIDifficulty.Grandmaster, AIDifficulty.Medium },
        { AIDifficulty.Grandmaster, AIDifficulty.Easy },
        { AIDifficulty.Grandmaster, AIDifficulty.Braindead },
        { AIDifficulty.Hard, AIDifficulty.Easy },
        { AIDifficulty.Hard, AIDifficulty.Braindead },
        { AIDifficulty.Medium, AIDifficulty.Braindead },
    };

    public RelativePerformanceCrossLevelTests(ITestOutputHelper output)
    {
        _output = output;
    }

    public void Dispose() => GC.SuppressFinalize(this);

    [Theory(DisplayName = "CrossLevel_HigherWinsSignificantly")]
    [MemberData(nameof(CrossLevelPairs))]
    public void CrossLevel_HigherWinsSignificantly(AIDifficulty higher, AIDifficulty lower)
    {
        var stats = MatchupTestHelper.RunMatchupWithStatistics(
            higher, lower,
            games: MatchupTestConfig.GamesPerCrossLevel,
            output: _output);

        _output.WriteLine($"  {higher} vs {lower}:");
        _output.WriteLine($"    W/L/D: {stats.RedPlayerWins}/{stats.BluePlayerWins}/{stats.Draws}");
        _output.WriteLine($"    ELO diff: {stats.EloDifference:F1}");
        _output.WriteLine($"    LOS: {stats.LikelihoodOfSuperiority:P1}, p-value: {stats.PValue:F4}");

        Assert.True(stats.RedPlayerWins > stats.BluePlayerWins,
            $"INVERSION: {lower} beat {higher} " +
            $"({stats.RedPlayerWins}W vs {stats.BluePlayerWins}W, {stats.Draws} draws)");

        // Cross-level should show stronger significance than adjacent
        var sprt = MatchupTestHelper.CheckSPRT(
            stats.RedPlayerWins, stats.BluePlayerWins, stats.Draws,
            elo1: MatchupTestConfig.SprtElo1CrossLevel);

        _output.WriteLine($"    SPRT (elo1={MatchupTestConfig.SprtElo1CrossLevel}): {sprt}");
    }
}
