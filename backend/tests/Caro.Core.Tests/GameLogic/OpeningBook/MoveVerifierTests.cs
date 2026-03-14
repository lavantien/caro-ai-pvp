using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Microsoft.Extensions.Logging;
using Moq;
using Xunit;

namespace Caro.Core.Tests.GameLogic.OpeningBook;

public sealed class MoveVerifierTests : IDisposable
{
    private readonly Mock<IStagingBookStore> _stagingStoreMock;
    private readonly Mock<ILogger<MoveVerifier>> _loggerMock;
    private readonly Mock<ILoggerFactory> _loggerFactoryMock;
    private readonly MoveVerifier _verifier;

    public MoveVerifierTests()
    {
        _stagingStoreMock = new Mock<IStagingBookStore>();
        _loggerMock = new Mock<ILogger<MoveVerifier>>();
        _loggerFactoryMock = new Mock<ILoggerFactory>();

        _loggerFactoryMock.Setup(f => f.CreateLogger(It.IsAny<string>()))
            .Returns(Mock.Of<ILogger>());

        _verifier = new MoveVerifier(
            _stagingStoreMock.Object,
            new PositionCanonicalizer(),
            _loggerFactoryMock.Object);
    }

    public void Dispose()
    {
        // Cleanup if needed
    }

    [Fact]
    public void GetThresholds_AllValuesArePowersOfTwo()
    {
        // Act
        var thresholds = MoveVerifier.GetThresholds();

        // Assert - Core values should be powers of 2
        Assert.True(IsPowerOfTwo(thresholds.MinPlayCount), $"MinPlayCount {thresholds.MinPlayCount} is not power of 2");
        Assert.True(IsPowerOfTwo(thresholds.MaxMovesPerPosition), $"MaxMovesPerPosition {thresholds.MaxMovesPerPosition} is not power of 2");
        Assert.True(IsPowerOfTwo(thresholds.VcfTriggerThreats), $"VcfTriggerThreats {thresholds.VcfTriggerThreats} is not power of 2");
        Assert.True(IsPowerOfTwo(thresholds.DefaultVerificationTimeMs), $"DefaultVerificationTimeMs {thresholds.DefaultVerificationTimeMs} is not power of 2");
        Assert.True(IsPowerOfTwo(thresholds.ExtendedVerificationTimeMs), $"ExtendedVerificationTimeMs {thresholds.ExtendedVerificationTimeMs} is not power of 2");
        Assert.True(IsPowerOfTwo(thresholds.VcfTimeLimitMs), $"VcfTimeLimitMs {thresholds.VcfTimeLimitMs} is not power of 2");

        // Score deltas are sums of powers of 2 (widened for yield optimization)
        Assert.Equal(768, thresholds.MaxScoreDelta);           // 512 + 256 = 2^9 + 2^8
        Assert.Equal(384, thresholds.InclusionScoreDelta);     // 256 + 128 = 2^8 + 2^7

        // Verify fraction-based thresholds (widened for yield optimization)
        Assert.Equal(0.70, thresholds.MinWinRate, 2);          // 70% (raised from 62.5%)
        Assert.Equal(0.30, thresholds.MaxWinRateForLoss, 2);   // 30% (lowered from 37.5%)
        Assert.Equal(0.8125, thresholds.MinConsensusRate);     // 13/16
    }

    [Fact]
    public async Task VerifyStagingAsync_SkipsLowVisitPositions()
    {
        // Arrange - Create games that will result in positions with low visit counts
        var thresholds = MoveVerifier.GetThresholds();

        // Create games with moves that all go to the same position
        // Since MinPlayCount is 512, we need 512+ visits to pass
        var games = new List<SelfPlayGameRecord>();

        // Create games with only 10 visits (below threshold of 512)
        for (int i = 0; i < 10; i++)
        {
            games.Add(new SelfPlayGameRecord
            {
                SgfMoves = "B[ii];",
                Winner = Player.Red,
                TotalMoves = 1,
                MoveList = new List<(int, int)> { (8, 8) }
            });
        }

        _stagingStoreMock.Setup(s => s.GetGames(It.IsAny<int>(), It.IsAny<int>()))
            .Returns(() =>
            {
                // Return games on first call, empty on subsequent calls
                var result = games;
                games = new List<SelfPlayGameRecord>();
                return result;
            });

        // Act
        var summary = await _verifier.VerifyStagingAsync(
            verificationTimeMs: 2048,
            maxPly: 16);

        // Assert
        Assert.Equal(1, summary.TotalPositionsProcessed);  // One unique position
        Assert.Equal(1, summary.FilteredLowPlayCount);  // Filtered due to low visit count
    }

