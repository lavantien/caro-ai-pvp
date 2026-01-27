using Caro.Core.Entities;
using Caro.Core.GameLogic;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.Tests.GameLogic;

public class EnhancedMoveOrderingTests
{
    private readonly ITestOutputHelper _output;

    public EnhancedMoveOrderingTests(ITestOutputHelper output)
    {
        _output = output;
    }

    [Fact]
    public void EnhancedMoveOrdering_PrioritizesWinningMoves()
    {
        // Arrange - Red has 4 in a row, should find winning move
        var board = new Board();
        board.PlaceStone(7, 5, Player.Red);
        board.PlaceStone(7, 6, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Red);

        // Act - Enhanced move ordering should prioritize winning move
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Medium);

        // Assert - Should complete the winning line
        Assert.Equal(7, move.x);
        Assert.True(move.y == 4 || move.y == 9 || move.y == 3,
            $"Should complete winning line, but played at ({move.x}, {move.y})");
    }

    [Fact]
    public void EnhancedMoveOrdering_PrioritizesBlockingMoves()
    {
        // Arrange - Blue has 3 in a row (open), Red must block
        var board = new Board();
        board.PlaceStone(7, 5, Player.Blue);
        board.PlaceStone(7, 6, Player.Blue);
        board.PlaceStone(7, 7, Player.Blue);

        board.PlaceStone(8, 6, Player.Red);

        // Act - Should block the threat
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Hard);

        // Assert - Should block at (7, 4) or (7, 8)
        Assert.Equal(7, move.x);
        Assert.True(move.y == 4 || move.y == 8,
            $"Should block at (7,4) or (7,8), but played at ({move.x}, {move.y})");
    }

    [Fact]
    public void EnhancedMoveOrdering_PrioritizesOpenThreats()
    {
        // Arrange - Position with multiple threat levels
        var board = new Board();

        // Red has open 3 (should be prioritized)
        board.PlaceStone(6, 7, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(8, 7, Player.Red);

        // Blue has closed 3 (less urgent)
        board.PlaceStone(6, 5, Player.Blue);
        board.PlaceStone(7, 5, Player.Blue);
        board.PlaceStone(8, 5, Player.Blue);
        board.PlaceStone(9, 5, Player.Blue);  // Blocks one end

        // Act - Should extend open 3
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Medium);

        // Assert - Should play on the open 3 line
        Assert.Equal(7, move.y);  // Should stay on column 7
        Assert.True(move.x == 5 || move.x == 9,
            $"Should extend open 3, but played at ({move.x}, {move.y})");
    }

    [Fact]
    public void EnhancedMoveOrdering_HandlesMultipleThreats()
    {
        // Arrange - Complex position with both attack and defense
        var board = new Board();

        // Red has open 3
        board.PlaceStone(6, 7, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(8, 7, Player.Red);

        // Blue has open 3 (must block)
        board.PlaceStone(6, 5, Player.Blue);
        board.PlaceStone(7, 5, Player.Blue);
        board.PlaceStone(8, 5, Player.Blue);

        // Act - Should prioritize based on tactical importance
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Hard);

        // Assert - Should either extend own threat or block opponent
        var cell = board.GetCell(move.x, move.y);
        Assert.True(cell.IsEmpty, "Move should be on an empty cell");

        // Move should be on one of the threat lines
        bool isOnRedLine = (move.y == 7 && move.x >= 5 && move.x <= 9);
        bool isOnBlueLine = (move.y == 5 && move.x >= 5 && move.x <= 9);
        Assert.True(isOnRedLine || isOnBlueLine,
            $"Move should be on a threat line, but played at ({move.x}, {move.y})");
    }

    [Fact]
    public void EnhancedMoveOrdering_ImprovesSearchEfficiency()
    {
        // Arrange - Mid-game position
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(8, 8, Player.Blue);
        board.PlaceStone(6, 6, Player.Red);
        board.PlaceStone(6, 7, Player.Blue);

        // Act - Enhanced ordering should find good moves faster
        var ai = new MinimaxAI();
        var stopwatch = System.Diagnostics.Stopwatch.StartNew();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Hard);
        stopwatch.Stop();

        _output.WriteLine($"Move: ({move.x}, {move.y}), Time: {stopwatch.ElapsedMilliseconds}ms");

        // Assert - Should complete quickly due to better move ordering
        // Parallel search has some overhead, so we allow more time
        Assert.True(stopwatch.ElapsedMilliseconds < 15000,
            $"Search took {stopwatch.ElapsedMilliseconds}ms, expected < 15000ms with enhanced ordering");

        // Move should be valid and strategic
        Assert.True(move.x >= 0 && move.x < 15);
        Assert.True(move.y >= 0 && move.y < 15);
    }

    [Fact]
    public void EnhancedMoveOrdering_DetectsFourInRow()
    {
        // Arrange - Red has 4 in a row (blocked one end)
        var board = new Board();
        board.PlaceStone(7, 5, Player.Red);
        board.PlaceStone(7, 6, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Red);
        board.PlaceStone(7, 9, Player.Blue);  // Blocked one end

        // Act - Should still play at the open end
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Hard);

        // Assert - Should play at (7, 4) or nearby
        Assert.InRange(move.x, 6, 8);
        Assert.InRange(move.y, 3, 5);

        var cell = board.GetCell(move.x, move.y);
        Assert.True(cell.IsEmpty, "Move should be on an empty cell");
    }

    [Fact]
    public void EnhancedMoveOrdering_DetectsOpenThree()
    {
        // Arrange - Red has open 3 (both ends open)
        var board = new Board();
        board.PlaceStone(7, 6, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Red);

        // Act - Should extend the open 3
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Medium);

        // Assert - Should extend at either end
        Assert.Equal(7, move.x);
        Assert.True(move.y == 5 || move.y == 9,
            $"Should extend open 3, but played at ({move.x}, {move.y})");
    }

    [Fact]
    public void EnhancedMoveOrdering_HandlesTacticalComplexity()
    {
        // Arrange - Multiple intersecting threats
        var board = new Board();

        // Horizontal threat
        board.PlaceStone(6, 7, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(8, 7, Player.Red);

        // Vertical threat
        board.PlaceStone(7, 5, Player.Blue);
        board.PlaceStone(7, 6, Player.Blue);
        board.PlaceStone(7, 8, Player.Blue);

        // Act - Should find best tactical move
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Hard);

        // Assert - Should make a tactical move
        Assert.True(move.x >= 0 && move.x < 15);
        Assert.True(move.y >= 0 && move.y < 15);

        var cell = board.GetCell(move.x, move.y);
        Assert.True(cell.IsEmpty, "Move should be on an empty cell");

        // Should be near the intersection
        Assert.InRange(move.x, 5, 9);
        Assert.InRange(move.y, 4, 9);
    }

    [Fact]
    public void EnhancedMoveOrdering_MaintainsConsistency()
    {
        // Arrange
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(8, 8, Player.Blue);

        // Act - Multiple searches should produce consistent results
        var ai = new MinimaxAI();
        var move1 = ai.GetBestMove(board, Player.Red, AIDifficulty.Medium);
        var move2 = ai.GetBestMove(board, Player.Red, AIDifficulty.Medium);
        var move3 = ai.GetBestMove(board, Player.Red, AIDifficulty.Medium);

        // Assert - Should be deterministic
        Assert.Equal(move1, move2);
        Assert.Equal(move2, move3);
    }

    [Fact]
    public void EnhancedMoveOrdering_WorksInEndgame()
    {
        // Arrange - Nearly full board with many tactical decisions
        var board = new Board();

        // Fill board with many stones
        for (int x = 5; x <= 9; x++)
        {
            for (int y = 5; y <= 9; y++)
            {
                if ((x + y) % 2 == 0)
                    board.PlaceStone(x, y, Player.Red);
                else
                    board.PlaceStone(x, y, Player.Blue);
            }
        }

        // Act - Should handle complex endgame
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Medium);

        // Assert - Should find valid move
        Assert.True(move.x >= 0 && move.x < 15);
        Assert.True(move.y >= 0 && move.y < 15);

        var cell = board.GetCell(move.x, move.y);
        Assert.True(cell.IsEmpty, "Move should be on an empty cell");
    }
}
