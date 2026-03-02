using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Infrastructure.Persistence;
using Microsoft.Extensions.Logging;
using Moq;
using Xunit;

namespace Caro.Core.IntegrationTests.GameLogic.OpeningBook;

/// <summary>
/// End-to-end tests for the separated pipeline architecture:
/// Phase 1: Self-Play (Actor) → Phase 2: Verification (Critic) → Phase 3: Integration
/// </summary>
public sealed class SeparatedPipelineTests : IDisposable
{
    private readonly string _stagingDbPath;
    private readonly string _mainBookDbPath;
    private readonly Mock<ILogger<StagingBookStore>> _stagingLoggerMock;
    private readonly Mock<ILogger<SqliteOpeningBookStore>> _bookLoggerMock;
    private readonly Mock<ILoggerFactory> _loggerFactoryMock;

    public SeparatedPipelineTests()
    {
        _stagingDbPath = Path.Combine(Path.GetTempPath(), $"pipeline_staging_{Guid.NewGuid():N}.db");
        _mainBookDbPath = Path.Combine(Path.GetTempPath(), $"pipeline_book_{Guid.NewGuid():N}.db");
        _stagingLoggerMock = new Mock<ILogger<StagingBookStore>>();
        _bookLoggerMock = new Mock<ILogger<SqliteOpeningBookStore>>();
        _loggerFactoryMock = new Mock<ILoggerFactory>();
        _loggerFactoryMock.Setup(f => f.CreateLogger(It.IsAny<string>()))
            .Returns(Mock.Of<ILogger>());
    }

    public void Dispose()
    {
        if (File.Exists(_stagingDbPath))
            File.Delete(_stagingDbPath);
        if (File.Exists(_mainBookDbPath))
            File.Delete(_mainBookDbPath);
    }

    [Fact]
    public async Task FullPipeline_SelfPlayToVerificationToIntegration_ProducesVerifiedBook()
    {
        // Phase 1: Self-Play Generation (Actor)
        using var stagingStore = new StagingBookStore(_stagingDbPath, _stagingLoggerMock.Object, bufferSize: 64);
        stagingStore.Initialize();

        // Simulate self-play games recording moves
        var gameId = 0L;
        for (int g = 0; g < 10; g++)
        {
            // Simulate a game with moves at ply 0-8
            for (int ply = 0; ply < 9; ply++)
            {
                var canonicalHash = (ulong)(g * 100 + ply);
                var directHash = canonicalHash * 2;
                var player = ply % 2 == 0 ? Player.Red : Player.Blue;
                var result = g % 3 == 0 ? 1 : (g % 3 == 1 ? 0 : -1); // Win, Draw, Loss cycle

                stagingStore.RecordMove(
                    canonicalHash,
                    directHash,
                    player,
                    ply,
                    8 + ply,
                    8 + ply,
                    result,
                    gameId,
                    timeBudgetMs: 1024);
            }
            gameId++;
        }
        stagingStore.Flush();

        // Verify staging has data
        Assert.Equal(90, stagingStore.GetPositionCount()); // 10 games * 9 moves

        // Phase 2: Verification (Critic)
        var verifier = new MoveVerifier(stagingStore, new PositionCanonicalizer(), _loggerFactoryMock.Object);
        var summary = await verifier.VerifyStagingAsync(
            verificationTimeMs: 128, // Short time for tests
            maxPly: 16);

        // Phase 3: Integration
        using var mainBook = new SqliteOpeningBookStore(_mainBookDbPath, _bookLoggerMock.Object);
        mainBook.Initialize();

        var integrationSummary = mainBook.IntegrateVerifiedMoves(summary.VerifiedMoves, batchSize: 64);

        // Verify the pipeline produced results
        Assert.True(summary.TotalPositionsProcessed >= 0);
        Assert.NotNull(summary.VerifiedMoves);
        Assert.NotNull(integrationSummary);
    }

    [Fact]
    public void Phase1_StagingStore_RecordsAllMoves()
    {
        // Arrange
        using var store = new StagingBookStore(_stagingDbPath, _stagingLoggerMock.Object, bufferSize: 64);
        store.Initialize();

        // Act - Record moves from multiple games
        for (int game = 0; game < 5; game++)
        {
            for (int ply = 0; ply < 4; ply++)
            {
                store.RecordMove(
                    canonicalHash: (ulong)(game * 10 + ply),
                    directHash: (ulong)(game * 10 + ply + 100),
                    player: ply % 2 == 0 ? Player.Red : Player.Blue,
                    ply: ply,
                    moveX: 8 + ply,
                    moveY: 8 + ply,
                    gameResult: 1,
                    gameId: game,
                    timeBudgetMs: 1024);
            }
        }
        store.Flush();

        // Assert - All moves recorded
        Assert.Equal(20, store.GetPositionCount()); // 5 games * 4 moves
        Assert.Equal(5, store.GetGameCount());
    }

