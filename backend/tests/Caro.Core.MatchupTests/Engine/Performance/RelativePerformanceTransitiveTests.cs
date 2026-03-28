using Caro.Core.GameLogic;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.MatchupTests.Engine.Performance;

[Trait("Category", "Integration")]
public class RelativePerformanceTransitiveTests : IDisposable
{
    private readonly ITestOutputHelper _output;

    private static readonly AIDifficulty[] DifficultyOrder =
    {
        AIDifficulty.Braindead, AIDifficulty.Easy, AIDifficulty.Medium,
        AIDifficulty.Hard, AIDifficulty.Grandmaster
    };

    public RelativePerformanceTransitiveTests(ITestOutputHelper output)
    {
        _output = output;
    }

    public void Dispose() => GC.SuppressFinalize(this);

    [Fact]
    public void TransitiveOrdering_MonotonicStrength()
    {
        // Run all 10 adjacent + non-adjacent pairs and verify monotonic Elo ordering
        var eloScores = new Dictionary<AIDifficulty, double>();
        var violations = new List<string>();

        for (int i = 0; i < DifficultyOrder.Length; i++)
        {
            for (int j = i + 1; j < DifficultyOrder.Length; j++)
            {
                var higher = DifficultyOrder[i + 1]; // Higher difficulty
                var lower = DifficultyOrder[i];       // Lower difficulty
                // Actually we want higher index = higher difficulty
                higher = DifficultyOrder[j];
                lower = DifficultyOrder[i];

                var stats = MatchupTestHelper.RunMatchupWithStatistics(
                    higher, lower,
                    games: 10,
                    output: _output);

                _output.WriteLine($"  {higher} vs {lower}: " +
                                  $"W/L/D {stats.RedPlayerWins}/{stats.BluePlayerWins}/{stats.Draws} " +
                                  $"ELO={stats.EloDifference:F0}");

                // Accumulate Elo for ranking
                if (!eloScores.ContainsKey(higher)) eloScores[higher] = 0;
                if (!eloScores.ContainsKey(lower)) eloScores[lower] = 0;
                eloScores[higher] += stats.EloDifference;
                eloScores[lower] -= stats.EloDifference;

                if (stats.RedPlayerWins <= stats.BluePlayerWins)
                {
                    violations.Add($"{lower} matched or beat {higher} " +
                                   $"({stats.RedPlayerWins}W vs {stats.BluePlayerWins}W)");
                }
            }
        }

        // Output ranking
        var ranking = eloScores
            .OrderByDescending(kv => kv.Value)
            .Select(kv => $"  {kv.Key}: {kv.Value:F0} Elo (cumulative)")
            .ToList();
        _output.WriteLine("  Cumulative Elo ranking:");
        foreach (var line in ranking)
            _output.WriteLine(line);

        // Verify monotonic ordering: Braindead < Easy < Medium < Hard < Grandmaster
        for (int i = 0; i < DifficultyOrder.Length - 1; i++)
        {
            var weaker = DifficultyOrder[i];
            var stronger = DifficultyOrder[i + 1];
            Assert.True(eloScores[stronger] > eloScores[weaker],
                $"Transitive violation: {stronger} Elo ({eloScores[stronger]:F0}) " +
                $"not higher than {weaker} ({eloScores[weaker]:F0})");
        }

        Assert.Empty(violations);
    }
}
