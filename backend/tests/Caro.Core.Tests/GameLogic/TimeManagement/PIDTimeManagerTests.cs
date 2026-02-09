using Caro.Core.GameLogic.TimeManagement;

namespace Caro.Core.Tests.GameLogic.TimeManagement;

public class PIDTimeManagerTests
{
    [Fact]
    public void PIDTimeManager_OnTrack_SmallAdjustment()
    {
        // Arrange
        var manager = new PIDTimeManager(targetTimeMs: 1000, remainingMoves: 10);

        // Act - On track: remainingTime matches expected
        // expected = 1000 * 10 = 10000ms total budget
        // remaining = 10000ms (on track)
        double adjustment = manager.CalculateTimeAdjustment(remainingTimeMs: 10000, movesRemaining: 10);

        // Assert - Small adjustment when on track
        Assert.True(adjustment >= -50 && adjustment <= 50, $"Adjustment should be small when on track, got: {adjustment}");
    }

    [Fact]
    public void PIDTimeManager_BehindTarget_IncreasesAllocation()
    {
        // Arrange
        var manager = new PIDTimeManager(targetTimeMs: 1000, remainingMoves: 10);

        // Act - Behind target: less time remaining than expected
        // expected = 1000 * 10 = 10000ms total budget
        // remaining = 5000ms (behind by half)
        double adjustment = manager.CalculateTimeAdjustment(remainingTimeMs: 5000, movesRemaining: 10);

        // Assert - Positive adjustment to use more time
        Assert.True(adjustment > 0, $"Should increase allocation when behind, got: {adjustment}");
    }

    [Fact]
    public void PIDTimeManager_AheadTarget_DecreasesAllocation()
    {
        // Arrange
        var manager = new PIDTimeManager(targetTimeMs: 1000, remainingMoves: 10);

        // Act - Ahead target: more time remaining than expected
        // expected = 1000 * 10 = 10000ms total budget
        // remaining = 15000ms (ahead by half)
        double adjustment = manager.CalculateTimeAdjustment(remainingTimeMs: 15000, movesRemaining: 10);

        // Assert - Negative adjustment to conserve time
        Assert.True(adjustment < 0, $"Should decrease allocation when ahead, got: {adjustment}");
    }

    [Fact]
    public void PIDTimeManager_IntegralWindup_Clamped()
    {
        // Arrange
        var manager = new PIDTimeManager(targetTimeMs: 1000, remainingMoves: 10);

        // Act - Multiple iterations of being behind
        // This tests that the integral term doesn't wind up infinitely
        for (int i = 0; i < 100; i++)
        {
            manager.CalculateTimeAdjustment(remainingTimeMs: 5000, movesRemaining: 10);
        }

        // Act - Check one more adjustment
        double adjustment = manager.CalculateTimeAdjustment(remainingTimeMs: 5000, movesRemaining: 10);

        // Assert - Adjustment should be bounded to reasonable limits
        Assert.True(adjustment < 5000, $"Integral should be clamped, got: {adjustment}");
    }

    [Fact]
    public void PIDTimeManager_ZeroMovesRemaining_ReturnsZero()
    {
        // Arrange
        var manager = new PIDTimeManager(targetTimeMs: 1000, remainingMoves: 1);

        // Act - No moves remaining
        double adjustment = manager.CalculateTimeAdjustment(remainingTimeMs: 1000, movesRemaining: 0);

        // Assert
        Assert.Equal(0, adjustment);
    }

    [Fact]
    public void PIDTimeManager_NegativeRemainingTime_ClampsToZero()
    {
        // Arrange
        var manager = new PIDTimeManager(targetTimeMs: 1000, remainingMoves: 10);

        // Act - Negative remaining time (shouldn't happen but defensive)
        double adjustment = manager.CalculateTimeAdjustment(remainingTimeMs: -1000, movesRemaining: 10);

        // Assert - Should handle gracefully with maximum positive adjustment
        // (we treat negative time as zero, so we're very behind)
        Assert.True(adjustment > 0, $"Should request more time for negative remaining: {adjustment}");
        Assert.True(adjustment <= 3000, $"Should be bounded: {adjustment}");
    }

    [Fact]
    public void PIDTimeManager_VaryingMovesLeft_CalculatesCorrectly()
    {
        // Arrange
        var manager = new PIDTimeManager(targetTimeMs: 1000, remainingMoves: 20);

        // Act - Fewer moves remaining (more urgent situation)
        // With 5 moves left and 5000ms: expected=5000, on track (0 adjustment)
        // With 15 moves left and 5000ms: expected=15000, behind by 10000 (positive adjustment)
        double adjustment1 = manager.CalculateTimeAdjustment(remainingTimeMs: 5000, movesRemaining: 5);
        double adjustment2 = manager.CalculateTimeAdjustment(remainingTimeMs: 5000, movesRemaining: 15);

        // Assert - Fewer moves (more urgent) should get less or same as more moves
        // When on track with fewer moves, adjustment should be minimal
        Assert.True(adjustment1 <= adjustment2, $"Fewer moves on track should get less allocation: {adjustment1} vs {adjustment2}");
    }

    [Fact]
    public void PIDTimeManager_Reset_ClearsAccumulatedError()
    {
        // Arrange
        var manager = new PIDTimeManager(targetTimeMs: 1000, remainingMoves: 10);

        // Act - Build up some error
        manager.CalculateTimeAdjustment(remainingTimeMs: 5000, movesRemaining: 10);
        manager.CalculateTimeAdjustment(remainingTimeMs: 5000, movesRemaining: 10);
        
        // Reset
        manager.Reset();

        // Act - After reset, error should start fresh
        double adjustment = manager.CalculateTimeAdjustment(remainingTimeMs: 5000, movesRemaining: 10);

        // Assert - After reset, should calculate fresh adjustment
        // Note: The adjustment will be the same magnitude since inputs are the same
        Assert.True(adjustment > 0, $"After reset, should calculate fresh adjustment: {adjustment}");
        Assert.True(adjustment < 5000, $"Adjustment should be reasonable: {adjustment}");
    }
}
