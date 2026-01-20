using Caro.Core.Entities;
using Caro.Core.Tournament;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.Tests.Tournament;

/// <summary>
/// Unit tests for statistical analysis functions.
/// Uses property-based testing where applicable and known-value tests for mathematical functions.
/// </summary>
public class StatisticalAnalyzerTests
{
    private readonly ITestOutputHelper _output;

    public StatisticalAnalyzerTests(ITestOutputHelper output)
    {
        _output = output;
    }

    public static TheoryData<int, int, int, double> LOS_Data =>
        new()
        {
            // Equal wins and losses = 50% LOS (no superiority)
            { 5, 5, 0, 0.5 },
            { 10, 10, 0, 0.5 },
            { 50, 50, 0, 0.5 },

            // Perfect record = ~100% LOS
            { 10, 0, 0, 1.0 },
            { 20, 0, 0, 1.0 },

            // Dominant but not perfect
            { 15, 5, 0, 0.97 },  // 75% win rate
            { 8, 2, 0, 0.95 },   // 80% win rate
        };

    [Theory]
    [MemberData(nameof(LOS_Data))]
    public void CalculateLOS_WithKnownValues_ReturnsExpectedLOS(
        int wins, int losses, int draws, double expectedLOS)
    {
        // Act
        var actualLOS = StatisticalAnalyzer.CalculateLOS(wins, losses, draws);

        // Assert - allow small tolerance for floating point
        Assert.InRange(actualLOS, expectedLOS - 0.05, expectedLOS + 0.05);
    }

    [Fact]
    public void CalculateLOS_MoreLossesThanWins_ReturnsLessThan50Percent()
    {
        // Arrange - weaker player loses more
        var wins = 3;
        var losses = 7;

        // Act
        var los = StatisticalAnalyzer.CalculateLOS(wins, losses);

        // Assert - LOS should be less than 50% when losing
        Assert.InRange(los, 0.0, 0.2);
    }

    public static TheoryData<int, int, int, double, double, double> EloWithCI_Data =>
        new()
        {
            // 50% win rate = 0 Elo difference
            { 10, 10, 0, 0, -80, 80 },

            // 75% win rate = ~190 Elo difference with tighter CI
            { 15, 5, 0, 190, 80, 300 },
        };

    [Theory]
    [MemberData(nameof(EloWithCI_Data))]
    public void CalculateEloWithCI_WithKnownValues_ReturnsExpectedRange(
        int wins, int losses, int draws,
        double expectedElo, double minCI, double maxCI)
    {
        // Act
        var (eloDiff, lowerCI, upperCI) = StatisticalAnalyzer.CalculateEloWithCI(wins, losses, draws);

        // Assert
        Assert.InRange(eloDiff, expectedElo - 80, expectedElo + 80);
        Assert.InRange(lowerCI, minCI - 80, minCI + 80);
        Assert.InRange(upperCI, maxCI - 80, maxCI + 80);
    }

    [Fact]
    public void CalculateEloWithCI_WithDraws_AccountsForDrawsCorrectly()
    {
        // Draw counts as half win for Elo calculation
        var (eloDiff, lowerCI, upperCI) = StatisticalAnalyzer.CalculateEloWithCI(
            wins: 5, losses: 3, draws: 4);  // 5 + 4/2 = 7 effective wins out of 12

        // Should be ~58% win rate = ~35 Elo
        Assert.InRange(eloDiff, 20, 60);
        Assert.True(lowerCI < eloDiff);
        Assert.True(upperCI > eloDiff);
    }

    [Fact]
    public void BinomialTestPValue_EqualWinRate_ReturnsHighPValue()
    {
        // Arrange - 50% win rate should not be statistically significant
        var wins = 25;
        var totalGames = 50;
        var expectedWinRate = 0.5;

        // Act
        var pValue = StatisticalAnalyzer.BinomialTestPValue(wins, totalGames, expectedWinRate);

        // Assert - p-value should be high (not significant)
        Assert.InRange(pValue, 0.4, 1.0);
    }

