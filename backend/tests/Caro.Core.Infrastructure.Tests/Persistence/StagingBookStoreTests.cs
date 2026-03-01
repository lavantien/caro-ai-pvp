using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Infrastructure.Persistence;
using Microsoft.Extensions.Logging;
using Moq;
using Xunit;

namespace Caro.Core.Infrastructure.Tests.Persistence;

public sealed class StagingBookStoreTests : IDisposable
{
    private readonly string _tempDbPath;
    private readonly Mock<ILogger<StagingBookStore>> _loggerMock;
    private readonly StagingBookStore _store;

    public StagingBookStoreTests()
    {
        _tempDbPath = Path.Combine(Path.GetTempPath(), $"staging_test_{Guid.NewGuid():N}.db");
        _loggerMock = new Mock<ILogger<StagingBookStore>>();
        _store = new StagingBookStore(_tempDbPath, _loggerMock.Object, bufferSize: 64);
        _store.Initialize();
    }

    public void Dispose()
    {
        _store.Dispose();

        // Give time for file handles to be released
        GC.Collect();
        GC.WaitForPendingFinalizers();
        Thread.Sleep(50);

        // Delete database and associated WAL/SHM files with retries
        TryDeleteFileWithRetry(_tempDbPath);
        TryDeleteFileWithRetry(_tempDbPath + "-wal");
        TryDeleteFileWithRetry(_tempDbPath + "-shm");
    }

    private static void TryDeleteFileWithRetry(string path)
    {
        var retries = 3;
        for (int i = 0; i < retries; i++)
        {
            try
            {
                if (File.Exists(path))
                {
                    File.Delete(path);
                    return;
                }
            }
            catch (IOException)
            {
                if (i < retries - 1)
                    Thread.Sleep(50);
            }
        }
    }

    [Fact]
    public void RecordMove_SingleMove_PersistsToDatabase()
    {
        // Arrange
        var canonicalHash = 12345UL;
        var directHash = 67890UL;
        var player = Player.Red;
        var ply = 0;
        var moveX = 8;
        var moveY = 8;
        var gameResult = 1;
        var gameId = 1L;
        var timeBudgetMs = 1024;

        // Act
        _store.RecordMove(canonicalHash, directHash, player, ply, moveX, moveY, gameResult, gameId, timeBudgetMs);
        _store.Flush();

        // Assert
        Assert.Equal(1, _store.GetPositionCount());
        Assert.Equal(1, _store.GetGameCount());
    }

    [Fact]
    public void RecordMove_BufferNotFull_RecordsInWriteBuffer()
    {
        // Arrange - write buffer size is 64 (internal constant)
        var store = new StagingBookStore(
            Path.Combine(Path.GetTempPath(), $"staging_test_noflush_{Guid.NewGuid():N}.db"),
            _loggerMock.Object,
            bufferSize: 64);

        store.Initialize();

        // Act - Record fewer moves than write buffer size (10 < 64)
        for (int i = 0; i < 10; i++)
        {
            store.RecordMove(100UL + (ulong)i, 200UL, Player.Red, i, i, i, 1, i, 1024);
        }

        // Assert - Should not have flushed to DB yet (records in write buffer)
        // GetPositionCount queries DB, not buffer, so expect 0
        Assert.Equal(0, store.GetPositionCount());

        // After explicit flush, all records should be in DB
        store.Flush();
        Assert.Equal(10, store.GetPositionCount());
        store.Dispose();
    }

    [Fact]
    public void RecordWriteBufferFull_AutoFlushesToDatabase()
    {
        // Arrange - write buffer size is 64 (internal constant)
        var store = new StagingBookStore(
            Path.Combine(Path.GetTempPath(), $"staging_test_autoflush_{Guid.NewGuid():N}.db"),
            _loggerMock.Object,
            bufferSize: 64);

        store.Initialize();

        // Act - Record exactly 64 moves to trigger auto-flush
        for (int i = 0; i < 64; i++)
        {
            store.RecordMove(100UL + (ulong)i, 200UL, Player.Red, i % 16, i % 16, i % 16, 1, i, 1024);
        }

        // Assert - 64 records should be in DB (auto-flushed)
        Assert.Equal(64, store.GetPositionCount());
        store.Dispose();
    }

    [Fact]
    public void GetPositionsForVerification_FiltersByMaxPly()
    {
        // Arrange
        for (int ply = 0; ply < 20; ply++)
        {
            _store.RecordMove(100UL + (ulong)ply, 200UL, Player.Red, ply, ply, ply, 1, ply, 1024);
        }
        _store.Flush();

        // Act
        var positions10 = _store.GetPositionsForVerification(maxPly: 10).ToList();
        var positions20 = _store.GetPositionsForVerification(maxPly: 20).ToList();

        // Assert
        Assert.Equal(11, positions10.Count);  // ply 0-10 inclusive
        Assert.Equal(20, positions20.Count);
    }

    [Fact]
    public void GetPositionStatistics_AggregatesCorrectly()
    {
        // Arrange - Record same position multiple times with different results
        var canonicalHash = 12345UL;
        var directHash = 67890UL;
        var player = Player.Red;

        // 10 plays: 6 wins, 2 draws, 2 losses
        for (int i = 0; i < 6; i++)
            _store.RecordMove(canonicalHash, directHash, player, 0, 8, 8, 1, i, 1024);
        for (int i = 6; i < 8; i++)
            _store.RecordMove(canonicalHash, directHash, player, 0, 8, 8, 0, i, 1024);
        for (int i = 8; i < 10; i++)
            _store.RecordMove(canonicalHash, directHash, player, 0, 8, 8, -1, i, 1024);

        _store.Flush();

        // Act
        var stats = _store.GetPositionStatistics();

        // Assert
        Assert.Single(stats);
        var positionStats = stats[(canonicalHash, directHash, player)];
        Assert.Equal(10, positionStats.PlayCount);
        Assert.Equal(6, positionStats.WinCount);
        Assert.Equal(2, positionStats.DrawCount);
        Assert.Equal(2, positionStats.LossCount);
        Assert.Equal(0.6, positionStats.WinRate, 2);
    }

