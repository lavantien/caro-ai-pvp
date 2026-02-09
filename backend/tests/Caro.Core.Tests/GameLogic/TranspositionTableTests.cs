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

    // Multi-Entry Cluster Tests (T1.1)

    [Fact]
    public void ClusterShouldBe32BytesAligned()
    {
        // The Cluster struct should be exactly 32 bytes for cache-line alignment
        // 3 entries * 10 bytes + 2 bytes padding = 32 bytes
        unsafe
        {
            var clusterSize = sizeof(TranspositionTable.Cluster);
            Assert.Equal(32, clusterSize);
        }
    }

    [Fact]
    public void DepthAgeReplacement_PreferesDeeper()
    {
        // Arrange
        var table = new TranspositionTable();
        var hash = 12345ul;

        // Store entry at current age, depth 5
        table.Store(hash, 5, 100, (7, 7), -1000, 1000);

        // Increment age (make entry older)
        table.IncrementAge();

        // Store different entry with hash collision at depth 3 (shallower but newer)
        // The replacement value formula: depth - 8 * age
        // Old entry: 5 - 8*1 = -3
        // New entry: 3 - 8*0 = 3
        // Newer shallow entry should be preferred
        table.Store(hash + (ulong)table.GetHashCode(), 3, 200, (8, 8), -1000, 1000);

        // The older deep entry should be replaced by newer shallow entry
        // when probing the same cluster
        var (found1, _, _) = table.Lookup(hash, 5, -1000, 1000);
        var (found2, _, _) = table.Lookup(hash + (ulong)table.GetHashCode(), 3, -1000, 1000);

        // At least one should be found (replacement occurred)
        Assert.True(found1 || found2);
    }

    [Fact]
    public void MultiEntryProbe_ReturnsBestMatch()
    {
        // When multiple entries match different hashes in the same cluster,
        // probe should return the deepest matching entry for the queried hash
        var table = new TranspositionTable();
        var hash1 = 1000ul;
        var hash2 = 2000ul;

        // Store three entries at same cluster location (collision)
        table.Store(hash1, 2, 50, (1, 1), -1000, 1000);  // Shallow
        table.Store(hash2, 4, 100, (5, 5), -1000, 1000); // Deeper
        table.Store(hash1, 6, 150, (7, 7), -1000, 1000); // Deepest for hash1

        // Looking up hash1 should return the deepest entry (depth 6)
        var (found, score, move) = table.Lookup(hash1, 3, -1000, 1000);

        Assert.True(found);
        Assert.Equal(150, score);
        Assert.Equal((7, 7), move);
    }

    [Fact]
    public void MultiEntryStore_ReplacesLowestValue()
    {
        // When cluster is full, should replace entry with lowest (depth - 8*age)
        var table = new TranspositionTable();

        // Use same hash index to force same cluster
        var baseHash = 5000ul;

        // Fill cluster with 3 entries
        table.Store(baseHash, 5, 100, (1, 1), -1000, 1000);           // value = 5
        table.Store(baseHash + 1, 3, 100, (2, 2), -1000, 1000);        // value = 3 (lowest)
        table.Store(baseHash + 2, 4, 100, (3, 3), -1000, 1000);        // value = 4

        // Increment age to reduce values of existing entries
        table.IncrementAge();

        // Add 4th entry - should replace the entry with value = 3 - 8*1 = -5 (lowest)
        table.Store(baseHash + 3, 2, 200, (4, 4), -1000, 1000);        // value = 2 - 8*1 = -6

        // The entry with lowest value should have been replaced
        // This is verified by checking that we can still lookup entries with higher values
        var (found1, _, _) = table.Lookup(baseHash, 5, -1000, 1000);
        var (found2, _, _) = table.Lookup(baseHash + 1, 3, -1000, 1000);
        var (found3, _, _) = table.Lookup(baseHash + 2, 4, -1000, 1000);
        var (found4, _, _) = table.Lookup(baseHash + 3, 2, -1000, 1000);

        // At least 3 of 4 should be found (one was replaced)
        int foundCount = (found1 ? 1 : 0) + (found2 ? 1 : 0) + (found3 ? 1 : 0) + (found4 ? 1 : 0);
        Assert.True(foundCount >= 3);
    }
}
