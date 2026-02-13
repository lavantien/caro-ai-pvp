using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Infrastructure.Persistence;
using FluentAssertions;
using Microsoft.Extensions.Logging;
using Xunit;

namespace Caro.Core.Infrastructure.Tests.Persistence;

public sealed class SqliteOpeningBookStoreTests : IDisposable
{
    private readonly string _dbPath;
    private readonly SqliteOpeningBookStore _store;
    private static readonly string _testDir = Path.Combine(Path.GetTempPath(), $"caro_test_{Guid.NewGuid()}");

    static SqliteOpeningBookStoreTests()
    {
        // Create test directory once for all tests
        if (!Directory.Exists(_testDir))
        {
            Directory.CreateDirectory(_testDir);
        }
    }

    public SqliteOpeningBookStoreTests()
    {
        _dbPath = Path.Combine(_testDir, $"test_book_{Guid.NewGuid()}.db");
        var logger = Microsoft.Extensions.Logging.Abstractions.NullLogger<SqliteOpeningBookStore>.Instance;
        _store = new SqliteOpeningBookStore(_dbPath, logger, readOnly: false);
        _store.Initialize();
    }

    public void Dispose()
    {
        _store.Dispose();
        // Note: We don't delete individual test files as they may still be locked by SQLite
        // The test directory can be cleaned up manually if needed
    }

    [Fact]
    public void Initialize_CreatesDatabaseTables()
    {
        // Arrange & Act - Initialize is called in constructor
        var stats = _store.GetStatistics();

        // Assert
        stats.Should().NotBeNull();
        stats.TotalEntries.Should().Be(0);
    }

    [Fact]
    public void StoreEntry_AndRetrieve_ReturnsCorrectData()
    {
        // Arrange
        var entry = CreateTestEntry(canonicalHash: 12345UL);

        // Act
        _store.StoreEntry(entry);
        var retrieved = _store.GetEntry(12345UL, Player.Red);

        // Assert
        retrieved.Should().NotBeNull();
        retrieved!.CanonicalHash.Should().Be(12345UL);
        retrieved.Depth.Should().Be(entry.Depth);
        retrieved.Player.Should().Be(Player.Red);
        retrieved.Moves.Length.Should().Be(entry.Moves.Length);
    }

    [Fact]
    public void StoreEntriesBatch_TransactionIsCommittedCorrectly()
    {
        // Arrange
        var entries = new List<OpeningBookEntry>
        {
            CreateTestEntry(canonicalHash: 100UL, depth: 1),
            CreateTestEntry(canonicalHash: 200UL, depth: 2),
            CreateTestEntry(canonicalHash: 300UL, depth: 3)
        };

        // Act - This test specifically verifies that transactions work correctly
        // The bug was that commands created within a transaction didn't have
        // their Transaction property set, causing SQLite to throw
        _store.StoreEntriesBatch(entries);

        // Assert
        _store.ContainsEntry(100UL).Should().BeTrue();
        _store.ContainsEntry(200UL).Should().BeTrue();
        _store.ContainsEntry(300UL).Should().BeTrue();
        _store.GetStatistics().TotalEntries.Should().Be(3);
    }

    [Fact]
    public void StoreEntriesBatch_LargeBatch_DoesNotThrowTransactionError()
    {
        // Arrange
        var entries = new List<OpeningBookEntry>();
        for (ulong i = 0; i < 100; i++)
        {
            entries.Add(CreateTestEntry(canonicalHash: i * 1000UL, depth: (int)i % 10));
        }

        // Act - This stress test ensures the transaction handling is robust
        var act = () => _store.StoreEntriesBatch(entries);

        // Assert - Should not throw SQLite transaction error
        act.Should().NotThrow();
        _store.GetStatistics().TotalEntries.Should().Be(100);
    }

