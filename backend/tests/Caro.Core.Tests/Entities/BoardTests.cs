using Xunit;
using FluentAssertions;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

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
        board = board.PlaceStone(7, 7, Player.Red);

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
        Action act = () => board = board.PlaceStone(x, y, Player.Red);

        // Assert
        act.Should().Throw<ArgumentOutOfRangeException>();
    }

    [Fact]
    public void PlaceStone_OnOccupiedCell_ThrowsInvalidOperationException()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);

        // Act
        Action act = () => board = board.PlaceStone(7, 7, Player.Blue);

        // Assert
        act.Should().Throw<InvalidOperationException>();
    }

    [Fact]
    public void BoardAssignment_ProducesExactCopy_OfCellStates()
    {
        // Arrange
        var original = new Board();
        original = original.PlaceStone(5, 5, Player.Red);
        original = original.PlaceStone(6, 6, Player.Blue);
        original = original.PlaceStone(7, 7, Player.Red);

        // Act - Board is immutable, so assignment creates a reference copy
        var clone = original;

        // Assert - all cell states match
        clone.GetCell(5, 5).Player.Should().Be(Player.Red);
        clone.GetCell(6, 6).Player.Should().Be(Player.Blue);
        clone.GetCell(7, 7).Player.Should().Be(Player.Red);
        clone.GetCell(0, 0).Player.Should().Be(Player.None);
    }

    [Fact]
    public void PlaceStone_DoesNotAffectOriginal_BoardIsImmutable()
    {
        // Arrange
        var original = new Board();
        original = original.PlaceStone(5, 5, Player.Red);

        // Act - PlaceStone returns a new Board (immutable)
        var clone = original.PlaceStone(6, 6, Player.Blue);

        // Assert - original is unchanged because Board is immutable
        original.GetCell(6, 6).Player.Should().Be(Player.None);
        clone.GetCell(6, 6).Player.Should().Be(Player.Blue);
    }

    [Fact]
    public void BoardAssignment_PreservesState_Exactly()
    {
        // Arrange
        var original = new Board();
        original = original.PlaceStone(9, 9, Player.Red);
        original = original.PlaceStone(10, 10, Player.Blue);

        // Act - Board is immutable, assignment creates a reference
        var clone = original;

        // Assert - board state is identical
        for (int x = 0; x < 19; x++)
        {
            for (int y = 0; y < 19; y++)
            {
                clone.GetCell(x, y).Player.Should().Be(original.GetCell(x, y).Player);
            }
        }
    }

    [Fact]
    public void BoardAssignment_PreservesBitBoards()
    {
        // Arrange
        var original = new Board();
        original = original.PlaceStone(5, 5, Player.Red);
        original = original.PlaceStone(6, 6, Player.Blue);
        original = original.PlaceStone(7, 7, Player.Red);

        // Act - Board is immutable, assignment creates a reference
        var clone = original;

        // Assert - BitBoards match
        var originalRed = original.GetBitBoard(Player.Red);
        var cloneRed = clone.GetBitBoard(Player.Red);

        var originalBlue = original.GetBitBoard(Player.Blue);
        var cloneBlue = clone.GetBitBoard(Player.Blue);

        originalRed.GetBit(5, 5).Should().BeTrue();
        cloneRed.GetBit(5, 5).Should().BeTrue();

        originalRed.GetBit(7, 7).Should().BeTrue();
        cloneRed.GetBit(7, 7).Should().BeTrue();

        originalBlue.GetBit(6, 6).Should().BeTrue();
        cloneBlue.GetBit(6, 6).Should().BeTrue();

        // Verify no extra bits set
        for (int x = 0; x < 19; x++)
        {
            for (int y = 0; y < 19; y++)
            {
                bool origRed = originalRed.GetBit(x, y);
                bool clRed = cloneRed.GetBit(x, y);
                clRed.Should().Be(origRed, $"Red BitBoard mismatch at ({x},{y})");

                bool origBlue = originalBlue.GetBit(x, y);
                bool clBlue = cloneBlue.GetBit(x, y);
                clBlue.Should().Be(origBlue, $"Blue BitBoard mismatch at ({x},{y})");
            }
        }
    }

    [Fact]
    public void PlaceStone_AllowsPlacingStones_OnAllCells()
    {
        // Arrange
        var original = new Board();
        original = original.PlaceStone(5, 5, Player.Red);
        original = original.PlaceStone(6, 6, Player.Blue);

        // Act - PlaceStone returns new Board (immutable)
        var clone = original.PlaceStone(7, 7, Player.Red);

        // Assert - can place stones on any empty cell
        clone = clone.PlaceStone(8, 8, Player.Blue);

        clone.GetCell(7, 7).Player.Should().Be(Player.Red);
        clone.GetCell(8, 8).Player.Should().Be(Player.Blue);
    }

    [Fact]
    public void PlaceStone_MultipleTimes_ProducesIndependentBoards()
    {
        // Arrange
        var original = new Board();
        original = original.PlaceStone(5, 5, Player.Red);

        // Act - Each PlaceStone returns a new Board (immutable)
        var clone1 = original.PlaceStone(6, 6, Player.Blue);
        var clone2 = original.PlaceStone(7, 7, Player.Blue);
        var clone3 = clone1.PlaceStone(8, 8, Player.Blue);

        // Assert - all copies are independent
        original.GetCell(6, 6).Player.Should().Be(Player.None);
        original.GetCell(7, 7).Player.Should().Be(Player.None);
        original.GetCell(8, 8).Player.Should().Be(Player.None);

        clone1.GetCell(6, 6).Player.Should().Be(Player.Blue);
        clone1.GetCell(7, 7).Player.Should().Be(Player.None);
        clone1.GetCell(8, 8).Player.Should().Be(Player.None);

        clone2.GetCell(6, 6).Player.Should().Be(Player.None);
        clone2.GetCell(7, 7).Player.Should().Be(Player.Blue);
        clone2.GetCell(8, 8).Player.Should().Be(Player.None);

        clone3.GetCell(6, 6).Player.Should().Be(Player.Blue);
        clone3.GetCell(7, 7).Player.Should().Be(Player.None);
        clone3.GetCell(8, 8).Player.Should().Be(Player.Blue);
    }
}
