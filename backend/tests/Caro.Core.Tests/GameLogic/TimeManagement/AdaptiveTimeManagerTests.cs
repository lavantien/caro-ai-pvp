using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic.TimeManagement;

namespace Caro.Core.Tests.GameLogic.TimeManagement;

public class AdaptiveTimeManagerTests
{
    private static Board CreateEmptyBoard() => new();

    [Fact]
    public void CalculateMoveTime_Opening_ShouldReturnReasonableAllocation()
    {
        var manager = new AdaptiveTimeManager();
        var board = CreateEmptyBoard();

        var allocation = manager.CalculateMoveTime(
            timeRemainingMs: 420_000, moveNumber: 1, candidateCount: 30,
            board, Player.Red, initialTimeSeconds: 420, incrementSeconds: 5);

        allocation.SoftBoundMs.Should().BeGreaterThan(0);
        allocation.HardBoundMs.Should().BeGreaterThanOrEqualTo(allocation.SoftBoundMs);
        allocation.OptimalTimeMs.Should().BeLessThanOrEqualTo(allocation.SoftBoundMs);
        allocation.Phase.Should().Be(GamePhase.Opening);
    }

    [Fact]
    public void CalculateMoveTime_PhaseDetection_ShouldMatchMoveNumber()
    {
        var manager = new AdaptiveTimeManager();
        var board = CreateEmptyBoard();

        var opening = manager.CalculateMoveTime(420_000, 5, 30, board, Player.Red);
        opening.Phase.Should().Be(GamePhase.Opening);

        manager.Reset();
        var earlyMid = manager.CalculateMoveTime(420_000, 15, 30, board, Player.Red);
        earlyMid.Phase.Should().Be(GamePhase.EarlyMid);

        manager.Reset();
        var lateMid = manager.CalculateMoveTime(420_000, 35, 30, board, Player.Red);
        lateMid.Phase.Should().Be(GamePhase.LateMid);

        manager.Reset();
        var endgame = manager.CalculateMoveTime(420_000, 50, 30, board, Player.Red);
        endgame.Phase.Should().Be(GamePhase.Endgame);
    }

    [Fact]
    public void CalculateMoveTime_SuddenDeath_ShouldBeMoreConservative()
    {
        var manager = new AdaptiveTimeManager();
        var board = CreateEmptyBoard();

        var withIncrement = manager.CalculateMoveTime(
            420_000, 10, 30, board, Player.Red,
            initialTimeSeconds: 420, incrementSeconds: 5);

        manager.Reset();
        var suddenDeath = manager.CalculateMoveTime(
            420_000, 10, 30, board, Player.Red,
            initialTimeSeconds: 420, incrementSeconds: 0);

        // Sudden death should allocate less time per move
        suddenDeath.SoftBoundMs.Should().BeLessThan(withIncrement.SoftBoundMs);
    }

    [Fact]
    public void CalculateMoveTime_TimeScramble_ShouldBeDetected()
    {
        var manager = new AdaptiveTimeManager();
        var board = CreateEmptyBoard();

        var allocation = manager.CalculateMoveTime(
            timeRemainingMs: 5_000, moveNumber: 20, candidateCount: 30,
            board, Player.Red, initialTimeSeconds: 420, incrementSeconds: 5);

        // Should produce very small allocations
        allocation.SoftBoundMs.Should().BeLessThan(5_000);
        allocation.HardBoundMs.Should().BeLessThan(5_000);
    }

    [Fact]
    public void CalculateMoveTime_Emergency_ShouldTriggerWhenTimeLow()
    {
        var manager = new AdaptiveTimeManager();
        var board = CreateEmptyBoard();

        // Under 5% of initial time (420s * 1000 * 0.05 = 21000ms) but also < 2000ms threshold
        var allocation = manager.CalculateMoveTime(
            timeRemainingMs: 1_500, moveNumber: 30, candidateCount: 30,
            board, Player.Red, initialTimeSeconds: 420, incrementSeconds: 5);

        allocation.IsEmergency.Should().BeTrue();
    }

    [Fact]
    public void CalculateMoveTime_Emergency_ShouldNotTriggerWithPlentyOfTime()
    {
        var manager = new AdaptiveTimeManager();
        var board = CreateEmptyBoard();

        var allocation = manager.CalculateMoveTime(
            timeRemainingMs: 300_000, moveNumber: 5, candidateCount: 30,
            board, Player.Red, initialTimeSeconds: 420, incrementSeconds: 5);

        allocation.IsEmergency.Should().BeFalse();
    }

