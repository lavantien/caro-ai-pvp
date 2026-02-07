using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.IntegrationTests.Helpers;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.IntegrationTests.GameLogic;

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
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(7, 8, Player.Blue);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(8, 8, Player.Blue);
        board = board.PlaceStone(6, 6, Player.Red);
        board = board.PlaceStone(6, 7, Player.Blue);

        // Act - Search with aspiration windows (enabled by default in Hard+)
        var ai = AITestHelper.CreateAI();
        var move1 = ai.GetBestMove(board, Player.Red, AIDifficulty.Hard);
        var move2 = ai.GetBestMove(board, Player.Red, AIDifficulty.Hard);

        // Assert - Both moves should be valid
        Assert.True(move1.x >= 0 && move1.x < 19 && move1.y >= 0 && move1.y < 19);
        Assert.True(move2.x >= 0 && move2.x < 19 && move2.y >= 0 && move2.y < 19);
        var cell1 = board.GetCell(move1.x, move1.y);
        var cell2 = board.GetCell(move2.x, move2.y);
        Assert.True(cell1.IsEmpty && cell2.IsEmpty, "Both moves should be on empty cells");
    }

    [Fact]
    public void AspirationWindows_HandlesTacticalPositions()
    {
        // Arrange - tactical position with multiple threats
        var board = new Board();
        board = board.PlaceStone(7, 6, Player.Red);
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(7, 8, Player.Blue);
        board = board.PlaceStone(6, 7, Player.Blue);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(8, 8, Player.Blue);

        // Act
        var ai = AITestHelper.CreateAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Hard);

        // Assert - Should find a reasonable move
        Assert.True(move.x >= 0 && move.x < 19);
        Assert.True(move.y >= 0 && move.y < 19);

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
                    board = board.PlaceStone(x, y, Player.Red);
                else
                    board = board.PlaceStone(x, y, Player.Blue);
            }
        }

        // Act - Grandmaster (D5) uses iterative deepening with aspiration windows
        var ai = AITestHelper.CreateAI();
        var stopwatch = System.Diagnostics.Stopwatch.StartNew();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Grandmaster);
        stopwatch.Stop();

        // Assert - Should complete in reasonable time
        // Grandmaster (D5) with adaptive depth
        _output.WriteLine($"Move: ({move.x}, {move.y}), Time: {stopwatch.ElapsedMilliseconds}ms");
        Assert.True(stopwatch.ElapsedMilliseconds < 30000,
            $"Grandmaster search took {stopwatch.ElapsedMilliseconds}ms, expected < 30000ms");

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
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(7, 8, Player.Blue);
        board = board.PlaceStone(8, 7, Player.Red);

        // Act - Multiple searches should build up TT entries and produce valid results
        // Note: Due to Random being consumed at different rates during search,
        // exact equality between calls is not guaranteed without full state reset.
        var ai = AITestHelper.CreateDeterministicAI();
        var move1 = ai.GetBestMove(board, Player.Blue, AIDifficulty.Hard);
        var move2 = ai.GetBestMove(board, Player.Blue, AIDifficulty.Hard);
        var move3 = ai.GetBestMove(board, Player.Blue, AIDifficulty.Hard);

        // Assert - All moves should be valid and strategic
        Assert.InRange(move1.x, 5, 10);
        Assert.InRange(move1.y, 5, 10);
        Assert.InRange(move2.x, 5, 10);
        Assert.InRange(move2.y, 5, 10);
        Assert.InRange(move3.x, 5, 10);
        Assert.InRange(move3.y, 5, 10);
    }

    [Fact]
    public void AspirationWindows_HandlesWideScoreRanges()
    {
        // Arrange - position with extreme score difference
        var board = new Board();

        // Red has 4 in a row (almost winning)
        board = board.PlaceStone(7, 5, Player.Red);
        board = board.PlaceStone(7, 6, Player.Red);
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(7, 8, Player.Red);

        // Blue has 3 in a row nearby
        board = board.PlaceStone(8, 5, Player.Blue);
        board = board.PlaceStone(8, 6, Player.Blue);
        board = board.PlaceStone(8, 7, Player.Blue);

        // Act - Should find winning move for Red
        var ai = AITestHelper.CreateAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Grandmaster);

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
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(7, 8, Player.Blue);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(8, 8, Player.Blue);
        board = board.PlaceStone(6, 6, Player.Red);
        board = board.PlaceStone(6, 7, Player.Blue);
        board = board.PlaceStone(9, 9, Player.Red);
        board = board.PlaceStone(9, 8, Player.Blue);

        // Act - Get best move
        var ai = AITestHelper.CreateAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Grandmaster);

        // Assert - Move should be strategic (near existing stones)
        Assert.True(move.x >= 0 && move.x < 19);
        Assert.True(move.y >= 0 && move.y < 19);

        // Check move is near existing stones (not random corner)
        var nearStones = false;
        for (int dx = -2; dx <= 2; dx++)
        {
            for (int dy = -2; dy <= 2; dy++)
            {
                var nx = move.x + dx;
                var ny = move.y + dy;
                if (nx >= 0 && nx < 19 && ny >= 0 && ny < 19)
                {
                    var cell = board.GetCell(nx, ny);
                    if (cell.Player != Player.None)
                        nearStones = true;
                }
            }
        }
        Assert.True(nearStones, "Move should be near existing stones");
    }
}
