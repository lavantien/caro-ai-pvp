using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using FluentAssertions;
using Xunit;

namespace Caro.Core.Tests.GameLogic;

/// <summary>
/// Tests to diagnose the diagonal win detection issue found in tournament games.
/// Tests are adjusted for 16x16 board.
/// </summary>
public class DiagonalWinDetectionTests
{
    [Fact]
    public void IsWinningMove_ShouldDetectDiagonalWin_AtCorner()
    {
        // Arrange: Test diagonal win detection
        // Blue has 4 stones on diagonal: (5,5), (6,6), (7,7), (8,8)
        // (9,9) should be detected as a winning move for Blue

        var board = new Board();

        // Place Blue stones on the diagonal
        board = board.PlaceStone(5, 5, Player.Blue);
        board = board.PlaceStone(6, 6, Player.Blue);
        board = board.PlaceStone(7, 7, Player.Blue);
        board = board.PlaceStone(8, 8, Player.Blue);

        // Place some Red stones (simulating the game state)
        board = board.PlaceStone(8, 8 + 1, Player.Red); // adjacent but not blocking
        board = board.PlaceStone(5, 5 + 1, Player.Red);

        var threatDetector = new ThreatDetector();

        // Act: Check if (9,9) is a winning move for Blue
        bool isWinning = threatDetector.IsWinningMove(board, 9, 9, Player.Blue);

        // Assert: Should be true - Blue gets 5 in a row on the diagonal
        isWinning.Should().BeTrue("Blue should win with 5 in a row: (5,5)-(6,6)-(7,7)-(8,8)-(9,9)");
    }

    [Fact]
    public void CheckWin_ShouldDetectDiagonalFiveInARow()
    {
        // Arrange: Simpler test - just 5 blue stones on a diagonal
        var board = new Board();

        board = board.PlaceStone(5, 5, Player.Blue);
        board = board.PlaceStone(6, 6, Player.Blue);
        board = board.PlaceStone(7, 7, Player.Blue);
        board = board.PlaceStone(8, 8, Player.Blue);
        board = board.PlaceStone(9, 9, Player.Blue);  // Completes 5 in a row

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

        board = board.PlaceStone(5, 5, Player.Blue);
        board = board.PlaceStone(6, 6, Player.Blue);
        board = board.PlaceStone(7, 7, Player.Blue);
        board = board.PlaceStone(8, 8, Player.Blue);
        // (9,9) is empty - would complete 5 in a row

        var winDetector = new WinDetector();

        // Act: Check current board - should NOT be a win yet
        var result = winDetector.CheckWin(board);

        // Assert
        result.HasWinner.Should().BeFalse("Only 4 stones, not a win yet");
    }
}
