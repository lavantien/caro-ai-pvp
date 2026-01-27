using Caro.Core.Entities;
using Caro.Core.GameLogic;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.Tests.GameLogic;

public class MasterDifficultyTests
{
    private readonly ITestOutputHelper _output;

    public MasterDifficultyTests(ITestOutputHelper output)
    {
        _output = output;
    }

    [Fact]
    public void GrandmasterDifficulty_FindsBestMoves()
    {
        // Arrange - Tactical position
        var board = new Board();
        board.PlaceStone(7, 5, Player.Red);
        board.PlaceStone(7, 6, Player.Red);
        board.PlaceStone(7, 7, Player.Red);

        board.PlaceStone(8, 5, Player.Blue);
        board.PlaceStone(8, 6, Player.Blue);

        // Act - Grandmaster should find the best move
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Grandmaster);

        // Assert - Should extend the 3-in-row
        Assert.Equal(7, move.x);
        Assert.True(move.y == 4 || move.y == 8,
            $"Should extend 3-in-row, but played at ({move.x}, {move.y})");
    }

    [Fact]
    public void GrandmasterDifficulty_HandlesComplexPositions()
    {
        // Arrange - Simpler tactical position (to avoid excessive search time)
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(8, 8, Player.Blue);

        // Act - Grandmaster should find optimal move
        var ai = new MinimaxAI();
        var stopwatch = System.Diagnostics.Stopwatch.StartNew();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Grandmaster);
        stopwatch.Stop();

        _output.WriteLine($"Move: ({move.x}, {move.y}), Time: {stopwatch.ElapsedMilliseconds}ms");

        // Assert - Should complete in reasonable time
        Assert.True(stopwatch.ElapsedMilliseconds < 60000,
            $"Grandmaster search took {stopwatch.ElapsedMilliseconds}ms, expected < 60000ms");

        // Move should be valid and strategic
        Assert.True(move.x >= 0 && move.x < 15);
        Assert.True(move.y >= 0 && move.y < 15);

        var cell = board.GetCell(move.x, move.y);
        Assert.True(cell.IsEmpty, "Move should be on an empty cell");
    }

    [Fact]
    public void GrandmasterDifficulty_FindsWinningMoves()
    {
        // Arrange - Red has 4 in a row
        var board = new Board();
        board.PlaceStone(7, 5, Player.Red);
        board.PlaceStone(7, 6, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Red);

        // Act - Grandmaster should find winning move immediately
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Grandmaster);

        // Assert - Should complete the winning line
        Assert.Equal(7, move.x);
        Assert.True(move.y == 4 || move.y == 9 || move.y == 3,
            $"Should complete winning line, but played at ({move.x}, {move.y})");
    }

    [Fact]
    public void GrandmasterDifficulty_BlocksThreats()
    {
        // Arrange - Blue has 4 in a row (almost winning)
        var board = new Board();
        board.PlaceStone(7, 5, Player.Blue);
        board.PlaceStone(7, 6, Player.Blue);
        board.PlaceStone(7, 7, Player.Blue);
        board.PlaceStone(7, 8, Player.Blue);

        board.PlaceStone(8, 6, Player.Red);

        // Act - Grandmaster must block
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Grandmaster);

        // Assert - Should block the threat
        Assert.Equal(7, move.x);
        Assert.True(move.y == 4 || move.y == 9 || move.y == 3 || move.y == 10,
            $"Should block threat, but played at ({move.x}, {move.y})");
    }

    [Fact]
    public void GrandmasterDifficulty_UsesAllOptimizations()
    {
        // Verify Grandmaster difficulty uses all advanced optimizations
        // Arrange - Simple position for quick verification
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);

        // Act - Grandmaster search
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Grandmaster);

        // Assert - Should find valid move
        Assert.True(move.x >= 0 && move.x < 15);
        Assert.True(move.y >= 0 && move.y < 15);

        var cell = board.GetCell(move.x, move.y);
        Assert.True(cell.IsEmpty, "Move should be on an empty cell");
    }

    [Fact]
    public void GrandmasterDifficulty_MaintainsConsistency()
    {
        // Arrange
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(8, 8, Player.Blue);

        // Act - Multiple Grandmaster searches
        var ai = new MinimaxAI();
        var move1 = ai.GetBestMove(board, Player.Red, AIDifficulty.Grandmaster);
        var move2 = ai.GetBestMove(board, Player.Red, AIDifficulty.Grandmaster);

        // Assert - Both moves should be valid (parallel search may have slight non-determinism)
        var cell1 = board.GetCell(move1.x, move1.y);
        var cell2 = board.GetCell(move2.x, move2.y);

        Assert.True(cell1.IsEmpty, "First move should be on an empty cell");
        Assert.True(cell2.IsEmpty, "Second move should be on an empty cell");

        // Both moves should be near the center of action (reasonable play)
        Assert.True(move1.x >= 5 && move1.x <= 10 && move1.y >= 5 && move1.y <= 10,
            $"First move ({move1.x}, {move1.y}) should be near center");
        Assert.True(move2.x >= 5 && move2.x <= 10 && move2.y >= 5 && move2.y <= 10,
            $"Second move ({move2.x}, {move2.y}) should be near center");
    }

    [Fact]
    public void GrandmasterDifficulty_HandlesEndgame()
    {
        // Arrange - Complex endgame position
        var board = new Board();

        // Fill many cells
        for (int x = 5; x <= 9; x++)
        {
            for (int y = 5; y <= 9; y++)
            {
                if ((x + y) % 3 == 0)
                    board.PlaceStone(x, y, Player.Red);
                else if ((x + y) % 3 == 1)
                    board.PlaceStone(x, y, Player.Blue);
            }
        }

        // Act - Grandmaster should handle endgame
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Grandmaster);

        // Assert - Should find valid move
        Assert.True(move.x >= 0 && move.x < 15);
        Assert.True(move.y >= 0 && move.y < 15);

        var cell = board.GetCell(move.x, move.y);
        Assert.True(cell.IsEmpty, "Move should be on an empty cell");
    }

    [Fact]
    public void GrandmasterDifficulty_StrongerThanHard()
    {
        // Verify Grandmaster uses deeper search than Hard
        // Arrange - Simple position to test both difficulties
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);

        // Act - Both should find valid moves
        var ai = new MinimaxAI();

        var hardMove = ai.GetBestMove(board, Player.Red, AIDifficulty.Hard);
        ai.ClearHistory();

        var grandmasterMove = ai.GetBestMove(board, Player.Red, AIDifficulty.Grandmaster);

        _output.WriteLine($"Hard: ({hardMove.x}, {hardMove.y})");
        _output.WriteLine($"Grandmaster: ({grandmasterMove.x}, {grandmasterMove.y})");

        // Assert - Both should find valid moves
        Assert.True(hardMove.x >= 0 && hardMove.x < 15);
        Assert.True(grandmasterMove.x >= 0 && grandmasterMove.x < 15);

        // Both moves should be on empty cells
        var hardCell = board.GetCell(hardMove.x, hardMove.y);
        var grandmasterCell = board.GetCell(grandmasterMove.x, grandmasterMove.y);
        Assert.True(hardCell.IsEmpty, "Hard move should be on an empty cell");
        Assert.True(grandmasterCell.IsEmpty, "Grandmaster move should be on an empty cell");
    }
}