    [Fact]
    public async Task VerifyStagingAsync_SkipsUnclearResults()
    {
        // Arrange - Create games that result in positions with unclear win rates (0.375-0.625)
        var games = new List<SelfPlayGameRecord>();

        // Create 600 games with 50% win rate (in the gray zone)
        for (int i = 0; i < 600; i++)
        {
            games.Add(new SelfPlayGameRecord
            {
                SgfMoves = "B[ii];",
                Winner = i % 2 == 0 ? Player.Red : Player.Blue,  // 50% win rate
                TotalMoves = 1,
                MoveList = new List<(int, int)> { (8, 8) }
            });
        }

        _stagingStoreMock.Setup(s => s.GetGames(It.IsAny<int>(), It.IsAny<int>()))
            .Returns(() =>
            {
                var result = games;
                games = new List<SelfPlayGameRecord>();
                return result;
            });

        // Act
        var summary = await _verifier.VerifyStagingAsync(
            verificationTimeMs: 2048,
            maxPly: 16);

        // Assert
        Assert.Equal(1, summary.TotalPositionsProcessed);
        Assert.Equal(1, summary.FilteredUnclearResult);  // Gray zone position filtered
    }

    [Fact]
    public void MinPlayCount_IsPowerOfTwo()
    {
        var thresholds = MoveVerifier.GetThresholds();
        Assert.Equal(512, thresholds.MinPlayCount);  // 2^9
        Assert.True(IsPowerOfTwo(thresholds.MinPlayCount));
    }

    [Fact]
    public void MinWinRate_IsSeventyPercent()
    {
        var thresholds = MoveVerifier.GetThresholds();
        Assert.Equal(0.70, thresholds.MinWinRate);  // 70%
    }

    [Fact]
    public void MaxWinRateForLoss_IsThirtyPercent()
    {
        var thresholds = MoveVerifier.GetThresholds();
        Assert.Equal(0.30, thresholds.MaxWinRateForLoss);  // 30%
    }

    [Fact]
    public void MinConsensusRate_IsThirteenSixteenths()
    {
        var thresholds = MoveVerifier.GetThresholds();
        Assert.Equal(0.8125, thresholds.MinConsensusRate);  // 13/16
    }

    [Fact]
    public void MaxScoreDelta_IsSevenSixtyEight()
    {
        var thresholds = MoveVerifier.GetThresholds();
        Assert.Equal(768, thresholds.MaxScoreDelta);  // 2^9 + 2^8 centipawns (widened for yield)
    }

    [Fact]
    public void InclusionScoreDelta_IsThreeEightyFour()
    {
        var thresholds = MoveVerifier.GetThresholds();
        Assert.Equal(384, thresholds.InclusionScoreDelta);  // 2^8 + 2^7 centipawns (widened for yield)
    }

    [Fact]
    public void MaxMovesPerPosition_IsPowerOfTwo()
    {
        var thresholds = MoveVerifier.GetThresholds();
        Assert.Equal(4, thresholds.MaxMovesPerPosition);  // 2^2
        Assert.True(IsPowerOfTwo(thresholds.MaxMovesPerPosition));
    }

    [Fact]
    public void VcfTriggerThreats_IsOne()
    {
        var thresholds = MoveVerifier.GetThresholds();
        Assert.Equal(1, thresholds.VcfTriggerThreats);  // Lowered to catch more tactical positions
    }

    [Fact]
    public void DefaultVerificationTimeMs_IsPowerOfTwo()
    {
        var thresholds = MoveVerifier.GetThresholds();
        Assert.Equal(4096, thresholds.DefaultVerificationTimeMs);  // 2^12 ms (quality-optimized)
        Assert.True(IsPowerOfTwo(thresholds.DefaultVerificationTimeMs));
    }

    [Fact]
    public void ExtendedVerificationTimeMs_IsPowerOfTwo()
    {
        var thresholds = MoveVerifier.GetThresholds();
        Assert.Equal(8192, thresholds.ExtendedVerificationTimeMs);  // 2^13 ms (survival zone, quality-optimized)
        Assert.True(IsPowerOfTwo(thresholds.ExtendedVerificationTimeMs));
    }

    [Fact]
    public void VcfTimeLimitMs_IsPowerOfTwo()
    {
        var thresholds = MoveVerifier.GetThresholds();
        Assert.Equal(128, thresholds.VcfTimeLimitMs);  // 2^7 ms
        Assert.True(IsPowerOfTwo(thresholds.VcfTimeLimitMs));
    }

    [Fact]
    public void SurvivalZone_GetsMoreTime()
    {
        var thresholds = MoveVerifier.GetThresholds();
        // Survival zone (ply 8-16) should get extended time
        Assert.Equal(thresholds.DefaultVerificationTimeMs * 2, thresholds.ExtendedVerificationTimeMs);
    }

