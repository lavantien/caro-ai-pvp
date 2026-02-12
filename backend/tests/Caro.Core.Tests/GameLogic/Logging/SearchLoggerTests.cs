using Caro.Core.GameLogic.Logging;
using Caro.Core.Domain.Entities;
using Xunit;
using System.IO;

namespace Caro.Core.Tests.GameLogic.Logging;

public class SearchLoggerTests : IDisposable
{
    private readonly string _tempDir;
    private readonly string _testLogDir;

    public SearchLoggerTests()
    {
        _tempDir = Path.Combine(Path.GetTempPath(), $"caro-test-{Guid.NewGuid()}");
        _testLogDir = Path.Combine(_tempDir, "logs");
    }

    [Fact]
    public void Constructor_CreatesLogDirectory()
    {
        // Arrange & Act
        using var logger = new SearchLogger(_testLogDir);

        // Assert
        Assert.True(Directory.Exists(_testLogDir));
    }

    [Fact]
    public void LogSearchCompletion_WritesToFile()
    {
        // Arrange
        string logPath;
        {
            using var logger = new SearchLogger(_testLogDir);
            logPath = logger.GetCurrentLogPath();

            // Act
            logger.LogSearchCompletion(
                playerId: "test-player",
                player: Player.Red,
                depth: 10,
                nodes: 1000000,
                nps: 500000,
                ttHits: 6000,
                ttProbes: 10000,
                timeMs: 2000,
                score: 150,
                bestMove: (9, 9));
            // Dispose waits for background task to complete
        }

        // Assert - read file after logger is disposed
        Assert.True(File.Exists(logPath));
        string content = File.ReadAllText(logPath);
        Assert.Contains("test-player", content);
        Assert.Contains("\"EntryType\":2", content); // SearchComplete = 2
    }

    [Fact]
    public void LogIteration_WritesToFile()
    {
        // Arrange
        string logPath;
        {
            using var logger = new SearchLogger(_testLogDir);
            logPath = logger.GetCurrentLogPath();

            // Act
            logger.LogIteration(
                playerId: "test-player",
                depth: 5,
                nodes: 50000,
                score: 100,
                bestMove: (8, 8),
                timeMs: 100);
            // Dispose waits for background task to complete
        }

        // Assert - read file after logger is disposed
        Assert.True(File.Exists(logPath));
        string content = File.ReadAllText(logPath);
        Assert.Contains("\"EntryType\":1", content); // Iteration = 1
    }

    [Fact]
    public void LogTTOperation_WritesToFile()
    {
        // Arrange
        string logPath;
        {
            using var logger = new SearchLogger(_testLogDir);
            logPath = logger.GetCurrentLogPath();

            // Act
            logger.LogTTOperation(
                playerId: "test-player",
                hash: 0x123456789ABCDEF0,
                depth: 10,
                found: true,
                score: 150,
                bestMove: (9, 9));
            // Dispose waits for background task to complete
        }

        // Assert - read file after logger is disposed
        Assert.True(File.Exists(logPath));
        string content = File.ReadAllText(logPath);
        Assert.Contains("\"EntryType\":3", content); // TTProbe = 3
    }

    [Fact]
    public void Dispose_FlushesPendingEntries()
    {
        // Arrange
        string logPath;
        {
            using var logger = new SearchLogger(_testLogDir);
            logPath = logger.GetCurrentLogPath();

            // Act - Log multiple entries
            for (int i = 0; i < 10; i++)
            {
                logger.LogIteration(
                    playerId: "test-player",
                    depth: i,
                    nodes: i * 1000,
                    score: i * 10,
                    bestMove: (i, i),
                    timeMs: i * 10);
            }
            // Dispose waits for background task to complete
        }

        // Assert - read file after logger is disposed
        string content = File.ReadAllText(logPath);
        // Should have multiple lines
        Assert.True(content.Count('\n') >= 10);
    }

    [Fact]
    public void GetCurrentLogPath_ReturnsValidPath()
    {
        // Arrange
        using var logger = new SearchLogger(_testLogDir, "test-");

        // Act
        string path = logger.GetCurrentLogPath();

        // Assert
        Assert.StartsWith(_testLogDir, path);
        Assert.Contains("test-", path);
        Assert.EndsWith(".log", path);
    }

    [Fact]
    public void LogMultipleEntries_AllPersisted()
    {
        // Arrange
        string logPath;
        {
            using var logger = new SearchLogger(_testLogDir);
            logPath = logger.GetCurrentLogPath();

            // Act - Log various entry types
            logger.LogSearchCompletion(
                playerId: "p1",
                player: Player.Red,
                depth: 8,
                nodes: 100000,
                nps: 100000,
                ttHits: 1000,
                ttProbes: 2000,
                timeMs: 1000,
                score: 100,
                bestMove: (5, 5));

            logger.LogTTOperation(
                playerId: "p1",
                hash: 0xABCD,
                depth: 8,
                found: false,
                score: null,
                bestMove: null);

            logger.LogIteration(
                playerId: "p1",
                depth: 9,
                nodes: 200000,
                score: 120,
                bestMove: (6, 6),
                timeMs: 2000);
            // Dispose waits for background task to complete
        }

        // Assert - read file after logger is disposed
        string content = File.ReadAllText(logPath);
        Assert.Contains("\"EntryType\":2", content); // SearchComplete = 2
        Assert.Contains("\"EntryType\":3", content); // TTProbe = 3
        Assert.Contains("\"EntryType\":1", content); // Iteration = 1
    }

    public void Dispose()
    {
        // Clean up temp directory
        try
        {
            if (Directory.Exists(_tempDir))
            {
                Directory.Delete(_tempDir, recursive: true);
            }
        }
        catch
        {
            // Ignore cleanup errors
        }
    }
}
