using Xunit;
using FluentAssertions;
using Caro.Core.Domain.Entities;
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
            board = board.PlaceStone(i + 5, 7, Player.Red);

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
            board = board.PlaceStone(i + 4, 7, Player.Red);

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
        board = board.PlaceStone(4, 7, Player.Blue);  // Block left
        for (int i = 0; i < 5; i++)
            board = board.PlaceStone(i + 5, 7, Player.Red);
        board = board.PlaceStone(10, 7, Player.Blue); // Block right

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
        board = board.PlaceStone(4, 7, Player.Blue);  // Block left
        for (int i = 0; i < 5; i++)
            board = board.PlaceStone(i + 5, 7, Player.Red);

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
            board = board.PlaceStone(7, i + 5, Player.Red);

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
            board = board.PlaceStone(5 + i, 5 + i, Player.Red);

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
            board = board.PlaceStone(9 + i, 5 - i, Player.Red);

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

    [Fact]
    public void CheckWin_HorizontalWin_ReturnsWinningLineCoordinates()
    {
        // Arrange
        var board = new Board();
        for (int i = 0; i < 5; i++)
            board = board.PlaceStone(i + 5, 7, Player.Red);

        // Act
        var result = _detector.CheckWin(board);

        // Assert
        result.WinningLine.Should().HaveCount(5);
        result.WinningLine[0].X.Should().Be(5);
        result.WinningLine[0].Y.Should().Be(7);
        result.WinningLine[1].X.Should().Be(6);
        result.WinningLine[1].Y.Should().Be(7);
        result.WinningLine[2].X.Should().Be(7);
        result.WinningLine[2].Y.Should().Be(7);
        result.WinningLine[3].X.Should().Be(8);
        result.WinningLine[3].Y.Should().Be(7);
        result.WinningLine[4].X.Should().Be(9);
        result.WinningLine[4].Y.Should().Be(7);
    }

    [Fact]
    public void CheckWin_VerticalWin_ReturnsWinningLineCoordinates()
    {
        // Arrange
        var board = new Board();
        for (int i = 0; i < 5; i++)
            board = board.PlaceStone(7, i + 5, Player.Red);

        // Act
        var result = _detector.CheckWin(board);

        // Assert
        result.WinningLine.Should().HaveCount(5);
        result.WinningLine[0].X.Should().Be(7);
        result.WinningLine[0].Y.Should().Be(5);
        result.WinningLine[1].X.Should().Be(7);
        result.WinningLine[1].Y.Should().Be(6);
        result.WinningLine[2].X.Should().Be(7);
        result.WinningLine[2].Y.Should().Be(7);
        result.WinningLine[3].X.Should().Be(7);
        result.WinningLine[3].Y.Should().Be(8);
        result.WinningLine[4].X.Should().Be(7);
        result.WinningLine[4].Y.Should().Be(9);
    }

    [Fact]
    public void CheckWin_DiagonalWin_ReturnsWinningLineCoordinates()
    {
        // Arrange
        var board = new Board();
        for (int i = 0; i < 5; i++)
            board = board.PlaceStone(5 + i, 5 + i, Player.Red);

        // Act
        var result = _detector.CheckWin(board);

        // Assert
        result.WinningLine.Should().HaveCount(5);
        result.WinningLine[0].X.Should().Be(5);
        result.WinningLine[0].Y.Should().Be(5);
        result.WinningLine[1].X.Should().Be(6);
        result.WinningLine[1].Y.Should().Be(6);
        result.WinningLine[2].X.Should().Be(7);
        result.WinningLine[2].Y.Should().Be(7);
        result.WinningLine[3].X.Should().Be(8);
        result.WinningLine[3].Y.Should().Be(8);
        result.WinningLine[4].X.Should().Be(9);
        result.WinningLine[4].Y.Should().Be(9);
    }
}

// Tests for the static CheckWinFromMove method (efficient last-move check)
public sealed class CheckWinFromMoveTests
{
    [Fact]
    public void CheckWinFromMove_Exactly5Horizontal_ReturnsWinningLine()
    {
        var board = new Board();
        for (int i = 0; i < 5; i++)
            board = board.PlaceStone(5 + i, 7, Player.Red);

        var result = WinDetector.CheckWinFromMove(board, 9, 7, Player.Red);

        result.Should().HaveCount(5);
        result[0].Should().Be(new Position(5, 7));
        result[4].Should().Be(new Position(9, 7));
    }

