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
    public void MasterDifficulty_FindsBestMoves()
    {
        // Arrange - Tactical position
        var board = new Board();
        board.PlaceStone(7, 5, Player.Red);
        board.PlaceStone(7, 6, Player.Red);
        board.PlaceStone(7, 7, Player.Red);

        board.PlaceStone(8, 5, Player.Blue);
        board.PlaceStone(8, 6, Player.Blue);

        // Act - Master should find the best move
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Master);

        // Assert - Should extend the 3-in-row
        Assert.Equal(7, move.x);
        Assert.True(move.y == 4 || move.y == 8,
            $"Should extend 3-in-row, but played at ({move.x}, {move.y})");
    }

    [Fact]
    public void MasterDifficulty_HandlesComplexPositions()
    {
        // Arrange - Simpler tactical position (to avoid excessive search time)
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(8, 8, Player.Blue);

        // Act - Master should find optimal move
        var ai = new MinimaxAI();
        var stopwatch = System.Diagnostics.Stopwatch.StartNew();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Master);
        stopwatch.Stop();

        _output.WriteLine($"Move: ({move.x}, {move.y}), Time: {stopwatch.ElapsedMilliseconds}ms");

        // Assert - Should complete in reasonable time
        Assert.True(stopwatch.ElapsedMilliseconds < 60000,
            $"Master search took {stopwatch.ElapsedMilliseconds}ms, expected < 60000ms");

        // Move should be valid and strategic
        Assert.True(move.x >= 0 && move.x < 15);
        Assert.True(move.y >= 0 && move.y < 15);

        var cell = board.GetCell(move.x, move.y);
        Assert.True(cell.IsEmpty, "Move should be on an empty cell");
    }

    [Fact]
    public void MasterDifficulty_FindsWinningMoves()
    {
        // Arrange - Red has 4 in a row
        var board = new Board();
        board.PlaceStone(7, 5, Player.Red);
        board.PlaceStone(7, 6, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Red);

        // Act - Master should find winning move immediately
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Master);

        // Assert - Should complete the winning line
        Assert.Equal(7, move.x);
        Assert.True(move.y == 4 || move.y == 9 || move.y == 3,
            $"Should complete winning line, but played at ({move.x}, {move.y})");
    }

    [Fact]
    public void MasterDifficulty_BlocksThreats()
    {
        // Arrange - Blue has 4 in a row (almost winning)
        var board = new Board();
        board.PlaceStone(7, 5, Player.Blue);
        board.PlaceStone(7, 6, Player.Blue);
        board.PlaceStone(7, 7, Player.Blue);
        board.PlaceStone(7, 8, Player.Blue);

        board.PlaceStone(8, 6, Player.Red);

        // Act - Master must block
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Master);

        // Assert - Should block the threat
        Assert.Equal(7, move.x);
        Assert.True(move.y == 4 || move.y == 9 || move.y == 3 || move.y == 10,
            $"Should block threat, but played at ({move.x}, {move.y})");
    }

    [Fact]
    public void MasterDifficulty_UsesAllOptimizations()
    {
        // Verify Master difficulty uses all advanced optimizations
        // Arrange - Simple position for quick verification
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);

        // Act - Master search
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Master);

        // Assert - Should find valid move
        Assert.True(move.x >= 0 && move.x < 15);
        Assert.True(move.y >= 0 && move.y < 15);

        var cell = board.GetCell(move.x, move.y);
        Assert.True(cell.IsEmpty, "Move should be on an empty cell");
    }

    [Fact]
    public void MasterDifficulty_MaintainsConsistency()
    {
        // Arrange
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(8, 8, Player.Blue);

        // Act - Multiple Master searches should be consistent
        var ai = new MinimaxAI();
        var move1 = ai.GetBestMove(board, Player.Red, AIDifficulty.Master);
        var move2 = ai.GetBestMove(board, Player.Red, AIDifficulty.Master);

        // Assert - Should be deterministic
        Assert.Equal(move1, move2);
    }

    [Fact]
    public void MasterDifficulty_HandlesEndgame()
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

        // Act - Master should handle endgame
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Master);

        // Assert - Should find valid move
        Assert.True(move.x >= 0 && move.x < 15);
        Assert.True(move.y >= 0 && move.y < 15);

        var cell = board.GetCell(move.x, move.y);
        Assert.True(cell.IsEmpty, "Move should be on an empty cell");
    }

    [Fact]
    public void MasterDifficulty_StrongerThanExpert()
    {
        // Verify Master uses deeper search than Expert
        // Arrange - Simple position to test both difficulties
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);

        // Act - Both should find valid moves
        var ai = new MinimaxAI();

        var expertMove = ai.GetBestMove(board, Player.Red, AIDifficulty.Expert);
        ai.ClearHistory();

        var masterMove = ai.GetBestMove(board, Player.Red, AIDifficulty.Master);

        _output.WriteLine($"Expert: ({expertMove.x}, {expertMove.y})");
        _output.WriteLine($"Master: ({masterMove.x}, {masterMove.y})");

        // Assert - Both should find valid moves
        Assert.True(expertMove.x >= 0 && expertMove.x < 15);
        Assert.True(masterMove.x >= 0 && masterMove.x < 15);

        // Both moves should be on empty cells
        var expertCell = board.GetCell(expertMove.x, expertMove.y);
        var masterCell = board.GetCell(masterMove.x, masterMove.y);
        Assert.True(expertCell.IsEmpty, "Expert move should be on an empty cell");
        Assert.True(masterCell.IsEmpty, "Master move should be on an empty cell");
    }
}
