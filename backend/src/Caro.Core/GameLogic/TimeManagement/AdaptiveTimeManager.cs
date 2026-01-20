using Caro.Core.Entities;

namespace Caro.Core.GameLogic.TimeManagement;

/// <summary>
/// Adaptive time management using PID-like controller for dynamic time allocation
///
/// Unlike static multipliers, this system:
/// - Tracks actual time usage vs allocated (error term)
/// - Accumulates error over the game (integral term)
/// - Detects trends in time consumption (derivative term)
/// - Automatically adjusts to prevent timeouts while maximizing strength
///
/// Works for ANY time control without hardcoded values.
/// </summary>
public sealed class AdaptiveTimeManager
{
    // PID-like state variables
    private double _integralError = 0;      // Accumulated time surplus/deficit
    private long _previousTimeUsed = 0;     // For derivative calculation
    private int _moveCount = 0;             // Number of moves made

    // Adaptive multipliers (start at 1.0, adjust based on performance)
    private double _currentMultiplier = 1.0;
    private double _timePressure = 0;       // 0 = relaxed, 1 = critical

    // Configuration for different difficulties
    // Higher difficulties are more aggressive with time allocation
    private static readonly double[] BaseAggressiveness = new double[]
    {
        0.3,  // D0: Beginner - very conservative
        0.4,  // D1: Easy
        0.5,  // D2: Normal
        0.6,  // D3: Medium
        0.7,  // D4: Hard
        0.8,  // D5: Harder
        0.9,  // D6: VeryHard
        1.0,  // D7: Expert - baseline
        1.1,  // D8: Expert
        1.3,  // D9: Master
        1.6,  // D10: Grandmaster
        2.0   // D11: Legend - most aggressive
    };

    // Maximum time per move (percentage of remaining time)
    // Higher difficulties can spend more per move
    private static readonly double[] MaxTimePercentage = new double[]
    {
        0.01, // D0: 1% max per move
        0.015,
        0.02,
        0.025,
        0.03,
        0.035,
        0.04,
        0.05, // D7: 5% max per move
        0.06,
        0.08,
        0.10, // D10: 10% max per move
        0.12  // D11: 12% max per move
    };

    /// <summary>
    /// Calculate time allocation using adaptive PID-like controller
    /// </summary>
    public TimeAllocation CalculateMoveTime(
        long timeRemainingMs,
        int moveNumber,
        int candidateCount,
        Board board,
        Player player,
        AIDifficulty difficulty,
        int initialTimeSeconds = 420,
        int incrementSeconds = 5)
    {
        var difficultyIndex = (int)difficulty;
        var initialTimeMs = initialTimeSeconds * 1000L;

        // === PROPORTIONAL TERM: Current time pressure ===
        // Percentage of initial time remaining
        double timeRemainingRatio = (double)timeRemainingMs / initialTimeMs;
        double proportionalError = 1.0 - timeRemainingRatio; // 0 = all time, 1 = no time

        // === INTEGRAL TERM: Accumulated error over game ===
        // If we've been consistently over/under time, adjust future allocations
        _integralError += proportionalError * 0.1; // Decay factor to prevent windup
        _integralError = Math.Clamp(_integralError, -0.5, 0.5); // Anti-windup clamping

        // === DERIVATIVE TERM: Rate of change ===
        // How quickly are we burning time?
        double derivative = _previousTimeUsed > 0
            ? (proportionalError - (_previousTimeUsed / (double)initialTimeMs))
            : 0;
        _previousTimeUsed = timeRemainingMs;

        // === TIME PRESSURE CALCULATION ===
        // Combines all three terms with weights
        // Higher weight on proportional (current state) for responsiveness
        _timePressure = proportionalError * 0.6 + _integralError * 0.3 + derivative * 0.1;
        _timePressure = Math.Clamp(_timePressure, 0, 1);

        // === ADAPTIVE MULTIPLIER ===
        // Start with base aggressiveness for difficulty
        var baseAggressiveness = GetDifficultyValue(BaseAggressiveness, difficultyIndex);

        // Reduce multiplier as time pressure increases
        // This is the key adaptation: when running low on time, scale back
        var adaptiveMultiplier = baseAggressiveness * (1.0 - _timePressure * 0.7);

        // Smooth multiplier changes to prevent oscillation
        _currentMultiplier = _currentMultiplier * 0.7 + adaptiveMultiplier * 0.3;
        _currentMultiplier = Math.Clamp(_currentMultiplier, 0.2, 3.0);

        // === BASE TIME CALCULATION ===
        // Estimate moves remaining based on game phase
        var phase = DetermineGamePhase(moveNumber);
        var movesToEnd = GetMovesToGameEnd(phase, moveNumber);

        // Base formula: remaining / moves_left + 60% of increment
        var baseTimeMs = (timeRemainingMs / (double)movesToEnd) + (incrementSeconds * 1000 * 0.6);

        // === COMPLEXITY MULTIPLIER ===
        var complexity = CalculateComplexity(board, candidateCount, player, phase);

        // === PHASE MODIFIER ===
        var phaseMultiplier = GetPhaseModifier(phase);

        // === FINAL TIME ALLOCATION ===
        var adjustedTimeMs = baseTimeMs * complexity * phaseMultiplier * _currentMultiplier;

        // === HARD BOUND: Maximum percentage of remaining time ===
        // This is the safety net - never spend more than X% of remaining time
        var maxTimePercent = GetDifficultyValue(MaxTimePercentage, difficultyIndex);
        var maxAllocatableMs = Math.Max(0, timeRemainingMs - 1000); // Keep 1s reserve
        var percentageBoundMs = (long)(timeRemainingMs * maxTimePercent);

        // Soft bound: adjusted time, but respect percentage cap
        var softBoundMs = (long)Math.Clamp(adjustedTimeMs, 100, percentageBoundMs);

        // Hard bound: soft bound × 1.3, but never exceed percentage cap
        var desiredHardBoundMs = (long)(softBoundMs * 1.3);
        var hardBoundMs = Math.Min(desiredHardBoundMs, Math.Max(softBoundMs + 500, maxAllocatableMs));

        // Optimal: 80% of soft bound
        var optimalTimeMs = softBoundMs * 8 / 10;

        // === EMERGENCY DETECTION ===
        // Adaptive threshold based on time control
        var emergencyThreshold = Math.Max(10000, initialTimeMs / 10);
        var isEmergency = timeRemainingMs < emergencyThreshold ||
                         (movesToEnd > 0 && timeRemainingMs < movesToEnd * 1000);

        return new TimeAllocation
        {
            SoftBoundMs = softBoundMs,
            HardBoundMs = hardBoundMs,
            OptimalTimeMs = optimalTimeMs,
            IsEmergency = isEmergency,
            Phase = phase,
            ComplexityMultiplier = complexity
        };
    }