    [Fact]
    public void GetMovesForPosition_ReturnsAllMoves()
    {
        // Arrange - Record different moves for same position
        var canonicalHash = 12345UL;
        var directHash = 67890UL;
        var player = Player.Red;

        _store.RecordMove(canonicalHash, directHash, player, 0, 8, 8, 1, 1, 1024);  // (8,8) wins
        _store.RecordMove(canonicalHash, directHash, player, 0, 8, 8, 1, 2, 1024);  // (8,8) wins
        _store.RecordMove(canonicalHash, directHash, player, 0, 7, 7, -1, 3, 1024); // (7,7) loses
        _store.RecordMove(canonicalHash, directHash, player, 0, 9, 9, 0, 4, 1024);  // (9,9) draws

        _store.Flush();

        // Act
        var moves = _store.GetMovesForPosition(canonicalHash, directHash, player);

        // Assert
        Assert.Equal(3, moves.Count);

        var move88 = moves.FirstOrDefault(m => m.MoveX == 8 && m.MoveY == 8);
        Assert.NotNull(move88);
        Assert.Equal(2, move88.PlayCount);
        Assert.Equal(1.0, move88.WinRate, 2);
    }

    [Fact]
    public void Clear_RemovesAllData()
    {
        // Arrange
        for (int i = 0; i < 10; i++)
        {
            _store.RecordMove(100UL + (ulong)i, 200UL, Player.Red, i, i, i, 1, i, 1024);
        }
        _store.Flush();
        Assert.Equal(10, _store.GetPositionCount());

        // Act
        _store.Clear();

        // Assert
        Assert.Equal(0, _store.GetPositionCount());
        Assert.Equal(0, _store.GetGameCount());
    }

    [Fact]
    public void Flush_ExplicitCall_PersistsBuffer()
    {
        // Arrange
        _store.RecordMove(100UL, 200UL, Player.Red, 0, 8, 8, 1, 1, 1024);

        // Act
        _store.Flush();

        // Assert - Should persist even without filling buffer
        Assert.Equal(1, _store.GetPositionCount());
    }

    [Fact]
    public void ThreadSafety_ConcurrentRecords_DoesNotCorrupt()
    {
        // Arrange
        var tasks = new List<Task>();
        var gamesPerThread = 10;

        // Act - Record moves from multiple threads
        for (int t = 0; t < 4; t++)
        {
            var threadId = t;
            tasks.Add(Task.Run(() =>
            {
                for (int g = 0; g < gamesPerThread; g++)
                {
                    var gameId = threadId * 1000 + g;
                    _store.RecordMove(
                        (ulong)gameId,
                        (ulong)gameId * 2,
                        Player.Red,
                        0,
                        8,
                        8,
                        1,
                        gameId,
                        1024);
                }
            }));
        }

        Task.WaitAll(tasks.ToArray());
        _store.Flush();

        // Assert
        Assert.Equal(4 * gamesPerThread, _store.GetPositionCount());
    }

    [Fact]
    public void PowersOfTwo_AllBufferSizesArePowersOf2()
    {
        // Arrange & Act
        var defaultBufferSize = 4096;  // 2^12
        var writeBufferSize = 64;      // 2^6

        // Assert - Verify they are powers of 2
        Assert.True(IsPowerOfTwo(defaultBufferSize), $"Default buffer size {defaultBufferSize} is not power of 2");
        Assert.True(IsPowerOfTwo(writeBufferSize), $"Write buffer size {writeBufferSize} is not power of 2");

        // Verify common buffer sizes used in the system
        var validSizes = new[] { 64, 128, 256, 512, 1024, 2048, 4096, 8192, 16384, 32768, 65536 };
        foreach (var size in validSizes)
        {
            Assert.True(IsPowerOfTwo(size), $"Size {size} is not power of 2");
        }
    }

    [Fact]
    public void Constructor_InvalidBufferSize_ThrowsArgumentException()
    {
        // Arrange & Act & Assert
        Assert.Throws<ArgumentException>(() =>
            new StagingBookStore(
                Path.Combine(Path.GetTempPath(), $"test_{Guid.NewGuid():N}.db"),
                _loggerMock.Object,
                bufferSize: 100));  // Not power of 2

        Assert.Throws<ArgumentException>(() =>
            new StagingBookStore(
                Path.Combine(Path.GetTempPath(), $"test_{Guid.NewGuid():N}.db"),
                _loggerMock.Object,
                bufferSize: 0));

        Assert.Throws<ArgumentException>(() =>
            new StagingBookStore(
                Path.Combine(Path.GetTempPath(), $"test_{Guid.NewGuid():N}.db"),
                _loggerMock.Object,
                bufferSize: -1));
    }

    [Fact]
    public void Constructor_ValidBufferSize_DoesNotThrow()
    {
        // Arrange & Act & Assert - Should not throw for powers of 2
        foreach (var size in new[] { 1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024, 2048, 4096 })
        {
            using var store = new StagingBookStore(
                Path.Combine(Path.GetTempPath(), $"test_{Guid.NewGuid():N}.db"),
                _loggerMock.Object,
                bufferSize: size);
            store.Initialize();
        }
    }

    private static bool IsPowerOfTwo(int n)
    {
        return n > 0 && (n & (n - 1)) == 0;
    }
}
