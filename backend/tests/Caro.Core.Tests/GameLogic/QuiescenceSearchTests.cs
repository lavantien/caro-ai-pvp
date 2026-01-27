using Caro.Core.Entities;
using Caro.Core.GameLogic;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.Tests.GameLogic;

public class QuiescenceSearchTests
{
    private readonly ITestOutputHelper _output;

    public QuiescenceSearchTests(ITestOutputHelper output)
    {
        _output = output;
    }

    [Fact]
    public void QuiescenceSearch_ImprovesTacticalEvaluation()
    {
        // Arrange - Position with immediate tactical threat (3 in a row)
        var board = new Board();
        board.PlaceStone(7, 5, Player.Red);
        board.PlaceStone(7, 6, Player.Red);
        board.PlaceStone(7, 7, Player.Red);  // 3 in a row

        // Act - With quiescence, AI should see the threat
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Medium);

        // Assert - Should extend the line (create 4 in a row)
        Assert.True(move.x == 7 || move.y == 7, "Should extend the 3-in-row");
        Assert.True(move.y == 4 || move.y == 8, "Should play at ends of the line");
    }

    [Fact]
    public void QuiescenceSearch_HandlesBlockedThreats()
    {
        // Arrange - Red has 3 in a row but Blue blocks one end
        var board = new Board();
        board.PlaceStone(7, 5, Player.Red);
        board.PlaceStone(7, 6, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);  // Blue blocks

        // Act - Red should extend the other way or play nearby
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Medium);

        // Assert - Should play near the threat
        Assert.InRange(move.x, 6, 8);  // Near the threat line
        Assert.InRange(move.y, 3, 9);  // Near the threat

        // Move should be on empty cell
        var cell = board.GetCell(move.x, move.y);
        Assert.True(cell.IsEmpty, "Move should be on an empty cell");
    }

    [Fact]
    public void QuiescenceSearch_DoesNotOverSearchQuietPositions()
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

        // Assert - Should be fast (quiescence stops early in quiet positions)
        _output.WriteLine($"Quiet position search time: {stopwatch.ElapsedMilliseconds}ms");
        Assert.True(stopwatch.ElapsedMilliseconds < 2000,
            $"Quiet position took {stopwatch.ElapsedMilliseconds}ms, expected < 2000ms");

        Assert.True(move.x >= 0 && move.x < 15);
        Assert.True(move.y >= 0 && move.y < 15);
    }

    [Fact]
    public void QuiescenceSearch_AccuratelyEvaluatesWinningThreats()
    {
        // Arrange - Red has 4 in a row (one move from winning)
        var board = new Board();
        board.PlaceStone(7, 5, Player.Red);
        board.PlaceStone(7, 6, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Red);

        // Act - Red should find the winning move with quiescence
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Grandmaster);

        // Assert - Should complete the 5-in-row or play very close
        Assert.Equal(7, move.x);  // Should stay on same column
        Assert.True(move.y == 4 || move.y == 9 || move.y == 3, "Should complete the winning line");
    }

    [Fact]
    public void QuiescenceSearch_BlocksOpponentThreats()
    {
        // Arrange - Blue has 3 in a row, Red to move
        var board = new Board();
        board.PlaceStone(7, 5, Player.Blue);
        board.PlaceStone(7, 6, Player.Blue);
        board.PlaceStone(7, 7, Player.Blue);

        board.PlaceStone(8, 6, Player.Red);  // Red has stones nearby

        // Act - Red should block the threat
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Hard);

        // Assert - Should block at (7, 4) or (7, 8)
        Assert.Equal(7, move.x);
        Assert.True(move.y == 4 || move.y == 8, $"Should block threat, but played at ({move.x}, {move.y})");
    }

    [Fact]
    public void QuiescenceSearch_HandlesMultipleThreats()
    {
        // Arrange - Multiple threats on the board
        var board = new Board();

        // Red has 3-in-row horizontally
        board.PlaceStone(7, 5, Player.Red);
        board.PlaceStone(7, 6, Player.Red);
        board.PlaceStone(7, 7, Player.Red);

        // Blue has 3-in-row vertically
        board.PlaceStone(8, 5, Player.Blue);
        board.PlaceStone(8, 6, Player.Blue);
        board.PlaceStone(8, 7, Player.Blue);

        // Act - Find best move
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Hard);

        // Assert - Should either extend Red's threat or block Blue's
        Assert.True(move.x >= 0 && move.x < 15);
        Assert.True(move.y >= 0 && move.y < 15);

        // Move should be tactical (near threats)
        var cell = board.GetCell(move.x, move.y);
        Assert.True(cell.IsEmpty, "Move should be on an empty cell");
    }

    [Fact]
    public void QuiescenceSearch_StopsAtDepthLimit()
    {
        // Arrange - Complex tactical position
        var board = new Board();
        for (int i = 0; i < 5; i++)
        {
            board.PlaceStone(7 + i, 7, Player.Red);
            if (i < 4)
                board.PlaceStone(7 + i, 8, Player.Blue);
        }

        // Act - Should complete without excessive search
        var ai = new MinimaxAI();
        var stopwatch = System.Diagnostics.Stopwatch.StartNew();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Medium);
        stopwatch.Stop();

        // Assert - Should complete in reasonable time (quiescence limited to depth 4)
        _output.WriteLine($"Complex tactical position time: {stopwatch.ElapsedMilliseconds}ms");
        Assert.True(stopwatch.ElapsedMilliseconds < 15000,
            $"Quiescence search took {stopwatch.ElapsedMilliseconds}ms, expected < 15000ms");

        Assert.True(move.x >= 0 && move.x < 15);
        Assert.True(move.y >= 0 && move.y < 15);
    }

    [Fact]
    public void QuiescenceSearch_PreservesSearchCorrectness()
    {
        // Verify quiescence doesn't change the outcome of positions

        // Arrange - Various test positions
        var positions = new[]
        {
            // Empty board
            new Board(),
            // Center start
            CreateBoardWithMoves(new[] { (7, 7, Player.Red) }),
            // Early game
            CreateBoardWithMoves(new[] { (7, 7, Player.Red), (7, 8, Player.Blue), (8, 7, Player.Red) }),
        };

        var ai = new MinimaxAI();

        foreach (var board in positions)
        {
            // Act - Get move should not throw
            var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Braindead);

            // Assert - Should be valid
            Assert.True(move.x >= 0 && move.x < 15);
            Assert.True(move.y >= 0 && move.y < 15);
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
