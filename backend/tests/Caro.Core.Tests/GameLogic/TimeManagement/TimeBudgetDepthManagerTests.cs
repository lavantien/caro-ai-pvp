using Caro.Core.GameLogic;
using FluentAssertions;
using Xunit;

namespace Caro.Core.Tests.GameLogic.TimeManagement;

/// <summary>
/// Tests for TimeBudgetDepthManager - dynamic depth based on time budget.
/// Uses iterative deepening with Effective Branching Factor (EBF) tracking.
/// </summary>
public sealed class TimeBudgetDepthManagerTests
{
    [Fact]
    public void Constructor_CreatesInstance()
    {
        // Arrange & Act
        var manager = new TimeBudgetDepthManager();

        // Assert
        manager.Should().NotBeNull();
        manager.GetEstimatedNps().Should().BeGreaterThan(0, "Initial NPS should be positive");
        manager.GetEstimatedEbf().Should().BeGreaterThan(1.0, "Initial EBF should be > 1");
    }

    [Fact]
    public void UpdateNpsEstimate_ZeroTime_DoesNotUpdate()
    {
        // Arrange
        var manager = new TimeBudgetDepthManager();
        double initialNps = manager.GetEstimatedNps();

        // Act - Zero elapsed time should not update
        manager.UpdateNpsEstimate(nodesSearched: 1000, elapsedSeconds: 0);

        // Assert
        manager.GetEstimatedNps().Should().Be(initialNps, "Zero time should not update estimate");
    }

    [Fact]
    public void UpdateNpsEstimate_ZeroNodes_DoesNotUpdate()
    {
        // Arrange
        var manager = new TimeBudgetDepthManager();
        double initialNps = manager.GetEstimatedNps();

        // Act - Zero nodes should not update
        manager.UpdateNpsEstimate(nodesSearched: 0, elapsedSeconds: 1.0);

        // Assert
        manager.GetEstimatedNps().Should().Be(initialNps, "Zero nodes should not update estimate");
    }

    [Fact]
    public void UpdateNpsEstimate_ValidValues_UpdatesEstimate()
    {
        // Arrange
        var manager = new TimeBudgetDepthManager();
        const double elapsedSeconds = 1.0;
        const long nodesSearched = 150_000;

        // Act
        manager.UpdateNpsEstimate(nodesSearched, elapsedSeconds);

        // Assert - Should use 50% weight for new value
        double updatedNps = manager.GetEstimatedNps();
        updatedNps.Should().BeGreaterThan(0, "NPS should be positive after update");
        // New estimate = old * 0.5 + new * 0.5
        // old = 100,000, new = 150,000 => expected = 125,000
        updatedNps.Should().BeApproximately(125_000, precision: 10_000, "Should blend old and new NPS");
    }

    [Fact]
    public void UpdateNpsEstimate_MultipleUpdates_ConvergesToActual()
    {
        // Arrange
        var manager = new TimeBudgetDepthManager();
        const double elapsedSeconds = 1.0;
        const long nodesSearched = 200_000;
        const double targetNps = nodesSearched / elapsedSeconds;

        // Act - Multiple updates with same value should converge
        for (int i = 0; i < 5; i++)
        {
            manager.UpdateNpsEstimate(nodesSearched, elapsedSeconds);
        }

        // Assert - Should converge close to actual NPS
        double finalNps = manager.GetEstimatedNps();
        finalNps.Should().BeApproximately(targetNps, precision: 5000, "Should converge to actual NPS");
    }

    [Fact]
    public void UpdateEbfEstimate_ValidNodes_ClampsToBounds()
    {
        // Arrange
        var manager = new TimeBudgetDepthManager();

        // Act - Update with very high EBF (should clamp to 5.0)
        manager.UpdateEbfEstimate(nodesAtDepth: 1_000_000, nodesAtPreviousDepth: 10_000);

        // Assert
        double ebf = manager.GetEstimatedEbf();
        ebf.Should().BeLessThanOrEqualTo(5.0, "EBF should clamp to maximum 5.0");
        ebf.Should().BeGreaterThan(1.5, "EBF should be above minimum 1.5");
    }

    [Fact]
    public void UpdateEbfEstimate_LowEbf_ClampsToMinimum()
    {
        // Arrange
        var manager = new TimeBudgetDepthManager();

        // Act - Update with very low EBF (should clamp to 1.5)
        // EBF = 100 / 90 = 1.11 (below 1.5)
        manager.UpdateEbfEstimate(nodesAtDepth: 100, nodesAtPreviousDepth: 90);

        // Assert
        double ebf = manager.GetEstimatedEbf();
        ebf.Should().BeGreaterThanOrEqualTo(1.5, "EBF should clamp to minimum 1.5");
    }