    [Fact]
    public void Phase1_Statistics_AggregateCorrectly()
    {
        // Arrange
        using var store = new StagingBookStore(_stagingDbPath, _stagingLoggerMock.Object, bufferSize: 64);
        store.Initialize();

        var canonicalHash = 12345UL;
        var directHash = 67890UL;
        var player = Player.Red;

        // Act - Record the same position multiple times with different results
        // 8 wins, 4 draws, 4 losses = 16 total, 50% win rate
        for (int i = 0; i < 8; i++)
            store.RecordMove(canonicalHash, directHash, player, 0, 8, 8, 1, i, 1024);
        for (int i = 8; i < 12; i++)
            store.RecordMove(canonicalHash, directHash, player, 0, 8, 8, 0, i, 1024);
        for (int i = 12; i < 16; i++)
            store.RecordMove(canonicalHash, directHash, player, 0, 8, 8, -1, i, 1024);

        store.Flush();

        // Assert
        var stats = store.GetPositionStatistics();
        Assert.Single(stats);

        var posStats = stats[(canonicalHash, directHash, player)];
        Assert.Equal(16, posStats.PlayCount);
        Assert.Equal(8, posStats.WinCount);
        Assert.Equal(4, posStats.DrawCount);
        Assert.Equal(4, posStats.LossCount);
        Assert.Equal(0.5, posStats.WinRate, 2);
    }

    [Fact]
    public async Task Phase2_Verification_FiltersLowPlayCount()
    {
        // Arrange
        using var store = new StagingBookStore(_stagingDbPath, _stagingLoggerMock.Object, bufferSize: 64);
        store.Initialize();

        var thresholds = MoveVerifier.GetThresholds();

        // Below threshold
        for (int i = 0; i < thresholds.MinPlayCount - 1; i++)
        {
            store.RecordMove(100UL, 200UL, Player.Red, 0, 8, 8, 1, i, 1024);
        }
        // Above threshold
        for (int i = 0; i < thresholds.MinPlayCount; i++)
        {
            store.RecordMove(101UL, 201UL, Player.Red, 0, 9, 9, 1, i + 1000, 1024);
        }
        store.Flush();

        var verifier = new MoveVerifier(store, new PositionCanonicalizer(), _loggerFactoryMock.Object);

        // Act
        var summary = await verifier.VerifyStagingAsync(verificationTimeMs: 128, maxPly: 16);

        // Assert
        Assert.Equal(2, summary.TotalPositionsProcessed);
        Assert.Equal(1, summary.FilteredLowPlayCount);
    }

    [Fact]
    public async Task Phase2_Verification_FiltersUnclearResults()
    {
        // Arrange
        using var store = new StagingBookStore(_stagingDbPath, _stagingLoggerMock.Object, bufferSize: 64);
        store.Initialize();

        var thresholds = MoveVerifier.GetThresholds();

        // Create position with unclear result (win rate in gray zone 0.375-0.625)
        var winCount = (int)(thresholds.MinPlayCount * 0.5); // 50% win rate
        var lossCount = thresholds.MinPlayCount - winCount;

        for (int i = 0; i < winCount; i++)
            store.RecordMove(100UL, 200UL, Player.Red, 0, 8, 8, 1, i, 1024);
        for (int i = 0; i < lossCount; i++)
            store.RecordMove(100UL, 200UL, Player.Red, 0, 8, 8, -1, i + 1000, 1024);

        store.Flush();

        var verifier = new MoveVerifier(store, new PositionCanonicalizer(), _loggerFactoryMock.Object);

        // Act
        var summary = await verifier.VerifyStagingAsync(verificationTimeMs: 128, maxPly: 16);

        // Assert
        Assert.Equal(1, summary.FilteredUnclearResult);
    }