    [Fact]
    public void ContainsEntry_ExistingEntry_ReturnsTrue()
    {
        // Arrange
        var entry = CreateTestEntry(canonicalHash: 5000UL);
        _store.StoreEntry(entry);

        // Act
        var exists = _store.ContainsEntry(5000UL);

        // Assert
        exists.Should().BeTrue();
    }

    [Fact]
    public void ContainsEntry_NonExistentEntry_ReturnsFalse()
    {
        // Act
        var exists = _store.ContainsEntry(99999UL);

        // Assert
        exists.Should().BeFalse();
    }

    [Fact]
    public void GetEntry_NonExistentEntry_ReturnsNull()
    {
        // Act
        var entry = _store.GetEntry(77777UL, Player.Red);

        // Assert
        entry.Should().BeNull();
    }

    [Fact]
    public void GetStatistics_ReturnsCorrectCounts()
    {
        // Arrange
        _store.StoreEntry(CreateTestEntry(canonicalHash: 1UL, depth: 0, movesCount: 5));
        _store.StoreEntry(CreateTestEntry(canonicalHash: 2UL, depth: 1, movesCount: 3));
        _store.StoreEntry(CreateTestEntry(canonicalHash: 3UL, depth: 1, movesCount: 7));

        // Act
        var stats = _store.GetStatistics();

        // Assert
        stats.TotalEntries.Should().Be(3);
        stats.MaxDepth.Should().Be(1);
        stats.TotalMoves.Should().Be(15);
        stats.CoverageByDepth[0].Should().Be(1);
        stats.CoverageByDepth[1].Should().Be(2);
    }

    [Fact]
    public void Clear_RemovesAllEntries()
    {
        // Arrange
        _store.StoreEntry(CreateTestEntry(canonicalHash: 1UL));
        _store.StoreEntry(CreateTestEntry(canonicalHash: 2UL));
        _store.StoreEntry(CreateTestEntry(canonicalHash: 3UL));

        // Act
        _store.Clear();
        var stats = _store.GetStatistics();

        // Assert
        stats.TotalEntries.Should().Be(0);
        _store.ContainsEntry(1UL).Should().BeFalse();
    }

    [Fact]
    public void StoreEntry_WithExistingHash_UpdatesEntry()
    {
        // Arrange
        var entry1 = CreateTestEntry(canonicalHash: 8000UL, movesCount: 3);
        _store.StoreEntry(entry1);

        // Act - Store same hash with different data
        var entry2 = CreateTestEntry(canonicalHash: 8000UL, movesCount: 7);
        _store.StoreEntry(entry2);

        // Assert
        var retrieved = _store.GetEntry(8000UL, Player.Red);
        retrieved.Should().NotBeNull();
        retrieved!.Moves.Length.Should().Be(7); // Should have updated
        _store.GetStatistics().TotalEntries.Should().Be(1); // Still only 1 entry
    }

    [Fact]
    public void StoreEntriesBatch_EmptyList_DoesNotThrow()
    {
        // Act
        var act = () => _store.StoreEntriesBatch(Array.Empty<OpeningBookEntry>());

        // Assert
        act.Should().NotThrow();
    }

    [Fact]
    public void SetMetadata_AndGetMetadata_PreservesValues()
    {
        // Act & Assert
        _store.SetMetadata("Version", "2.0");
        _store.SetMetadata("GeneratedAt", DateTime.UtcNow.ToString("o"));

        _store.GetMetadata("Version").Should().Be("2.0");
        _store.GetMetadata("GeneratedAt").Should().NotBeNullOrEmpty();
        _store.GetMetadata("NonExistent").Should().BeNull();
    }