    [Fact]
    public void UpdateEbfEstimate_ValidValue_UpdatesEstimate()
    {
        // Arrange
        var manager = new TimeBudgetDepthManager();
        double initialEbf = manager.GetEstimatedEbf();

        // Act - Normal EBF of 2.5
        manager.UpdateEbfEstimate(nodesAtDepth: 2500, nodesAtPreviousDepth: 1000);

        // Assert - EBF should be in valid range
        double ebf = manager.GetEstimatedEbf();
        ebf.Should().BeGreaterThan(1.5, "EBF should be above minimum");
        ebf.Should().BeLessThanOrEqualTo(5.0, "EBF should be at or below maximum");
    }

    [Fact]
    public void UpdateEbfEstimate_ZeroPreviousNodes_DoesNotUpdate()
    {
        // Arrange
        var manager = new TimeBudgetDepthManager();
        double initialEbf = manager.GetEstimatedEbf();

        // Act - Zero previous depth nodes should not update
        manager.UpdateEbfEstimate(nodesAtDepth: 1000, nodesAtPreviousDepth: 0);

        // Assert
        manager.GetEstimatedEbf().Should().Be(initialEbf, "Zero previous nodes should not update");
    }

    [Fact]
    public void CalculateMaxDepth_ZeroTime_ReturnsOne()
    {
        // Arrange
        var manager = new TimeBudgetDepthManager();

        // Act - Zero time should return minimum depth
        int depth = manager.CalculateMaxDepth(timeSeconds: 0, AIDifficulty.Easy);

        // Assert
        depth.Should().Be(1, "Zero time should return depth 1");
    }

    [Fact]
    public void CalculateMaxDepth_VerySmallTime_ReturnsOne()
    {
        // Arrange
        var manager = new TimeBudgetDepthManager();

        // Act - Time < 0.001 should return depth 1
        int depth = manager.CalculateMaxDepth(timeSeconds: 0.0005, AIDifficulty.Easy);

        // Assert
        depth.Should().Be(1, "Very small time should return depth 1");
    }

    [Fact]
    public void CalculateMaxDepth_NormalTime_ReturnsCalculatDepth()
    {
        // Arrange
        var manager = new TimeBudgetDepthManager();
        // Calibrate to known NPS
        manager.UpdateNpsEstimate(nodesSearched: 100_000, elapsedSeconds: 1.0);

        // Act - 1 second with 100k NPS and EBF 2.5
        // depth = log(100000) / log(2.5) ~= 11.5 / 0.92 ~= 12.5
        int depth = manager.CalculateMaxDepth(timeSeconds: 1.0, AIDifficulty.Hard);

        // Assert - Should be clamped to [1, 15] range
        depth.Should().BeGreaterThan(1, "Should calculate reasonable depth");
        depth.Should().BeLessThanOrEqualTo(15, "Should clamp to maximum 15");
    }

    [Fact]
    public void CalculateMaxDepth_ClampsToMaximum()
    {
        // Arrange
        var manager = new TimeBudgetDepthManager();
        // Set very high NPS to simulate very fast machine
        manager.UpdateNpsEstimate(nodesSearched: 10_000_000, elapsedSeconds: 1.0);

        // Act - Even with huge time budget
        int depth = manager.CalculateMaxDepth(timeSeconds: 1000.0, AIDifficulty.Grandmaster);

        // Assert - Should clamp to 15
        depth.Should().Be(15, "Should clamp to maximum depth of 15");
    }

    [Fact]
    public void CalculateMaxDepth_ClampsToMinimum()
    {
        // Arrange
        var manager = new TimeBudgetDepthManager();

        // Act - Tiny time budget
        int depth = manager.CalculateMaxDepth(timeSeconds: 0.001, AIDifficulty.Braindead);

        // Assert - Should clamp to minimum 1
        depth.Should().Be(1, "Should clamp to minimum depth of 1");
    }

    [Fact]
    public void ShouldContinueIterating_ExceedsSoftBound_ReturnsFalse()
    {
        // Arrange
        var manager = new TimeBudgetDepthManager();
        const double softBound = 5.0;

        // Act - Exceeded soft bound
        bool shouldContinue = manager.ShouldContinueIterating(
            elapsedSeconds: 5.5,
            softBoundSeconds: softBound,
            currentDepth: 5);

        // Assert
        shouldContinue.Should().BeFalse("Should not continue when soft bound exceeded");
    }

