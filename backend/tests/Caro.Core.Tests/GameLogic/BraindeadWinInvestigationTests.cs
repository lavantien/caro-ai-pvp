using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using FluentAssertions;
using Xunit;

namespace Caro.Core.Tests.GameLogic;

/// <summary>
/// Tests to investigate the 2 games where Braindead beat Grandmaster.
/// We need to find what threats Grandmaster missed.
/// </summary>
public class BraindeadWinInvestigationTests
{
    /// <summary>
    /// Game 3: Braindead (Red) wins with (20,18) on move 31.
    /// Red had a diagonal threat that Grandmaster missed.
    /// </summary>
    [Fact]
    public void Game3_ShouldDetectDiagonalThreat_Before_20_18()
    {
        // Arrange: Recreate board state before move 31 (Grandmaster's move 30 was (15,13))
        var board = new Board();

        // Red stones (Braindead) from Game 3 moves 1,3,5,7,9,11,13,15,17,19,21,23,25,27,29
        // Move order: (0,20), (13,16), (7,5), (14,16), (12,16), (11,16), (14,15), (18,16),
        //             (16,14), (16,19), (15,14), (19,18), (19,17), (14,17), (17,15)
        board = board.PlaceStone(0, 20, Player.Red);
        board = board.PlaceStone(13, 16, Player.Red);
        board = board.PlaceStone(7, 5, Player.Red);
        board = board.PlaceStone(14, 16, Player.Red);
        board = board.PlaceStone(12, 16, Player.Red);
        board = board.PlaceStone(11, 16, Player.Red);
        board = board.PlaceStone(14, 15, Player.Red);
        board = board.PlaceStone(18, 16, Player.Red);
        board = board.PlaceStone(16, 14, Player.Red);  // Key stone for diagonal
        board = board.PlaceStone(16, 19, Player.Red);
        board = board.PlaceStone(15, 14, Player.Red);
        board = board.PlaceStone(19, 18, Player.Red);
        board = board.PlaceStone(19, 17, Player.Red);  // Key stone for diagonal
        board = board.PlaceStone(14, 17, Player.Red);
        board = board.PlaceStone(17, 15, Player.Red);  // Key stone for diagonal

        // Blue stones (Grandmaster) from Game 3 moves 2,4,6,8,10,12,14,16,18,20,22,24,26,28,30
        // Move order: (7,16), (16,16), (0,0), (16,17), (15,16), (10,16), (17,16), (16,15),
        //             (16,18), (18,17), (16,13), (17,17), (15,17), (14,18), (15,13)
        board = board.PlaceStone(7, 16, Player.Blue);
        board = board.PlaceStone(16, 16, Player.Blue);
        board = board.PlaceStone(0, 0, Player.Blue);
        board = board.PlaceStone(16, 17, Player.Blue);
        board = board.PlaceStone(15, 16, Player.Blue);
        board = board.PlaceStone(10, 16, Player.Blue);
        board = board.PlaceStone(17, 16, Player.Blue);
        board = board.PlaceStone(16, 15, Player.Blue);
        board = board.PlaceStone(16, 18, Player.Blue);
        board = board.PlaceStone(18, 17, Player.Blue);
        board = board.PlaceStone(16, 13, Player.Blue);
        board = board.PlaceStone(17, 17, Player.Blue);
        board = board.PlaceStone(15, 17, Player.Blue);
        board = board.PlaceStone(14, 18, Player.Blue);
        board = board.PlaceStone(15, 13, Player.Blue);  // Move 30 - Grandmaster's last move before loss

        var threatDetector = new ThreatDetector();

        // Act: Check if (20,18) is a winning move for Red (diagonal 5 in a row)
        // Diagonal: (16,14), (17,15), (18,16), (19,17), (20,18)
        bool isWinning = threatDetector.IsWinningMove(board, 20, 18, Player.Red);

        // Also check if this is detected as an immediate threat Blue should have blocked
        bool isBlockingRequired = threatDetector.IsWinningMove(board, 20, 18, Player.Blue);

        // Assert
        isWinning.Should().BeTrue("Red should win with diagonal: (16,14)-(17,15)-(18,16)-(19,17)-(20,18)");

        // Check what Grandmaster should have blocked - Red had open four at (16,14),(17,15),(18,16),(19,17)
        // Grandmaster played (15,13) which didn't block (20,18)
    }