    [Fact]
    public void BinomialTestPValue_DominantWinRate_ReturnsLowPValue()
    {
        // Arrange - 70% win rate is significantly different from 50%
        var wins = 35;
        var totalGames = 50;
        var expectedWinRate = 0.5;

        // Act
        var pValue = StatisticalAnalyzer.BinomialTestPValue(wins, totalGames, expectedWinRate);

        // Assert - p-value should be low (significant at p < 0.05)
        Assert.InRange(pValue, 0.0, 0.05);
    }

    [Fact]
    public void BinomialTestPValue_PerfectRecord_ReturnsVeryLowPValue()
    {
        // Perfect 10-0 record
        var pValue = StatisticalAnalyzer.BinomialTestPValue(10, 10, 0.5);

        // Should be extremely significant
        Assert.InRange(pValue, 0.0, 0.01);
    }

    [Fact]
    public void DetectColorAdvantage_NoAdvantage_ReturnsFalse()
    {
        // Arrange - Perfectly balanced 50/50 Red vs Blue wins
        // Alternating pattern: Red wins when first, Blue wins when first
        var results = new List<(bool isRed, Player winner)>
        {
            (true, Player.Red), (false, Player.Red),  // Red wins both positions
            (true, Player.Blue), (false, Player.Blue),  // Blue wins both positions
            (true, Player.Red), (false, Player.Red),
            (true, Player.Blue), (false, Player.Blue),
            (true, Player.Red), (false, Player.Red),
            (true, Player.Blue), (false, Player.Blue),
            (true, Player.Red), (false, Player.Red),
            (true, Player.Blue), (false, Player.Blue),
            (true, Player.Red), (false, Player.Red),
            (true, Player.Blue), (false, Player.Blue),
        };

        // Act
        var (hasAdvantage, effectSize, pValue) = StatisticalAnalyzer.DetectColorAdvantage(results);

        // Assert - No significant color advantage (10 Red, 10 Blue = 50/50)
        Assert.False(hasAdvantage);
        Assert.InRange(effectSize, -0.1, 0.1);
        Assert.True(pValue > 0.5);  // High p-value for balanced results
    }

    [Fact]
    public void DetectColorAdvantage_RedAdvantage_ReturnsTrue()
    {
        // Arrange - Red wins 50 out of 60 games (very strong signal)
        // Mix of positions but Red dominates overall
        var results = new List<(bool isRed, Player winner)>();
        for (int i = 0; i < 50; i++)
        {
            results.Add((true, Player.Red));  // Red wins as Red player
        }
        for (int i = 0; i < 10; i++)
        {
            results.Add((false, Player.Blue));  // Blue wins as Blue player (counted as Blue win)
        }

        // Act
        var (hasAdvantage, effectSize, pValue) = StatisticalAnalyzer.DetectColorAdvantage(results);

        // Assert - Red has strong advantage (50/60 = 83%)
        Assert.True(hasAdvantage);
        Assert.True(effectSize > 0.2);
        Assert.True(pValue < 0.05);
    }

    [Fact]
    public void DetectColorAdvantage_BlueAdvantage_ReturnsTrue()
    {
        // Arrange - Blue wins 50 out of 60 games (very strong signal)
        var results = new List<(bool isRed, Player winner)>();
        for (int i = 0; i < 50; i++)
        {
            results.Add((true, Player.Blue));  // Blue wins as Red player
        }
        for (int i = 0; i < 10; i++)
        {
            results.Add((false, Player.Red));  // Red wins as Blue player (counted as Red win)
        }

        // Act
        var (hasAdvantage, effectSize, pValue) = StatisticalAnalyzer.DetectColorAdvantage(results);

        // Assert - Blue has strong advantage (10/60 = 17% Red = 83% Blue)
        Assert.True(hasAdvantage);
        Assert.True(effectSize < -0.2);
        Assert.True(pValue < 0.05);
    }