    [Fact]
    public void StoreEntriesBatch_SingleEntry_DoesNotThrowArgumentOutOfRangeException()
    {
        // Arrange - This test specifically guards against the bug where parameter reuse
        // caused SqliteConnection.RemoveCommand to throw ArgumentOutOfRangeException
        // when disposing commands with reused parameter objects
        var singleEntry = CreateTestEntry(canonicalHash: 99999UL, depth: 5, movesCount: 3);

        // Act
        var act = () => _store.StoreEntriesBatch(new[] { singleEntry });

        // Assert - Should not throw ArgumentOutOfRangeException during command disposal
        act.Should().NotThrow();
        _store.ContainsEntry(99999UL).Should().BeTrue();
    }

    [Fact]
    public void StoreEntriesBatch_CommitSucceeds_DoesNotThrowOnRollbackInFinally()
    {
        // Arrange - This test guards against the bug where Commit() succeeds but
        // the finally block tries to rollback an already-committed transaction
        var entry = CreateTestEntry(canonicalHash: 77777UL, depth: 3, movesCount: 4);

        // Act - Store a single entry (commit succeeds, transaction should be null before finally)
        var act = () => _store.StoreEntriesBatch(new[] { entry });

        // Assert - Should not throw "cannot rollback - no transaction is active"
        act.Should().NotThrow();
        _store.ContainsEntry(77777UL).Should().BeTrue();
    }

    [Fact]
    public void StoreEntriesBatch_FollowedByAnotherBatch_DoesNotThrow()
    {
        // Arrange - This tests that command disposal doesn't leave the connection
        // in a corrupted state that affects subsequent batch operations
        var batch1 = new List<OpeningBookEntry>
        {
            CreateTestEntry(canonicalHash: 1000UL, depth: 1),
            CreateTestEntry(canonicalHash: 1001UL, depth: 1)
        };
        var batch2 = new List<OpeningBookEntry>
        {
            CreateTestEntry(canonicalHash: 2000UL, depth: 2),
            CreateTestEntry(canonicalHash: 2001UL, depth: 2)
        };

        // Act
        _store.StoreEntriesBatch(batch1);
        var act = () => _store.StoreEntriesBatch(batch2);

        // Assert
        act.Should().NotThrow();
        _store.GetStatistics().TotalEntries.Should().Be(4);
    }

    [Fact]
    public void StoreEntriesBatch_MultipleBatchesSequentially_AllSucceed()
    {
        // Arrange - Stress test for sequential batch operations
        // This ensures proper resource cleanup between batches
        var allBatches = new List<List<OpeningBookEntry>>();
        for (int batch = 0; batch < 10; batch++)
        {
            var batchEntries = new List<OpeningBookEntry>();
            for (int i = 0; i < 5; i++)
            {
                batchEntries.Add(CreateTestEntry(
                    canonicalHash: (ulong)(batch * 100 + i),
                    depth: batch,
                    movesCount: (i % 5) + 1
                ));
            }
            allBatches.Add(batchEntries);
        }

        // Act
        foreach (var batch in allBatches)
        {
            _store.StoreEntriesBatch(batch);
        }

        // Assert
        _store.GetStatistics().TotalEntries.Should().Be(50); // 10 batches × 5 entries
    }

    private static OpeningBookEntry CreateTestEntry(
        ulong canonicalHash,
        Player player = Player.Red,
        int depth = 0,
        int movesCount = 5,
        ulong? directHash = null)
    {
        var moves = new BookMove[movesCount];
        for (int i = 0; i < movesCount; i++)
        {
            moves[i] = new BookMove
            {
                RelativeX = i,
                RelativeY = i,
                WinRate = 50,
                DepthAchieved = 10,
                NodesSearched = 1000 - i * 100,
                Score = 100 - i * 10,
                IsForcing = false,
                Priority = i,
                IsVerified = true
            };
        }

        return new OpeningBookEntry
        {
            CanonicalHash = canonicalHash,
            DirectHash = directHash ?? canonicalHash,  // Default to canonical hash if not specified
            Depth = depth,
            Player = player,
            Symmetry = SymmetryType.Identity,
            IsNearEdge = false,
            Moves = moves
        };
    }
}
