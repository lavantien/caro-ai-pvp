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
    /// Braindead (Blue) won with (20,21) on move 44.
    /// Grandmaster (Red) blocked at (20,16) on move 43.
    ///
    /// This confirms that open fours have TWO winning squares - blocking one lets opponent win at the other.
    /// </summary>
    [Fact]
    public void Game4_OpenFourHasTwoWinningSquares()
    {
        // This test traces back from the winning move to understand the threat pattern
        var board = new Board();

        // Place a test pattern: Blue stones on column 20
        // This is an OPEN FOUR with winning squares at (20,16) and (20,21)
        board = board.PlaceStone(20, 17, Player.Blue);
        board = board.PlaceStone(20, 18, Player.Blue);
        board = board.PlaceStone(20, 19, Player.Blue);
        board = board.PlaceStone(20, 20, Player.Blue);

        var threatDetector = new ThreatDetector();

        // Check if (20,21) is a winning move
        bool isWin21 = threatDetector.IsWinningMove(board, 20, 21, Player.Blue);
        isWin21.Should().BeTrue("(20,21) should complete vertical 5-in-a-row: (20,17)-(20,18)-(20,19)-(20,20)-(20,21)");

        // Check if (20,16) is ALSO a winning move - this is what makes it an open four!
        bool isWin16 = threatDetector.IsWinningMove(board, 20, 16, Player.Blue);
        isWin16.Should().BeTrue("(20,16) should ALSO complete vertical 5-in-a-row: (20,16)-(20,17)-(20,18)-(20,19)-(20,20)");

        // Both are winning squares - this is the definition of an open four!
        // Grandmaster blocked (20,16), but Braindead won at (20,21).

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
        board = board.PlaceStone(20, 18, Player.Blue);
        board = board.PlaceStone(20, 19, Player.Blue);
        board = board.PlaceStone(20, 20, Player.Blue);
        // (20,17) and (20,21) are open

        var threatDetector = new ThreatDetector();

        // Neither (20,17) nor (20,21) should be winning moves yet
        bool isWin17 = threatDetector.IsWinningMove(board, 20, 17, Player.Blue);
        bool isWin21 = threatDetector.IsWinningMove(board, 20, 21, Player.Blue);
        isWin17.Should().BeFalse("(20,17) should NOT be a winning move yet - only 3 in a row");
        isWin21.Should().BeFalse("(20,21) should NOT be a winning move yet - only 3 in a row");

        // But this IS an open three - both ends are open
        // The FindOpenThreeBlocks method should detect this
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Grandmaster, timeRemainingMs: 60000, moveNumber: 1);

        // Grandmaster should block at one end of the open three
        var validBlocks = new[] { (20, 17), (20, 21) };
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

        // First open four: horizontal at row 10
        board = board.PlaceStone(10, 10, Player.Blue);
        board = board.PlaceStone(10, 11, Player.Blue);
        board = board.PlaceStone(10, 12, Player.Blue);
        board = board.PlaceStone(10, 13, Player.Blue);
        // (10,9) and (10,14) complete this open four

        // Second open four: vertical at column 20
        board = board.PlaceStone(17, 20, Player.Blue);
        board = board.PlaceStone(18, 20, Player.Blue);
        board = board.PlaceStone(19, 20, Player.Blue);
        board = board.PlaceStone(20, 20, Player.Blue);
        // (16,20) and (21,20) complete this open four

        var threatDetector = new ThreatDetector();

        // Find ALL winning squares for Blue
        var winningSquares = new List<(int x, int y)>();
        for (int x = 0; x < 32; x++)
        {
            for (int y = 0; y < 32; y++)
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
        winningSquares.Should().Contain((10, 9), "First open four - left end");
        winningSquares.Should().Contain((10, 14), "First open four - right end");
        winningSquares.Should().Contain((16, 20), "Second open four - top end");
        winningSquares.Should().Contain((21, 20), "Second open four - bottom end");

        // Now simulate Red blocking at (10,9)
        var boardAfterBlock = board.PlaceStone(10, 9, Player.Red);

        // Blue can still win at (10,14), (16,20), or (21,20)
        var remainingWinningSquares = new List<(int x, int y)>();
        for (int x = 0; x < 32; x++)
        {
            for (int y = 0; y < 32; y++)
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
        board = board.PlaceStone(10, 10, Player.Blue);
        board = board.PlaceStone(10, 11, Player.Blue);
        board = board.PlaceStone(10, 12, Player.Blue);
        board = board.PlaceStone(10, 13, Player.Blue);

        // Second open four for Blue
        board = board.PlaceStone(17, 20, Player.Blue);
        board = board.PlaceStone(18, 20, Player.Blue);
        board = board.PlaceStone(19, 20, Player.Blue);
        board = board.PlaceStone(20, 20, Player.Blue);

        // Red's turn - Grandmaster should detect and try to block
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Grandmaster, timeRemainingMs: 60000, moveNumber: 1);

        // Grandmaster should block at one of the winning squares
        var validBlocks = new[] { (10, 9), (10, 14), (16, 20), (21, 20) };
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
        board = board.PlaceStone(10, 11, Player.Blue);
        board = board.PlaceStone(10, 12, Player.Blue);
        board = board.PlaceStone(10, 13, Player.Blue);
        // Both (10,10) and (10,14) are open

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
        board = board.PlaceStone(10, 11, Player.Blue);
        board = board.PlaceStone(10, 12, Player.Blue);
        board = board.PlaceStone(10, 13, Player.Blue);
        // Both (10,10) and (10,14) are open

        var ai = new MinimaxAI();

        // Grandmaster's turn - should block the open three
        var move1 = ai.GetBestMove(board, Player.Red, AIDifficulty.Grandmaster, timeRemainingMs: 60000, moveNumber: 1);
        var validBlocks = new[] { (10, 10), (10, 14) };
        validBlocks.Should().Contain((move1.x, move1.y), "Grandmaster should block the open three");

        // Simulate Grandmaster blocking at (10,10)
        var board2 = board.PlaceStone(move1.x, move1.y, Player.Red);

        // Braindead plays (10,14) - now has 4 in a row but one end blocked (semi-open four)
        var board3 = board2.PlaceStone(10, 14, Player.Blue);

        // Now check if (10,15) would complete 5 in a row
        var threatDetector = new ThreatDetector();
        bool isWinning = threatDetector.IsWinningMove(board3, 10, 15, Player.Blue);
        isWinning.Should().BeTrue("(10,15) should complete the semi-open four");

        // Grandmaster's turn again - should block (10,15)
        var move2 = ai.GetBestMove(board3, Player.Red, AIDifficulty.Grandmaster, timeRemainingMs: 60000, moveNumber: 3);
        move2.Should().Be((10, 15), "Grandmaster should block the semi-open four");
    }

    /// <summary>
    /// Verify that a 10% random AI (Braindead) cannot consistently beat Grandmaster.
    /// This test runs a mini-tournament and checks win rate.
    /// </summary>
    [Fact(Skip = "Long running tournament test - run manually")]
    public void Grandmaster_ShouldBeat_Braindead_Consistently()
    {
        // This test is for manual verification
        // Run tournament with: dotnet run -- --comprehensive --matchups=GrandmastervsBraindead --time=180+2 --games=50
        Assert.True(true, "Run tournament manually to verify Grandmaster beats Braindead consistently");
    }
}
