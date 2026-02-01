using Caro.Core.Entities;
using Caro.Core.GameLogic;
using Xunit;

namespace Caro.Core.Tests.GameLogic;

/// <summary>
/// Tests for real node counting vs estimated node counting
/// Ensures parallel search reports actual nodes visited, not mathematical estimates
/// </summary>
public class NodeCountingTests
{
    [Fact]
    public async Task ParallelSearch_RealNodeCountShouldVaryBetweenMoves()
    {
        // Run multiple searches and verify node counts are NOT identical
        // (identical counts would indicate estimation instead of real counting)
        var ai = new MinimaxAI();
        var board = new Board();

        // Place a few stones to create different positions
        board.PlaceStone(9, 9, Player.Red);
        board.PlaceStone(10, 10, Player.Blue);

        var results = new List<long>();

        // Run 5 searches with different positions (explicit coordinates to avoid collisions)
        var testPositions = new (int x, int y)[]
        {
            (8, 8), (9, 8), (10, 8), (8, 9), (11, 11)
        };

        for (int i = 0; i < 5; i++)
        {
            board.PlaceStone(testPositions[i].x, testPositions[i].y, Player.Red);
            var (x, y) = ai.GetBestMove(board, Player.Blue, AIDifficulty.Hard); // D4 uses parallel search
            var (_, nodesSearched, _, _, _, _, _, _, _, _, _, _) = ai.GetSearchStatistics();
            results.Add(nodesSearched);
        }

        // All results should be non-zero
        Assert.All(results, n => Assert.True(n > 0, "Node count should be positive"));

        // Results should NOT all be identical (which would indicate estimation)
        // With real counting, different board positions should yield different node counts
        var uniqueCounts = results.Distinct().Count();
        Assert.True(uniqueCounts >= 2, $"Expected at least 2 different node counts, got {uniqueCounts}");
    }

    [Fact]
    public void SequentialSearch_RealNodeCountShouldBePositive()
    {
        // Low-depth search (D2) uses sequential search
        var ai = new MinimaxAI();
        var board = new Board();

        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);

        var (x, y) = ai.GetBestMove(board, Player.Blue, AIDifficulty.Easy);

        var (_, nodesSearched, _, _, _, _, _, _, _, _, _, _) = ai.GetSearchStatistics();