    /// <summary>
    /// Game 3: Verify the exact diagonal pattern Red had before winning.
    /// </summary>
    [Fact]
    public void Game3_VerifyRedDiagonalOpenFour()
    {
        var board = new Board();

        // Place Red's diagonal stones: (16,14), (17,15), (18,16), (19,17)
        board = board.PlaceStone(16, 14, Player.Red);
        board = board.PlaceStone(17, 15, Player.Red);
        board = board.PlaceStone(18, 16, Player.Red);
        board = board.PlaceStone(19, 17, Player.Red);

        var threatDetector = new ThreatDetector();

        // Check that (20,18) would win
        bool isWinning = threatDetector.IsWinningMove(board, 20, 18, Player.Red);

        // Also check (15,13) which is the other end of this diagonal
        bool isWinningOtherEnd = threatDetector.IsWinningMove(board, 15, 13, Player.Red);

        isWinning.Should().BeTrue("(20,18) should complete 5 in a row");
        isWinningOtherEnd.Should().BeTrue("(15,13) should also complete 5 in a row");

        // This is an OPEN FOUR - Grandmaster should have detected this as critical threat
    }

    /// <summary>
    /// Game 10: Braindead (Blue) wins with (10,19) on move 120.
    /// Investigate what winning pattern Blue had.
    /// </summary>
    [Fact]
    public void Game10_InvestigateWinningPattern_10_19()
    {
        // This game is long (120 moves), let's trace Blue's stones around (10,19)
        // From the log, Blue played: (10,15), (10,17), (10,18), (10,19)
        // And also: (9,15), (11,18)

        var board = new Board();

        // Place Blue stones along row 10
        board = board.PlaceStone(10, 15, Player.Blue);
        board = board.PlaceStone(10, 17, Player.Blue);
        board = board.PlaceStone(10, 18, Player.Blue);
        // (10,19) is the winning move

        // Check what winning pattern this creates
        var winDetector = new WinDetector();
        board = board.PlaceStone(10, 19, Player.Blue);
        var result = winDetector.CheckWin(board);

        result.HasWinner.Should().BeFalse("4 in row with gap at (10,16) should not be a win");

        // Let's check the antidiagonal instead
        var board2 = new Board();
        board2 = board2.PlaceStone(14, 15, Player.Blue);
        board2 = board2.PlaceStone(13, 16, Player.Blue);
        board2 = board2.PlaceStone(12, 17, Player.Blue);
        board2 = board2.PlaceStone(11, 18, Player.Blue);
        // (10,19) completes antidiagonal

        var result2 = winDetector.CheckWin(board2.PlaceStone(10, 19, Player.Blue));
        result2.HasWinner.Should().BeTrue("Antidiagonal 5 in a row: (14,15)-(13,16)-(12,17)-(11,18)-(10,19)");
    }

    /// <summary>
    /// Game 3: Check board state BEFORE Red's move 29 (17,15).
    /// At this point Red should have only 3 on the diagonal, which Grandmaster should have seen
    /// as a developing threat but not an immediate win.
    /// </summary>
    [Fact]
    public void Game3_CheckState_BeforeRedMove29()
    {
        // Before Red's move 29 (17,15), Red had these diagonal stones:
        // (16,14), (18,16), (19,17) - only 3 stones, with a GAP at (17,15)
        // Blue had (16,17) which is adjacent but not blocking the diagonal

        var board = new Board();
        board = board.PlaceStone(16, 14, Player.Red);
        board = board.PlaceStone(18, 16, Player.Red);
        board = board.PlaceStone(19, 17, Player.Red);
        // (17,15) is empty - Red will play here on move 29

        var threatDetector = new ThreatDetector();

        // Check if (17,15) would create an immediate threat
        bool isWinning = threatDetector.IsWinningMove(board, 17, 15, Player.Red);

        // (17,15) does NOT win immediately - it creates an open four
        // Check the full diagonal: (15,13), (16,14), (17,15), (18,16), (19,17), (20,18)
        // With (16,14), (17,15), (18,16), (19,17) Red has 4, but not 5 yet
        isWinning.Should().BeFalse("(17,15) creates an open four, not immediate win");

        // The issue is that open fours are not detected as "immediate wins" but are
        // still critical threats that need to be blocked!
    }

