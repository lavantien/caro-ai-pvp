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

        // Assert - All values should be powers of 2 or fractions based on powers of 2
        Assert.True(IsPowerOfTwo(thresholds.MinPlayCount), $"MinPlayCount {thresholds.MinPlayCount} is not power of 2");
        Assert.True(IsPowerOfTwo(thresholds.MaxScoreDelta), $"MaxScoreDelta {thresholds.MaxScoreDelta} is not power of 2");
        Assert.True(IsPowerOfTwo(thresholds.InclusionScoreDelta), $"InclusionScoreDelta {thresholds.InclusionScoreDelta} is not power of 2");
        Assert.True(IsPowerOfTwo(thresholds.MaxMovesPerPosition), $"MaxMovesPerPosition {thresholds.MaxMovesPerPosition} is not power of 2");
        Assert.True(IsPowerOfTwo(thresholds.VcfTriggerThreats), $"VcfTriggerThreats {thresholds.VcfTriggerThreats} is not power of 2");
        Assert.True(IsPowerOfTwo(thresholds.DefaultVerificationTimeMs), $"DefaultVerificationTimeMs {thresholds.DefaultVerificationTimeMs} is not power of 2");
        Assert.True(IsPowerOfTwo(thresholds.ExtendedVerificationTimeMs), $"ExtendedVerificationTimeMs {thresholds.ExtendedVerificationTimeMs} is not power of 2");
        Assert.True(IsPowerOfTwo(thresholds.VcfTimeLimitMs), $"VcfTimeLimitMs {thresholds.VcfTimeLimitMs} is not power of 2");

        // Verify fraction-based thresholds
        Assert.Equal(0.625, thresholds.MinWinRate);           // 5/8
        Assert.Equal(0.375, thresholds.MaxWinRateForLoss);    // 3/8
        Assert.Equal(0.8125, thresholds.MinConsensusRate);    // 13/16
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
    public void MinWinRate_IsFiveEighths()
    {
        var thresholds = MoveVerifier.GetThresholds();
        Assert.Equal(0.625, thresholds.MinWinRate);  // 5/8
    }

    [Fact]
    public void MaxWinRateForLoss_IsThreeEighths()
    {
        var thresholds = MoveVerifier.GetThresholds();
        Assert.Equal(0.375, thresholds.MaxWinRateForLoss);  // 3/8
    }

    [Fact]
    public void MinConsensusRate_IsThirteenSixteenths()
    {
        var thresholds = MoveVerifier.GetThresholds();
        Assert.Equal(0.8125, thresholds.MinConsensusRate);  // 13/16
    }

    [Fact]
    public void MaxScoreDelta_IsPowerOfTwo()
    {
        var thresholds = MoveVerifier.GetThresholds();
        Assert.Equal(512, thresholds.MaxScoreDelta);  // 2^9 centipawns
        Assert.True(IsPowerOfTwo(thresholds.MaxScoreDelta));
    }

    [Fact]
    public void InclusionScoreDelta_IsPowerOfTwo()
    {
        var thresholds = MoveVerifier.GetThresholds();
        Assert.Equal(256, thresholds.InclusionScoreDelta);  // 2^8 centipawns
        Assert.True(IsPowerOfTwo(thresholds.InclusionScoreDelta));
    }

    [Fact]
    public void MaxMovesPerPosition_IsPowerOfTwo()
    {
        var thresholds = MoveVerifier.GetThresholds();
        Assert.Equal(4, thresholds.MaxMovesPerPosition);  // 2^2
        Assert.True(IsPowerOfTwo(thresholds.MaxMovesPerPosition));
    }

    [Fact]
    public void VcfTriggerThreats_IsTwo()
    {
        var thresholds = MoveVerifier.GetThresholds();
        Assert.Equal(2, thresholds.VcfTriggerThreats);
    }

    [Fact]
    public void DefaultVerificationTimeMs_IsPowerOfTwo()
    {
        var thresholds = MoveVerifier.GetThresholds();
        Assert.Equal(2048, thresholds.DefaultVerificationTimeMs);  // 2^11 ms
        Assert.True(IsPowerOfTwo(thresholds.DefaultVerificationTimeMs));
    }

    [Fact]
    public void ExtendedVerificationTimeMs_IsPowerOfTwo()
    {
        var thresholds = MoveVerifier.GetThresholds();
        Assert.Equal(4096, thresholds.ExtendedVerificationTimeMs);  // 2^12 ms (survival zone)
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
            totalMovesVerified: 50,
            vcfSolvedCount: 5,
            consensusRate: 0.85,
            duration: TimeSpan.FromMinutes(5));

        // Assert
        Assert.Single(summary.VerifiedMoves);
        Assert.Equal(100, summary.TotalPositionsProcessed);
        Assert.Equal(20, summary.FilteredLowPlayCount);
        Assert.Equal(15, summary.FilteredUnclearResult);
        Assert.Equal(50, summary.TotalMovesVerified);
        Assert.Equal(5, summary.VcfSolvedCount);
        Assert.Equal(0.85, summary.ConsensusRate);
        Assert.Equal(TimeSpan.FromMinutes(5), summary.Duration);
    }

    [Fact]
    public void VerificationThresholds_AllExpectedValues()
    {
        // Act
        var thresholds = MoveVerifier.GetThresholds();

        // Assert - Verify all expected values from the plan
        Assert.Equal(512, thresholds.MinPlayCount);            // 2^9
        Assert.Equal(0.625, thresholds.MinWinRate);            // 5/8
        Assert.Equal(0.375, thresholds.MaxWinRateForLoss);     // 3/8
        Assert.Equal(0.8125, thresholds.MinConsensusRate);     // 13/16
        Assert.Equal(512, thresholds.MaxScoreDelta);           // 2^9 cp
        Assert.Equal(256, thresholds.InclusionScoreDelta);     // 2^8 cp
        Assert.Equal(4, thresholds.MaxMovesPerPosition);       // 2^2
        Assert.Equal(2, thresholds.VcfTriggerThreats);
        Assert.Equal(2048, thresholds.DefaultVerificationTimeMs);  // 2^11 ms
        Assert.Equal(4096, thresholds.ExtendedVerificationTimeMs); // 2^12 ms
        Assert.Equal(128, thresholds.VcfTimeLimitMs);          // 2^7 ms
    }

    private static bool IsPowerOfTwo(int n)
    {
        return n > 0 && (n & (n - 1)) == 0;
    }
}
