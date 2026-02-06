using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Xunit;

namespace Caro.Core.Tests.GameLogic;

public class TranspositionTableTests
{
    [Fact]
    public void CalculateHash_EmptyBoard_ReturnsZero()
    {
        // Arrange
        var table = new TranspositionTable();
        var board = new Board();

        // Act
        var hash = table.CalculateHash(board);

        // Assert - empty board XORs no values = 0
        Assert.Equal(0ul, hash);
    }

    [Fact]
    public void CalculateHash_SameBoard_ReturnsSameHash()
    {
        // Arrange
        var table = new TranspositionTable();
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);

        // Act
        var hash1 = table.CalculateHash(board);
        var hash2 = table.CalculateHash(board);

        // Assert
        Assert.Equal(hash1, hash2);
    }

    [Fact]
    public void CalculateHash_DifferentBoards_ReturnsDifferentHashes()
    {
        // Arrange
        var table = new TranspositionTable();
        var board1 = new Board();
        var board2 = new Board();

        board1 = board1.PlaceStone(7, 7, Player.Red);
        board2 = board2.PlaceStone(7, 8, Player.Red); // Different position

        // Act
        var hash1 = table.CalculateHash(board1);
        var hash2 = table.CalculateHash(board2);

        // Assert
        Assert.NotEqual(hash1, hash2);
    }

    [Fact]
    public void CalculateHash_MoveSequence_SameHashAfterUndo()
    {
        // Arrange
        var table = new TranspositionTable();
        var board = new Board();

        board = board.PlaceStone(7, 7, Player.Red);
        var hash1 = table.CalculateHash(board);

        // Make move
        var boardAfterMove = board.PlaceStone(7, 8, Player.Blue);
        var hash2 = table.CalculateHash(boardAfterMove);

        // Undo move - original board state is still in 'board'
        var hash3 = table.CalculateHash(board);

        // Assert
        Assert.NotEqual(hash1, hash2); // Different after move
        Assert.Equal(hash1, hash3);    // Same after undo
    }

    [Fact]
    public void StoreAndLookup_ExactScore_ReturnsCachedValue()
    {
        // Arrange
        var table = new TranspositionTable();
        var hash = 12345ul;
        int depth = 3;
        int score = 100;
        var bestMove = (7, 7);
        int alpha = -1000;
        int beta = 1000;

        // Act
        table.Store(hash, depth, score, bestMove, alpha, beta);
        var (found, cachedScore, cachedMove) = table.Lookup(hash, depth, alpha, beta);

        // Assert
        Assert.True(found);
        Assert.Equal(score, cachedScore);
        Assert.Equal(bestMove, cachedMove);
    }

    [Fact]
    public void StoreAndLookup_LowerBound_ReturnsScoreOnlyWhenBetaCutoff()
    {
        // Arrange
        var table = new TranspositionTable();
        var hash = 12345ul;
        int depth = 3;
        int score = 500; // High score (beta cutoff)
        var bestMove = (7, 7);
        int alpha = -1000;
        int beta = 400; // score > beta = LowerBound

        // Act
        table.Store(hash, depth, score, bestMove, alpha, beta);

        // Should return cached value when score >= beta
        var (found1, _, _) = table.Lookup(hash, depth, alpha, 400);
        Assert.True(found1);

        // Should NOT return cached value when score < beta
        var (found2, _, _) = table.Lookup(hash, depth, alpha, 600);
        Assert.False(found2);
    }

    [Fact]
    public void StoreAndLookup_UpperBound_ReturnsScoreOnlyWhenAlphaCutoff()
    {
        // Arrange
        var table = new TranspositionTable();
        var hash = 12345ul;
        int depth = 3;
        int score = -500; // Low score (alpha cutoff)
        var bestMove = (7, 7);
        int alpha = -400; // score <= alpha = UpperBound
        int beta = 1000;

        // Act
        table.Store(hash, depth, score, bestMove, alpha, beta);

        // Should return cached value when score <= alpha
        var (found1, _, _) = table.Lookup(hash, depth, -400, beta);
        Assert.True(found1);

        // Should NOT return cached value when score > alpha
        var (found2, _, _) = table.Lookup(hash, depth, -600, beta);
        Assert.False(found2);
    }

    [Fact]
    public void Lookup_DifferentHash_ReturnsNotFound()
    {
        // Arrange
        var table = new TranspositionTable();
        table.Store(12345ul, 3, 100, (7, 7), -1000, 1000);

        // Act
        var (found, _, _) = table.Lookup(54321ul, 3, -1000, 1000);

        // Assert
        Assert.False(found);
    }

    [Fact]
    public void Lookup_ShallowDepthStored_DeepLookupNotUsesCache()
    {
        // Arrange
        var table = new TranspositionTable();
        var hash = 12345ul;

        // Store at depth 2
        table.Store(hash, 2, 100, (7, 7), -1000, 1000);

        // Lookup at depth 3 (deeper than stored)
        var (found, _, _) = table.Lookup(hash, 3, -1000, 1000);

        // Assert - should not use shallow cache for deep search
        Assert.False(found);
    }

    [Fact]
    public void Lookup_DeepDepthStored_ShallowLookupUsesCache()
    {
        // Arrange
        var table = new TranspositionTable();
        var hash = 12345ul;

        // Store at depth 5
        table.Store(hash, 5, 100, (7, 7), -1000, 1000);

        // Lookup at depth 3 (shallower than stored)
        var (found, score, move) = table.Lookup(hash, 3, -1000, 1000);

        // Assert - can use deep cache for shallow search
        Assert.True(found);
        Assert.Equal(100, score);
    }

    [Fact]
    public void Store_OverwritesOldEntry_WhenSameHashDeeperDepth()
    {
        // Arrange
        var table = new TranspositionTable();
        var hash = 12345ul;

        // Store at depth 2
        table.Store(hash, 2, 50, (5, 5), -1000, 1000);

        // Store at depth 4 (should overwrite)
        table.Store(hash, 4, 100, (7, 7), -1000, 1000);

        // Lookup should return deeper entry
        var (found, score, move) = table.Lookup(hash, 3, -1000, 1000);

        Assert.True(found);
        Assert.Equal(100, score); // Deeper entry's score
        Assert.Equal((7, 7), move); // Deeper entry's move
    }

    [Fact]
    public void Clear_EmptiesTable()
    {
        // Arrange
        var table = new TranspositionTable();
        table.Store(12345ul, 3, 100, (7, 7), -1000, 1000);

        // Act
        table.Clear();

        // Assert
        var (found, _, _) = table.Lookup(12345ul, 3, -1000, 1000);
        Assert.False(found);

        var (used, usage) = table.GetStats();
        Assert.Equal(0, used);
        Assert.Equal(0.0, usage);
    }

    [Fact]
    public void GetStats_TracksUsage()
    {
        // Arrange
        var table = new TranspositionTable();
        table.IncrementAge();

        // Act
        table.Store(12345ul, 3, 100, (7, 7), -1000, 1000);
        table.Store(23456ul, 3, 200, (8, 8), -1000, 1000);

        // Assert
        var (used, usage) = table.GetStats();
        Assert.True(used >= 2); // At least 2 entries used
        Assert.True(usage > 0); // Some percentage used
    }

    [Fact]
    public void IncrementAge_AffectsReplacementStrategy()
    {
        // Arrange
        var table = new TranspositionTable();
        var hash = 12345ul;

        table.IncrementAge();
        table.Store(hash, 3, 100, (7, 7), -1000, 1000);

        table.IncrementAge();

        // Store different entry with hash collision - should replace old age
        table.Store(hash + (ulong)table.GetHashCode(), 3, 200, (8, 8), -1000, 1000);

        // Old entry should be replaced (different age)
        var stats = table.GetStats();
        Assert.True(stats.used <= 1); // Only new age entries counted
    }

    [Fact]
    public void HashCollisionDetection_DifferentPositionsDifferentHashes()
    {
        // Arrange
        var table = new TranspositionTable();
        var board1 = new Board();
        var board2 = new Board();

        // Place same pieces in different positions
        board1 = board1.PlaceStone(0, 0, Player.Red);
        board1 = board1.PlaceStone(1, 1, Player.Blue);

        board2 = board2.PlaceStone(14, 14, Player.Red);
        board2 = board2.PlaceStone(13, 13, Player.Blue);

        // Act
        var hash1 = table.CalculateHash(board1);
        var hash2 = table.CalculateHash(board2);

        // Assert - hashes should be different (collision probability extremely low)
        Assert.NotEqual(hash1, hash2);
    }

    [Fact]
    public void Lookup_NonExactScore_ReturnsBestMoveEvenIfScoreNotUsable()
    {
        // Arrange
        var table = new TranspositionTable();
        var hash = 12345ul;
        int score = 500; // LowerBound
        int alpha = -1000;
        int beta = 400; // score > beta
        var bestMove = (7, 7);

        table.Store(hash, 3, score, bestMove, alpha, beta);

        // Lookup with different beta (score not usable)
        var (found, _, cachedMove) = table.Lookup(hash, 3, alpha, 600);

        // Assert - score not found, but best move should still be returned
        Assert.False(found);
        Assert.Equal(bestMove, cachedMove); // Best move available for move ordering
    }
}
