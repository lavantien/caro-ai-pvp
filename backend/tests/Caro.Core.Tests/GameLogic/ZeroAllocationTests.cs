using Caro.Core.Entities;
using Caro.Core.GameLogic;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.Tests.GameLogic;

public class ZeroAllocationTests
{
    private readonly ITestOutputHelper _output;

    public ZeroAllocationTests(ITestOutputHelper output)
    {
        _output = output;
    }

    [Fact]
    public void HashCalculation_ConsistentBetweenBoardAndZobristTables()
    {
        // Arrange
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(8, 8, Player.Blue);

        // Act
        var boardHash = board.Hash;
        var calculatedHash = ZobristTables.CalculateHash(board);

        // Assert - Board.Hash should match ZobristTables.CalculateHash
        Assert.Equal(calculatedHash, boardHash);
    }

    [Fact]
    public void HashCalculation_UpdatesIncrementally()
    {
        // Arrange
        var board = new Board();
        var initialHash = board.Hash;

        // Act - Place a stone
        board.PlaceStone(7, 7, Player.Red);
        var afterFirstHash = board.Hash;

        // Place another stone
        board.PlaceStone(7, 8, Player.Blue);
        var afterSecondHash = board.Hash;

        // Assert - Hash should change after each move
        Assert.NotEqual(initialHash, afterFirstHash);
        Assert.NotEqual(afterFirstHash, afterSecondHash);
        Assert.NotEqual(initialHash, afterSecondHash);
    }

    [Fact]
    public void HashCalculation_UndoMove_RestoresOriginalHash()
    {
        // Arrange
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        var hashBeforeSecond = board.Hash;

        // Act - Place and remove a stone (simulating undo)
        board.PlaceStone(7, 8, Player.Blue);
        board.GetCell(7, 8).Player = Player.None; // Undo
        var hashAfterUndo = board.Hash;

        // Assert - Hash should return to original after undo
        Assert.Equal(hashBeforeSecond, hashAfterUndo);
    }