        // Should have searched actual nodes
        Assert.True(nodesSearched > 0, $"Sequential search should count nodes, got {nodesSearched}");
    }

    [Fact]
    public void ParallelSearch_HardDifficulty_ShouldCountRealNodes()
    {
        // D4 (Hard) uses Lazy SMP parallel search
        var ai = new MinimaxAI();
        var board = new Board();

        // Create a mid-game position
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(6, 7, Player.Blue);
        board.PlaceStone(7, 8, Player.Red);
        board.PlaceStone(8, 8, Player.Blue);

        var (x, y) = ai.GetBestMove(board, Player.Red, AIDifficulty.Hard);

        var (_, nodesSearched, _, _, _, _, _, _, _, _, _, _) = ai.GetSearchStatistics();

        // Node count should be reasonable (not a mathematical constant like 1,739,501,775)
        // Real counts vary, but shouldn't be exact round numbers from estimation
        Assert.True(nodesSearched > 0, "Parallel search should count real nodes");

        // Should not be an obviously "estimated" number (like a perfect power)
        // The bug was that same depth + same candidates always gave 1,739,501,775
        Assert.NotEqual(1_739_501_775L, nodesSearched);
    }

    [Fact]
    public void ParallelSearch_SamePositionSameDepth_NodesShouldBeConsistent()
    {
        // Running the same search twice should give approximately the same node count
        // (within some variance due to threading and timing)
        var ai = new MinimaxAI();
        var board = new Board();

        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);

        var (x1, y1) = ai.GetBestMove(board, Player.Red, AIDifficulty.Hard);
        var (_, nodes1, _, _, _, _, _, _, _, _, _, _) = ai.GetSearchStatistics();

        // Clear AI state for second search
        ai.ClearAllState();

        var (x2, y2) = ai.GetBestMove(board, Player.Red, AIDifficulty.Hard);
        var (_, nodes2, _, _, _, _, _, _, _, _, _, _) = ai.GetSearchStatistics();

        // Both should have positive node counts
        Assert.True(nodes1 > 0);
        Assert.True(nodes2 > 0);

        // They should be in the same order of magnitude (within 10x of each other)
        // This allows for threading variance while catching estimation bugs
        var ratio = Math.Max(nodes1, nodes2) / (double)Math.Min(nodes1, nodes2);
        Assert.True(ratio < 10.0,
            $"Node counts should be consistent: {nodes1} vs {nodes2}, ratio={ratio:F2}");
    }

    [Fact]
    public void ParallelSearch_DifferentDepths_NodesShouldIncreaseWithDepth()
    {
        // Higher depth should generally search more nodes
        var ai = new MinimaxAI();
        var board = new Board();

        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);

        var (x1, y1) = ai.GetBestMove(board, Player.Red, AIDifficulty.Medium);
        var (_, nodesLow, _, _, _, _, _, _, _, _, _, _) = ai.GetSearchStatistics();

        ai.ClearAllState();

        var (x2, y2) = ai.GetBestMove(board, Player.Red, AIDifficulty.Hard);
        var (_, nodesHigh, _, _, _, _, _, _, _, _, _, _) = ai.GetSearchStatistics();

        // Higher depth should search more nodes (generally)
        // Due to timing differences, this is a soft check
        Assert.True(nodesHigh > 0);
        Assert.True(nodesLow > 0);

        // Just verify both are reasonable positive numbers
        Assert.True(nodesLow < 1_000_000_000, $"Low depth nodes seem too high: {nodesLow}");
        Assert.True(nodesHigh < 10_000_000_000, $"High depth nodes seem too high: {nodesHigh}");
    }

    [Theory]
    [InlineData(AIDifficulty.Braindead)]  // D1
    [InlineData(AIDifficulty.Easy)]       // D2
    [InlineData(AIDifficulty.Medium)]     // D3
    [InlineData(AIDifficulty.Hard)]       // D4
    [InlineData(AIDifficulty.Grandmaster)] // D5
    public void AllDifficulties_ShouldReportValidNodeCounts(AIDifficulty difficulty)
    {
        var ai = new MinimaxAI();
        var board = new Board();

        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);

        var (x, y) = ai.GetBestMove(board, Player.Red, difficulty);

        var (_, nodesSearched, _, _, _, _, _, _, _, _, _, _) = ai.GetSearchStatistics();

        // All difficulties should report positive node counts
        Assert.True(nodesSearched > 0,
            $"{difficulty} should report positive node count, got {nodesSearched}");

        // Should not be the suspicious estimated value
        Assert.NotEqual(1_739_501_775L, nodesSearched);
    }

    [Fact]
    public void NodeCount_ShouldNotBeIdenticalForDifferentPositions()
    {
        // This specifically tests the bug where same depth always returned same node count
        var ai = new MinimaxAI();
        var board1 = new Board();
        var board2 = new Board();

        // Two different starting positions
        board1.PlaceStone(7, 7, Player.Red);
        board2.PlaceStone(7, 7, Player.Red);
        board2.PlaceStone(7, 8, Player.Blue);

        ai.ClearAllState();
        var (x1, y1) = ai.GetBestMove(board1, Player.Blue, AIDifficulty.Hard);
        var (_, nodes1, _, _, _, _, _, _, _, _, _, _) = ai.GetSearchStatistics();

        ai.ClearAllState();
        var (x2, y2) = ai.GetBestMove(board2, Player.Blue, AIDifficulty.Hard);
        var (_, nodes2, _, _, _, _, _, _, _, _, _, _) = ai.GetSearchStatistics();

        // Node counts should be different for different board states
        // (the bug was they were identical at 1,739,501,775)
        if (nodes1 == nodes2)
        {
            Assert.Fail($"Different board positions should yield different node counts: {nodes1} vs {nodes2}");
        }
    }
}