    [Fact]
    public void CheckWinFromMove_Exactly5Vertical_ReturnsWinningLine()
    {
        var board = new Board();
        for (int i = 0; i < 5; i++)
            board = board.PlaceStone(7, 5 + i, Player.Red);

        var result = WinDetector.CheckWinFromMove(board, 7, 9, Player.Red);

        result.Should().HaveCount(5);
        result[0].Should().Be(new Position(7, 5));
        result[4].Should().Be(new Position(7, 9));
    }

    [Fact]
    public void CheckWinFromMove_Exactly5DiagonalDownRight_ReturnsWinningLine()
    {
        var board = new Board();
        for (int i = 0; i < 5; i++)
            board = board.PlaceStone(5 + i, 5 + i, Player.Red);

        var result = WinDetector.CheckWinFromMove(board, 9, 9, Player.Red);

        result.Should().HaveCount(5);
        result[0].Should().Be(new Position(5, 5));
        result[4].Should().Be(new Position(9, 9));
    }

    [Fact]
    public void CheckWinFromMove_Exactly5DiagonalDownLeft_ReturnsWinningLine()
    {
        // Direction (1, -1): stones at (5,9), (6,8), (7,7), (8,6), (9,5)
        var board = new Board();
        for (int i = 0; i < 5; i++)
            board = board.PlaceStone(5 + i, 9 - i, Player.Red);

        var result = WinDetector.CheckWinFromMove(board, 5, 9, Player.Red);

        result.Should().HaveCount(5);
        result[0].Should().Be(new Position(5, 9));
        result[4].Should().Be(new Position(9, 5));
    }

    [Fact]
    public void CheckWinFromMove_Overline6InRow_ReturnsEmpty()
    {
        var board = new Board();
        for (int i = 0; i < 6; i++)
            board = board.PlaceStone(4 + i, 7, Player.Red);

        var result = WinDetector.CheckWinFromMove(board, 9, 7, Player.Red);

        result.Should().BeEmpty();
    }

    [Fact]
    public void CheckWinFromMove_BothEndsBlocked_ReturnsEmpty()
    {
        var board = new Board();
        board = board.PlaceStone(4, 7, Player.Blue);
        for (int i = 0; i < 5; i++)
            board = board.PlaceStone(5 + i, 7, Player.Red);
        board = board.PlaceStone(10, 7, Player.Blue);

        var result = WinDetector.CheckWinFromMove(board, 9, 7, Player.Red);

        result.Should().BeEmpty();
    }

    [Fact]
    public void CheckWinFromMove_OneEndBlocked_ReturnsWin()
    {
        var board = new Board();
        board = board.PlaceStone(4, 7, Player.Blue);
        for (int i = 0; i < 5; i++)
            board = board.PlaceStone(5 + i, 7, Player.Red);

        var result = WinDetector.CheckWinFromMove(board, 9, 7, Player.Red);

        result.Should().HaveCount(5);
    }

    [Fact]
    public void CheckWinFromMove_WinAtBoardEdge_ReturnsWin()
    {
        var board = new Board();
        for (int i = 0; i < 5; i++)
            board = board.PlaceStone(i, 7, Player.Red);

        var result = WinDetector.CheckWinFromMove(board, 4, 7, Player.Red);

        result.Should().HaveCount(5);
        result[0].X.Should().Be(0);
        result[4].X.Should().Be(4);
    }

    [Fact]
    public void CheckWinFromMove_NoWin_ReturnsEmpty()
    {
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 8, Player.Blue);

        var result = WinDetector.CheckWinFromMove(board, 8, 8, Player.Blue);

        result.Should().BeEmpty();
    }

    [Fact]
    public void CheckWinFromMove_WrongPlayer_ReturnsEmpty()
    {
        var board = new Board();
        for (int i = 0; i < 5; i++)
            board = board.PlaceStone(5 + i, 7, Player.Red);

        var result = WinDetector.CheckWinFromMove(board, 9, 7, Player.Blue);

        result.Should().BeEmpty();
    }
}
