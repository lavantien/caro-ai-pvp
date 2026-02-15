using Xunit;
using FluentAssertions;
using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

namespace Caro.Core.Tests.GameLogic.OpeningBook;

/// <summary>
/// Tests to debug the symmetry bug where moves from the book
/// transform to occupied cells.
///
/// Bug report:
/// - Move from book: (11,11), Symmetry=Rotate90, Depth=11
/// - Transforms to (7,11) which is occupied by Blue
/// - Board state: Red=6, Blue=5, Total=11
/// </summary>
public class SymmetryTransformationBugTests
{
    [Fact]
    public void Verify_Rotate90_Transformations_AreCorrect()
    {
        var canonicalizer = new PositionCanonicalizer();

        // Test ApplySymmetry(Rotate90)
        var (tx1, ty1) = canonicalizer.ApplySymmetry(5, 8, SymmetryType.Rotate90);
        // Expected: (y, 31-x) = (8, 26) for 32x32 board
        tx1.Should().Be(8);
        ty1.Should().Be(26);

        // Test ApplyInverseSymmetry(Rotate90)
        var (tx2, ty2) = canonicalizer.ApplyInverseSymmetry(8, 26, SymmetryType.Rotate90);
        // Expected: (31-y, x) = (5, 8) - inverse of above
        tx2.Should().Be(5);
        ty2.Should().Be(8);
    }

    [Fact]
    public void Verify_BugScenario_Transformation_IsCorrect()
    {
        var canonicalizer = new PositionCanonicalizer();

        // For 32x32 board: ApplyInverseSymmetry(Rotate90) on (11,11)
        // Formula: (31-y, x) = (20, 11)
        var (actualX, actualY) = canonicalizer.ApplyInverseSymmetry(11, 11, SymmetryType.Rotate90);
        actualX.Should().Be(20);
        actualY.Should().Be(11);

        // Verify round-trip: If actual was (20, 11), canonical should be (11, 11)
        var (canonicalX, canonicalY) = canonicalizer.ApplySymmetry(20, 11, SymmetryType.Rotate90);
        canonicalX.Should().Be(11);
        canonicalY.Should().Be(11);
    }

    [Fact]
    public void CreateBoard_WithSpecificStones_Verify_CellIsOccupied()
    {
        // Create a board with specific stone pattern for testing
        // Red: (9,9), (8,10), (8,9), (7,10), (7,9) = 5 stones
        // Blue: (9,10), (10,10), (10,9), (11,10), (11,9), (7,11) = 6 stones
        var board = new Board();
        board = board.PlaceStone(9, 9, Player.Red);
        board = board.PlaceStone(9, 10, Player.Blue);
        board = board.PlaceStone(8, 10, Player.Red);
        board = board.PlaceStone(10, 10, Player.Blue);
        board = board.PlaceStone(8, 9, Player.Red);
        board = board.PlaceStone(10, 9, Player.Blue);
        board = board.PlaceStone(7, 10, Player.Red);
        board = board.PlaceStone(11, 10, Player.Blue);
        board = board.PlaceStone(7, 9, Player.Red);
        board = board.PlaceStone(11, 9, Player.Blue);
        board = board.PlaceStone(7, 11, Player.Blue); // This is the occupied cell!

        // Verify (7, 11) is occupied by Blue
        var cell = board.GetCell(7, 11);
        cell.Player.Should().Be(Player.Blue, "Cell (7, 11) should be occupied by Blue");

        // Count stones
        int redCount = 0, blueCount = 0;
        for (int x = 0; x < GameConstants.BoardSize; x++)
        {
            for (int y = 0; y < GameConstants.BoardSize; y++)
            {
                var c = board.GetCell(x, y);
                if (c.Player == Player.Red) redCount++;
                else if (c.Player == Player.Blue) blueCount++;
            }
        }
        redCount.Should().Be(5);
        blueCount.Should().Be(6);
    }

    [Fact]
    public void Canonicalize_BoardWithStones_ReturnsConsistentResult()
    {
        var canonicalizer = new PositionCanonicalizer();

        // Create a board
        var board = new Board();
        board = board.PlaceStone(9, 9, Player.Red);
        board = board.PlaceStone(9, 10, Player.Blue);
        board = board.PlaceStone(8, 10, Player.Red);
        board = board.PlaceStone(10, 10, Player.Blue);
        board = board.PlaceStone(8, 9, Player.Red);
        board = board.PlaceStone(10, 9, Player.Blue);

        // Canonicalize twice
        var canonical1 = canonicalizer.Canonicalize(board);
        var canonical2 = canonicalizer.Canonicalize(board);

        // Should be identical
        canonical1.CanonicalHash.Should().Be(canonical2.CanonicalHash);
        canonical1.SymmetryApplied.Should().Be(canonical2.SymmetryApplied);
        canonical1.IsNearEdge.Should().Be(canonical2.IsNearEdge);
    }

    [Fact]
    public void TransformToActual_WithRotate90_ReturnsCorrectCoordinate()
    {
        var canonicalizer = new PositionCanonicalizer();
        var board = new Board();

        // TransformToActual should call ApplyInverseSymmetry
        var (actualX, actualY) = canonicalizer.TransformToActual(
            (11, 11),
            SymmetryType.Rotate90,
            board
        );

        // Should equal ApplyInverseSymmetry
        var (expectedX, expectedY) = canonicalizer.ApplyInverseSymmetry(11, 11, SymmetryType.Rotate90);

        actualX.Should().Be(expectedX);
        actualY.Should().Be(expectedY);
    }

    [Fact]
    public void AllSymmetries_AreSelfConsistent()
    {
        var canonicalizer = new PositionCanonicalizer();
        var symmetries = new[]
        {
            SymmetryType.Identity,
            SymmetryType.Rotate90,
            SymmetryType.Rotate180,
            SymmetryType.Rotate270,
            SymmetryType.FlipHorizontal,
            SymmetryType.FlipVertical,
            SymmetryType.DiagonalA,
            SymmetryType.DiagonalB
        };

        var testPoints = new[] { (5, 8), (7, 11), (9, 9), (11, 11), (0, 0), (18, 18) };

        foreach (var symmetry in symmetries)
        {
            foreach (var (x, y) in testPoints)
            {
                // Apply symmetry then inverse should return original
                var (tx, ty) = canonicalizer.ApplySymmetry(x, y, symmetry);
                var (rx, ry) = canonicalizer.ApplyInverseSymmetry(tx, ty, symmetry);

                rx.Should().Be(x, $"Round-trip failed for {symmetry} ({x},{y}) -> ({tx},{ty}) -> ({rx},{ry})");
                ry.Should().Be(y, $"Round-trip failed for {symmetry} ({x},{y}) -> ({tx},{ty}) -> ({rx},{ry})");
            }
        }
    }
}
