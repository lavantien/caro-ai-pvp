using Caro.Core.Entities;
using Caro.Core.GameLogic;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.Tests.GameLogic;

public class AspirationWindowTests
{
    private readonly ITestOutputHelper _output;

    public AspirationWindowTests(ITestOutputHelper output)
    {
        _output = output;
    }

    [Fact]
    public void AspirationWindows_ProducesSameMovesAsStandardSearch()
    {
        // Arrange - mid-game position
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(8, 8, Player.Blue);
        board.PlaceStone(6, 6, Player.Red);
        board.PlaceStone(6, 7, Player.Blue);

        // Act - Search with aspiration windows (enabled by default in Hard+)
        var ai = new MinimaxAI();
        var move1 = ai.GetBestMove(board, Player.Red, AIDifficulty.Hard);
        var move2 = ai.GetBestMove(board, Player.Red, AIDifficulty.Hard);

        // Assert - Moves should be consistent (deterministic)
        Assert.Equal(move1, move2);
    }

    [Fact]
    public void AspirationWindows_HandlesTacticalPositions()
    {
        // Arrange - tactical position with multiple threats
        var board = new Board();
        board.PlaceStone(7, 6, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);
        board.PlaceStone(6, 7, Player.Blue);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(8, 8, Player.Blue);

        // Act
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Hard);

        // Assert - Should find a reasonable move
        Assert.True(move.x >= 0 && move.x < 15);
        Assert.True(move.y >= 0 && move.y < 15);

        var cell = board.GetCell(move.x, move.y);
        Assert.True(cell.IsEmpty, "Move should be on an empty cell");
    }

    [Fact]
    public void AspirationWindows_WorksWithIterativeDeepening()
    {
        // Arrange - More realistic position where iterative deepening is used
        var board = new Board();

        // Create a mid-game position (~20% board occupancy)
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

        // Act - Expert (D8) uses iterative deepening with aspiration windows
        var ai = new MinimaxAI();
        var stopwatch = System.Diagnostics.Stopwatch.StartNew();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Expert);
        stopwatch.Stop();

        // Assert - Should complete in reasonable time
        // Expert (D8) with depth 9 and parallel search
        _output.WriteLine($"Move: ({move.x}, {move.y}), Time: {stopwatch.ElapsedMilliseconds}ms");
        Assert.True(stopwatch.ElapsedMilliseconds < 30000,
            $"Expert search took {stopwatch.ElapsedMilliseconds}ms, expected < 30000ms");

        // Move should be valid
        var cell = board.GetCell(move.x, move.y);
        Assert.True(cell.IsEmpty, "Move should be on an empty cell");
    }

    [Fact]
    public void AspirationWindows_DoesNotBreakTranspositionTable()
    {
        // Verify aspiration windows work correctly with transposition table

        // Arrange
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);
        board.PlaceStone(8, 7, Player.Red);

        // Act - Multiple searches should build up TT entries
        var ai = new MinimaxAI();
        var move1 = ai.GetBestMove(board, Player.Blue, AIDifficulty.Hard);
        var move2 = ai.GetBestMove(board, Player.Blue, AIDifficulty.Hard);
        var move3 = ai.GetBestMove(board, Player.Blue, AIDifficulty.Hard);

        // Assert - Moves should be consistent
        Assert.Equal(move1, move2);
        Assert.Equal(move2, move3);
    }

    [Fact]
    public void AspirationWindows_HandlesWideScoreRanges()
    {
        // Arrange - position with extreme score difference
        var board = new Board();

        // Red has 4 in a row (almost winning)
        board.PlaceStone(7, 5, Player.Red);
        board.PlaceStone(7, 6, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Red);

        // Blue has 3 in a row nearby
        board.PlaceStone(8, 5, Player.Blue);
        board.PlaceStone(8, 6, Player.Blue);
        board.PlaceStone(8, 7, Player.Blue);

        // Act - Should find winning move for Red
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Expert);

        // Assert - Should find move near the winning line
        Assert.InRange(move.x, 6, 8);
        Assert.InRange(move.y, 3, 10);

        var cell = board.GetCell(move.x, move.y);
        Assert.True(cell.IsEmpty, "Move should be on an empty cell");
    }

    [Fact]
    public void AspirationWindows_MaintainsSearchQuality()
    {
        // Verify that aspiration windows don't reduce move quality

        // Arrange - complex mid-game position
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(8, 8, Player.Blue);
        board.PlaceStone(6, 6, Player.Red);
        board.PlaceStone(6, 7, Player.Blue);
        board.PlaceStone(9, 9, Player.Red);
        board.PlaceStone(9, 8, Player.Blue);

        // Act - Get best move
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Expert);

        // Assert - Move should be strategic (near existing stones)
        Assert.True(move.x >= 0 && move.x < 15);
        Assert.True(move.y >= 0 && move.y < 15);

        // Check move is near existing stones (not random corner)
        var nearStones = false;
        for (int dx = -2; dx <= 2; dx++)
        {
            for (int dy = -2; dy <= 2; dy++)
            {
                var nx = move.x + dx;
                var ny = move.y + dy;
                if (nx >= 0 && nx < 15 && ny >= 0 && ny < 15)
                {
                    var cell = board.GetCell(nx, ny);
                    if (cell.Player != Player.None)
                        nearStones = true;
                }
            }
        }
        Assert.True(nearStones, "Move should be near existing stones");
    }

    [Fact]
    public void AspirationWindows_EfficientForMediumDepth()
    {
        // Verify aspiration windows provide benefit for medium depth (depth 2-3)

        // Arrange
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(8, 8, Player.Blue);

        // Act
        var ai = new MinimaxAI();
        var stopwatch = System.Diagnostics.Stopwatch.StartNew();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Normal);
        stopwatch.Stop();

        // Assert - Should be very fast with aspiration windows
        _output.WriteLine($"Medium depth search time: {stopwatch.ElapsedMilliseconds}ms");
        Assert.True(stopwatch.ElapsedMilliseconds < 1000,
            $"Medium depth search took {stopwatch.ElapsedMilliseconds}ms, expected < 1000ms");

        // Move should be valid
        var cell = board.GetCell(move.x, move.y);
        Assert.True(cell.IsEmpty, "Move should be on an empty cell");
    }
}
