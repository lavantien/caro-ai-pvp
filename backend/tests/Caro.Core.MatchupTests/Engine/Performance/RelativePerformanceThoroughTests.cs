using Caro.Core.GameLogic;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.MatchupTests.Engine.Performance;

[Trait("Category", "Integration")]
public class RelativePerformanceThoroughTests : IDisposable
{
    private readonly ITestOutputHelper _output;

    private static readonly AIDifficulty[] AllDifficulties =
    {
        AIDifficulty.Braindead, AIDifficulty.Easy, AIDifficulty.Medium,
        AIDifficulty.Hard, AIDifficulty.Grandmaster
    };

    public RelativePerformanceThoroughTests(ITestOutputHelper output)
    {
        _output = output;
    }

    public void Dispose() => GC.SuppressFinalize(this);

    [Fact]
    public void RoundRobin_FullEloRanking()
    {
        var eloScores = new Dictionary<AIDifficulty, double>();
        var gameRecords = new List<string>();

        for (int i = 0; i < AllDifficulties.Length; i++)
        {
            for (int j = i + 1; j < AllDifficulties.Length; j++)
            {
                var diff1 = AllDifficulties[i];
                var diff2 = AllDifficulties[j];

                var stats = MatchupTestHelper.RunMatchupWithStatistics(
                    diff2, diff1,
                    games: 10,
                    output: _output);

                var record = $"{diff2} vs {diff1}: " +
                             $"W{stats.RedPlayerWins}/L{stats.BluePlayerWins}/D{stats.Draws} " +
                             $"Elo={stats.EloDifference:F0} LOS={stats.LikelihoodOfSuperiority:P0}";
                gameRecords.Add(record);
                _output.WriteLine($"  {record}");

                if (!eloScores.ContainsKey(diff1)) eloScores[diff1] = 1000;
                if (!eloScores.ContainsKey(diff2)) eloScores[diff2] = 1000;
                eloScores[diff2] += stats.EloDifference / 2;
                eloScores[diff1] -= stats.EloDifference / 2;
            }
        }

        _output.WriteLine("");
        _output.WriteLine("  === FINAL ELO RANKING ===");
        var rank = 1;
        foreach (var (diff, elo) in eloScores.OrderByDescending(kv => kv.Value))
        {
            _output.WriteLine($"  #{rank}: {diff} = {elo:F0}");
            rank++;
        }

        _output.WriteLine("");
        _output.WriteLine("  === ALL MATCHUPS ===");
        foreach (var record in gameRecords)
            _output.WriteLine($"  {record}");
    }
}
