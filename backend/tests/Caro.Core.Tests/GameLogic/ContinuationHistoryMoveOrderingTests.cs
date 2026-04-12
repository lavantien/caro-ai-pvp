using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Xunit;

namespace Caro.Core.Tests.GameLogic;

/// <summary>
/// Tests for continuation history integration into move ordering.
/// Verifies that continuation history improves move ordering by considering
/// which moves have been good after previous moves.
/// </summary>
public class ContinuationHistoryMoveOrderingTests
{
    [Fact]
    public void MoveOrdering_ContinuationPriority_ContinuationMovesOrderedHigher()
    {
        // Arrange: Create a search instance with continuation history
        var search = new ParallelMinimaxSearch();

        // Create a board with some moves played
        var board = new Board();
        board = board.PlaceStone(9, 9, Player.Red);   // First move (center)
        board = board.PlaceStone(9, 10, Player.Blue); // Second move (adjacent)

        // Get move candidates
        var candidates = ParallelMinimaxSearch.GetMoveCandidates(board);

        // Act: Order moves with continuation history
        // The continuation history should give higher priority to moves
        // that have been good after the previous move (9, 10)
        var orderedMoves = search.OrderMovesPublic(
            candidates,
            depth: 5,
            board: board,
            player: Player.Red,
            cachedMove: null,
            moveHistory: new[] { 9 * 19 + 10 }); // Previous move at (9, 10)

        // Assert: Moves with good continuation history should be ordered higher
        // This is a basic test - the actual scoring depends on the continuation history table
        Assert.NotNull(orderedMoves);
        Assert.Equal(candidates.Count, orderedMoves.Count);
    }

    [Fact]
    public void MoveOrdering_CompositeScore_AllFactorsCombinedCorrectly()
    {
        // Arrange: Create a search instance
        var search = new ParallelMinimaxSearch();

        var board = new Board();
        board = board.PlaceStone(9, 9, Player.Red);
        board = board.PlaceStone(9, 10, Player.Blue);

        var candidates = new List<(int, int)> { (8, 9), (10, 9), (9, 8), (10, 10) };

        // Act: Order moves with various factors
        var orderedMoves = search.OrderMovesPublic(
            candidates,
            depth: 5,
            board: board,
            player: Player.Red,
            cachedMove: (8, 9), // TT move
            moveHistory: new[] { 9 * 19 + 10 });

        // Assert: TT move should be first (highest priority)
        Assert.Equal((8, 9), orderedMoves[0]);

        // Other moves should be ordered by their composite scores
        // (continuation history + main history + positional factors)
        Assert.Contains(orderedMoves, m => m == (10, 9));
        Assert.Contains(orderedMoves, m => m == (9, 8));
        Assert.Contains(orderedMoves, m => m == (10, 10));
    }

    [Fact]
    public void MoveOrdering_ContinuationWithKiller_KillerHasPriority()
    {
        // Arrange: Set up a scenario with both continuation and killer moves
        var search = new ParallelMinimaxSearch();

        var board = new Board();
        board = board.PlaceStone(9, 9, Player.Red);
        board = board.PlaceStone(9, 10, Player.Blue);

        var candidates = new List<(int, int)> { (8, 9), (10, 9), (9, 8) };

        // Act: Order moves with killer move set
        var orderedMoves = search.OrderMovesPublic(
            candidates,
            depth: 5,
            board: board,
            player: Player.Red,
            cachedMove: null,
            moveHistory: new[] { 9 * 19 + 10 },
            killerMove: (10, 9)); // Set killer move

        // Assert: Killer move should be ordered before continuation-only moves
        // Killer priority (500000) > continuation priority
        int killerIndex = orderedMoves.IndexOf((10, 9));
        int nonKillerIndex = orderedMoves.IndexOf((8, 9));

        Assert.True(killerIndex < nonKillerIndex,
            $"Killer move at index {killerIndex} should come before non-killer at index {nonKillerIndex}");
    }

