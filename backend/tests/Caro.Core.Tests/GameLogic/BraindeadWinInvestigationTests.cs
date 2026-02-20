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
}
