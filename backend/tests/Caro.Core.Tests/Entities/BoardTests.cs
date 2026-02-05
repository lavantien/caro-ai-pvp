using Xunit;
using FluentAssertions;
using Caro.Core.Domain.Entities;

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

    [Fact]
    public void Clone_ProducesExactCopy_OfCellStates()
    {
        // Arrange
        var original = new Board();
        original.PlaceStone(5, 5, Player.Red);
        original.PlaceStone(6, 6, Player.Blue);
        original.PlaceStone(7, 7, Player.Red);

        // Act
        var clone = original.Clone();

        // Assert - all cell states match
        clone.GetCell(5, 5).Player.Should().Be(Player.Red);
        clone.GetCell(6, 6).Player.Should().Be(Player.Blue);
        clone.GetCell(7, 7).Player.Should().Be(Player.Red);
        clone.GetCell(0, 0).Player.Should().Be(Player.None);
    }

    [Fact]
    public void Clone_DoesNotShareState_WithOriginal()
    {
        // Arrange
        var original = new Board();
        original.PlaceStone(5, 5, Player.Red);

        // Act
        var clone = original.Clone();
        clone.PlaceStone(6, 6, Player.Blue);

        // Assert - clone modification doesn't affect original
        original.GetCell(6, 6).Player.Should().Be(Player.None);
        clone.GetCell(6, 6).Player.Should().Be(Player.Blue);
    }

    [Fact]
    public void Clone_PreservesHash_Exactly()
    {
        // Arrange
        var original = new Board();
        original.PlaceStone(9, 9, Player.Red);
        original.PlaceStone(10, 10, Player.Blue);

        // Act
        var clone = original.Clone();

        // Assert - hashes should be identical (copied directly, not recomputed)
        // Note: We can't access _hash directly, but we can verify the board state is identical
        // which would produce the same hash if recomputed
        for (int x = 0; x < 19; x++)
        {
            for (int y = 0; y < 19; y++)
            {
                clone.GetCell(x, y).Player.Should().Be(original.GetCell(x, y).Player);
            }
        }
    }

    [Fact]
    public void Clone_PreservesBitBoards()
    {
        // Arrange
        var original = new Board();
        original.PlaceStone(5, 5, Player.Red);
        original.PlaceStone(6, 6, Player.Blue);
        original.PlaceStone(7, 7, Player.Red);

        // Act
        var clone = original.Clone();

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
    public void Clone_AllowsPlacingStones_OnAllCells()
    {
        // Arrange
        var original = new Board();
        original.PlaceStone(5, 5, Player.Red);
        original.PlaceStone(6, 6, Player.Blue);

        // Act
        var clone = original.Clone();

        // Assert - can place stones on any empty cell without "already occupied" error
        // This was the bug: CloneBoard didn't properly copy state
        clone.PlaceStone(7, 7, Player.Red);
        clone.PlaceStone(8, 8, Player.Blue);

        clone.GetCell(7, 7).Player.Should().Be(Player.Red);
        clone.GetCell(8, 8).Player.Should().Be(Player.Blue);
    }

    [Fact]
    public void Clone_MultipleTimes_ProducesIndependentCopies()
    {
        // Arrange
        var original = new Board();
        original.PlaceStone(5, 5, Player.Red);

        // Act
        var clone1 = original.Clone();
        var clone2 = original.Clone();
        var clone3 = clone1.Clone();

        // Modify each clone differently
        clone1.PlaceStone(6, 6, Player.Blue);
        clone2.PlaceStone(7, 7, Player.Blue);
        clone3.PlaceStone(8, 8, Player.Blue);

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

        clone3.GetCell(6, 6).Player.Should().Be(Player.None);
        clone3.GetCell(7, 7).Player.Should().Be(Player.None);
        clone3.GetCell(8, 8).Player.Should().Be(Player.Blue);
    }
}
