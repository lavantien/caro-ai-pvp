using Xunit;
using FluentAssertions;
using Caro.Core.Entities;

namespace Caro.Core.Tests.Entities;

public class BoardTests
{
    [Fact]
    public void Board_InitialState_HasCorrectDimensions()
    {
        // Act
        var board = new Board();

        // Assert
        board.BoardSize.Should().Be(19);
        board.Cells.Should().HaveCount(361);
    }

    [Fact]
    public void PlaceStone_ValidPosition_UpdatesCellState()
    {
        // Arrange
        var board = new Board();

        // Act
        board.PlaceStone(7, 7, Player.Red);

        // Assert
        board.GetCell(7, 7).Player.Should().Be(Player.Red);
        board.GetCell(7, 7).IsEmpty.Should().BeFalse();
    }

    [Theory]
    [InlineData(-1, 0)]
    [InlineData(0, -1)]
    [InlineData(19, 0)]
    [InlineData(0, 19)]
    public void PlaceStone_InvalidPosition_ThrowsArgumentOutOfRangeException(int x, int y)
    {
        // Arrange
        var board = new Board();

        // Act
        Action act = () => board.PlaceStone(x, y, Player.Red);

        // Assert
        act.Should().Throw<ArgumentOutOfRangeException>();
    }

    [Fact]
    public void PlaceStone_OnOccupiedCell_ThrowsInvalidOperationException()
    {
        // Arrange
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);

        // Act
        Action act = () => board.PlaceStone(7, 7, Player.Blue);

        // Assert
        act.Should().Throw<InvalidOperationException>();
    }
}
