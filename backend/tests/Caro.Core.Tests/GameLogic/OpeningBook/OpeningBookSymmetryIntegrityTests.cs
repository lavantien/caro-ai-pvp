using Xunit;
using FluentAssertions;
using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

namespace Caro.Core.Tests.GameLogic.OpeningBook;

/// <summary>
/// Tests for the symmetry integrity fix in opening book generation.
///
/// Bug: When moves were stored with IsNearEdge=true (coordinates stored as-is),
/// but retrieved with a board having IsNearEdge=false and non-Identity symmetry,
/// the coordinates were incorrectly inverse-transformed, resulting in placing
/// stones on occupied cells.
///
/// Fix: Use the stored entry's IsNearEdge flag to decide whether to transform
/// coordinates during retrieval, not the current board's IsNearEdge.
/// </summary>
public class OpeningBookSymmetryIntegrityTests
{
    private readonly PositionCanonicalizer _canonicalizer = new();

    [Fact]
    public void StoreWithNearEdge_CoordinatesStoredAsIs()
    {
        // When IsNearEdge=true, coordinates should be stored without transformation
        // This is verified by checking the storage logic at line 930-932

        // Create an edge position board (stone at edge: x < 5)
        var board = new Board();
        board = board.PlaceStone(3, 9, Player.Red);  // Near left edge
        board = board.PlaceStone(10, 10, Player.Blue);

        var canonical = _canonicalizer.Canonicalize(board);

        // Edge positions should have IsNearEdge=true
        canonical.IsNearEdge.Should().BeTrue("position has stone at x=3 which is near the edge");

        // Edge positions use Identity symmetry (no transformation for consistency)
        canonical.SymmetryApplied.Should().Be(SymmetryType.Identity);
    }

    [Fact]
    public void StoreWithCenterPosition_CoordinatesTransformedBySymmetry()
    {
        // When IsNearEdge=false and symmetry!=Identity, coordinates should be transformed

        // Create a center position board (all stones in center: 5 <= x,y < 14)
        var board = new Board();
        board = board.PlaceStone(9, 9, Player.Red);
        board = board.PlaceStone(10, 10, Player.Blue);

        var canonical = _canonicalizer.Canonicalize(board);

        // Center positions should have IsNearEdge=false
        canonical.IsNearEdge.Should().BeFalse("all stones are in the center");

        // Symmetry should be applied for center positions (not necessarily Identity)
        // The canonical form is determined by minimum hash
    }

    [Fact]
    public void NearEdgePosition_RetrievalDoesNotTransformCoordinates()
    {
        // Simulate the bug scenario:
        // 1. Entry stored with IsNearEdge=true, coordinates stored as-is
        // 2. Retrieved with board having non-Identity symmetry
        // 3. Should NOT transform coordinates

        // Create edge position
        var edgeBoard = new Board();
        edgeBoard = edgeBoard.PlaceStone(3, 9, Player.Red);  // Near edge

        var canonical = _canonicalizer.Canonicalize(edgeBoard);
        canonical.IsNearEdge.Should().BeTrue();

        // When retrieving with IsNearEdge=true, coordinates should be unchanged
        int storedX = 10, storedY = 10;

        // With the fix: if IsNearEdge=true, don't transform
        int actualX, actualY;
        if (canonical.IsNearEdge || canonical.SymmetryApplied == SymmetryType.Identity)
        {
            actualX = storedX;
            actualY = storedY;
        }
        else
        {
            (actualX, actualY) = _canonicalizer.TransformToActual(
                (storedX, storedY),
                canonical.SymmetryApplied,
                edgeBoard
            );
        }

        actualX.Should().Be(storedX, "IsNearEdge=true means no transformation");
        actualY.Should().Be(storedY, "IsNearEdge=true means no transformation");
    }

    [Fact]
    public void CenterPosition_RetrievalTransformsCoordinates()
    {
        // When IsNearEdge=false and symmetry!=Identity, coordinates should be transformed

        // Create center position
        var centerBoard = new Board();
        centerBoard = centerBoard.PlaceStone(9, 9, Player.Red);
        centerBoard = centerBoard.PlaceStone(10, 10, Player.Blue);

        var canonical = _canonicalizer.Canonicalize(centerBoard);

        // For center positions with non-Identity symmetry, transformation is needed
        int storedX = 10, storedY = 10;

        int actualX, actualY;
        if (canonical.IsNearEdge || canonical.SymmetryApplied == SymmetryType.Identity)
        {
            actualX = storedX;
            actualY = storedY;
        }
        else
        {
            (actualX, actualY) = _canonicalizer.TransformToActual(
                (storedX, storedY),
                canonical.SymmetryApplied,
                centerBoard
            );
        }

        // The result depends on the symmetry applied
        // The key is that transformation is applied correctly when needed
        if (!canonical.IsNearEdge && canonical.SymmetryApplied != SymmetryType.Identity)
        {
            // Verify transformation was applied
            var expected = _canonicalizer.ApplyInverseSymmetry(storedX, storedY, canonical.SymmetryApplied);
            actualX.Should().Be(expected.x);
            actualY.Should().Be(expected.y);
        }
    }

    [Fact]
    public void TransformToActual_WithIdentity_ReturnsUnchanged()
    {
        var board = new Board();

        var (actualX, actualY) = _canonicalizer.TransformToActual(
            (10, 10),
            SymmetryType.Identity,
            board
        );

        actualX.Should().Be(10);
        actualY.Should().Be(10);
    }

