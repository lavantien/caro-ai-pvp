using Caro.Core.GameLogic;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.MatchupTests.Engine.Performance;

[Trait("Category", "Integration")]
public class RelativePerformanceSelfPlayTests : IDisposable
{
    private readonly ITestOutputHelper _output;

    public static readonly TheoryData<AIDifficulty> AllDifficulties = new()
    {
        AIDifficulty.Braindead,
        AIDifficulty.Easy,
        AIDifficulty.Medium,
        AIDifficulty.Hard,
        AIDifficulty.Grandmaster,
    };

    public RelativePerformanceSelfPlayTests(ITestOutputHelper output)
    {
        _output = output;
    }

    public void Dispose() => GC.SuppressFinalize(this);

    [Theory(DisplayName = "SelfPlay_NoColorAdvantage")]
    [MemberData(nameof(AllDifficulties))]
    public void SelfPlay_NoColorAdvantage(AIDifficulty difficulty)
    {
        var stats = MatchupTestHelper.RunMatchupWithStatistics(
            difficulty, difficulty,
            games: MatchupTestConfig.GamesPerSelfPlay,
            output: _output);

        _output.WriteLine($"  {difficulty} vs {difficulty}:");
        _output.WriteLine($"    W/L/D: {stats.RedPlayerWins}/{stats.BluePlayerWins}/{stats.Draws}");
        _output.WriteLine($"    Color advantage: {stats.HasColorAdvantage} (effect={stats.ColorAdvantageEffectSize:F3})");

        var totalDecisive = stats.RedPlayerWins + stats.BluePlayerWins;
        if (totalDecisive > 0)
        {
            var redWinRate = (double)stats.RedPlayerWins / totalDecisive;
            _output.WriteLine($"    Red win rate: {redWinRate:P1}");

            Assert.InRange(redWinRate,
                MatchupTestConfig.SelfPlayMinRedWinRate,
                MatchupTestConfig.SelfPlayMaxRedWinRate);
        }
        else
        {
            _output.WriteLine("    All games were draws - no color advantage possible");
        }
    }
}
