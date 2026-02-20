using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using FluentAssertions;
using Xunit;

namespace Caro.Core.Tests.GameLogic;

/// <summary>
/// Diagnostic tests to understand why Braindead keeps winning against Grandmaster.
/// Investigates threat detection and blocking behavior.
/// </summary>
public class ThreatDetectionDiagnosticTests
{
    /// <summary>
    /// Simulate Game 4 from the 5-game tournament where Braindead won.
    /// Braindead (Blue) won with (10,11) on move 44.
    /// Grandmaster (Red) blocked at (10,6) on move 43.
    ///
    /// This confirms that open fours have TWO winning squares - blocking one lets opponent win at the other.
    /// </summary>
    [Fact]
    public void Game4_OpenFourHasTwoWinningSquares()
    {
        // This test traces back from the winning move to understand the threat pattern
        var board = new Board();

        // Place a test pattern: Blue stones on column 10 (vertical line)
        // This is an OPEN FOUR with winning squares at (10,6) and (10,11)
        // Using coordinates valid for 16x16 board (max 15)
        board = board.PlaceStone(10, 7, Player.Blue);
        board = board.PlaceStone(10, 8, Player.Blue);
        board = board.PlaceStone(10, 9, Player.Blue);
        board = board.PlaceStone(10, 10, Player.Blue);

        var threatDetector = new ThreatDetector();

        // Check if (10,11) is a winning move
        bool isWin11 = threatDetector.IsWinningMove(board, 10, 11, Player.Blue);
        isWin11.Should().BeTrue("(10,11) should complete vertical 5-in-a-row: (10,7)-(10,8)-(10,9)-(10,10)-(10,11)");

        // Check if (10,6) is ALSO a winning move - this is what makes it an open four!
        bool isWin6 = threatDetector.IsWinningMove(board, 10, 6, Player.Blue);
        isWin6.Should().BeTrue("(10,6) should ALSO complete vertical 5-in-a-row: (10,6)-(10,7)-(10,8)-(10,9)-(10,10)");

        // Both are winning squares - this is the definition of an open four!
        // Grandmaster blocked (10,6), but Braindead won at (10,11).

        // The real issue: why didn't Grandmaster block when it was an open THREE?
    }

    /// <summary>
    /// Test that open THREE should be detected and blocked BEFORE it becomes open four.
    /// </summary>
    [Fact]
    public void OpenThree_ShouldBeBlocked_BeforeItBecomesOpenFour()
    {
        // Create an open three for Blue (3 in a row with both ends open)
        var board = new Board();
        board = board.PlaceStone(10, 8, Player.Blue);
        board = board.PlaceStone(10, 9, Player.Blue);
        board = board.PlaceStone(10, 10, Player.Blue);
        // (10,7) and (10,11) are open

        var threatDetector = new ThreatDetector();

        // Neither (10,7) nor (10,11) should be winning moves yet
        bool isWin7 = threatDetector.IsWinningMove(board, 10, 7, Player.Blue);
        bool isWin11 = threatDetector.IsWinningMove(board, 10, 11, Player.Blue);
        isWin7.Should().BeFalse("(10,7) should NOT be a winning move yet - only 3 in a row");
        isWin11.Should().BeFalse("(10,11) should NOT be a winning move yet - only 3 in a row");

        // But this IS an open three - both ends are open
        // The FindOpenThreeBlocks method should detect this
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Grandmaster, timeRemainingMs: 60000, moveNumber: 1);

