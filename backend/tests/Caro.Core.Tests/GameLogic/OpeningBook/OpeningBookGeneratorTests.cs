using Xunit;
using FluentAssertions;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

namespace Caro.Core.Tests.GameLogic.OpeningBook;

/// <summary>
/// Tests for OpeningBookGenerator focusing on board cloning edge cases.
/// Specifically tests the "Cell is already occupied" bug that occurred when
/// processing stored moves from the book.
/// </summary>
public class OpeningBookGeneratorTests
{
    [Fact]
    public void PlaceStone_DoesNotAffectOriginal_BoardIsImmutable()
    {
        // This is a focused test for the exact bug pattern:
        // Place a stone, creating a new board.
        // With immutability, the original is never affected.

        // Arrange
        var original = new Board();
        original = original.PlaceStone(9, 9, Player.Red);

        // Act - PlaceStone returns a new Board (immutable)
        var clone = original.PlaceStone(9, 10, Player.Blue);

        // Assert - Verify original is unchanged
        original.GetCell(9, 10).Player.Should().Be(Player.None);
        clone.GetCell(9, 10).Player.Should().Be(Player.Blue);
    }

    [Fact]
    public void PlaceStone_AfterMultiplePlacements_AllowsFurtherPlacements()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(5, 5, Player.Red);
        board = board.PlaceStone(6, 6, Player.Blue);
        board = board.PlaceStone(7, 7, Player.Red);

        // Act - PlaceStone returns new Board (immutable)
        var clone = board.PlaceStone(8, 8, Player.Blue);

        // These should all succeed without "already occupied" errors
        clone = clone.PlaceStone(9, 9, Player.Red);
        clone = clone.PlaceStone(10, 10, Player.Blue);

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
        original = original.PlaceStone(5, 5, Player.Red);
        original = original.PlaceStone(6, 6, Player.Blue);
        original = original.PlaceStone(7, 7, Player.Red);
        original = original.PlaceStone(8, 8, Player.Blue);

        // Act
        var clone = original; // Board is immutable, reference copy is safe

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
    public void PlaceStone_WithSameBase_ProducesIndependentBoards()
    {
        // Arrange
        var board = new Board();
        board = board.PlaceStone(9, 9, Player.Red);

        // Act - Each PlaceStone returns a new Board (immutable)
        var clone1 = board.PlaceStone(9, 10, Player.Blue);
        var clone2 = board.PlaceStone(10, 9, Player.Blue);

        // Assert - Each board is independent
        board.GetCell(9, 10).Player.Should().Be(Player.None);
        board.GetCell(10, 9).Player.Should().Be(Player.None);

        clone1.GetCell(9, 10).Player.Should().Be(Player.Blue);
        clone1.GetCell(10, 9).Player.Should().Be(Player.None);

        clone2.GetCell(9, 10).Player.Should().Be(Player.None);
        clone2.GetCell(10, 9).Player.Should().Be(Player.Blue);
    }

    [Fact]
    public void PlaceStone_SequentialFromBase_ProducesIndependentBoards()
    {
        // Tests the pattern with immutable boards:
        // var clone1 = board.PlaceStone(...);
        // var clone2 = clone1.PlaceStone(...);

        // Arrange
        var board = new Board();
        board = board.PlaceStone(9, 9, Player.Red);

        // Act - Each PlaceStone returns a new Board (immutable)
        var clone1 = board.PlaceStone(9, 10, Player.Blue);
        var clone2 = clone1.PlaceStone(10, 9, Player.Red);

        // Assert - Original and first clone unaffected
        board.GetCell(9, 10).Player.Should().Be(Player.None);
        board.GetCell(10, 9).Player.Should().Be(Player.None);
        clone1.GetCell(10, 9).Player.Should().Be(Player.None);
        clone2.GetCell(9, 10).Player.Should().Be(Player.Blue);
        clone2.GetCell(10, 9).Player.Should().Be(Player.Red);
    }

    [Fact]
    public void PlaceStone_EmptyBoard_AllowsAnyPlacement()
    {
        // Arrange
        var emptyBoard = new Board();

        // Act - Each PlaceStone returns a new Board (immutable)
        var clone = emptyBoard.PlaceStone(0, 0, Player.Red);

        // Should be able to place anywhere
        clone = clone.PlaceStone(18, 18, Player.Blue);
        clone = clone.PlaceStone(9, 9, Player.Red);

        // Assert
        clone.GetCell(0, 0).Player.Should().Be(Player.Red);
        clone.GetCell(18, 18).Player.Should().Be(Player.Blue);
        clone.GetCell(9, 9).Player.Should().Be(Player.Red);
    }

    #region Integration Tests: From Book Move Application