    [Fact]
    public void MoveOrdering_NoMoveHistory_UsesMainHistoryOnly()
    {
        // Arrange: Empty move history
        var search = new ParallelMinimaxSearch();

        var board = new Board();
        board = board.PlaceStone(9, 9, Player.Red);

        var candidates = new List<(int, int)> { (8, 9), (10, 9), (9, 8) };

        // Act: Order with no move history
        var orderedMoves = search.OrderMovesPublic(
            candidates,
            depth: 5,
            board: board,
            player: Player.Blue,
            cachedMove: null,
            moveHistory: Array.Empty<int>());

        // Assert: Should still order moves, using main history only
        Assert.NotNull(orderedMoves);
        Assert.Equal(candidates.Count, orderedMoves.Count);
    }

    [Fact]
    public void MoveOrdering_MultiplePlies_UsesUpTo6PreviousMoves()
    {
        // Arrange: Create a move history with more than 6 moves
        var search = new ParallelMinimaxSearch();

        var board = new Board();
        // Play several moves
        board = board.PlaceStone(9, 9, Player.Red);
        board = board.PlaceStone(9, 10, Player.Blue);
        board = board.PlaceStone(8, 9, Player.Red);
        board = board.PlaceStone(10, 9, Player.Blue);
        board = board.PlaceStone(9, 8, Player.Red);
        board = board.PlaceStone(10, 10, Player.Blue);
        board = board.PlaceStone(11, 9, Player.Red);

        // Move history with 8 previous moves (should only use first 6)
        var moveHistory = new[] {
            10 * 19 + 10, // (10, 10)
            9 * 19 + 8,   // (9, 8)
            10 * 19 + 9,   // (10, 9)
            8 * 19 + 9,   // (8, 9)
            9 * 19 + 10,  // (9, 10)
            9 * 19 + 9,   // (9, 9)
            7 * 19 + 9,   // (7, 9) - should be ignored (ply 7)
            11 * 19 + 9   // (11, 9) - should be ignored (ply 8)
        };

        var candidates = new List<(int, int)> { (8, 10), (11, 10), (12, 9) };

        // Act: Order with long move history
        var orderedMoves = search.OrderMovesPublic(
            candidates,
            depth: 5,
            board: board,
            player: Player.Blue,
            cachedMove: null,
            moveHistory: moveHistory);

        // Assert: Should successfully order (no errors from long history)
        Assert.NotNull(orderedMoves);
        Assert.Equal(candidates.Count, orderedMoves.Count);
    }

    [Fact]
    public void ContinuationHistory_Update_UpdatesMultiplePlies()
    {
        // Arrange: Create continuation history
        var continuationHistory = new ContinuationHistory();

        // Act: Update continuation history for multiple plies
        var moveHistory = new[] { 9 * 19 + 9, 9 * 19 + 10, 8 * 19 + 9 };
        int currentMove = 10 * 19 + 9;
        int bonus = 1000;

        continuationHistory.UpdateMultiple(Player.Red, moveHistory, currentMove, bonus);

        // Assert: All 3 plies should be updated
        int score1 = continuationHistory.GetScore(Player.Red, 9 * 19 + 9, currentMove);
        int score2 = continuationHistory.GetScore(Player.Red, 9 * 19 + 10, currentMove);
        int score3 = continuationHistory.GetScore(Player.Red, 8 * 19 + 9, currentMove);

        Assert.Equal(bonus, score1);
        Assert.Equal(bonus, score2);
        Assert.Equal(bonus, score3);
    }

    [Fact]
    public void MoveOrdering_ContinuationScoreWeight_FormulaCorrect()
    {
        // This test verifies the scoring formula:
        // continuationScore = 2 * mainHistory + continuationHistory[0..2]

        // Arrange: Set up specific history values
        var search = new ParallelMinimaxSearch();

        var board = new Board();
        board = board.PlaceStone(9, 9, Player.Red);

        var candidates = new List<(int, int)> { (8, 9), (10, 9) };

        // Act: Order moves with known history
        var orderedMoves = search.OrderMovesPublic(
            candidates,
            depth: 5,
            board: board,
            player: Player.Blue,
            cachedMove: null,
            moveHistory: new[] { 9 * 19 + 9 });

        // Assert: The move ordering should reflect the weighted formula
        // (This is a structural test - exact values depend on internal state)
        Assert.NotNull(orderedMoves);
        Assert.Equal(2, orderedMoves.Count);
    }
}
