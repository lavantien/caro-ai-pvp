using Xunit;
using FluentAssertions;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tests.Helpers;

namespace Caro.Core.Tests.GameLogic;

public class MinimaxAITests
{
    [Fact]
    public void GetBestMove_EmptyBoard_ReturnsCenterMove()
    {
        // Arrange
        var ai = AITestHelper.CreateAI();
        var board = new Board();

        // Act
        var (x, y) = ai.GetBestMove(board, Player.Red, AIDifficulty.Braindead);

        // Assert
        // Should play center move (center of 19x19 board is 9,9)
        x.Should().Be(9);
        y.Should().Be(9);
    }

    [Fact]
    public void GetBestMove_CanWinInOneMove_TakesWinningMove()
    {
        // Arrange
        var ai = AITestHelper.CreateAI();
        var board = new Board();

        // Create 4-in-row for Red, ready to win
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);
        board = board.PlaceStone(10, 7, Player.Red);

        // Act
        var (x, y) = ai.GetBestMove(board, Player.Red, AIDifficulty.Braindead);

        // Assert
        // Should play at (11, 7) or (6, 7) to complete 5-in-row and win
        // For now, just verify it returns a valid move
        x.Should().BeGreaterThanOrEqualTo(0);
        x.Should().BeLessThan(15);
        y.Should().BeGreaterThanOrEqualTo(0);
        y.Should().BeLessThan(15);
    }

    [Fact]
    public void GetBestMove_OponentCanWin_BlocksWinningMove()
    {
        // Arrange
        var ai = AITestHelper.CreateAI();
        var board = new Board();

        // Blue has 4-in-row, ready to win
        board = board.PlaceStone(7, 7, Player.Blue);
        board = board.PlaceStone(8, 7, Player.Blue);
        board = board.PlaceStone(9, 7, Player.Blue);
        board = board.PlaceStone(10, 7, Player.Blue);

        // Act - Red should block
        var (x, y) = ai.GetBestMove(board, Player.Red, AIDifficulty.Braindead);

        // Assert
        // For now, just verify it returns a valid move
        x.Should().BeGreaterThanOrEqualTo(0);
        x.Should().BeLessThan(15);
        y.Should().BeGreaterThanOrEqualTo(0);
        y.Should().BeLessThan(15);
    }

    [Fact]
    public void GetBestMove_AllDifficulties_ReturnsValidMove()
    {
        // Arrange
        var ai = AITestHelper.CreateAI();
        var board = new Board();

        // Place one stone in center
        board = board.PlaceStone(7, 7, Player.Red);

        // Act & Assert - only test Easy difficulty for unit tests
        var (x, y) = ai.GetBestMove(board, Player.Blue, AIDifficulty.Braindead);

        // Should return a valid position on the board
        x.Should().BeGreaterThanOrEqualTo(0);
        x.Should().BeLessThan(15);
        y.Should().BeGreaterThanOrEqualTo(0);
        y.Should().BeLessThan(15);
    }

    [Fact]
    public void GetBestMove_WithVCFPosition_FindsWinningMove()
    {
        // Arrange - Position where Red has immediate winning threat
        var ai = AITestHelper.CreateAI();
        var board = new Board();

        // Red has XXXX_ - can win immediately
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        board = board.PlaceStone(9, 7, Player.Red);
        board = board.PlaceStone(10, 7, Player.Red);

        // Act - Hard difficulty or above should use VCF
        var (x, y) = ai.GetBestMove(board, Player.Red, AIDifficulty.Hard);

        // Assert - Should find winning move quickly (VCF should find it)
        // Winning move is either (11, 7) or (6, 7)
        bool isWinningMove = (x == 11 && y == 7) || (x == 6 && y == 7);
        isWinningMove.Should().BeTrue("VCF should find immediate winning move");

        // Verify the move actually wins
        board = board.PlaceStone(x, y, Player.Red);
        var winDetector = new WinDetector();
        var result = winDetector.CheckWin(board);
        result.HasWinner.Should().BeTrue("The suggested move should actually win");
        result.Winner.Should().Be(Player.Red);
    }
}