    [Fact]
    public void ReportTimeUsed_Timeout_ShouldReduceMultiplier()
    {
        var manager = new AdaptiveTimeManager();
        var board = CreateEmptyBoard();

        manager.CalculateMoveTime(420_000, 1, 30, board, Player.Red);
        manager.ReportTimeUsed(actualTimeMs: 10_000, allocatedMs: 5_000, timedOut: true);

        var debug = manager.GetDebugInfo();
        debug.multiplier.Should().BeLessThan(1.0);
    }

    [Fact]
    public void ReportTimeUsed_UnderBudget_ShouldIncreaseMultiplier()
    {
        var manager = new AdaptiveTimeManager();
        var board = CreateEmptyBoard();

        manager.CalculateMoveTime(420_000, 1, 30, board, Player.Red);
        manager.ReportTimeUsed(actualTimeMs: 1_000, allocatedMs: 5_000, timedOut: false);

        var debug = manager.GetDebugInfo();
        debug.multiplier.Should().BeGreaterThan(1.0);
    }

    [Fact]
    public void ReportTimeUsed_OverBudget_ShouldReduceMultiplier()
    {
        var manager = new AdaptiveTimeManager();
        var board = CreateEmptyBoard();

        manager.CalculateMoveTime(420_000, 1, 30, board, Player.Red);
        var beforeMultiplier = manager.GetDebugInfo().multiplier;
        manager.ReportTimeUsed(actualTimeMs: 4_800, allocatedMs: 5_000, timedOut: false);

        var debug = manager.GetDebugInfo();
        // Over 90% budget → multiplier reduced by 5%
        debug.multiplier.Should().BeLessThan(beforeMultiplier);
    }

    [Fact]
    public void Reset_ShouldClearAllState()
    {
        var manager = new AdaptiveTimeManager();
        var board = CreateEmptyBoard();

        manager.CalculateMoveTime(420_000, 1, 30, board, Player.Red);
        manager.ReportTimeUsed(5_000, 5_000, false);
        manager.CalculateMoveTime(415_000, 2, 30, board, Player.Red);
        manager.Reset();

        var debug = manager.GetDebugInfo();
        debug.multiplier.Should().Be(1.0);
        debug.pressure.Should().Be(0);
        debug.moves.Should().Be(0);
    }

    [Fact]
    public void CalculateMoveTime_Complexity_ShouldVaryWithCandidateCount()
    {
        var manager = new AdaptiveTimeManager();
        var board = CreateEmptyBoard();

        var fewCandidates = manager.CalculateMoveTime(
            420_000, 10, 10, board, Player.Red, 420, 5);

        manager.Reset();
        var manyCandidates = manager.CalculateMoveTime(
            420_000, 10, 60, board, Player.Red, 420, 5);

        // More candidates means more complexity -> higher time allocation
        manyCandidates.ComplexityMultiplier.Should().BeGreaterThan(fewCandidates.ComplexityMultiplier);
    }

    [Fact]
    public void CalculateMoveTime_Bounds_ShouldSatisfyInvariants()
    {
        var manager = new AdaptiveTimeManager();
        var board = CreateEmptyBoard();

        var allocation = manager.CalculateMoveTime(
            420_000, 10, 30, board, Player.Red, 420, 5);

        // Optimal <= SoftBound <= HardBound
        allocation.OptimalTimeMs.Should().BeLessThanOrEqualTo(allocation.SoftBoundMs);
        allocation.SoftBoundMs.Should().BeLessThanOrEqualTo(allocation.HardBoundMs);
        // HardBound should not exceed remaining time
        allocation.HardBoundMs.Should().BeLessThan(420_000);
    }

    [Fact]
    public void CalculateMoveTime_SuddenDeathEmergency_ShouldBeConservative()
    {
        var manager = new AdaptiveTimeManager();
        var board = CreateEmptyBoard();

        // Sudden death with very little time (< emergencyThreshold = max(2000, 60000/20=3000))
        var allocation = manager.CalculateMoveTime(
            timeRemainingMs: 2_000, moveNumber: 30, candidateCount: 30,
            board, Player.Red, initialTimeSeconds: 60, incrementSeconds: 0);

        allocation.IsEmergency.Should().BeTrue();
        allocation.SoftBoundMs.Should().BeLessThan(2_000);
    }

    [Fact]
    public void GetDebugInfo_ShouldReturnCurrentState()
    {
        var manager = new AdaptiveTimeManager();

        var (multiplier, pressure, moves) = manager.GetDebugInfo();

        multiplier.Should().Be(1.0);
        pressure.Should().Be(0);
        moves.Should().Be(0);
    }
}