    /// <summary>
    /// Integration test for the "from book" scenario where moves are loaded
    /// from the opening book and applied to a board. This tests the exact
    /// code path that caused the "Cell is already occupied" error.
    /// </summary>
    [Fact]
    public void FromBook_ApplyStoredMoves_NoCellOccupiedErrors()
    {
        // Arrange - Create a board with the starting position
        var board = new Board();
        board = board.PlaceStone(9, 9, Player.Red);  // Center move
        board = board.PlaceStone(9, 10, Player.Blue); // Response

        // Create canonicalizer for coordinate transformation (same as used in OpeningBookGenerator)
        var canonicalizer = new Caro.Core.GameLogic.PositionCanonicalizer();

        // Simulate stored book moves - these would be loaded from the database
        // In the book, moves are stored in canonical coordinates
        var bookMoves = new[]
        {
        new BookMove
        {
            RelativeX = 8,
            RelativeY = 10,
            WinRate = 50,
            DepthAchieved = 10,
            NodesSearched = 1000,
            Score = 0,
            IsForcing = false,
            Priority = 1,
            IsVerified = true
        },
        new BookMove
        {
            RelativeX = 10,
            RelativeY = 10,
            WinRate = 48,
            DepthAchieved = 10,
            NodesSearched = 1000,
            Score = -20,
            IsForcing = false,
            Priority = 2,
            IsVerified = true
        }
    };

        // The stored position would have associated symmetry info
        // For edge positions, this would be Identity (no transformation)
        SymmetryType storedSymmetry = SymmetryType.Identity;

        // Act & Assert - Apply each stored move to create new board
        foreach (var move in bookMoves)
        {
            // Create a new board by placing a stone (immutable pattern)
            var newBoard = board;

            // Transform canonical coordinates back to actual (same logic as line ~301-304)
            (int actualX, int actualY) = canonicalizer.TransformToActual(
                (move.RelativeX, move.RelativeY),
                storedSymmetry,
                board
            );

            // Verify the cell is empty BEFORE placing (defensive check)
            var existingCell = newBoard.GetCell(actualX, actualY);
            existingCell.Player.Should().Be(Player.None,
                $"Cell ({actualX},{actualY}) should be empty before placing stone from book move ({move.RelativeX},{move.RelativeY})");

            // Place the stone - this should not throw "Cell is already occupied"
            // Note: PlaceStone returns a new Board (immutable), so we capture the result
            newBoard = newBoard.PlaceStone(actualX, actualY, Player.Red);

            // Verify the stone was placed
            newBoard.GetCell(actualX, actualY).Player.Should().Be(Player.Red);

            // Verify original board is unchanged
            board.GetCell(actualX, actualY).Player.Should().Be(Player.None);
        }
    }

    /// <summary>
    /// Integration test with symmetry transformation - simulates the center
    /// position scenario where moves are stored in canonical coordinates
    /// after symmetry reduction.
    /// </summary>
    [Fact]
    public void FromBook_ApplyStoredMovesWithSymmetry_TransformsCorrectly()
    {
        // Arrange - Create a board with center position (not near edge)
        var board = new Board();
        board = board.PlaceStone(9, 9, Player.Red);
        board = board.PlaceStone(8, 8, Player.Blue);

        var canonicalizer = new Caro.Core.GameLogic.PositionCanonicalizer();

        // Simulate a position stored with Rotate90 symmetry
        // The book stores moves in canonical space, which may be rotated
        SymmetryType storedSymmetry = SymmetryType.Rotate90;

        // A move stored at (9, 10) in canonical space with Rotate90 symmetry
        // should transform to actual space when applied
        var bookMove = new BookMove
        {
            RelativeX = 9,
            RelativeY = 10,
            WinRate = 50,
            DepthAchieved = 10,
            NodesSearched = 1000,
            Score = 0,
            IsForcing = false,
            Priority = 1,
            IsVerified = true
        };

        // Act - Start with current board and apply the stored move
        var newBoard = board;

        (int actualX, int actualY) = canonicalizer.TransformToActual(
            (bookMove.RelativeX, bookMove.RelativeY),
            storedSymmetry,
            board
        );

        // Assert - The transformed coordinates should be valid
        actualX.Should().BeGreaterThanOrEqualTo(0);
        actualX.Should().BeLessThan(19);
        actualY.Should().BeGreaterThanOrEqualTo(0);
        actualY.Should().BeLessThan(19);

        // Cell should be empty before placing
        var existingCell = newBoard.GetCell(actualX, actualY);
        existingCell.Player.Should().Be(Player.None,
            $"Cell ({actualX},{actualY}) should be empty before placing stone");

        // Place the stone - should not throw
        // Note: PlaceStone returns a new Board (immutable), so we capture the result
        newBoard = newBoard.PlaceStone(actualX, actualY, Player.Red);

        // Verify the stone was placed at the transformed coordinates
        newBoard.GetCell(actualX, actualY).Player.Should().Be(Player.Red);
    }

