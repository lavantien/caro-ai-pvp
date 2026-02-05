using Xunit;
using FluentAssertions;
using Caro.Core.Entities;

namespace Caro.Core.Tests.GameLogic.OpeningBook;

/// <summary>
/// Tests for OpeningBookGenerator focusing on board cloning edge cases.
/// Specifically tests the "Cell is already occupied" bug that occurred when
/// processing stored moves from the book.
/// </summary>
public class OpeningBookGeneratorTests
{
    [Fact]
    public void BoardClone_ThenPlaceStone_DoesNotAffectOriginal()
    {
        // This is a focused test for the exact bug pattern:
        // Clone a board, then place a stone on the clone.
        // The bug was that CloneBoard didn't properly copy state.

        // Arrange
        var original = new Board();
        original.PlaceStone(9, 9, Player.Red);

        // Act - Clone and place stone on clone
        var clone = original.Clone();
        var action = () => clone.PlaceStone(9, 10, Player.Blue);

        // Assert - Should not throw "Cell is already occupied"
        action.Should().NotThrow();

        // Verify original is unchanged
        original.GetCell(9, 10).Player.Should().Be(Player.None);
        clone.GetCell(9, 10).Player.Should().Be(Player.Blue);
    }

    [Fact]
    public void BoardClone_AfterMultiplePlacements_AllowsFurtherPlacements()
    {
        // Arrange
        var board = new Board();
        board.PlaceStone(5, 5, Player.Red);
        board.PlaceStone(6, 6, Player.Blue);
        board.PlaceStone(7, 7, Player.Red);

        // Act
        var clone = board.Clone();

        // These should all succeed without "already occupied" errors
        clone.PlaceStone(8, 8, Player.Blue);
        clone.PlaceStone(9, 9, Player.Red);
        clone.PlaceStone(10, 10, Player.Blue);

        // Assert
        clone.GetCell(8, 8).Player.Should().Be(Player.Blue);
        clone.GetCell(9, 9).Player.Should().Be(Player.Red);
        clone.GetCell(10, 10).Player.Should().Be(Player.Blue);
    }

    [Fact]
    public void BoardClone_PreservesBitBoards_ExactMatch()
    {
        // Arrange
        var original = new Board();
        original.PlaceStone(5, 5, Player.Red);
        original.PlaceStone(6, 6, Player.Blue);
        original.PlaceStone(7, 7, Player.Red);
        original.PlaceStone(8, 8, Player.Blue);

        // Act
        var clone = original.Clone();

        // Assert - Verify all bits match exactly
        var originalRed = original.GetBitBoard(Player.Red);
        var cloneRed = clone.GetBitBoard(Player.Red);

        var originalBlue = original.GetBitBoard(Player.Blue);
        var cloneBlue = clone.GetBitBoard(Player.Blue);

        // Each placed stone should be set in both original and clone
        originalRed.GetBit(5, 5).Should().BeTrue();
        cloneRed.GetBit(5, 5).Should().BeTrue();

        originalRed.GetBit(7, 7).Should().BeTrue();
        cloneRed.GetBit(7, 7).Should().BeTrue();

        originalBlue.GetBit(6, 6).Should().BeTrue();
        cloneBlue.GetBit(6, 6).Should().BeTrue();

        originalBlue.GetBit(8, 8).Should().BeTrue();
        cloneBlue.GetBit(8, 8).Should().BeTrue();

        // No extra bits should be set in clone that weren't in original
        for (int x = 0; x < 19; x++)
        {
            for (int y = 0; y < 19; y++)
            {
                cloneRed.GetBit(x, y).Should().Be(originalRed.GetBit(x, y), $"Red BitBoard mismatch at ({x},{y})");
                cloneBlue.GetBit(x, y).Should().Be(originalBlue.GetBit(x, y), $"Blue BitBoard mismatch at ({x},{y})");
            }
        }
    }

    [Fact]
    public void BoardClone_WithSameBoardTwice_ProducesIndependentCopies()
    {
        // Arrange
        var board = new Board();
        board.PlaceStone(9, 9, Player.Red);

        // Act
        var clone1 = board.Clone();
        var clone2 = board.Clone();

        // Modify each clone independently
        clone1.PlaceStone(9, 10, Player.Blue);
        clone2.PlaceStone(10, 9, Player.Blue);

        // Assert - Each clone is independent
        board.GetCell(9, 10).Player.Should().Be(Player.None);
        board.GetCell(10, 9).Player.Should().Be(Player.None);

        clone1.GetCell(9, 10).Player.Should().Be(Player.Blue);
        clone1.GetCell(10, 9).Player.Should().Be(Player.None);

        clone2.GetCell(9, 10).Player.Should().Be(Player.None);
        clone2.GetCell(10, 9).Player.Should().Be(Player.Blue);
    }

    [Fact]
    public void BoardClone_FromClonedBoard_ProducesIndependentCopy()
    {
        // Tests the pattern in OpeningBookGenerator:
        // var candidateBoard = board.Clone();
        // var searchBoard = candidateBoard.Clone();

        // Arrange
        var board = new Board();
        board.PlaceStone(9, 9, Player.Red);

        // Act - Clone from clone
        var clone1 = board.Clone();
        var clone2 = clone1.Clone();

        // Place on the second clone
        clone2.PlaceStone(9, 10, Player.Blue);

        // Assert - Original and first clone unaffected
        board.GetCell(9, 10).Player.Should().Be(Player.None);
        clone1.GetCell(9, 10).Player.Should().Be(Player.None);
        clone2.GetCell(9, 10).Player.Should().Be(Player.Blue);
    }

    [Fact]
    public void BoardClone_EmptyBoard_AllowsAnyPlacement()
    {
        // Arrange
        var emptyBoard = new Board();

        // Act
        var clone = emptyBoard.Clone();

        // Should be able to place anywhere
        clone.PlaceStone(0, 0, Player.Red);
        clone.PlaceStone(18, 18, Player.Blue);
        clone.PlaceStone(9, 9, Player.Red);

        // Assert
        clone.GetCell(0, 0).Player.Should().Be(Player.Red);
        clone.GetCell(18, 18).Player.Should().Be(Player.Blue);
        clone.GetCell(9, 9).Player.Should().Be(Player.Red);
    }
}
