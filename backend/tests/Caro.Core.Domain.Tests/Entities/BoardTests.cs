using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using FluentAssertions;

namespace Caro.Core.Domain.Tests.Entities;

public class BoardTests
{
    [Fact]
    public void Constructor_ReturnsEmptyBoard()
    {
        // Act
        var board = new Board();

        // Assert
        board.IsEmpty().Should().BeTrue();
    }

    [Fact]
    public void BoardSize_ReturnsCorrectSize()
    {
        // Arrange
        var board = new Board();

        // Act
        var size = board.BoardSize;

        // Assert
        size.Should().Be(GameConstants.BoardSize);
    }

    [Fact]
    public void PlaceStone_ReturnsNewBoard_WithoutMutatingOriginal()
    {
        // Arrange
        var originalBoard = new Board();

        // Act
        var newBoard = originalBoard.PlaceStone(9, 9, Player.Red);

        // Assert
        originalBoard.IsEmpty().Should().BeTrue("Original board should still be empty");
        newBoard.IsEmpty().Should().BeFalse("New board should have a stone");
        newBoard.GetPlayerAt(9, 9).Should().Be(Player.Red);
    }

    [Fact]
    public void PlaceStone_AtOccupiedCell_ThrowsInvalidOperationException()
    {
        // Arrange
        var board = new Board();
        var boardWithStone = board.PlaceStone(9, 9, Player.Red);

        // Act & Assert
        Action act = () => boardWithStone.PlaceStone(9, 9, Player.Blue);
        act.Should().Throw<InvalidOperationException>()
            .WithMessage("*already occupied*");
    }

    [Fact]
    public void PlaceStone_AtInvalidPosition_ThrowsArgumentOutOfRangeException()
    {
        // Arrange
        var board = new Board();

        // Act & Assert
        Action act = () => board.PlaceStone(-1, 9, Player.Red);
        act.Should().Throw<ArgumentOutOfRangeException>();

        act = () => board.PlaceStone(9, GameConstants.BoardSize, Player.Red);
        act.Should().Throw<ArgumentOutOfRangeException>();
    }

    [Fact]
    public void PlaceStone_MultipleTimes_ReturnsDistinctInstances()
    {
        // Arrange
        var board = new Board();

        // Act
        var board1 = board.PlaceStone(0, 0, Player.Red);
        var board2 = board1.PlaceStone(1, 0, Player.Blue);
        var board3 = board2.PlaceStone(2, 0, Player.Red);

        // Assert
        board.IsEmpty().Should().BeTrue();
        board1.IsEmpty().Should().BeFalse();
        board1.GetPlayerAt(0, 0).Should().Be(Player.Red);
        board2.GetPlayerAt(1, 0).Should().Be(Player.Blue);
        board3.GetPlayerAt(2, 0).Should().Be(Player.Red);

        // Verify distinct instances
        board1.Should().NotBeSameAs(board);
        board2.Should().NotBeSameAs(board1);
        board3.Should().NotBeSameAs(board2);
    }

    [Fact]
    public void GetCell_ReturnsCorrectCell()
    {
        // Arrange
        var board = new Board()
            .PlaceStone(0, 0, Player.Red)
            .PlaceStone(1, 0, Player.Blue);

        // Act
        var cell1 = board.GetCell(0, 0);
        var cell2 = board.GetCell(1, 0);
        var cell3 = board.GetCell(2, 0);

        // Assert
        cell1.X.Should().Be(0);
        cell1.Y.Should().Be(0);
        cell1.Player.Should().Be(Player.Red);

        cell2.X.Should().Be(1);
        cell2.Y.Should().Be(0);
        cell2.Player.Should().Be(Player.Blue);

        cell3.X.Should().Be(2);
        cell3.Y.Should().Be(0);
        cell3.Player.Should().Be(Player.None);
    }

    [Fact]
    public void IsEmpty_ReturnsTrueForEmptyBoard()
    {
        // Arrange
        var board = new Board();

        // Act & Assert
        board.IsEmpty().Should().BeTrue();
    }

    [Fact]
    public void IsEmpty_ReturnsTrueForEmptyCells()
    {
        // Arrange
        var board = new Board().PlaceStone(9, 9, Player.Red);

        // Act & Assert
        board.IsEmpty(9, 9).Should().BeFalse();
        board.IsEmpty(10, 10).Should().BeTrue();
    }

    [Fact]
    public void GetPlayerAt_ReturnsPlayerAtPosition()
    {
        // Arrange
        var board = new Board()
            .PlaceStone(0, 0, Player.Red)
            .PlaceStone(1, 0, Player.Blue);

        // Act & Assert
        board.GetPlayerAt(0, 0).Should().Be(Player.Red);
        board.GetPlayerAt(1, 0).Should().Be(Player.Blue);
        board.GetPlayerAt(2, 0).Should().Be(Player.None);
    }

    [Fact]
    public void Cells_EnumeratesAllCells()
    {
        // Arrange
        var board = new Board().PlaceStone(5, 5, Player.Red);

        // Act
        var allCells = board.Cells.ToList();

        // Assert
        allCells.Count.Should().Be(GameConstants.BoardSize * GameConstants.BoardSize);
        allCells.Should().Contain(c => c.X == 5 && c.Y == 5 && c.Player == Player.Red);
    }

    [Fact]
    public void Cell_WithPlayer_ReturnsNewCell()
    {
        // Arrange
        var cell = new Cell(5, 5, Player.None);

        // Act
        var newCell = cell.WithPlayer(Player.Red);

        // Assert
        cell.Player.Should().Be(Player.None);
        newCell.X.Should().Be(5);
        newCell.Y.Should().Be(5);
        newCell.Player.Should().Be(Player.Red);
    }
}