    /// <summary>
    /// Integration test for multiple sequential moves from the book.
    /// Simulates the scenario of processing multiple stored moves in succession.
    /// </summary>
    [Fact]
    public void FromBook_ApplyMultipleStoredMoves_Sequentially()
    {
        // Arrange - Starting position with 2 moves
        var board = new Board();
        board = board.PlaceStone(9, 9, Player.Red);
        board = board.PlaceStone(9, 10, Player.Blue);

        var canonicalizer = new Caro.Core.GameLogic.PositionCanonicalizer();

        // Simulate processing depth 2 -> depth 3 -> depth 4
        // Each move is applied to a clone of the current position
        var moveSequence = new[]
        {
        (RelX: 8, RelY: 10, Player: Player.Red),
        (RelX: 10, RelY: 9, Player: Player.Blue),
        (RelX: 8, RelY: 8, Player: Player.Red)
    };

        Board currentBoard = board;
        SymmetryType symmetry = SymmetryType.Identity;

        // Act - Apply each move sequentially
        for (int i = 0; i < moveSequence.Length; i++)
        {
            var (relX, relY, player) = moveSequence[i];

            // Start with current board (immutable pattern)
            var newBoard = currentBoard;

            // Transform to actual coordinates
            (int actualX, int actualY) = canonicalizer.TransformToActual(
                (relX, relY),
                symmetry,
                currentBoard
            );

            // Place stone - should not throw
            // Note: PlaceStone returns a new Board (immutable), so we capture the result
            newBoard = newBoard.PlaceStone(actualX, actualY, player);

            // Verify placement
            newBoard.GetCell(actualX, actualY).Player.Should().Be(player);

            // This board becomes the base for next iteration
            currentBoard = newBoard;
        }

        // Assert - Final board should have 2 (initial) + 3 (new) = 5 stones
        int redCount = 0, blueCount = 0;
        for (int x = 0; x < 19; x++)
        {
            for (int y = 0; y < 19; y++)
            {
                var cell = currentBoard.GetCell(x, y);
                if (cell.Player == Player.Red) redCount++;
                else if (cell.Player == Player.Blue) blueCount++;
            }
        }

        redCount.Should().Be(3);  // Initial Red + 2 more Red moves
        blueCount.Should().Be(2); // Initial Blue + 1 more Blue move
    }

    /// <summary>
    /// Integration test that verifies the exact pattern from OpeningBookGenerator
    /// where positions are loaded from the book and child positions are generated.
    /// </summary>
    [Fact]
    public void FromBook_GenerateChildPositions_MatchesOpeningBookGeneratorPattern()
    {
        // This test reproduces the exact pattern from lines ~285-329 in OpeningBookGenerator.cs

        // Arrange - Position data similar to what's loaded from the book
        var board = new Board();
        board = board.PlaceStone(9, 9, Player.Red);
        board = board.PlaceStone(9, 10, Player.Blue);
        board = board.PlaceStone(8, 10, Player.Red);  // Third move

        var canonicalizer = new Caro.Core.GameLogic.PositionCanonicalizer();

        // Simulate PositionToProcess data structure (from book)
        var posData = new
        {
            board = board,
            player = Player.Blue,  // Blue's turn
            depth = 3,             // Ply 3 (after 3 stones)
            symmetry = SymmetryType.Identity,
            moves = new[]
            {
            new BookMove { RelativeX = 10, RelativeY = 10, WinRate = 50, DepthAchieved = 10, NodesSearched = 1000, Score = 0, IsForcing = false, Priority = 1, IsVerified = true },
            new BookMove { RelativeX = 7, RelativeY = 9, WinRate = 48, DepthAchieved = 10, NodesSearched = 1000, Score = -20, IsForcing = false, Priority = 2, IsVerified = true }
        }
        };

        var nextLevelPositions = new List<(Board board, Player player, int depth)>();

        // Act - Process moves
        foreach (var move in posData.moves.Take(2))  // maxChildren = 2 for this depth
        {
            // Start with current board (immutable pattern)
            var newBoard = posData.board;

            // Transform canonical to actual (lines ~301-304)
            (int actualX, int actualY) = canonicalizer.TransformToActual(
                (relX: move.RelativeX, relY: move.RelativeY),
                posData.symmetry,
                posData.board
            );

            // Defensive check (lines ~307-324)
            var existingCell = newBoard.GetCell(actualX, actualY);
            if (existingCell.Player != Player.None)
            {
                // This should never happen in the test
                throw new InvalidOperationException($"Cell ({actualX},{actualY}) already occupied by {existingCell.Player}");
            }

            // Place stone (line ~326)
            newBoard = newBoard.PlaceStone(actualX, actualY, posData.player);
            var nextPlayer = posData.player == Player.Red ? Player.Blue : Player.Red;

            // Add to next level (line ~333)
            nextLevelPositions.Add((newBoard, nextPlayer, posData.depth + 1));
        }

        // Assert - Verify child positions were generated correctly
        nextLevelPositions.Should().HaveCount(2);

        // First child: Blue at (10, 10)
        var child1 = nextLevelPositions[0];
        child1.board.GetCell(10, 10).Player.Should().Be(Player.Blue);
        child1.player.Should().Be(Player.Red);  // Red's turn next
        child1.depth.Should().Be(4);

        // Second child: Blue at (7, 9)
        var child2 = nextLevelPositions[1];
        child2.board.GetCell(7, 9).Player.Should().Be(Player.Blue);
        child2.player.Should().Be(Player.Red);  // Red's turn next
        child2.depth.Should().Be(4);

        // Original board should be unchanged
        board.GetCell(10, 10).Player.Should().Be(Player.None);
        board.GetCell(7, 9).Player.Should().Be(Player.None);
    }

    #endregion
}
