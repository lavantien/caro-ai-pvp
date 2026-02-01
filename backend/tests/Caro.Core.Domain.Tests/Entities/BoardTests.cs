using Caro.Core.Domain.Entities;
using Caro.Core.Domain.ValueObjects;
using FluentAssertions;

namespace Caro.Core.Domain.Tests.Entities;

public class BoardTests
{
    [Fact]
    public void CreateEmpty_ReturnsEmptyBoard()
    {
        // Act
        var board = Board.CreateEmpty();

        // Assert
        board.IsEmpty().Should().BeTrue();
        board.TotalStones().Should().Be(0);
        board.Hash.Should().NotBe(0);
    }

    [Fact]
    public void PlaceStone_ReturnsNewBoard_WithoutMutatingOriginal()
    {
        // Arrange
        var originalBoard = Board.CreateEmpty();

        // Act
        var newBoard = originalBoard.PlaceStone(9, 9, Player.Red);

        // Assert
        originalBoard.IsEmpty().Should().BeTrue("Original board should still be empty");
        newBoard.IsEmpty().Should().BeFalse("New board should have a stone");
        newBoard.GetCell(9, 9).Should().Be(Player.Red);
    }

    [Fact]
    public void PlaceStone_UpdatesHash()
    {
        // Arrange
        var board = Board.CreateEmpty();
        var originalHash = board.Hash;

        // Act
        var newBoard = board.PlaceStone(9, 9, Player.Red);

        // Assert
        newBoard.Hash.Should().NotBe(originalHash);
    }

    [Fact]
    public void PlaceStone_UpdatesBitBoard()
    {
        // Arrange
        var board = Board.CreateEmpty();

        // Act
        var newBoard = board.PlaceStone(9, 9, Player.Red);

        // Assert
        newBoard.RedBitBoard.GetBit(9, 9).Should().BeTrue();
        newBoard.BlueBitBoard.GetBit(9, 9).Should().BeFalse();
    }

    [Fact]
    public void PlaceStone_MultipleTimes_ReturnsDistinctInstances()
    {
        // Arrange
        var board = Board.CreateEmpty();

        // Act
        var board1 = board.PlaceStone(0, 0, Player.Red);
        var board2 = board1.PlaceStone(1, 0, Player.Blue);
        var board3 = board2.PlaceStone(2, 0, Player.Red);

        // Assert
        board.IsEmpty().Should().BeTrue();
        board1.TotalStones().Should().Be(1);
        board2.TotalStones().Should().Be(2);
        board3.TotalStones().Should().Be(3);
    }

    [Fact]
    public void PlaceStone_AtOccupiedCell_ThrowsInvalidMoveException()
    {
        // Arrange
        var board = Board.CreateEmpty();
        var boardWithStone = board.PlaceStone(9, 9, Player.Red);

        // Act & Assert
        Action act = () => boardWithStone.PlaceStone(9, 9, Player.Blue);
        act.Should().Throw<InvalidMoveException>()
            .WithMessage("*already occupied*");
    }

    [Fact]
    public void PlaceStone_AtInvalidPosition_ThrowsInvalidMoveException()
    {
        // Arrange
        var board = Board.CreateEmpty();

        // Act & Assert
        Action act = () => board.PlaceStone(-1, 9, Player.Red);
        act.Should().Throw<InvalidMoveException>()
            .WithMessage("*out of bounds*");

        act = () => board.PlaceStone(9, 19, Player.Red);
        act.Should().Throw<InvalidMoveException>()
            .WithMessage("*out of bounds*");
    }

    [Fact]
    public void RemoveStone_ReturnsBoardWithoutStone()
    {
        // Arrange
        var board = Board.CreateEmpty();
        var boardWithStone = board.PlaceStone(9, 9, Player.Red);

        // Act
        var boardWithoutStone = boardWithStone.RemoveStone(9, 9);

        // Assert
        boardWithStone.GetCell(9, 9).Should().Be(Player.Red);
        boardWithoutStone.GetCell(9, 9).Should().Be(Player.None);
        boardWithoutStone.IsEmpty().Should().BeTrue();
    }

    [Fact]
    public void RemoveStone_AtEmptyPosition_ReturnsSameInstance()
    {
        // Arrange
        var board = Board.CreateEmpty();

        // Act
        var result = board.RemoveStone(9, 9);

        // Assert
        result.Should().BeSameAs(board);
    }

    [Fact]
    public void GetCell_ReturnsCorrectPlayer()
    {
        // Arrange
        var board = Board.CreateEmpty()
            .PlaceStone(0, 0, Player.Red)
            .PlaceStone(1, 0, Player.Blue);

        // Act & Assert
        board.GetCell(0, 0).Should().Be(Player.Red);
        board.GetCell(1, 0).Should().Be(Player.Blue);
        board.GetCell(2, 0).Should().Be(Player.None);
    }

    [Fact]
    public void IsEmpty_ReturnsTrueForEmptyCells()
    {
        // Arrange
        var board = Board.CreateEmpty().PlaceStone(9, 9, Player.Red);

        // Act & Assert
        board.IsEmpty(9, 9).Should().BeFalse();
        board.IsEmpty(10, 10).Should().BeTrue();
    }

    [Fact]
    public void CountStones_ReturnsCorrectCount()
    {
        // Arrange
        var board = Board.CreateEmpty()
            .PlaceStone(0, 0, Player.Red)
            .PlaceStone(1, 0, Player.Red)
            .PlaceStone(2, 0, Player.Blue);

        // Act & Assert
        board.CountStones(Player.Red).Should().Be(2);
        board.CountStones(Player.Blue).Should().Be(1);
        board.CountStones(Player.None).Should().Be(0);
    }