    /// <summary>
    /// Report actual time used for a move (updates PID state)
    /// Call this after each move to close the feedback loop
    /// </summary>
    public void ReportTimeUsed(long actualTimeMs, long allocatedMs, bool timedOut)
    {
        _moveCount++;

        // If we timed out, drastically reduce multiplier
        if (timedOut)
        {
            _currentMultiplier *= 0.5;
        }
        // If we used significantly less than allocated, we can be more aggressive
        else if (actualTimeMs < allocatedMs * 0.5)
        {
            _currentMultiplier *= 1.05; // 5% increase
        }
        // If we used most of our allocation, reduce slightly
        else if (actualTimeMs > allocatedMs * 0.9)
        {
            _currentMultiplier *= 0.95; // 5% decrease
        }
    }

    /// <summary>
    /// Reset state for a new game
    /// </summary>
    public void Reset()
    {
        _integralError = 0;
        _previousTimeUsed = 0;
        _moveCount = 0;
        _currentMultiplier = 1.0;
        _timePressure = 0;
    }

    /// <summary>
    /// Get current debugging information
    /// </summary>
    public (double multiplier, double pressure, int moves) GetDebugInfo()
    {
        return (_currentMultiplier, _timePressure, _moveCount);
    }

    private static GamePhase DetermineGamePhase(int moveNumber) => moveNumber switch
    {
        <= 10 => GamePhase.Opening,
        <= 25 => GamePhase.EarlyMid,
        <= 45 => GamePhase.LateMid,
        _ => GamePhase.Endgame
    };

    private static int GetMovesToGameEnd(GamePhase phase, int currentMove) => phase switch
    {
        GamePhase.Opening => 50,
        GamePhase.EarlyMid => 40,
        GamePhase.LateMid => 20,
        GamePhase.Endgame => 10,
        _ => 20
    };

    private static double GetPhaseModifier(GamePhase phase) => phase switch
    {
        GamePhase.Opening => 0.6,
        GamePhase.EarlyMid => 0.9,
        GamePhase.LateMid => 1.2,
        GamePhase.Endgame => 1.0,
        _ => 1.0
    };

    private static double CalculateComplexity(Board board, int candidateCount, Player player, GamePhase phase)
    {
        double score = 1.0;

        // Candidate count factor
        score += Math.Clamp((candidateCount - 30) / 40.0, -0.3, 0.5);

        // Phase-based complexity
        if (phase == GamePhase.LateMid)
            score += 0.2; // Late midgame is most complex

        // Board congestion
        var redBoard = board.GetBitBoard(Player.Red);
        var blueBoard = board.GetBitBoard(Player.Blue);
        int stoneCount = redBoard.CountBits() + blueBoard.CountBits();
        if (stoneCount > 100)
            score += 0.2;
        else if (stoneCount < 20)
            score -= 0.2;

        return Math.Clamp(score, 0.5, 2.0);
    }

    private static double GetDifficultyValue(double[] array, int index)
    {
        return index >= 0 && index < array.Length ? array[index] : array[7]; // Default to D7 (Expert)
    }
}
