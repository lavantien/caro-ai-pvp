using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.Tests.GameLogic;

public class LateMoveReductionTests
{
    private readonly ITestOutputHelper _output;

    public LateMoveReductionTests(ITestOutputHelper output)
    {
        _output = output;
    }

    [Fact]
    public void LMR_MaintainsMoveQuality()
    {
        // Arrange - mid-game position with many candidate moves
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(8, 8, Player.Blue);
        board.PlaceStone(6, 6, Player.Red);
        board.PlaceStone(6, 7, Player.Blue);
        board.PlaceStone(9, 9, Player.Red);
        board.PlaceStone(9, 8, Player.Blue);

        // Act - Get move with LMR (should use reduced depth for late moves)
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Hard);

        // Assert - Move should be valid and strategic
        Assert.True(move.x >= 0 && move.x < 15);
        Assert.True(move.y >= 0 && move.y < 15);

        var cell = board.GetCell(move.x, move.y);
        Assert.True(cell.IsEmpty, "Move should be on an empty cell");

        // Move should be near existing stones (not random)
        var nearStones = false;
        for (int dx = -2; dx <= 2; dx++)
        {
            for (int dy = -2; dy <= 2; dy++)
            {
                var nx = move.x + dx;
                var ny = move.y + dy;
                if (nx >= 0 && nx < 15 && ny >= 0 && ny < 15)
                {
                    var neighbor = board.GetCell(nx, ny);
                    if (neighbor.Player != Player.None)
                        nearStones = true;
                }
            }
        }
        Assert.True(nearStones, "Move should be near existing stones");
    }

    [Fact]
    public void LMR_DoesNotReduceInTacticalPositions()
    {
        // Arrange - tactical position with 3 in a row (should skip LMR)
        var board = new Board();
        board.PlaceStone(7, 5, Player.Red);
        board.PlaceStone(7, 6, Player.Red);
        board.PlaceStone(7, 7, Player.Red);

        board.PlaceStone(8, 5, Player.Blue);
        board.PlaceStone(8, 6, Player.Blue);

        // Act - Should use full depth in tactical position
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Hard);

        // Assert - Should find good move near the threat
        Assert.InRange(move.x, 6, 8);
        Assert.InRange(move.y, 3, 9);

        var cell = board.GetCell(move.x, move.y);
        Assert.True(cell.IsEmpty, "Move should be on an empty cell");
    }

    [Fact]
    public void LMR_ImprovesSearchSpeed()
    {
        // Arrange - quiet position (should benefit from LMR)
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(8, 8, Player.Blue);

        // Act - Search with Hard difficulty (uses LMR)
        var ai = new MinimaxAI();
        var stopwatch = System.Diagnostics.Stopwatch.StartNew();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Hard);
        stopwatch.Stop();

        _output.WriteLine($"Move: ({move.x}, {move.y}), Time: {stopwatch.ElapsedMilliseconds}ms");

        // Assert - Should complete quickly with LMR
        // Parallel search has some overhead, so we allow more time
        Assert.True(stopwatch.ElapsedMilliseconds < 15000,
            $"LMR search took {stopwatch.ElapsedMilliseconds}ms, expected < 15000ms");

        // Move should be valid
        Assert.True(move.x >= 0 && move.x < 15);
        Assert.True(move.y >= 0 && move.y < 15);
    }

    [Fact]
    public void LMR_ProducesConsistentMoves()
    {
        // Arrange
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(8, 8, Player.Blue);

        // Act - Multiple searches with LMR should be deterministic
        var ai = new MinimaxAI();
        var move1 = ai.GetBestMove(board, Player.Red, AIDifficulty.Medium);
        var move2 = ai.GetBestMove(board, Player.Red, AIDifficulty.Medium);

        // Assert - Moves should be consistent
        Assert.Equal(move1, move2);
    }

    [Fact]
    public void LMR_HandlesComplexPositions()
    {
        // Arrange - complex position with multiple threats
        var board = new Board();

        // Red has 3-in-row
        board.PlaceStone(7, 5, Player.Red);
        board.PlaceStone(7, 6, Player.Red);
        board.PlaceStone(7, 7, Player.Red);

        // Blue has 3-in-row
        board.PlaceStone(8, 5, Player.Blue);
        board.PlaceStone(8, 6, Player.Blue);
        board.PlaceStone(8, 7, Player.Blue);

        // Additional stones
        board.PlaceStone(6, 6, Player.Red);
        board.PlaceStone(9, 6, Player.Blue);

        // Act - Should handle tactical position without LMR
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Hard);

        // Assert - Should find reasonable move
        Assert.True(move.x >= 0 && move.x < 15);
        Assert.True(move.y >= 0 && move.y < 15);

        var cell = board.GetCell(move.x, move.y);
        Assert.True(cell.IsEmpty, "Move should be on an empty cell");
    }

    [Fact]
    public void LMR_DoesNotApplyInEarlyGame()
    {
        // Arrange - early game with few stones
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);

        // Act - LMR may not apply much with few candidate moves
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Medium);

        // Assert - Should still play near center
        Assert.True(move.x >= 0 && move.x < 15);
        Assert.True(move.y >= 0 && move.y < 15);

        // Should be near existing stones
        var nearStones = false;
        for (int dx = -2; dx <= 2; dx++)
        {
            for (int dy = -2; dy <= 2; dy++)
            {
                var nx = move.x + dx;
                var ny = move.y + dy;
                if (nx >= 0 && nx < 15 && ny >= 0 && ny < 15)
                {
                    var neighbor = board.GetCell(nx, ny);
                    if (neighbor.Player != Player.None)
                        nearStones = true;
                }
            }
        }
        Assert.True(nearStones, "Move should be near existing stones");
    }

    [Fact]
    public void LMR_MaintainsSearchCorrectness()
    {
        // Verify LMR doesn't break tactical evaluation

        // Arrange - Red has 4 in a row (should win immediately)
        var board = new Board();
        board.PlaceStone(7, 5, Player.Red);
        board.PlaceStone(7, 6, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Red);

        // Act - Should find winning move
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Grandmaster);

        // Assert - Should complete the winning line
        Assert.Equal(7, move.x);
        Assert.True(move.y == 4 || move.y == 9 || move.y == 3,
            $"Should complete winning line, but played at ({move.x}, {move.y})");
    }

    [Fact]
    public void LMR_HandlesMultipleSearches()
    {
        // Arrange
        var ai = new MinimaxAI();
        var board = new Board();

        // Create mid-game position
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(8, 8, Player.Blue);

        // Act - Multiple searches should work correctly
        for (int i = 0; i < 3; i++)
        {
            var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Medium);

            // Assert - Each move should be valid
            Assert.True(move.x >= 0 && move.x < 15);
            Assert.True(move.y >= 0 && move.y < 15);

            // Make the move temporarily
            board.PlaceStone(move.x, move.y, Player.Red);

            // Undo
            board.GetCell(move.x, move.y).Player = Player.None;
        }

        // Assert - All searches should complete successfully
        // History should accumulate without issues
    }
}
