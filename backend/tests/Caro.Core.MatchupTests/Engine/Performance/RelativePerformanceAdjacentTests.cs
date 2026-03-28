using Caro.Core.GameLogic;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.MatchupTests.Engine.Performance;

[Trait("Category", "Integration")]
public class RelativePerformanceAdjacentTests : IDisposable
{
    private readonly ITestOutputHelper _output;

    public static readonly TheoryData<AIDifficulty, AIDifficulty> AdjacentPairs = new()
    {
        { AIDifficulty.Grandmaster, AIDifficulty.Hard },
        { AIDifficulty.Hard, AIDifficulty.Medium },
        { AIDifficulty.Medium, AIDifficulty.Easy },
        { AIDifficulty.Easy, AIDifficulty.Braindead },
    };

    public RelativePerformanceAdjacentTests(ITestOutputHelper output)
    {
        _output = output;
    }

    public void Dispose() => GC.SuppressFinalize(this);

    [Theory(DisplayName = "AdjacentDifficulty_HigherWinsSignificantly")]
    [MemberData(nameof(AdjacentPairs))]
    public void AdjacentDifficulty_HigherWinsSignificantly(AIDifficulty higher, AIDifficulty lower)
    {
        var stats = MatchupTestHelper.RunMatchupWithStatistics(
            higher, lower,
            games: MatchupTestConfig.GamesPerAdjacentPair,
            output: _output);

        _output.WriteLine($"  {higher} vs {lower}:");
        _output.WriteLine($"    W/L/D: {stats.RedPlayerWins}/{stats.BluePlayerWins}/{stats.Draws}");
        _output.WriteLine($"    ELO diff: {stats.EloDifference:F1} ({stats.ConfidenceIntervalLower:F1}, {stats.ConfidenceIntervalUpper:F1})");
        _output.WriteLine($"    LOS: {stats.LikelihoodOfSuperiority:P1}, p-value: {stats.PValue:F4}");
        _output.WriteLine($"    Conclusion: {stats.Conclusion}");

        Assert.True(stats.RedPlayerWins > stats.BluePlayerWins,
            $"INVERSION: {lower} beat {higher} " +
            $"({stats.RedPlayerWins}W vs {stats.BluePlayerWins}W, {stats.Draws} draws)");

        // Check SPRT at intermediate points
        var sprt = MatchupTestHelper.CheckSPRT(
            stats.RedPlayerWins, stats.BluePlayerWins, stats.Draws);

        _output.WriteLine($"    SPRT: {sprt}");
    }
}