    /// <summary>
    /// Game 3: After Red's move 29, verify there are exactly 2 winning squares.
    /// </summary>
    [Fact]
    public void Game3_AfterRedMove29_ShouldHaveTwoWinningSquares()
    {
        // After Red plays (17,15), the diagonal is: (16,14), (17,15), (18,16), (19,17)
        // Both (15,13) and (20,18) complete 5-in-a-row
        var board = new Board();
        board = board.PlaceStone(16, 14, Player.Red);
        board = board.PlaceStone(17, 15, Player.Red);
        board = board.PlaceStone(18, 16, Player.Red);
        board = board.PlaceStone(19, 17, Player.Red);

        var threatDetector = new ThreatDetector();
        var winningSquares = new List<(int, int)>();

        // Scan for winning squares
        for (int x = 0; x < 32; x++)
        {
            for (int y = 0; y < 32; y++)
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
        winningSquares.Should().Contain((15, 13), "(15,13) is one winning square");
        winningSquares.Should().Contain((20, 18), "(20,18) is the other winning square");

        // This confirms the issue: Grandmaster blocked ONE square (15,13) but the other (20,18) won
    }

    /// <summary>
    /// Test that Grandmaster's threat detection would find these patterns.
    /// </summary>
    [Fact]
    public void ThreatDetector_ShouldFindOpenFour_WithGapInMiddle()
    {
        // Test pattern: 4 stones with one gap
        // X _ X X X  (positions 0, 2, 3, 4 - gap at 1)
        var board = new Board();
        board = board.PlaceStone(10, 10, Player.Red);
        board = board.PlaceStone(10, 12, Player.Red);
        board = board.PlaceStone(10, 13, Player.Red);
        board = board.PlaceStone(10, 14, Player.Red);

        var threatDetector = new ThreatDetector();

        // Check if opponent needs to block at (10,11) or (10,15)
        bool mustBlock11 = threatDetector.IsWinningMove(board, 10, 11, Player.Red);
        bool mustBlock15 = threatDetector.IsWinningMove(board, 10, 15, Player.Red);

        // (10,11) fills the gap to make 5: (10,10)-(10,11)-(10,12)-(10,13)-(10,14)
        mustBlock11.Should().BeTrue("Filling gap at (10,11) creates 5 in a row");
        // (10,15) does NOT win because the pattern is X _ X X X and (10,15) gives X _ X X X X (still a gap at 10,11)
        mustBlock15.Should().BeFalse("(10,15) does not complete 5 in a row due to gap at (10,11)");
    }

    /// <summary>
    /// Test proactive defense: AI should block open threes before they become open fours.
    /// </summary>
    [Fact]
    public void ProactiveDefense_ShouldBlockOpenThree()
    {
        // Arrange: Create an open three (3 in a row with both ends open)
        // Pattern: _ X X X _ (positions 10,11-13 with gaps at 10,10 and 10,14)
        var board = new Board();
        board = board.PlaceStone(10, 11, Player.Red);
        board = board.PlaceStone(10, 12, Player.Red);
        board = board.PlaceStone(10, 13, Player.Red);

        // Blue (Grandmaster) needs to block this open three
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Blue, AIDifficulty.Grandmaster, timeRemainingMs: 30000, moveNumber: 1);

        // Assert: Blue should block at either end of the open three
        var validBlocks = new[] { (10, 10), (10, 14) };
        validBlocks.Should().Contain((move.x, move.y),
            "Blue should block the open three at one of its ends before it becomes an open four");
    }

    /// <summary>
    /// Test proactive defense: AI should block open three on diagonal.
    /// </summary>
    [Fact]
    public void ProactiveDefense_ShouldBlockDiagonalOpenThree()
    {
        // Arrange: Create a diagonal open three
        // Pattern: diagonal from (16,14) to (18,16) with gaps at (15,13) and (19,17)
        var board = new Board();
        board = board.PlaceStone(16, 14, Player.Red);
        board = board.PlaceStone(17, 15, Player.Red);
        board = board.PlaceStone(18, 16, Player.Red);

        // Blue (Grandmaster) needs to block this diagonal open three
        var ai = new MinimaxAI();
        var move = ai.GetBestMove(board, Player.Blue, AIDifficulty.Grandmaster, timeRemainingMs: 30000, moveNumber: 1);

        // Assert: Blue should block at either end of the diagonal open three
        var validBlocks = new[] { (15, 13), (19, 17) };
        validBlocks.Should().Contain((move.x, move.y),
            "Blue should block the diagonal open three before it becomes an open four");
    }
}
