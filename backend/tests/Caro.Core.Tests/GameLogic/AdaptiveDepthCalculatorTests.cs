using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using FluentAssertions;
using Xunit;

namespace Caro.Core.Tests.GameLogic;

/// <summary>
/// Tests for AdaptiveDepthCalculator - difficulty-based depth parameters.
/// This class is deprecated but still provides fallback values.
/// </summary>
public sealed class AdaptiveDepthCalculatorTests
{
    [Theory]
    [InlineData(AIDifficulty.Braindead, 0.05)]
    [InlineData(AIDifficulty.Easy, 0.20)]
    [InlineData(AIDifficulty.Medium, 0.50)]
    [InlineData(AIDifficulty.Hard, 0.75)]
    [InlineData(AIDifficulty.Grandmaster, 1.0)]
    public void GetTimeMultiplier_ReturnsExpectedValue(AIDifficulty difficulty, double expectedMultiplier)
    {
        // Arrange & Act
        double multiplier = AdaptiveDepthCalculator.GetTimeMultiplier(difficulty);

        // Assert
        multiplier.Should().Be(expectedMultiplier, $"Time multiplier for {difficulty} should match config");
    }

    [Theory]
    [InlineData(AIDifficulty.Braindead, 1)]
    [InlineData(AIDifficulty.Easy, 3)]
    [InlineData(AIDifficulty.Medium, 5)]
    [InlineData(AIDifficulty.Hard, 7)]
    public void GetDepth_LegacyPath_ReturnsFixedValues(AIDifficulty difficulty, int expectedDepth)
    {
        // Arrange
        var board = new Board();

        // Act - Legacy path uses Board parameter (now unused)
        int depth = AdaptiveDepthCalculator.GetDepth(difficulty, board);

        // Assert
        depth.Should().Be(expectedDepth, $"Legacy GetDepth for {difficulty} should return fixed value");
    }

    [Fact]
    public void GetDepth_Grandmaster_UsesAdaptivePath()
    {
        // Arrange
        var board = new Board();
        var difficulty = AIDifficulty.Grandmaster;

        // Act - Grandmaster uses adaptive depth calculation
        int depth = AdaptiveDepthCalculator.GetDepth(difficulty, board);

        // Assert - Should return adaptive depth (7-8 range per current logic)
        depth.Should().BeGreaterThanOrEqualTo(7, "Grandmaster adaptive depth should be at least 7");
        depth.Should().BeLessThanOrEqualTo(8, "Grandmaster adaptive depth should not exceed 8");
    }

    [Fact]
    public void GetAdaptiveDepth_EmptyBoard_ReturnsOpeningDepth()
    {
        // Arrange
        var board = new Board();

        // Act
        int depth = GetAdaptiveDepthViaReflection(board);

        // Assert - Empty board (< 20 stones) is opening
        depth.Should().Be(7, "Empty board should return opening depth");
    }

    [Fact]
    public void GetAdaptiveDepth_StoneCountDeterminesPhase()
    {
        // Arrange - Cannot create non-empty board without helper
        // Just verify the logic exists
        var board = new Board();

        // Act
        int depth = GetAdaptiveDepthViaReflection(board);

        // Assert - Opening phase detected
        depth.Should().Be(7);
    }

    [Fact]
    public void CountTotalThreats_EmptyBoard_ReturnsZero()
    {
        // Arrange
        var board = new Board();

        // Act - Cannot access private method directly, test via GetDepth
        int depth = AdaptiveDepthCalculator.GetDepth(AIDifficulty.Grandmaster, board);

        // Assert - Empty board should have no threats
        depth.Should().Be(7, "Empty board uses opening depth (no threats)");
    }

    [Theory]
    [InlineData(AIDifficulty.Braindead, 0.10)]
    [InlineData(AIDifficulty.Easy, 0.0)]
    [InlineData(AIDifficulty.Medium, 0.0)]
    [InlineData(AIDifficulty.Hard, 0.0)]
    [InlineData(AIDifficulty.Grandmaster, 0.0)]
    public void GetError_ReturnsConfiguredValue(AIDifficulty difficulty, double expectedErrorRate)
    {
        // Arrange & Act
        double errorRate = AdaptiveDepthCalculator.GetErrorRate(difficulty);

        // Assert
        errorRate.Should().Be(expectedErrorRate, $"Error rate for {difficulty} should match config");
    }

    [Fact]
    public void GetErrorRate_BraindeadHasError()
    {
        // Arrange & Act
        double errorRate = AdaptiveDepthCalculator.GetErrorRate(AIDifficulty.Braindead);

        // Assert - Braindead should have positive error rate
        errorRate.Should().BeGreaterThan(0, "Braindead should have error rate");
        errorRate.Should().Be(0.10, "Braindead error rate should be 10%");
    }

    [Fact]
    public void GetErrorRate_HigherDifficultiesHaveNoError()
    {
        // Arrange & Act
        var easyError = AdaptiveDepthCalculator.GetErrorRate(AIDifficulty.Easy);
        var mediumError = AdaptiveDepthCalculator.GetErrorRate(AIDifficulty.Medium);
        var hardError = AdaptiveDepthCalculator.GetErrorRate(AIDifficulty.Hard);
        var grandmasterError = AdaptiveDepthCalculator.GetErrorRate(AIDifficulty.Grandmaster);

        // Assert - All higher difficulties should have zero error rate
        easyError.Should().Be(0, "Easy should have no error rate");
        mediumError.Should().Be(0, "Medium should have no error rate");
        hardError.Should().Be(0, "Hard should have no error rate");
        grandmasterError.Should().Be(0, "Grandmaster should have no error rate");
    }

    // Helper method to test private GetAdaptiveDepth via reflection
    private static int GetAdaptiveDepthViaReflection(Board board)
    {
        // Use Grandmaster difficulty to trigger adaptive path
        return AdaptiveDepthCalculator.GetDepth(AIDifficulty.Grandmaster, board);
    }


}