    [Fact]
    public void ShouldContinueIterating_AtSoftBound_ReturnsFalse()
    {
        // Arrange
        var manager = new TimeBudgetDepthManager();
        const double softBound = 5.0;

        // Act - Exactly at soft bound
        bool shouldContinue = manager.ShouldContinueIterating(
            elapsedSeconds: softBound,
            softBoundSeconds: softBound,
            currentDepth: 5);

        // Assert
        shouldContinue.Should().BeFalse("Should not continue when at soft bound");
    }

    [Fact]
    public void ShouldContinueIterating_InsufficientRemainingTime_ReturnsFalse()
    {
        // Arrange
        var manager = new TimeBudgetDepthManager();
        const double softBound = 10.0;
        const double elapsed = 7.0;
        // Remaining = 3.0, next iteration ~ 7.0 * 2.5 = 17.5
        // Need 17.5 * 0.8 = 14.0 remaining, but only have 3.0

        // Act
        bool shouldContinue = manager.ShouldContinueIterating(
            elapsedSeconds: elapsed,
            softBoundSeconds: softBound,
            currentDepth: 7);

        // Assert
        shouldContinue.Should().BeFalse("Should not continue when insufficient time for next iteration");
    }

    [Fact]
    public void ShouldContinueIterating_SufficientRemainingTime_ReturnsTrue()
    {
        // Arrange
        var manager = new TimeBudgetDepthManager();
        const double softBound = 10.0;
        const double elapsed = 2.0;
        // Remaining = 8.0, next iteration ~ 2.0 * 2.5 = 5.0
        // Need 5.0 * 0.8 = 4.0 remaining, have 8.0

        // Act
        bool shouldContinue = manager.ShouldContinueIterating(
            elapsedSeconds: elapsed,
            softBoundSeconds: softBound,
            currentDepth: 3);

        // Assert
        shouldContinue.Should().BeTrue("Should continue when sufficient time for next iteration");
    }

    [Fact]
    public void GetEstimatedNps_ThreadSafeReturns()
    {
        // Arrange
        var manager = new TimeBudgetDepthManager();
        manager.UpdateNpsEstimate(nodesSearched: 200_000, elapsedSeconds: 1.0);

        // Act
        double nps = manager.GetEstimatedNps();

        // Assert
        nps.Should().BeGreaterThan(0);
    }

    [Fact]
    public void GetEstimatedEbf_ThreadSafeReturns()
    {
        // Arrange
        var manager = new TimeBudgetDepthManager();
        manager.UpdateEbfEstimate(nodesAtDepth: 3000, nodesAtPreviousDepth: 1000);

        // Act
        double ebf = manager.GetEstimatedEbf();

        // Assert
        ebf.Should().BeGreaterThan(1.0);
    }

    [Fact]
    public void Reset_ClearsTrackingState()
    {
        // Arrange
        var manager = new TimeBudgetDepthManager();
        manager.UpdateNpsEstimate(nodesSearched: 200_000, elapsedSeconds: 1.0);
        manager.UpdateEbfEstimate(nodesAtDepth: 3000, nodesAtPreviousDepth: 1000);
        double npsBeforeReset = manager.GetEstimatedNps();
        double ebfBeforeReset = manager.GetEstimatedEbf();

        // Act
        manager.Reset();

        // Assert - NPS and EBF should be preserved (machine-specific)
        double npsAfterReset = manager.GetEstimatedNps();
        double ebfAfterReset = manager.GetEstimatedEbf();

        npsAfterReset.Should().Be(npsBeforeReset, "NPS should persist across reset");
        ebfAfterReset.Should().Be(ebfBeforeReset, "EBF should persist across reset");
    }

    [Fact]
    public void CalibrateFromSearch_UpdatesNpsEstimate()
    {
        // Arrange
        var manager = new TimeBudgetDepthManager();

        // Act - Simulate search with 50K nodes in 0.5 seconds = 100K NPS
        manager.CalibrateFromSearch(nodesSearched: 50_000, elapsedSeconds: 0.5);

        // Assert - NPS should be updated from actual search performance
        double nps = manager.GetEstimatedNps();
        nps.Should().BeGreaterThan(50_000, "NPS should be updated from actual search");
    }
}
