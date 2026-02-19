using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using FluentAssertions;
using Xunit;

namespace Caro.Core.Tests.GameLogic;

/// <summary>
/// Tests to diagnose the diagonal win detection issue found in tournament Game 8.
/// Braindead won against Grandmaster because (17,9) wasn't detected as a winning square.
/// </summary>
public class DiagonalWinDetectionTests
{
    [Fact]
    public void IsWinningMove_ShouldDetectDiagonalWin_At_17_9()
    {
        // Arrange: Recreate the position from Game 8 before move 61
        // Blue has 4 stones on diagonal: (13,13), (14,12), (15,11), (16,10)
        // Red has various stones including (12,14) (but this was played on move 61)
        // Before move 61, (17,9) should be detected as a winning move for Blue

        var board = new Board();

        // Place Blue stones on the diagonal (in order they were played in Game 8)
        board = board.PlaceStone(13, 13, Player.Blue);  // Move 32
        board = board.PlaceStone(14, 12, Player.Blue);  // Move 42
        board = board.PlaceStone(15, 11, Player.Blue);  // Move 60
        board = board.PlaceStone(16, 10, Player.Blue);  // Move 52

        // Place some Red stones (simulating the game state)
        // These are approximate - exact positions from game log
        board = board.PlaceStone(16, 16, Player.Red);
        board = board.PlaceStone(13, 16, Player.Red);
        board = board.PlaceStone(16, 17, Player.Red);
        board = board.PlaceStone(13, 18, Player.Red);
        board = board.PlaceStone(16, 18, Player.Red);
        board = board.PlaceStone(17, 16, Player.Red);
        board = board.PlaceStone(11, 17, Player.Red);
        board = board.PlaceStone(18, 16, Player.Red);
        board = board.PlaceStone(19, 16, Player.Red);
        board = board.PlaceStone(17, 17, Player.Red);
        board = board.PlaceStone(19, 15, Player.Red);
        board = board.PlaceStone(15, 17, Player.Red);
        board = board.PlaceStone(14, 16, Player.Red);
        board = board.PlaceStone(14, 19, Player.Red);
        board = board.PlaceStone(15, 15, Player.Red);
        board = board.PlaceStone(14, 14, Player.Red);
        board = board.PlaceStone(16, 14, Player.Red);
        board = board.PlaceStone(17, 15, Player.Red);
        board = board.PlaceStone(16, 13, Player.Red);
        board = board.PlaceStone(17, 13, Player.Red);
        board = board.PlaceStone(17, 11, Player.Red);
        board = board.PlaceStone(16, 12, Player.Red);
        board = board.PlaceStone(16, 11, Player.Red);
        board = board.PlaceStone(11, 16, Player.Red);
        board = board.PlaceStone(15, 14, Player.Red);
        board = board.PlaceStone(14, 15, Player.Red);
        board = board.PlaceStone(15, 12, Player.Red);

        var threatDetector = new ThreatDetector();

        // Act: Check if (17,9) is a winning move for Blue
        bool isWinning = threatDetector.IsWinningMove(board, 17, 9, Player.Blue);

        // Assert: Should be true - Blue gets 5 in a row on the diagonal
        isWinning.Should().BeTrue("Blue should win with 5 in a row: (13,13)-(14,12)-(15,11)-(16,10)-(17,9)");
    }

    [Fact]
    public void CheckWin_ShouldDetectDiagonalFiveInARow()
    {
        // Arrange: Simpler test - just 5 blue stones on a diagonal
        var board = new Board();

        board = board.PlaceStone(13, 13, Player.Blue);
        board = board.PlaceStone(14, 12, Player.Blue);
        board = board.PlaceStone(15, 11, Player.Blue);
        board = board.PlaceStone(16, 10, Player.Blue);
        board = board.PlaceStone(17, 9, Player.Blue);  // Completes 5 in a row

        var winDetector = new WinDetector();

        // Act
        var result = winDetector.CheckWin(board);

        // Assert
        result.HasWinner.Should().BeTrue("Should detect 5 in a row on diagonal");
        result.Winner.Should().Be(Player.Blue);
    }

    [Fact]
    public void CheckWin_ShouldDetectFourInARow_WithOpenEnd()
    {
        // Arrange: 4 blue stones on a diagonal, open at one end
        var board = new Board();

        board = board.PlaceStone(13, 13, Player.Blue);
        board = board.PlaceStone(14, 12, Player.Blue);
        board = board.PlaceStone(15, 11, Player.Blue);
        board = board.PlaceStone(16, 10, Player.Blue);
        // (17,9) is empty - would complete 5 in a row

        var winDetector = new WinDetector();

        // Act: Check current board - should NOT be a win yet
        var result = winDetector.CheckWin(board);

        // Assert
        result.HasWinner.Should().BeFalse("Only 4 stones, not a win yet");
    }
}
