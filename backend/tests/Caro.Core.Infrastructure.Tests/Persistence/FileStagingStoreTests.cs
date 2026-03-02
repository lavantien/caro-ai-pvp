using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Infrastructure.Persistence;
using Microsoft.Extensions.Logging;
using Moq;
using Xunit;

namespace Caro.Core.Infrastructure.Tests.Persistence;

public sealed class FileStagingStoreTests : IDisposable
{
    private readonly string _tempBasePath;
    private readonly Mock<ILogger<FileStagingBookStore>> _loggerMock;
    private readonly FileStagingBookStore _store;

    public FileStagingStoreTests()
    {
        _tempBasePath = Path.Combine(Path.GetTempPath(), $"file_staging_test_{Guid.NewGuid():N}");
        _loggerMock = new Mock<ILogger<FileStagingBookStore>>();
        _store = new FileStagingBookStore(_tempBasePath, _loggerMock.Object);
        _store.Initialize();
    }

    public void Dispose()
    {
        _store.Dispose();

        // Give time for file handles to be released
        GC.Collect();
        GC.WaitForPendingFinalizers();
        Thread.Sleep(50);

        // Delete temp directory with retries
        TryDeleteDirectoryWithRetry(_tempBasePath);
    }

    private static void TryDeleteDirectoryWithRetry(string path)
    {
        var retries = 3;
        for (int i = 0; i < retries; i++)
        {
            try
            {
                if (Directory.Exists(path))
                {
                    Directory.Delete(path, true);
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
    public void Initialize_CreatesDirectoryStructure()
    {
        // Assert
        Assert.True(Directory.Exists(Path.Combine(_tempBasePath, "buffer")));
        Assert.True(Directory.Exists(Path.Combine(_tempBasePath, "committed")));
        Assert.True(File.Exists(Path.Combine(_tempBasePath, "manifest.json")));
    }

    [Fact]
    public void RecordGame_SingleGame_StoresInBuffer()
    {
        // Arrange
        var game = CreateTestGame(1, Player.Red);

        // Act
        _store.RecordGame(game);

        // Assert - Game should be in buffer (not committed yet)
        Assert.Equal(0, _store.GetGameCount());
    }

    [Fact]
    public void RecordGame_BufferFull_CommitsBatch()
    {
        // Arrange - Record enough games to fill a single worker's buffer (256)
        // Use game IDs that all map to the same worker (worker 0)
        // gameId % Environment.ProcessorCount = 0 means gameId must be multiples of ProcessorCount
        var processorCount = Environment.ProcessorCount;
        for (int i = 0; i < 256; i++)
        {
            var gameId = i * processorCount; // All games map to worker 0
            var game = CreateTestGame(gameId, i % 2 == 0 ? Player.Red : Player.Blue);
            _store.RecordGame(game);
        }

        // Assert - Should auto-commit when buffer is full
        Assert.Equal(256, _store.GetGameCount());
        Assert.True(File.Exists(Path.Combine(_tempBasePath, "committed", "batch_0000.sgf")));
    }

    [Fact]
    public void Flush_CommitsRemainingBuffer()
    {
        // Arrange
        for (int i = 0; i < 10; i++)
        {
            var game = CreateTestGame(i, Player.Red);
            _store.RecordGame(game);
        }

        // Act
        _store.Flush();

        // Assert
        Assert.Equal(10, _store.GetGameCount());
    }

    [Fact]
    public void GetGames_ReturnsCommittedGames()
    {
        // Arrange
        for (int i = 0; i < 10; i++)
        {
            var game = CreateTestGame(i, i % 2 == 0 ? Player.Red : Player.Blue);
            _store.RecordGame(game);
        }
        _store.Flush();

        // Act
        var games = _store.GetGames(limit: 100);

        // Assert
        Assert.Equal(10, games.Count);
    }

    [Fact]
    public void GetGamesByResult_FiltersCorrectly()
    {
        // Arrange
        for (int i = 0; i < 10; i++)
        {
            var winner = i < 5 ? Player.Red : Player.Blue;
            var game = CreateTestGame(i, winner);
            _store.RecordGame(game);
        }
        _store.Flush();

        // Act
        var redWins = _store.GetGamesByResult(1, limit: 100);  // 1 = Red wins
        var blueWins = _store.GetGamesByResult(-1, limit: 100); // -1 = Blue wins

        // Assert - verify counts are correct
        Assert.Equal(5, redWins.Count);
        Assert.Equal(5, blueWins.Count);

        // Verify all red wins have Red winner
        Assert.All(redWins, game => Assert.Equal(Player.Red, game.Winner));
        // Verify all blue wins have Blue winner
        Assert.All(blueWins, game => Assert.Equal(Player.Blue, game.Winner));
    }

    [Fact]
    public void Clear_RemovesAllData()
    {
        // Arrange
        for (int i = 0; i < 10; i++)
        {
            var game = CreateTestGame(i, Player.Red);
            _store.RecordGame(game);
        }
        _store.Flush();
        Assert.Equal(10, _store.GetGameCount());

        // Act
        _store.Clear();

        // Assert
        Assert.Equal(0, _store.GetGameCount());
        Assert.False(Directory.Exists(Path.Combine(_tempBasePath, "committed")));
    }

    [Fact]
    public void GetPositionStatistics_ReconstructsPositions()
    {
        // Arrange
        var game = CreateTestGame(1, Player.Red);
        _store.RecordGame(game);
        _store.Flush();

        // Act
        var stats = _store.GetPositionStatistics();

        // Assert - Should have reconstructed some positions from the game
        Assert.NotEmpty(stats);
    }

    [Fact]
    public void ThreadSafety_ConcurrentGameRecording_DoesNotCorrupt()
    {
        // Arrange
        var tasks = new List<Task>();
        var gamesPerThread = 50;

        // Act - Record games from multiple threads
        for (int t = 0; t < 4; t++)
        {
            var threadId = t;
            tasks.Add(Task.Run(() =>
            {
                for (int g = 0; g < gamesPerThread; g++)
                {
                    var gameId = threadId * 1000 + g;
                    var game = CreateTestGame(gameId, gameId % 2 == 0 ? Player.Red : Player.Blue);
                    _store.RecordGame(game);
                }
            }));
        }

        Task.WaitAll(tasks.ToArray());
        _store.Flush();

        // Assert
        Assert.Equal(4 * gamesPerThread, _store.GetGameCount());
    }

    [Fact]
    public void Manifest_PersistsAcrossSessions()
    {
        // Arrange
        for (int i = 0; i < 10; i++)
        {
            var game = CreateTestGame(i, Player.Red);
            _store.RecordGame(game);
        }
        _store.Flush();

        // Act - Create new store instance
        _store.Dispose();
        var newStore = new FileStagingBookStore(_tempBasePath, _loggerMock.Object);
        newStore.Initialize();

        // Assert
        Assert.Equal(10, newStore.GetGameCount());
        newStore.Dispose();
    }

    private static SelfPlayGameRecord CreateTestGame(long gameId, Player winner)
    {
        // Create a simple SGF with a few moves
        var moves = new List<(int X, int Y)>
        {
            (9, 9),  // Center
            (10, 10),
            (8, 8),
            (11, 11),
            (7, 7)
        };

        var sgfBuilder = new System.Text.StringBuilder();
        sgfBuilder.Append("(;GM[1]SZ[19]RE[").Append(winner == Player.Red ? "B" : "W").Append("]");

        for (int i = 0; i < moves.Count; i++)
        {
            var (x, y) = moves[i];
            var coord = $"{(char)('a' + x)}{(char)('a' + y)}";
            sgfBuilder.Append(i % 2 == 0 ? ";B[" : ";W[").Append(coord).Append("]");
        }

        sgfBuilder.Append(")");

        return new SelfPlayGameRecord
        {
            GameId = gameId,
            SgfMoves = sgfBuilder.ToString(),
            Winner = winner,
            TotalMoves = moves.Count,
            MoveList = moves,
            Temperature = 1.0,
            Difficulty = AIDifficulty.Grandmaster,
            CreatedAt = DateTime.UtcNow
        };
    }
}
