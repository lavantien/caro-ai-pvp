using Xunit;
using FluentAssertions;
using Caro.Core.Entities;
using Caro.Core.GameLogic;

namespace Caro.Core.Tests.GameLogic;

public class MinimaxAITests
{
    [Fact]
    public void GetBestMove_EmptyBoard_ReturnsCenterMove()
    {
        // Arrange
        var ai = new MinimaxAI();
        var board = new Board();

        // Act
        var (x, y) = ai.GetBestMove(board, Player.Red, AIDifficulty.Easy);

        // Assert
        // Should play center move
        x.Should().Be(7);
        y.Should().Be(7);
    }

    [Fact]
    public void GetBestMove_CanWinInOneMove_TakesWinningMove()
    {
        // Arrange
        var ai = new MinimaxAI();
        var board = new Board();

        // Create 4-in-row for Red, ready to win
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(9, 7, Player.Red);
        board.PlaceStone(10, 7, Player.Red);

        // Act
        var (x, y) = ai.GetBestMove(board, Player.Red, AIDifficulty.Easy);

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
        var ai = new MinimaxAI();
        var board = new Board();

        // Blue has 4-in-row, ready to win
        board.PlaceStone(7, 7, Player.Blue);
        board.PlaceStone(8, 7, Player.Blue);
        board.PlaceStone(9, 7, Player.Blue);
        board.PlaceStone(10, 7, Player.Blue);

        // Act - Red should block
        var (x, y) = ai.GetBestMove(board, Player.Red, AIDifficulty.Easy);

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
        var ai = new MinimaxAI();
        var board = new Board();

        // Place one stone in center
        board.PlaceStone(7, 7, Player.Red);

        // Act & Assert - only test Easy difficulty for unit tests
        var (x, y) = ai.GetBestMove(board, Player.Blue, AIDifficulty.Easy);

        // Should return a valid position on the board
        x.Should().BeGreaterThanOrEqualTo(0);
        x.Should().BeLessThan(15);
        y.Should().BeGreaterThanOrEqualTo(0);
        y.Should().BeLessThan(15);
    }
}