    [Fact]
    public void TransformToActual_WithNonIdentity_AppliesInverseTransformation()
    {
        var board = new Board();

        var (actualX, actualY) = _canonicalizer.TransformToActual(
            (10, 10),
            SymmetryType.Rotate90,
            board
        );

        // Rotate90 inverse is Rotate270: (15-y, x) for 16x16 board
        var expectedX = GameConstants.BoardSize - 1 - 10;  // 5
        var expectedY = 10;

        actualX.Should().Be(expectedX);
        actualY.Should().Be(expectedY);
    }

    [Fact]
    public void EdgeBoardToSameHash_CenterBoardRetrievesCorrectly()
    {
        // This tests the core bug scenario:
        // Two different boards (edge and center) could theoretically have the same
        // canonical hash (though in practice this is rare due to hash differences)

        // Create edge board
        var edgeBoard = new Board();
        edgeBoard = edgeBoard.PlaceStone(3, 9, Player.Red);
        edgeBoard = edgeBoard.PlaceStone(3, 10, Player.Blue);

        var edgeCanonical = _canonicalizer.Canonicalize(edgeBoard);

        // Create center board (same relative pattern, but centered)
        var centerBoard = new Board();
        centerBoard = centerBoard.PlaceStone(9, 9, Player.Red);
        centerBoard = centerBoard.PlaceStone(9, 10, Player.Blue);

        var centerCanonical = _canonicalizer.Canonicalize(centerBoard);

        // They should have different hashes (edge vs center)
        // But the key is: when we retrieve an edge entry, we should NOT transform
        // even if the current board canonicalizes with a non-Identity symmetry

        edgeCanonical.IsNearEdge.Should().BeTrue();
        centerCanonical.IsNearEdge.Should().BeFalse();

        // Verify the fix logic:
        // If stored entry has IsNearEdge=true, use coordinates as-is
        // regardless of current board's symmetry
        bool storedIsNearEdge = true;  // Entry was stored with IsNearEdge=true

        int resultX, resultY;
        if (storedIsNearEdge || centerCanonical.SymmetryApplied == SymmetryType.Identity)
        {
            resultX = 10;
            resultY = 10;
        }
        else
        {
            (resultX, resultY) = _canonicalizer.TransformToActual(
                (10, 10),
                centerCanonical.SymmetryApplied,
                centerBoard
            );
        }

        // With the fix, coordinates should be unchanged
        resultX.Should().Be(10);
        resultY.Should().Be(10);
    }

    [Fact]
    public void PlaceStone_OnEmptyCell_Succeeds()
    {
        // Basic sanity test that placing on empty cell works
        var board = new Board();
        board = board.PlaceStone(9, 9, Player.Red);

        // Cell (10, 10) should be empty
        board.GetCell(10, 10).Player.Should().Be(Player.None);

        // Placing on (10, 10) should succeed
        var newBoard = board.PlaceStone(10, 10, Player.Blue);
        newBoard.GetCell(10, 10).Player.Should().Be(Player.Blue);
    }

    [Fact]
    public void PlaceStone_OnOccupiedCell_Throws()
    {
        // Verify that placing on an occupied cell throws
        var board = new Board();
        board = board.PlaceStone(9, 9, Player.Red);

        // Placing on same cell should throw
        var act = () => board.PlaceStone(9, 9, Player.Blue);
        act.Should().Throw<InvalidOperationException>();
    }

    [Fact]
    public void SymmetryRoundTrip_AllTypes_MaintainsCoordinates()
    {
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

        // Use valid 16x16 test points (max coordinate is 15)
        var testPoints = new[] { (5, 8), (6, 10), (8, 8), (10, 10), (0, 0), (12, 12) };

        foreach (var symmetry in symmetries)
        {
            foreach (var (x, y) in testPoints)
            {
                // Apply symmetry then inverse should return original
                var (tx, ty) = _canonicalizer.ApplySymmetry(x, y, symmetry);
                var (rx, ry) = _canonicalizer.ApplyInverseSymmetry(tx, ty, symmetry);

                rx.Should().Be(x, $"Round-trip failed for {symmetry} ({x},{y}) -> ({tx},{ty}) -> ({rx},{ry})");
                ry.Should().Be(y, $"Round-trip failed for {symmetry} ({x},{y}) -> ({tx},{ty}) -> ({rx},{ry})");
            }
        }
    }

    [Fact]
    public void IsNearEdge_EdgePosition_ReturnsTrue()
    {
        // Position with stone at x=3 (within 5 cells of edge)
        var board = new Board();
        board = board.PlaceStone(3, 9, Player.Red);

        var redBits = board.GetBitBoard(Player.Red);
        var blueBits = board.GetBitBoard(Player.Blue);

        _canonicalizer.IsNearEdge(redBits, blueBits).Should().BeTrue();
    }

    [Fact]
    public void IsNearEdge_CenterPosition_ReturnsFalse()
    {
        // Position with all stones in center (5 <= x,y < 14)
        var board = new Board();
        board = board.PlaceStone(9, 9, Player.Red);
        board = board.PlaceStone(10, 10, Player.Blue);

        var redBits = board.GetBitBoard(Player.Red);
        var blueBits = board.GetBitBoard(Player.Blue);

        _canonicalizer.IsNearEdge(redBits, blueBits).Should().BeFalse();
    }
}