    [Fact]
    public void VerifiedMove_PropertiesAreSetCorrectly()
    {
        // Arrange & Act
        var move = new VerifiedMove
        {
            CanonicalHash = 12345UL,
            DirectHash = 67890UL,
            Player = Player.Red,
            Ply = 5,
            Move = (8, 8),
            Score = 500,
            ScoreDelta = 0,
            Source = MoveSource.Solved,
            WinRate = 0.95,
            PlayCount = 1000,
            TimeBudgetMs = 2048,
            IsVerified = true
        };

        // Assert
        Assert.Equal(12345UL, move.CanonicalHash);
        Assert.Equal(67890UL, move.DirectHash);
        Assert.Equal(Player.Red, move.Player);
        Assert.Equal(5, move.Ply);
        Assert.Equal(8, move.Move.X);
        Assert.Equal(8, move.Move.Y);
        Assert.Equal(500, move.Score);
        Assert.Equal(0, move.ScoreDelta);
        Assert.Equal(MoveSource.Solved, move.Source);
        Assert.Equal(0.95, move.WinRate);
        Assert.Equal(1000, move.PlayCount);
        Assert.Equal(2048, move.TimeBudgetMs);
        Assert.True(move.IsVerified);
    }

    [Fact]
    public void VerificationSummary_PropertiesAreSetCorrectly()
    {
        // Arrange
        var moves = new List<VerifiedMove>
        {
            new VerifiedMove
            {
                CanonicalHash = 1UL,
                DirectHash = 2UL,
                Player = Player.Red,
                Ply = 0,
                Move = (8, 8),
                Score = 500,
                ScoreDelta = 0,
                Source = MoveSource.Solved,
                WinRate = 1.0,
                PlayCount = 512,
                TimeBudgetMs = 2048,
                IsVerified = true
            }
        };

        // Act
        var summary = new VerificationSummary(
            verifiedMoves: moves,
            totalPositionsProcessed: 100,
            filteredLowPlayCount: 20,
            filteredUnclearResult: 15,
            filteredMoveWinRate: 10,
            filteredScoreDelta: 8,
            totalMovesVerified: 50,
            consensusCount: 42,
            consensusAttempted: 50,
            vcfAttempted: 100,
            vcfTriggered: 30,
            vcfSolvedCount: 5,
            consensusRate: 0.84,
            duration: TimeSpan.FromMinutes(5));

        // Assert
        Assert.Single(summary.VerifiedMoves);
        Assert.Equal(100, summary.TotalPositionsProcessed);
        Assert.Equal(20, summary.FilteredLowPlayCount);
        Assert.Equal(15, summary.FilteredUnclearResult);
        Assert.Equal(10, summary.FilteredMoveWinRate);
        Assert.Equal(8, summary.FilteredScoreDelta);
        Assert.Equal(50, summary.TotalMovesVerified);
        Assert.Equal(42, summary.ConsensusCount);
        Assert.Equal(50, summary.ConsensusAttempted);
        Assert.Equal(100, summary.VcfAttempted);
        Assert.Equal(30, summary.VcfTriggered);
        Assert.Equal(5, summary.VcfSolvedCount);
        Assert.Equal(0.84, summary.ConsensusRate);
        Assert.Equal(TimeSpan.FromMinutes(5), summary.Duration);
    }

    [Fact]
    public void VerificationThresholds_AllExpectedValues()
    {
        // Act
        var thresholds = MoveVerifier.GetThresholds();

        // Assert - Verify all expected values (widened for yield optimization)
        Assert.Equal(512, thresholds.MinPlayCount);            // 2^9
        Assert.Equal(0.70, thresholds.MinWinRate, 2);          // 70% (raised from 62.5%)
        Assert.Equal(0.30, thresholds.MaxWinRateForLoss, 2);   // 30% (lowered from 37.5%)
        Assert.Equal(0.8125, thresholds.MinConsensusRate);     // 13/16
        Assert.Equal(768, thresholds.MaxScoreDelta);           // 2^9 + 2^8 cp (widened)
        Assert.Equal(384, thresholds.InclusionScoreDelta);     // 2^8 + 2^7 cp (widened)
        Assert.Equal(4, thresholds.MaxMovesPerPosition);       // 2^2
        Assert.Equal(1, thresholds.VcfTriggerThreats);         // Lowered from 2 to catch more
        Assert.Equal(4096, thresholds.DefaultVerificationTimeMs);  // 2^12 ms (quality-optimized)
        Assert.Equal(8192, thresholds.ExtendedVerificationTimeMs); // 2^13 ms (survival zone)
        Assert.Equal(128, thresholds.VcfTimeLimitMs);          // 2^7 ms
    }

    private static bool IsPowerOfTwo(int n)
    {
        return n > 0 && (n & (n - 1)) == 0;
    }
}
