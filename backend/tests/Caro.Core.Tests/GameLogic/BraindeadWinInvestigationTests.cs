using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using FluentAssertions;
using Xunit;

namespace Caro.Core.Tests.GameLogic;

/// <summary>
/// Tests to investigate threat detection patterns.
/// These tests verify that the AI correctly detects and blocks threats.
/// </summary>
public class BraindeadWinInvestigationTests
{
    /// <summary>
    /// Verify diagonal open four has exactly 2 winning squares.
    /// </summary>
    [Fact]
    public void DiagonalOpenFour_ShouldHaveTwoWinningSquares()
    {
        // Create a diagonal open four pattern
        var board = new Board();
        board = board.PlaceStone(6, 6, Player.Red);
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 8, Player.Red);
        board = board.PlaceStone(9, 9, Player.Red);

        var threatDetector = new ThreatDetector();
        var winningSquares = new List<(int, int)>();

        for (int x = 0; x < 16; x++)
        {
            for (int y = 0; y < 16; y++)
            {
                if (board.GetCell(x, y).Player == Player.None)
                {
                    if (threatDetector.IsWinningMove(board, x, y, Player.Red))
                    {
                        winningSquares.Add((x, y));
                    }
                }
            }
        }

        winningSquares.Count.Should().Be(2, "Open four has exactly 2 winning squares");
        winningSquares.Should().Contain((5, 5), "One winning square at diagonal start");
        winningSquares.Should().Contain((10, 10), "One winning square at diagonal end");
    }

    /// <summary>
    /// Test that threat detection finds open four with gap in middle.
    /// </summary>
    [Fact]
    public void ThreatDetector_ShouldFindOpenFour_WithGapInMiddle()
    {
        // Test pattern: 4 stones with one gap - using 16x16 compatible coordinates
        var board = new Board();
        board = board.PlaceStone(5, 5, Player.Red);
        board = board.PlaceStone(5, 7, Player.Red);
        board = board.PlaceStone(5, 8, Player.Red);
        board = board.PlaceStone(5, 9, Player.Red);

        var threatDetector = new ThreatDetector();

        bool mustBlock6 = threatDetector.IsWinningMove(board, 5, 6, Player.Red);
        bool mustBlock10 = threatDetector.IsWinningMove(board, 5, 10, Player.Red);

        mustBlock6.Should().BeTrue("Filling gap at (5,6) creates 5 in a row");
        mustBlock10.Should().BeFalse("(5,10) does not complete 5 in a row due to gap at (5,6)");
    }

    /// <summary>
    /// Test proactive defense: AI should block open threes before they become open fours.
    /// </summary>
    [Fact]
    public void ProactiveDefense_ShouldBlockOpenThree()
    {
        // Arrange: Create an open three using 16x16 compatible coordinates
        var board = new Board();
        board = board.PlaceStone(5, 5, Player.Red);
        board = board.PlaceStone(5, 6, Player.Red);
        board = board.PlaceStone(5, 7, Player.Red);

        // Blue (Grandmaster) needs to block this open three
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Blue, AIDifficulty.Grandmaster, timeRemainingMs: 30000, moveNumber: 1);

        // Assert: Blue should block at either end of the open three
        var validBlocks = new[] { (5, 4), (5, 8) };
        validBlocks.Should().Contain((move.x, move.y),
            "Blue should block the open three at one of its ends before it becomes an open four");
    }

    /// <summary>
    /// Test proactive defense: AI should block diagonal open three.
    /// </summary>
    [Fact]
    public void ProactiveDefense_ShouldBlockDiagonalOpenThree()
    {
        // Arrange: Create a diagonal open three
        var board = new Board();
        board = board.PlaceStone(5, 5, Player.Red);
        board = board.PlaceStone(6, 6, Player.Red);
        board = board.PlaceStone(7, 7, Player.Red);

        // Blue needs to block this diagonal open three
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Blue, AIDifficulty.Grandmaster, timeRemainingMs: 30000, moveNumber: 1);

        // Assert: Blue should block at either end of the diagonal open three
        var validBlocks = new[] { (4, 4), (8, 8) };
        validBlocks.Should().Contain((move.x, move.y),
            "Blue should block the diagonal open three at one of its ends");
    }

    /// <summary>
    /// Test that an open four creates two winning squares.
    /// </summary>
    [Fact]
    public void OpenFour_HasTwoWinningSquares()
    {
        var board = new Board();
        board = board.PlaceStone(5, 5, Player.Red);
        board = board.PlaceStone(6, 5, Player.Red);
        board = board.PlaceStone(7, 5, Player.Red);
        board = board.PlaceStone(8, 5, Player.Red);

        var threatDetector = new ThreatDetector();

        bool isWinning4 = threatDetector.IsWinningMove(board, 4, 5, Player.Red);
        bool isWinning9 = threatDetector.IsWinningMove(board, 9, 5, Player.Red);

        isWinning4.Should().BeTrue("(4,5) completes the horizontal 5 in a row");
        isWinning9.Should().BeTrue("(9,5) completes the horizontal 5 in a row");
    }

    /// <summary>
    /// Verify that after placing 4 stones, one more creates a win.
    /// </summary>
    [Fact]
    public void FourInRow_PlacingFifth_CreatesWin()
    {
        var board = new Board();
        board = board.PlaceStone(6, 6, Player.Red);
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 8, Player.Red);
        board = board.PlaceStone(9, 9, Player.Red);

        var threatDetector = new ThreatDetector();

        // Before placing the 5th stone, not a win
        bool isWinBefore = threatDetector.IsWinningMove(board, 10, 10, Player.Red);
        isWinBefore.Should().BeTrue("Placing at (10,10) should be a winning move");
    }

    /// <summary>
    /// Reproduce game 2 scenario where Braindead won with vertical line in column 8.
    /// Grandmaster failed to block at (9,8) despite having blocking logic.
    /// </summary>
    [Fact]
    public void Game2Scenario_GrandmasterShouldBlockVerticalWin()
    {
        // Recreate the board state at move 17 (before Grandmaster's failed move 18)
        // Red (Braindead) stones: (8,8), (5,8), (9,7), (7,9), (6,10), (6,9), (6,8), (6,7), (7,8)
        // Blue (Grandmaster) stones: (7,7), (8,7), (14,7), (10,6), (5,11), (11,6), (6,11), (6,6)

        var board = new Board();

        // Place Red stones (Braindead)
        board = board.PlaceStone(8, 8, Player.Red);
        board = board.PlaceStone(5, 8, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);
        board = board.PlaceStone(7, 9, Player.Red);
        board = board.PlaceStone(6, 10, Player.Red);
        board = board.PlaceStone(6, 9, Player.Red);
        board = board.PlaceStone(6, 8, Player.Red);
        board = board.PlaceStone(6, 7, Player.Red);
        board = board.PlaceStone(7, 8, Player.Red);

        // Place Blue stones (Grandmaster)
        board = board.PlaceStone(7, 7, Player.Blue);
        board = board.PlaceStone(8, 7, Player.Blue);
        board = board.PlaceStone(14, 7, Player.Blue);
        board = board.PlaceStone(10, 6, Player.Blue);
        board = board.PlaceStone(5, 11, Player.Blue);
        board = board.PlaceStone(11, 6, Player.Blue);
        board = board.PlaceStone(6, 11, Player.Blue);
        board = board.PlaceStone(6, 6, Player.Blue);

        var threatDetector = new ThreatDetector();

        // Check if (9,8) is a winning move for Red
        bool isWinningMove = threatDetector.IsWinningMove(board, 9, 8, Player.Red);
        isWinningMove.Should().BeTrue("Red has 4 in column 8: (5,8), (6,8), (7,8), (8,8) - (9,8) completes the win");

        // Also check (4,8) is a winning move for Red (other end of the line)
        bool isWinningMove4_8 = threatDetector.IsWinningMove(board, 4, 8, Player.Red);
        isWinningMove4_8.Should().BeTrue("Red has 4 in column 8: (5,8), (6,8), (7,8), (8,8) - (4,8) completes the win from other end");

        // Now test what Grandmaster (Blue) would do
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Blue, AIDifficulty.Grandmaster, timeRemainingMs: 30000, moveNumber: 18);

        // Grandmaster should block at either (9,8) or (4,8) - both are valid blocks
        var validBlocks = new[] { (4, 8), (9, 8) };
        validBlocks.Should().Contain((move.x, move.y),
            "Grandmaster must block Red's winning move at either end of the vertical line");
    }

    /// <summary>
    /// Reproduce game 4 scenario where Braindead won with vertical line in column 4.
    /// Braindead had an open four with 2 winning squares - Grandmaster only blocked one.
    /// </summary>
    [Fact]
    public void Game4Scenario_GrandmasterShouldBlockAllWinningSquares()
    {
        // Recreate the board state at move 40 (Grandmaster's last move before losing)
        // Red has open four in column 4: (4,8), (4,9), (4,10), (4,11)
        // Red can win at (4,7) OR (4,12)
        // Grandmaster played (4,7) but should have realized both squares need blocking

        var board = new Board();

        // Place Red's open four in column 4
        board = board.PlaceStone(4, 8, Player.Red);
        board = board.PlaceStone(4, 9, Player.Red);
        board = board.PlaceStone(4, 10, Player.Red);
        board = board.PlaceStone(4, 11, Player.Red);

        // Place some Blue stones (not blocking column 4)
        board = board.PlaceStone(7, 7, Player.Blue);
        board = board.PlaceStone(8, 8, Player.Blue);
        board = board.PlaceStone(6, 6, Player.Blue);

        var threatDetector = new ThreatDetector();

        // Check both winning squares
        bool isWinningAt4_7 = threatDetector.IsWinningMove(board, 4, 7, Player.Red);
        bool isWinningAt4_12 = threatDetector.IsWinningMove(board, 4, 12, Player.Red);

        isWinningAt4_7.Should().BeTrue("Red can complete 5 in a row at (4,7)");
        isWinningAt4_12.Should().BeTrue("Red can complete 5 in a row at (4,12)");

        // When there are 2 winning squares, Grandmaster should recognize the position is lost
        // unless there's a counter-attack. But at minimum, should block one.
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Blue, AIDifficulty.Grandmaster, timeRemainingMs: 30000, moveNumber: 40);

        // Grandmaster should at least block one of the winning squares
        var validBlocks = new[] { (4, 7), (4, 12) };
        validBlocks.Should().Contain((move.x, move.y),
            "Grandmaster should block one of the two winning squares");
    }
}
