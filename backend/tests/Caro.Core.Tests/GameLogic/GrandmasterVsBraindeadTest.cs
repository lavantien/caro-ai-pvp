using Caro.Core.Entities;
using Caro.Core.GameLogic;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.Tests.GameLogic;

public class GrandmasterVsBraindeadTest
{
    private readonly ITestOutputHelper _output;

    public GrandmasterVsBraindeadTest(ITestOutputHelper output)
    {
        _output = output;
    }

    [Fact]
    public void Braindead_ShouldUseLessTimeThan_Grandmaster()
    {
        var board = new Board();
        var gmAI = new MinimaxAI();
        var bdAI = new MinimaxAI();

        // Give both 5 seconds
        long timeMs = 5000;

        var sw = System.Diagnostics.Stopwatch.StartNew();
        gmAI.GetBestMove(board, Player.Red, AIDifficulty.Grandmaster, timeMs, moveNumber: 1, ponderingEnabled: false, parallelSearchEnabled: false);
        var gmTime = sw.ElapsedMilliseconds;

        sw.Restart();
        bdAI.GetBestMove(board, Player.Blue, AIDifficulty.Braindead, timeMs, moveNumber: 2, ponderingEnabled: false, parallelSearchEnabled: false);
        var bdTime = sw.ElapsedMilliseconds;

        _output.WriteLine($"Grandmaster time: {gmTime}ms");
        _output.WriteLine($"Braindead time: {bdTime}ms");

        // Braindead should use significantly less time than Grandmaster
        // The time multiplier (1% for Braindead, 100% for Grandmaster) should create
        // a proportional difference in search time, but exact values depend on machine capability
        Assert.True(bdTime < gmTime, $"Braindead time {bdTime}ms should be < Grandmaster time {gmTime}ms");
    }

    [Fact]
    public void Braindead_ShouldMakeRandomMoves_Approximately20PercentOfTime()
    {
        var board = new Board();
        var ai = new MinimaxAI();

        int randomMoveCount = 0;
        int totalMoves = 100;

        for (int i = 0; i < totalMoves; i++)
        {
            ai.GetBestMove(
                board,
                Player.Red,
                AIDifficulty.Braindead,
                timeRemainingMs: 420000,
                moveNumber: 1,
                ponderingEnabled: false,
                parallelSearchEnabled: false
            );

            // Check if it was an instant random move (depth 1, 1 node)
            var stats = ai.GetSearchStatistics();
            if (stats.DepthAchieved == 1 && stats.NodesSearched == 1)
            {
                randomMoveCount++;
            }
        }

        double actualErrorRate = (double)randomMoveCount / totalMoves;
        _output.WriteLine($"Error rate: {randomMoveCount}/{totalMoves} = {actualErrorRate:P1}");
        _output.WriteLine($"Expected: 20%");

        // Allow some variance due to randomness, but should be approximately 20%
        Assert.InRange(actualErrorRate, 0.10, 0.30);
    }
}