    [Fact]
    public void TotalStones_ReturnsSumOfAllStones()
    {
        // Arrange
        var board = Board.CreateEmpty()
            .PlaceStone(0, 0, Player.Red)
            .PlaceStone(1, 0, Player.Blue)
            .PlaceStone(2, 0, Player.Red);

        // Act
        var total = board.TotalStones();

        // Assert
        total.Should().Be(3);
    }

    [Fact]
    public void GetBitBoard_ReturnsCorrectBitBoard()
    {
        // Arrange
        var board = Board.CreateEmpty()
            .PlaceStone(5, 5, Player.Red)
            .PlaceStone(6, 5, Player.Blue);

        // Act
        var redBitBoard = board.GetBitBoard(Player.Red);
        var blueBitBoard = board.GetBitBoard(Player.Blue);
        var noneBitBoard = board.GetBitBoard(Player.None);

        // Assert
        redBitBoard.GetBit(5, 5).Should().BeTrue();
        redBitBoard.GetBit(6, 5).Should().BeFalse();

        blueBitBoard.GetBit(6, 5).Should().BeTrue();
        blueBitBoard.GetBit(5, 5).Should().BeFalse();

        noneBitBoard.IsEmpty.Should().BeTrue();
    }

    [Fact]
    public void AllStones_ReturnsCombinedBitBoard()
    {
        // Arrange
        var board = Board.CreateEmpty()
            .PlaceStone(5, 5, Player.Red)
            .PlaceStone(6, 5, Player.Blue);

        // Act
        var allStones = board.AllStones;

        // Assert
        allStones.GetBit(5, 5).Should().BeTrue();
        allStones.GetBit(6, 5).Should().BeTrue();
        allStones.GetBit(7, 5).Should().BeFalse();
    }

    [Fact]
    public void GetEmptyCells_ReturnsOnlyEmptyCells()
    {
        // Arrange
        var board = Board.CreateEmpty()
            .PlaceStone(9, 9, Player.Red)
            .PlaceStone(10, 10, Player.Blue);

        // Act
        var emptyCells = board.GetEmptyCells().ToList();

        // Assert
        emptyCells.Count.Should().Be(Position.TotalCells - 2);
        emptyCells.Should().NotContain(c => c.X == 9 && c.Y == 9);
        emptyCells.Should().NotContain(c => c.X == 10 && c.Y == 10);
    }

    [Fact]
    public void GetOccupiedCells_ReturnsOnlyOccupiedCells()
    {
        // Arrange
        var board = Board.CreateEmpty()
            .PlaceStone(9, 9, Player.Red)
            .PlaceStone(10, 10, Player.Blue);

        // Act
        var redCells = board.GetOccupiedCells(Player.Red).ToList();
        var blueCells = board.GetOccupiedCells(Player.Blue).ToList();

        // Assert
        redCells.Should().ContainSingle(p => p.X == 9 && p.Y == 9);
        blueCells.Should().ContainSingle(p => p.X == 10 && p.Y == 10);
    }

    [Fact]
    public void ApplyMoves_AppliesMultipleMoves()
    {
        // Arrange
        var board = Board.CreateEmpty();
        var moves = new[]
        {
            new Move(9, 9, Player.Red),
            new Move(10, 10, Player.Blue),
            new Move(11, 11, Player.Red)
        };

        // Act
        var result = board.ApplyMoves(moves);

        // Assert
        board.IsEmpty().Should().BeTrue();
        result.TotalStones().Should().Be(3);
        result.GetCell(9, 9).Should().Be(Player.Red);
        result.GetCell(10, 10).Should().Be(Player.Blue);
        result.GetCell(11, 11).Should().Be(Player.Red);
    }

    [Fact]
    public void ToString_ReturnsReadableBoard()
    {
        // Arrange
        var board = Board.CreateEmpty().PlaceStone(0, 0, Player.Red);

        // Act
        var output = board.ToString();

        // Assert
        output.Should().Contain("R");
        output.Should().Contain("...");
    }

    [Fact]
    public void HashCode_IsBasedOnZobristHash()
    {
        // Arrange
        var board1 = Board.CreateEmpty().PlaceStone(9, 9, Player.Red);
        var board2 = Board.CreateEmpty().PlaceStone(9, 9, Player.Red);

        // Act & Assert
        board1.GetHashCode().Should().Be(board2.GetHashCode());
    }

    [Fact]
    public void Equality_SameBoardsAreEqual()
    {
        // Arrange
        var board1 = Board.CreateEmpty().PlaceStone(9, 9, Player.Red);
        var board2 = Board.CreateEmpty().PlaceStone(9, 9, Player.Red);

        // Act & Assert
        board1.Should().Be(board2);
    }

    [Fact]
    public void Equality_DifferentBoardsAreNotEqual()
    {
        // Arrange
        var board1 = Board.CreateEmpty().PlaceStone(9, 9, Player.Red);
        var board2 = Board.CreateEmpty().PlaceStone(10, 10, Player.Red);

        // Act & Assert
        board1.Should().NotBe(board2);
    }

    [Fact]
    public void WithPosition_ReturnsCellAtPosition()
    {
        // Arrange
        var position = new Position(9, 9);

        // Act
        var cell = Cell.Empty(position);

        // Assert
        cell.Position.Should().Be(position);
        cell.IsEmpty.Should().BeTrue();
    }
}