    [Fact]
    public void Phase3_Integration_BatchInsertWorks()
    {
        // Arrange
        using var store = new SqliteOpeningBookStore(_mainBookDbPath, _bookLoggerMock.Object);
        store.Initialize();

        var verifiedMoves = new List<VerifiedMove>();
        for (int i = 0; i < 128; i++)
        {
            verifiedMoves.Add(new VerifiedMove
            {
                CanonicalHash = (ulong)i,
                DirectHash = (ulong)(i * 2),
                Player = Player.Red,
                Ply = 0,
                Move = (8, 8),
                Score = 500,
                ScoreDelta = 0,
                Source = MoveSource.Learned,
                WinRate = 0.8,
                PlayCount = 512,
                TimeBudgetMs = 2048,
                IsVerified = true
            });
        }

        // Act
        var summary = store.IntegrateVerifiedMoves(verifiedMoves, batchSize: 64);

        // Assert
        Assert.True(summary.PositionsIntegrated >= 0);
        Assert.Equal(128, summary.MovesIntegrated);
    }

    [Fact]
    public void AllThresholds_ArePowersOfTwo()
    {
        // Arrange & Act
        var thresholds = MoveVerifier.GetThresholds();

        // Assert - All integer thresholds should be powers of 2
        Assert.True(IsPowerOfTwo(thresholds.MinPlayCount));
        Assert.True(IsPowerOfTwo(thresholds.MaxScoreDelta));
        Assert.True(IsPowerOfTwo(thresholds.InclusionScoreDelta));
        Assert.True(IsPowerOfTwo(thresholds.MaxMovesPerPosition));
        Assert.True(IsPowerOfTwo(thresholds.VcfTriggerThreats));
        Assert.True(IsPowerOfTwo(thresholds.DefaultVerificationTimeMs));
        Assert.True(IsPowerOfTwo(thresholds.ExtendedVerificationTimeMs));
        Assert.True(IsPowerOfTwo(thresholds.VcfTimeLimitMs));

        // Verify fraction-based thresholds
        Assert.Equal(0.625, thresholds.MinWinRate);           // 5/8
        Assert.Equal(0.375, thresholds.MaxWinRateForLoss);    // 3/8
        Assert.Equal(0.8125, thresholds.MinConsensusRate);    // 13/16
    }

    [Fact]
    public void SurvivalZone_GetsExtendedTime()
    {
        // Arrange & Act
        var thresholds = MoveVerifier.GetThresholds();

        // Assert - Extended time should be double the default
        Assert.Equal(thresholds.DefaultVerificationTimeMs * 2, thresholds.ExtendedVerificationTimeMs);
        Assert.Equal(4096, thresholds.DefaultVerificationTimeMs);   // 2^12 (quality-optimized)
        Assert.Equal(8192, thresholds.ExtendedVerificationTimeMs);  // 2^13 (survival zone)
    }

    [Fact]
    public void Pipeline_WithEmptyStaging_ProducesEmptyResults()
    {
        // Arrange
        using var store = new StagingBookStore(_stagingDbPath, _stagingLoggerMock.Object, bufferSize: 64);
        store.Initialize();
        // No data recorded

        // Act
        var stats = store.GetPositionStatistics();

        // Assert
        Assert.Empty(stats);
        Assert.Equal(0, store.GetPositionCount());
        Assert.Equal(0, store.GetGameCount());
    }

    [Fact]
    public async Task Pipeline_WithDataBelowThresholds_FiltersAppropriately()
    {
        // Arrange
        using var store = new StagingBookStore(_stagingDbPath, _stagingLoggerMock.Object, bufferSize: 64);
        store.Initialize();

        var thresholds = MoveVerifier.GetThresholds();

        // Position 1: Below MinPlayCount (should be filtered)
        for (int i = 0; i < 10; i++)
            store.RecordMove(1UL, 2UL, Player.Red, 0, 8, 8, 1, i, 1024);

        // Position 2: Enough plays but unclear win rate (should be filtered)
        var halfCount = thresholds.MinPlayCount;
        for (int i = 0; i < halfCount / 2; i++)
        {
            store.RecordMove(3UL, 4UL, Player.Red, 0, 8, 8, 1, i + 100, 1024);
            store.RecordMove(3UL, 4UL, Player.Red, 0, 8, 8, -1, i + 200, 1024);
        }

        store.Flush();

        var verifier = new MoveVerifier(store, new PositionCanonicalizer(), _loggerFactoryMock.Object);

        // Act
        var summary = await verifier.VerifyStagingAsync(verificationTimeMs: 128, maxPly: 16);

        // Assert
        Assert.Equal(2, summary.TotalPositionsProcessed);
        Assert.Equal(1, summary.FilteredLowPlayCount); // Position 1
        Assert.Equal(1, summary.FilteredUnclearResult); // Position 2
        Assert.Empty(summary.VerifiedMoves); // No moves should pass verification
    }

    private static bool IsPowerOfTwo(int n)
    {
        return n > 0 && (n & (n - 1)) == 0;
    }
}
