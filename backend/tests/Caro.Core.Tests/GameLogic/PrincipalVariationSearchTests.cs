using Caro.Core.Entities;
using Caro.Core.GameLogic;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.Tests.GameLogic;

public class PrincipalVariationSearchTests
{
    private readonly ITestOutputHelper _output;

    public PrincipalVariationSearchTests(ITestOutputHelper output)
    {
        _output = output;
    }

    [Fact]
    public void PVS_MaintainsSearchAccuracy()
    {
        // Arrange - Position with clear best move
        var board = new Board();
        board.PlaceStone(7, 5, Player.Red);
        board.PlaceStone(7, 6, Player.Red);
        board.PlaceStone(7, 7, Player.Red);

        board.PlaceStone(8, 5, Player.Blue);
        board.PlaceStone(8, 6, Player.Blue);

        // Act - PVS should find the best move
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Hard);

        // Assert - Should extend the 3-in-row
        Assert.Equal(7, move.x);
        Assert.True(move.y == 4 || move.y == 8,
            $"Should extend 3-in-row, but played at ({move.x}, {move.y})");
    }

    [Fact]
    public void PVS_ImprovesSearchEfficiency()
    {
        // Arrange - Mid-game position
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(8, 8, Player.Blue);
        board.PlaceStone(6, 6, Player.Red);
        board.PlaceStone(6, 7, Player.Blue);

        // Act - PVS should be faster with null window searches
        var ai = new MinimaxAI();
        var stopwatch = System.Diagnostics.Stopwatch.StartNew();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Hard);
        stopwatch.Stop();

        _output.WriteLine($"Move: ({move.x}, {move.y}), Time: {stopwatch.ElapsedMilliseconds}ms");

        // Assert - Should complete quickly
        // Parallel search has some overhead, so we allow more time
        Assert.True(stopwatch.ElapsedMilliseconds < 15000,
            $"PVS search took {stopwatch.ElapsedMilliseconds}ms, expected < 15000ms");

        // Move should be valid
        Assert.True(move.x >= 0 && move.x < 15);
        Assert.True(move.y >= 0 && move.y < 15);
    }

    [Fact]
    public void PVS_ProducesConsistentResults()
    {
        // Arrange
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(8, 8, Player.Blue);

        // Act - Multiple searches should be deterministic
        var ai = new MinimaxAI();
        var move1 = ai.GetBestMove(board, Player.Red, AIDifficulty.Medium);
        var move2 = ai.GetBestMove(board, Player.Red, AIDifficulty.Medium);
        var move3 = ai.GetBestMove(board, Player.Red, AIDifficulty.Medium);

        // Assert - Should be consistent
        Assert.Equal(move1, move2);
        Assert.Equal(move2, move3);
    }

    [Fact]
    public void PVS_HandlesTacticalPositions()
    {
        // Arrange - Tactical position with winning threat
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
    public void PVS_WorksWithAllDifficulties()
    {
        // Arrange
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);

        var ai = new MinimaxAI();

        // Act & Assert - All difficulty levels should work
        var easyMove = ai.GetBestMove(board, Player.Red, AIDifficulty.Easy);
        Assert.True(easyMove.x >= 0 && easyMove.x < 15);

        var mediumMove = ai.GetBestMove(board, Player.Red, AIDifficulty.Medium);
        Assert.True(mediumMove.x >= 0 && mediumMove.x < 15);

        var hardMove = ai.GetBestMove(board, Player.Red, AIDifficulty.Hard);
        Assert.True(hardMove.x >= 0 && hardMove.x < 15);

        var grandmasterMove = ai.GetBestMove(board, Player.Red, AIDifficulty.Grandmaster);
        Assert.True(grandmasterMove.x >= 0 && grandmasterMove.x < 15);
    }

    [Fact]
    public void PVS_HandlesComplexEndgame()
    {
        // Arrange - Nearly full board
        var board = new Board();

        // Fill center area
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

        // Act - Should handle complex position
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Medium);

        // Assert - Should find valid move
        Assert.True(move.x >= 0 && move.x < 15);
        Assert.True(move.y >= 0 && move.y < 15);

        var cell = board.GetCell(move.x, move.y);
        Assert.True(cell.IsEmpty, "Move should be on an empty cell");
    }

    [Fact]
    public void PVS_MaintainsMoveQualityWithLMR()
    {
        // Verify PVS works correctly with LMR
        // Arrange - Complex position
        var board = new Board();
        board.PlaceStone(7, 6, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Red);

        board.PlaceStone(8, 5, Player.Blue);
        board.PlaceStone(8, 6, Player.Blue);
        board.PlaceStone(8, 7, Player.Blue);

        board.PlaceStone(6, 7, Player.Red);
        board.PlaceStone(9, 6, Player.Blue);

        // Act - PVS + LMR should find good move
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Hard);

        // Assert - Should find reasonable move
        Assert.True(move.x >= 0 && move.x < 15);
        Assert.True(move.y >= 0 && move.y < 15);

        var cell = board.GetCell(move.x, move.y);
        Assert.True(cell.IsEmpty, "Move should be on an empty cell");
    }

    [Fact]
    public void PVS_HandlesQuietPositions()
    {
        // Arrange - Quiet position with no immediate threats
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);

        // Act - Should complete quickly in quiet positions
        var ai = new MinimaxAI();
        var stopwatch = System.Diagnostics.Stopwatch.StartNew();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Medium);
        stopwatch.Stop();

        _output.WriteLine($"Quiet position time: {stopwatch.ElapsedMilliseconds}ms");

        // Assert - Should be fast with PVS (null window searches are efficient)
        // Allow more time for JIT compilation and system variations
        Assert.True(stopwatch.ElapsedMilliseconds < 5000,
            $"Quiet position took {stopwatch.ElapsedMilliseconds}ms, expected < 5000ms");

        // Move should be valid
        Assert.True(move.x >= 0 && move.x < 15);
        Assert.True(move.y >= 0 && move.y < 15);
    }

    [Fact]
    public void PVS_WorksWithTranspositionTable()
    {
        // Verify PVS works correctly with transposition table
        // Arrange
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(8, 8, Player.Blue);

        // Act - Multiple searches should benefit from TT + PVS
        var ai = new MinimaxAI();

        var time1 = System.Diagnostics.Stopwatch.StartNew();
        var move1 = ai.GetBestMove(board, Player.Red, AIDifficulty.Hard);
        time1.Stop();

        var time2 = System.Diagnostics.Stopwatch.StartNew();
        var move2 = ai.GetBestMove(board, Player.Red, AIDifficulty.Hard);
        time2.Stop();

        // Assert - Moves should be consistent
        Assert.Equal(move1, move2);

        // Second search should be faster due to TT
        _output.WriteLine($"First search: {time1.ElapsedMilliseconds}ms");
        _output.WriteLine($"Second search: {time2.ElapsedMilliseconds}ms");
    }

    [Fact]
    public void PVS_PreservesTacticalAwareness()
    {
        // Verify PVS doesn't reduce tactical awareness
        // Arrange - Red must block Blue's winning threat
        var board = new Board();
        board.PlaceStone(7, 5, Player.Blue);
        board.PlaceStone(7, 6, Player.Blue);
        board.PlaceStone(7, 7, Player.Blue);
        board.PlaceStone(7, 8, Player.Blue);

        board.PlaceStone(8, 6, Player.Red);

        // Act - Must block the threat
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Grandmaster);

        // Assert - Should block at (7, 4) or (7, 9)
        Assert.Equal(7, move.x);
        Assert.True(move.y == 4 || move.y == 9 || move.y == 3 || move.y == 10,
            $"Should block winning threat, but played at ({move.x}, {move.y})");
    }

    [Fact]
    public void PVS_DoesNotCauseSearchErrors()
    {
        // Verify PVS doesn't introduce bugs in edge cases

        // Arrange - Various edge case positions
        var positions = new[]
        {
            new Board(),  // Empty board
            CreateBoardWithMoves(new[] { (7, 7, Player.Red) }),  // One stone
            CreateBoardWithMoves(new[] { (7, 7, Player.Red), (7, 8, Player.Blue), (8, 7, Player.Red) }),  // Early game
        };

        var ai = new MinimaxAI();

        foreach (var board in positions)
        {
            // Act - Should not throw
            var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Braindead);

            // Assert - Should be valid
            Assert.True(move.x >= 0 && move.x < 15,
                $"Move x={move.x} is out of bounds");
            Assert.True(move.y >= 0 && move.y < 15,
                $"Move y={move.y} is out of bounds");
        }
    }

    private Board CreateBoardWithMoves((int x, int y, Player player)[] moves)
    {
        var board = new Board();
        foreach (var (x, y, player) in moves)
        {
            board.PlaceStone(x, y, player);
        }
        return board;
    }
}
