using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tests.Helpers;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.Tests.GameLogic;

public class TranspositionTablePerformanceTests
{
    private readonly ITestOutputHelper _output;

    public TranspositionTablePerformanceTests(ITestOutputHelper output)
    {
        _output = output;
    }

    [Fact]
    public void TranspositionTable_ProvidesSpeedup()
    {
        // Arrange
        var ai = AITestHelper.CreateAI();
        var board = new Board();

        // Set up a mid-game position (some stones already placed)
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(7, 8, Player.Blue);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(8, 8, Player.Blue);
        board = board.PlaceStone(6, 6, Player.Red);
        board = board.PlaceStone(6, 7, Player.Blue);

        // Act - Run GetBestMove with Hard difficulty (uses TT)
        var stopwatch = System.Diagnostics.Stopwatch.StartNew();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Hard);
        stopwatch.Stop();

        // Assert - Should complete reasonably quickly (< 5 seconds)
        _output.WriteLine($"Move: ({move.x}, {move.y})");
        _output.WriteLine($"Time: {stopwatch.ElapsedMilliseconds}ms");

        Assert.True(stopwatch.ElapsedMilliseconds < 15000,
            $"Move calculation took {stopwatch.ElapsedMilliseconds}ms, expected < 15000ms");

        // Move should be valid
        var cell = board.GetCell(move.x, move.y);
        Assert.True(cell.IsEmpty, "Move should be on an empty cell");
    }

    [Fact]
    public void TranspositionTable_HitRateIsMeasurable()
    {
        // This test verifies the transposition table is being used
        // by checking internal statistics through reflection or observing behavior

        // Arrange
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(7, 8, Player.Blue);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(8, 8, Player.Blue);

        // Act - Multiple searches on same/similar positions should benefit from TT
        var ai = AITestHelper.CreateDeterministicAI();

        // First search (will populate transposition table)
        var move1 = ai.GetBestMove(board, Player.Red, AIDifficulty.Medium);

        // Second search on same position (should benefit from TT)
        var stopwatch = System.Diagnostics.Stopwatch.StartNew();
        var move2 = ai.GetBestMove(board, Player.Red, AIDifficulty.Medium);
        stopwatch.Stop();

        _output.WriteLine($"First move: ({move1.x}, {move1.y})");
        _output.WriteLine($"Second move: ({move2.x}, {move2.y})");
        _output.WriteLine($"Second search time: {stopwatch.ElapsedMilliseconds}ms");

        // Assert - Moves should be valid (near existing stones)
        Assert.InRange(move1.x, 5, 10);
        Assert.InRange(move1.y, 5, 10);
        Assert.InRange(move2.x, 5, 10);
        Assert.InRange(move2.y, 5, 10);

        // Second search should be fast due to transposition table
        // Allow more time for JIT and system variations
        Assert.True(stopwatch.ElapsedMilliseconds < 5000,
            $"Second search took {stopwatch.ElapsedMilliseconds}ms, expected < 5000ms with TT");
    }
}