        // Grandmaster should block at one end of the open three
        var validBlocks = new[] { (10, 7), (10, 11) };
        validBlocks.Should().Contain((move.x, move.y),
            "Grandmaster should block the open three at one end before it becomes an open four");
    }

    /// <summary>
    /// Test scenario: Grandmaster blocks at one location but Braindead has another threat.
    /// This is the "multiple independent threats" scenario.
    /// </summary>
    [Fact]
    public void MultipleIndependentThreats_ShouldDetectBoth()
    {
        // Create two separate open fours for Blue
        var board = new Board();

        // First open four: horizontal at row 5
        board = board.PlaceStone(5, 5, Player.Blue);
        board = board.PlaceStone(5, 6, Player.Blue);
        board = board.PlaceStone(5, 7, Player.Blue);
        board = board.PlaceStone(5, 8, Player.Blue);
        // (5,4) and (5,9) complete this open four

        // Second open four: vertical at column 10
        board = board.PlaceStone(7, 10, Player.Blue);
        board = board.PlaceStone(8, 10, Player.Blue);
        board = board.PlaceStone(9, 10, Player.Blue);
        board = board.PlaceStone(10, 10, Player.Blue);
        // (6,10) and (11,10) complete this open four

        var threatDetector = new ThreatDetector();

        // Find ALL winning squares for Blue
        var winningSquares = new List<(int x, int y)>();
        for (int x = 0; x < GameConstants.BoardSize; x++)
        {
            for (int y = 0; y < GameConstants.BoardSize; y++)
            {
                if (board.GetCell(x, y).Player == Player.None)
                {
                    if (threatDetector.IsWinningMove(board, x, y, Player.Blue))
                    {
                        winningSquares.Add((x, y));
                    }
                }
            }
        }

        // Should find 4 winning squares (2 per open four)
        winningSquares.Count.Should().Be(4, "Two open fours = 4 winning squares total");
        winningSquares.Should().Contain((5, 4), "First open four - left end");
        winningSquares.Should().Contain((5, 9), "First open four - right end");
        winningSquares.Should().Contain((6, 10), "Second open four - top end");
        winningSquares.Should().Contain((11, 10), "Second open four - bottom end");

        // Now simulate Red blocking at (5,4)
        var boardAfterBlock = board.PlaceStone(5, 4, Player.Red);

        // Blue can still win at (5,9), (6,10), or (11,10)
        var remainingWinningSquares = new List<(int x, int y)>();
        for (int x = 0; x < GameConstants.BoardSize; x++)
        {
            for (int y = 0; y < GameConstants.BoardSize; y++)
            {
                if (boardAfterBlock.GetCell(x, y).Player == Player.None)
                {
                    if (threatDetector.IsWinningMove(boardAfterBlock, x, y, Player.Blue))
                    {
                        remainingWinningSquares.Add((x, y));
                    }
                }
            }
        }

        remainingWinningSquares.Count.Should().Be(3, "After blocking one, 3 winning squares remain");
    }

    /// <summary>
    /// Test the immediate block logic in MinimaxAI with multiple independent threats.
    /// </summary>
    [Fact]
    public void MinimaxAI_ImmediateBlock_ShouldHandleMultipleIndependentThreats()
    {
        // Create a board with two independent open fours
        var board = new Board();

        // First open four for Blue (opponent)
        board = board.PlaceStone(5, 5, Player.Blue);
        board = board.PlaceStone(5, 6, Player.Blue);
        board = board.PlaceStone(5, 7, Player.Blue);
        board = board.PlaceStone(5, 8, Player.Blue);

        // Second open four for Blue
        board = board.PlaceStone(7, 10, Player.Blue);
        board = board.PlaceStone(8, 10, Player.Blue);
        board = board.PlaceStone(9, 10, Player.Blue);
        board = board.PlaceStone(10, 10, Player.Blue);

        // Red's turn - Grandmaster should detect and try to block
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Grandmaster, timeRemainingMs: 60000, moveNumber: 1);

        // Grandmaster should block at one of the winning squares
        var validBlocks = new[] { (5, 4), (5, 9), (6, 10), (11, 10) };
        validBlocks.Should().Contain((move.x, move.y), "Grandmaster should block at a winning square");

        // But this is expected to fail - there's no single move that blocks both open fours!
        // The AI should block one, and then Blue wins at the other.
    }

    /// <summary>
    /// Test that FindOpenThreeBlocks correctly identifies open threes.
    /// </summary>
    [Fact]
    public void FindOpenThreeBlocks_ShouldDetectOpenThree()
    {
        // Create an open three for Blue
        var board = new Board();
        board = board.PlaceStone(5, 6, Player.Blue);
        board = board.PlaceStone(5, 7, Player.Blue);
        board = board.PlaceStone(5, 8, Player.Blue);
        // Both (5,5) and (5,9) are open

        var threatDetector = new ThreatDetector();
        var threats = threatDetector.DetectThreats(board, Player.Blue);

        // Should detect the open three
        threats.Count.Should().BeGreaterThan(0, "Should detect open three");

        // The threat should be a StraightThree with 2 gain squares
        var straightThree = threats.FirstOrDefault(t => t.Type == ThreatType.StraightThree);
        straightThree.Should().NotBeNull("Should detect StraightThree");
        straightThree!.GainSquares.Count.Should().Be(2, "Open three has 2 gain squares (both ends)");
    }

    /// <summary>
    /// Test that Grandmaster blocks open three BEFORE it becomes open four.
    /// This simulates the sequence: Braindead creates open three -> Grandmaster blocks -> Braindead creates new threat
    /// </summary>
    [Fact]
    public void Grandmaster_ShouldBlockOpenThree_ThenHandleNextThreat()
    {
        // Create an open three for Blue (Braindead)
        var board = new Board();
        board = board.PlaceStone(5, 6, Player.Blue);
        board = board.PlaceStone(5, 7, Player.Blue);
        board = board.PlaceStone(5, 8, Player.Blue);
        // Both (5,5) and (5,9) are open

        var ai = new MinimaxAI();

        // Grandmaster's turn - should block the open three
        var move1 = ai.GetBestMove(board, Player.Red, AIDifficulty.Grandmaster, timeRemainingMs: 60000, moveNumber: 1);
        var validBlocks = new[] { (5, 5), (5, 9) };
        validBlocks.Should().Contain((move1.x, move1.y), "Grandmaster should block the open three");

        // Simulate Grandmaster blocking
        var board2 = board.PlaceStone(move1.x, move1.y, Player.Red);

        // Braindead plays at the other end of the open three
        // If Grandmaster blocked at (5,5), Braindead plays (5,9)
        // If Grandmaster blocked at (5,9), Braindead plays (5,5)
        var (braindeadX, braindeadY) = move1.x == 5 && move1.y == 5 ? (5, 9) : (5, 5);
        var board3 = board2.PlaceStone(braindeadX, braindeadY, Player.Blue);

        // Now Blue has 4 in a row with one end blocked (semi-open four)
        // Determine which square would complete the 5 in a row
        var threatDetector = new ThreatDetector();
        int winningY = braindeadY == 9 ? 10 : 4; // If Blue extended to (5,9), winning square is (5,10); otherwise (5,4)

        // But wait - if Blue played at (5,5) and Grandmaster blocked at (5,9),
        // Blue has stones at (5,5), (5,6), (5,7), (5,8) with (5,9) blocked
        // The winning square would be (5,4)
        if (braindeadY == 5)
        {
            // Blue extended left: (5,5)-(5,6)-(5,7)-(5,8) with (5,9) blocked
            // Winning square is (5,4)
            winningY = 4;
        }

        bool isWinning = threatDetector.IsWinningMove(board3, 5, winningY, Player.Blue);
        isWinning.Should().BeTrue($"(5,{winningY}) should complete the semi-open four");

        // Grandmaster's turn again - should block the winning square
        var move2 = ai.GetBestMove(board3, Player.Red, AIDifficulty.Grandmaster, timeRemainingMs: 60000, moveNumber: 3);
        move2.Should().Be((5, winningY), "Grandmaster should block the semi-open four");
    }

}
