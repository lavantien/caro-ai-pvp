using Xunit;
using FluentAssertions;
using Caro.Core.Entities;
using Caro.Core.GameLogic;

namespace Caro.Core.Tests.GameLogic;

public class WinDetectorTests
{
    private readonly WinDetector _detector = new();

    [Fact]
    public void CheckWin_Exactly5InRow_ReturnsWin()
    {
        // Arrange
        var board = new Board();
        for (int i = 0; i < 5; i++)
            board.PlaceStone(i + 5, 7, Player.Red);

        // Act
        var result = _detector.CheckWin(board);

        // Assert
        result.HasWinner.Should().BeTrue();
        result.Winner.Should().Be(Player.Red);
    }

    [Fact]
    public void CheckWin_6InRow_ReturnsNoWin()  // Overline rule
    {
        // Arrange
        var board = new Board();
        for (int i = 0; i < 6; i++)
            board.PlaceStone(i + 4, 7, Player.Red);

        // Act
        var result = _detector.CheckWin(board);

        // Assert
        result.HasWinner.Should().BeFalse();
    }

    [Fact]
    public void CheckWin_5InRowWithBlockedEnds_ReturnsNoWin()
    {
        // Arrange
        var board = new Board();
        board.PlaceStone(4, 7, Player.Blue);  // Block left
        for (int i = 0; i < 5; i++)
            board.PlaceStone(i + 5, 7, Player.Red);
        board.PlaceStone(10, 7, Player.Blue); // Block right

        // Act
        var result = _detector.CheckWin(board);

        // Assert
        result.HasWinner.Should().BeFalse();
    }

    [Fact]
    public void CheckWin_5InRowWithOneBlockedEnd_ReturnsWin()
    {
        // Arrange
        var board = new Board();
        board.PlaceStone(4, 7, Player.Blue);  // Block left
        for (int i = 0; i < 5; i++)
            board.PlaceStone(i + 5, 7, Player.Red);

        // Act
        var result = _detector.CheckWin(board);

        // Assert
        result.HasWinner.Should().BeTrue();
        result.Winner.Should().Be(Player.Red);
    }

    [Fact]
    public void CheckWin_5InColumn_ReturnsWin()
    {
        // Arrange
        var board = new Board();
        for (int i = 0; i < 5; i++)
            board.PlaceStone(7, i + 5, Player.Red);

        // Act
        var result = _detector.CheckWin(board);

        // Assert
        result.HasWinner.Should().BeTrue();
        result.Winner.Should().Be(Player.Red);
    }

    [Fact]
    public void CheckWin_5InDiagonalDownRight_ReturnsWin()
    {
        // Arrange
        var board = new Board();
        for (int i = 0; i < 5; i++)
            board.PlaceStone(5 + i, 5 + i, Player.Red);

        // Act
        var result = _detector.CheckWin(board);

        // Assert
        result.HasWinner.Should().BeTrue();
        result.Winner.Should().Be(Player.Red);
    }

    [Fact]
    public void CheckWin_5InDiagonalDownLeft_ReturnsWin()
    {
        // Arrange
        var board = new Board();
        for (int i = 0; i < 5; i++)
            board.PlaceStone(9 + i, 5 - i, Player.Red);

        // Act
        var result = _detector.CheckWin(board);

        // Assert
        result.HasWinner.Should().BeTrue();
        result.Winner.Should().Be(Player.Red);
    }

    [Fact]
    public void CheckWin_EmptyBoard_ReturnsNoWin()
    {
        // Arrange
        var board = new Board();

        // Act
        var result = _detector.CheckWin(board);

        // Assert
        result.HasWinner.Should().BeFalse();
    }
}