    [Fact]
    public void SPRT_EarlyTermination_H1Accepted()
    {
        // SPRT should accept H1 (significant difference) with dominant results
        var sprtResult = StatisticalAnalyzer.SPRT(
            wins: 50, losses: 10, draws: 5,
            elo0: 0, elo1: 50);

        // Should accept H1 (true superiority)
        Assert.Equal(StatisticalAnalyzer.SPRTResult.H1, sprtResult);
    }

    [Fact]
    public void SPRT_EarlyTermination_H0Accepted()
    {
        // SPRT should accept H0 when results clearly favor H0 (lower win rate than expected)
        // Use slightly worse results to push LLR negative enough
        var sprtResult = StatisticalAnalyzer.SPRT(
            wins: 60, losses: 100, draws: 10,
            elo0: 0, elo1: 50);

        // Should accept H0 (no significant difference - results are worse than H0 expects)
        Assert.Equal(StatisticalAnalyzer.SPRTResult.H0, sprtResult);
    }

    [Fact]
    public void SPRT_InsufficientData_ReturnsContinue()
    {
        // Too few games to make a decision
        var sprtResult = StatisticalAnalyzer.SPRT(
            wins: 3, losses: 2, draws: 0,
            elo0: 0, elo1: 50);

        // Should continue collecting data
        Assert.Equal(StatisticalAnalyzer.SPRTResult.Continue, sprtResult);
    }

    [Fact]
    public void SPRT_LargeSampleSize_DecidesOnH1()
    {
        // With enough evidence, even small advantages become significant
        var sprtResult = StatisticalAnalyzer.SPRT(
            wins: 60, losses: 30, draws: 10,
            elo0: 0, elo1: 50);

        // ~65% win rate over 100 games should be significant
        Assert.Equal(StatisticalAnalyzer.SPRTResult.H1, sprtResult);
    }

    public static TheoryData<int, int, int> LOS_Range_Data =>
        new()
        {
            { 1, 0, 0 },  // Single win
            { 0, 1, 0 },  // Single loss
            { 50, 0, 0 },  // 50 wins
            { 0, 50, 0 },  // 50 losses
            { 10, 10, 10 },  // With draws
            { 25, 25, 0 },  // Even
            { 99, 1, 0 },  // Extreme
            { 0, 0, 10 },  // All draws
        };

    [Theory]
    [MemberData(nameof(LOS_Range_Data))]
    public void CalculateLOS_AlwaysReturnsBetween0And1(int wins, int losses, int draws)
    {
        // Act
        var los = StatisticalAnalyzer.CalculateLOS(wins, losses, draws);

        // Assert - LOS should always be between 0 and 1
        Assert.InRange(los, 0.0, 1.0);
    }

    public static TheoryData<int, int, int> EloCI_Data =>
        new()
        {
            { 10, 0, 0 }, { 5, 5, 0 }, { 1, 0, 0 }, { 50, 50, 0 }, { 20, 10, 5 },
        };

    [Theory]
    [MemberData(nameof(EloCI_Data))]
    public void CalculateEloWithCI_UpperCIAlwaysGreaterThanLowerCI(int wins, int losses, int draws)
    {
        // Act
        var (_, lowerCI, upperCI) = StatisticalAnalyzer.CalculateEloWithCI(wins, losses, draws);

        // Assert - Upper CI should always be >= Lower CI
        Assert.True(upperCI >= lowerCI, $"Upper CI ({upperCI}) should be >= Lower CI ({lowerCI})");
    }

    [Theory]
    [InlineData(10)]
    [InlineData(20)]
    [InlineData(30)]
    [InlineData(50)]
    public void CalculateEloWithCI_SymmetricResults_ReturnsNearZeroElo(int games)
    {
        // Arrange - Perfectly balanced results
        var wins = games / 2;
        var losses = games / 2;
        var draws = games % 2;

        // Act
        var (eloDiff, _, _) = StatisticalAnalyzer.CalculateEloWithCI(wins, losses, draws);

        // Assert - Near-zero Elo difference for balanced results
        Assert.InRange(eloDiff, -100, 100);
    }
}