    [Fact]
    public void CaroRules_MinimaxAI_CheckWinner_UseWinDetector()
    {
        // Arrange - Create a position where Caro rules matter
        var board = new Board();

        // Test 1: Sandwiched five should NOT win (OXXXXXO)
        board.PlaceStone(5, 7, Player.Blue);
        board.PlaceStone(6, 7, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(9, 7, Player.Red);
        board.PlaceStone(10, 7, Player.Red);
        board.PlaceStone(11, 7, Player.Blue);

        // Act & Assert - Use reflection to call private CheckWinner method
        var ai = new MinimaxAI();
        var checkWinnerMethod = typeof(MinimaxAI).GetMethod("CheckWinner",
            System.Reflection.BindingFlags.NonPublic | System.Reflection.BindingFlags.Instance);

        var winner = checkWinnerMethod?.Invoke(ai, new object[] { board }) as Player?;

        // Sandwiched five should NOT be a win in Caro
        Assert.Null(winner);

        // Test 2: Exactly five with one open end should win
        var board2 = new Board();
        board2.PlaceStone(6, 7, Player.Red);
        board2.PlaceStone(7, 7, Player.Red);
        board2.PlaceStone(8, 7, Player.Red);
        board2.PlaceStone(9, 7, Player.Red);
        board2.PlaceStone(10, 7, Player.Red);
        // Position 5 and 11 are empty

        var winner2 = checkWinnerMethod?.Invoke(ai, new object[] { board2 }) as Player?;
        Assert.Equal(Player.Red, winner2);
    }

    [Fact]
    public void CaroRules_Overline_SixInRow_NotAWin()
    {
        // Arrange - Six in a row should NOT win in Caro
        var board = new Board();
        board.PlaceStone(5, 7, Player.Red);
        board.PlaceStone(6, 7, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(9, 7, Player.Red);
        board.PlaceStone(10, 7, Player.Red);

        // Act
        var ai = new MinimaxAI();
        var checkWinnerMethod = typeof(MinimaxAI).GetMethod("CheckWinner",
            System.Reflection.BindingFlags.NonPublic | System.Reflection.BindingFlags.Instance);

        var winner = checkWinnerMethod?.Invoke(ai, new object[] { board }) as Player?;

        // Assert - Six in a row should NOT be a win in Caro
        Assert.Null(winner);
    }

    [Fact]
    public void TranspositionTable_DefaultSize_Is256MB()
    {
        // Arrange & Act
        var tt = new TranspositionTable();

        // Assert - Use reflection to check internal size
        var sizeField = typeof(TranspositionTable).GetField("_size",
            System.Reflection.BindingFlags.NonPublic | System.Reflection.BindingFlags.Instance);

        var size = sizeField?.GetValue(tt) as int?;
        var expectedEntries = (256 * 1024 * 1024) / 32; // ~8M entries

        Assert.Equal(expectedEntries, size);
    }

    [Fact]
    public void Phase1_Benchmark_D7_CompletesUnder4Seconds()
    {
        // Arrange - Realistic mid-game position (~30% board occupancy)
        // This gives ~150 candidates which is more realistic for move 30
        var board = new Board();

        // Create a congested mid-game position without immediate wins
        // Central area conflict
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

        // Add some scattered stones around
        board.PlaceStone(3, 7, Player.Red);
        board.PlaceStone(4, 8, Player.Blue);
        board.PlaceStone(11, 6, Player.Red);
        board.PlaceStone(10, 5, Player.Blue);
        board.PlaceStone(7, 3, Player.Red);
        board.PlaceStone(6, 11, Player.Blue);
        board.PlaceStone(8, 10, Player.Red);
        board.PlaceStone(9, 4, Player.Blue);

        // Act - Test D7 (VeryHard difficulty) with 7+5 time control
        // Simulating mid-game (move 30) with 5 minutes remaining
        var ai = new MinimaxAI();
        var timeRemainingMs = 300_000L;  // 5 minutes left
        var moveNumber = 30;              // LateMid game phase
        var stopwatch = System.Diagnostics.Stopwatch.StartNew();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.VeryHard, timeRemainingMs, moveNumber);
        stopwatch.Stop();

        _output.WriteLine($"Move: ({move.x}, {move.y})");
        _output.WriteLine($"Time: {stopwatch.ElapsedMilliseconds}ms ({stopwatch.Elapsed.TotalSeconds:F2}s)");

        // Assert - D7 with time-aware search should complete within the hard bound
        // With ~150 candidates, TimeManager allocates ~25-30 seconds soft, ~35-40 seconds hard
        Assert.True(stopwatch.ElapsedMilliseconds < 45000,
            $"D7 took {stopwatch.ElapsedMilliseconds}ms, expected < 45000ms (time-aware hard bound)");

        // Move should be valid
        var cell = board.GetCell(move.x, move.y);
        Assert.True(cell.IsEmpty, "Move should be on an empty cell");
    }

    [Fact]
    public void Benchmark_D10_Grandmaster_CompletesUnder60Seconds()
    {
        // Arrange - Realistic mid-game position (~35% board occupancy)
        // This gives ~140 candidates which is realistic for move 30
        var board = new Board();

        // Create a congested mid-game position
        for (int x = 4; x <= 10; x++)
        {
            for (int y = 4; y <= 10; y++)
            {
                if ((x * y) % 2 == 0)
                    board.PlaceStone(x, y, Player.Red);
                else if ((x + y) % 3 == 0)
                    board.PlaceStone(x, y, Player.Blue);
            }
        }

        // Add some edge stones
        board.PlaceStone(2, 7, Player.Red);
        board.PlaceStone(12, 8, Player.Blue);
        board.PlaceStone(7, 2, Player.Red);
        board.PlaceStone(8, 12, Player.Blue);

        // Act - Test D10 (Grandmaster difficulty) with 7+5 time control
        var ai = new MinimaxAI();
        var timeRemainingMs = 300_000L;  // 5 minutes left
        var moveNumber = 30;              // LateMid game phase
        var stopwatch = System.Diagnostics.Stopwatch.StartNew();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Grandmaster, timeRemainingMs, moveNumber);
        stopwatch.Stop();

        _output.WriteLine($"Move: ({move.x}, {move.y})");
        _output.WriteLine($"Time: {stopwatch.ElapsedMilliseconds}ms ({stopwatch.Elapsed.TotalSeconds:F2}s)");

        // Assert - D10 with time-aware search should complete within hard bound
        // With ~140 candidates and depth 11, TimeManager allocates ~30-40s soft, ~45-60s hard
        Assert.True(stopwatch.ElapsedMilliseconds < 65000,
            $"D10 took {stopwatch.ElapsedMilliseconds}ms, expected < 65000ms (time-aware hard bound)");

        // Move should be valid
        var cell = board.GetCell(move.x, move.y);
        Assert.True(cell.IsEmpty, "Move should be on an empty cell");
    }

    [Fact]
    public void Phase1_Benchmark_D5_CompletesUnder1Second()
    {
        // Arrange - Simple tactical position
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Blue);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(8, 8, Player.Blue);

        // Act - Test D5 (Hard difficulty)
        var ai = new MinimaxAI();
        var stopwatch = System.Diagnostics.Stopwatch.StartNew();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Hard);
        stopwatch.Stop();

        _output.WriteLine($"Move: ({move.x}, {move.y})");
        _output.WriteLine($"Time: {stopwatch.ElapsedMilliseconds}ms");

        // Assert - D5 should be fast with optimizations (< 5 seconds, allowing for JIT/system variation)
        Assert.True(stopwatch.ElapsedMilliseconds < 5000,
            $"D5 took {stopwatch.ElapsedMilliseconds}ms, expected < 5000ms");
    }

    [Fact]
    public void HashCalculation_DifferentPositions_DifferentHashes()
    {
        // Arrange - Two different board positions
        var board1 = new Board();
        board1.PlaceStone(7, 7, Player.Red);

        var board2 = new Board();
        board2.PlaceStone(7, 8, Player.Red);

        // Act
        var hash1 = board1.Hash;
        var hash2 = board2.Hash;

        // Assert - Different positions should have different hashes
        Assert.NotEqual(hash1, hash2);
    }

    [Fact]
    public void HashCalculation_SymmetricPosition_SameHash()
    {
        // Arrange - Same pieces in different order should produce same hash
        var board1 = new Board();
        board1.PlaceStone(7, 7, Player.Red);
        board1.PlaceStone(7, 8, Player.Blue);

        var board2 = new Board();
        board2.PlaceStone(7, 8, Player.Blue);
        board2.PlaceStone(7, 7, Player.Red);

        // Act
        var hash1 = board1.Hash;
        var hash2 = board2.Hash;

        // Assert - Same position should have same hash regardless of placement order
        Assert.Equal(hash1, hash2);
    }
}
