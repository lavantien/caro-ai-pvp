using Caro.Core.GameLogic;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.MatchupTests.Engine.Performance;

[Trait("Category", "Smoke")]
public class RelativePerformanceSmokeTests : IDisposable
{
    private readonly ITestOutputHelper _output;

    public RelativePerformanceSmokeTests(ITestOutputHelper output)
    {
        _output = output;
    }

    public void Dispose() => GC.SuppressFinalize(this);

    [Fact]
    public void Smoke_AdjacentPairs_NoInversion()
    {
        var adjacentPairs = new (AIDifficulty higher, AIDifficulty lower)[]
        {
            (AIDifficulty.Hard, AIDifficulty.Medium),
            (AIDifficulty.Medium, AIDifficulty.Easy),
            (AIDifficulty.Easy, AIDifficulty.Braindead),
        };

        foreach (var (higher, lower) in adjacentPairs)
        {
            var stats = MatchupTestHelper.RunMatchupWithStatistics(
                higher, lower,
                games: MatchupTestConfig.GamesPerSmokePair,
                output: _output,
                initialTimeSeconds: 120,
                incrementSeconds: 2);

            _output.WriteLine($"  {higher} vs {lower}: " +
                              $"RedWins={stats.RedPlayerWins} BlueWins={stats.BluePlayerWins} " +
                              $"Draws={stats.Draws} ELO={stats.EloDifference:F0}");

            // The higher difficulty should win more games than the lower
            Assert.True(stats.RedPlayerWins > stats.BluePlayerWins,
                $"INVERSION: {lower} beat {higher} " +
                $"({stats.RedPlayerWins} vs {stats.BluePlayerWins} draws={stats.Draws})");
        }
    }

    [Fact]
    public void Smoke_ExtremeGap_GrandmasterBeatsBraindead()
    {
        var stats = MatchupTestHelper.RunMatchupWithStatistics(
            AIDifficulty.Grandmaster, AIDifficulty.Braindead,
            games: MatchupTestConfig.GamesPerSmokePair,
            output: _output,
            initialTimeSeconds: 120,
            incrementSeconds: 2);

        _output.WriteLine($"  Grandmaster vs Braindead: " +
                          $"RedWins={stats.RedPlayerWins} BlueWins={stats.BluePlayerWins} " +
                          $"Draws={stats.Draws}");

        // Grandmaster should win ALL games against Braindead
        Assert.Equal(0, stats.BluePlayerWins);
        Assert.True(stats.RedPlayerWins == MatchupTestConfig.GamesPerSmokePair,
            $"Grandmaster failed to win all games vs Braindead: " +
            $"{stats.RedPlayerWins}W-{stats.BluePlayerWins}L-{stats.Draws}D");
    }

    [Fact]
    public void Smoke_SelfPlay_Symmetric()
    {
        var stats = MatchupTestHelper.RunMatchupWithStatistics(
            AIDifficulty.Grandmaster, AIDifficulty.Grandmaster,
            games: 2,
            output: _output,
            initialTimeSeconds: 120,
            incrementSeconds: 2);

        var totalDecisive = stats.RedPlayerWins + stats.BluePlayerWins;
        if (totalDecisive > 0)
        {
            var redRate = (double)stats.RedPlayerWins / totalDecisive;
            Assert.InRange(redRate, 0.10, 0.90);
        }

        _output.WriteLine($"  GM vs GM: RedWins={stats.RedPlayerWins} BlueWins={stats.BluePlayerWins} " +
                          $"Draws={stats.Draws}");
    }
}
