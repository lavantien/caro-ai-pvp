using Caro.Core.Domain.Entities;

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
    // Updated to match AIDifficulty enum values (1-7)
    // Index: 0=unused, 1=Braindead, 2=Easy, 3=Medium, 4=Hard, 5=Grandmaster, 6=Experimental, 7=BookGeneration
    // Higher difficulties are more aggressive with time allocation
    private static readonly double[] BaseAggressiveness = new double[]
    {
        1.0,  // [0] unused
        0.3,  // [1] Braindead: very conservative
        0.6,  // [2] Easy: moderate aggressiveness
        1.0,  // [3] Medium: standard aggressiveness
        1.5,  // [4] Hard: aggressive for parallel search
        2.5,  // [5] Grandmaster: very aggressive
        3.0,  // [6] Experimental: maximum aggressiveness
        3.0   // [7] BookGeneration: maximum aggressiveness
    };

    // Maximum time per move (percentage of remaining time)
    // These are SAFETY CAPS for time controls WITH increment
    // For sudden death (no increment), we use much lower caps
    // CRITICAL: These values are only used as upper bounds, not targets
    private static readonly double[] MaxTimePercentage = new double[]
    {
        0.10, // [0] unused
        0.05, // [1] Braindead: 5% max per move
        0.08, // [2] Easy: 8%
        0.10, // [3] Medium: 10%
        0.12, // [4] Hard: 12%
        0.15, // [5] Grandmaster: 15% - reduced from 40% which was too aggressive
        0.20, // [6] Experimental: 20%
        0.20  // [7] BookGeneration: 20%
    };

    // Maximum time per move for SUDDEN DEATH (no increment) time controls
    // Must be much more conservative since there's no time recovery
    // For a 60s game with ~50 expected moves, each move should average ~1.2s
    // So the max per move should be roughly 2% of remaining time
    private static readonly double[] MaxTimePercentageSuddenDeath = new double[]
    {
        0.02, // [0] unused
        0.01, // [1] Braindead: 1% max per move
        0.015, // [2] Easy: 1.5%
        0.02, // [3] Medium: 2%
        0.025, // [4] Hard: 2.5%
        0.03, // [5] Grandmaster: 3% - for 60s, this is ~1.8s max per move
        0.04, // [6] Experimental: 4%
        0.04  // [7] BookGeneration: 4%
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

        // Detect sudden death early (needed for adaptive multiplier)
        var incrementMs = incrementSeconds * 1000L;
        var isSuddenDeath = incrementMs <= 0 || incrementSeconds <= 0;

        // === ADAPTIVE MULTIPLIER ===
        // Start with base aggressiveness for difficulty
        var baseAggressiveness = GetDifficultyValue(BaseAggressiveness, difficultyIndex);

        // CRITICAL FIX: For sudden death, use much lower aggressiveness
        // In sudden death, we cannot afford to be aggressive with time usage
        if (isSuddenDeath)
        {
            // Reduce base aggressiveness significantly for sudden death
            // Maximum of 1.0 for any difficulty to prevent time burn
            baseAggressiveness = Math.Min(baseAggressiveness, 1.0);
        }

        // Reduce multiplier as time pressure increases
        // This is the key adaptation: when running low on time, scale back
        var adaptiveMultiplier = baseAggressiveness * (1.0 - _timePressure * 0.7);

        // Smooth multiplier changes to prevent oscillation
        _currentMultiplier = _currentMultiplier * 0.7 + adaptiveMultiplier * 0.3;

        // CRITICAL FIX: For sudden death, cap multiplier much more aggressively
        // This prevents the multiplier from growing and burning through time
        var maxMultiplier = isSuddenDeath ? 1.2 : 3.0;
        _currentMultiplier = Math.Clamp(_currentMultiplier, 0.2, maxMultiplier);

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
        // (isSuddenDeath and incrementMs already calculated above)

        // Choose appropriate percentage cap based on time control type
        var maxTimePercent = isSuddenDeath
            ? GetDifficultyValue(MaxTimePercentageSuddenDeath, difficultyIndex)
            : GetDifficultyValue(MaxTimePercentage, difficultyIndex);

        // Time scramble detection: different logic for sudden death vs increment
        // Sudden death: less than 20 seconds remaining is time scramble
        // With increment: less than 3x increment OR less than 30 seconds remaining
        var isInTimeScramble = isSuddenDeath
            ? timeRemainingMs < 20000
            : timeRemainingMs < Math.Min(incrementMs * 3, 30000);

        long maxAllocatableMs;
        long percentageBoundMs;

        // FIX: Time allotment formula - different for sudden death vs increment time controls
        // Sudden death: Must budget evenly across expected moves (no time recovery)
        // With increment: Can be more aggressive (time recovery each move)
        double baseTimeAllotMs;
        if (isSuddenDeath)
        {
            // SUDDEN DEATH FORMULA: remaining / expected_moves with safety factor
            // Expected moves in Caro: ~40-50 depending on phase
            // Use safety factor of 0.8 to ensure we don't run out
            var safetyFactor = 0.8;
            baseTimeAllotMs = (timeRemainingMs / (double)movesToEnd) * safetyFactor;
        }
        else
        {
            // INCREMENT FORMULA: More aggressive, can use more time per move
            // (initial_time / 25) + (increment * 1.5)
            // For 180+2: 180/25 + 2*1.5 = 7.2 + 3 = 10.2s max per move
            // For 300+3: 300/25 + 3*1.5 = 12 + 4.5 = 16.5s max per move
            baseTimeAllotMs = ((initialTimeSeconds / 25.0) + (incrementSeconds * 1.5)) * 1000.0;
        }

        // Cap complexity multiplier to prevent excessive time usage
        var complexityCap = Math.Min(complexity, 1.5);
        var timeAllotMaxMs = (long)(baseTimeAllotMs * complexityCap);

        // FIX: Safety scaling - when remaining time is low, scale back time allotment
        // This prevents burning through the clock early in the game
        if (timeRemainingMs < timeAllotMaxMs * 2)
        {
            // When remaining time < 2x our planned allotment, use 40% of remaining
            timeAllotMaxMs = (long)(timeRemainingMs * 0.4);
        }

        if (isInTimeScramble)
        {
            // In time scramble: CRITICAL - use very conservative time allocation
            if (isSuddenDeath)
            {
                // Sudden death time scramble: budget for 5 more moves minimum
                maxAllocatableMs = Math.Max(timeRemainingMs / 6, 300); // ~17% of remaining, min 300ms
            }
            else
            {
                // With increment: spend less than increment per move
                // Use 40% of increment as max (leaves 60% safety margin)
                // For 2 second increment: max 800ms per move
                // For 5 second increment: max 2000ms per move
                maxAllocatableMs = Math.Max(incrementMs * 2 / 5, 300);
            }
            percentageBoundMs = maxAllocatableMs;
        }
        else
        {
            // Normal case: keep 1s reserve from remaining time
            maxAllocatableMs = Math.Max(0, timeRemainingMs - 1000);
            percentageBoundMs = (long)(timeRemainingMs * maxTimePercent);

            // Apply time allot formula cap to prevent burning through the clock
            percentageBoundMs = Math.Min(percentageBoundMs, timeAllotMaxMs);
        }

        // Soft bound: adjusted time, but respect percentage cap
        // Use 1% of percentageBoundMs as minimum to handle edge cases where percentageBoundMs is very small
        var softBoundMs = (long)Math.Clamp(adjustedTimeMs, Math.Max(1, percentageBoundMs / 100), percentageBoundMs);

        // Hard bound: soft bound × 1.3, but never exceed percentage cap
        // In time scramble, use very tight bounds
        var desiredHardBoundMs = (long)(softBoundMs * 1.3);
        if (isInTimeScramble)
        {
            if (isSuddenDeath)
            {
                // Sudden death time scramble: cap at percentage of remaining
                desiredHardBoundMs = Math.Min(desiredHardBoundMs, (long)(timeRemainingMs * 0.20));
            }
            else
            {
                // With increment: cap at 50% of increment
                desiredHardBoundMs = Math.Min(desiredHardBoundMs, incrementMs / 2);
            }
        }
        else
        {
            // Cap hard bound at time allot formula
            desiredHardBoundMs = Math.Min(desiredHardBoundMs, timeAllotMaxMs);
        }
        var hardBoundMs = Math.Min(desiredHardBoundMs, Math.Max(softBoundMs + 100, maxAllocatableMs));

        // Optimal: 80% of soft bound
        var optimalTimeMs = softBoundMs * 8 / 10;

        // === EMERGENCY DETECTION ===
        // Adaptive threshold based on time control - scales with initial time
        // Use 5% of initial time as minimum, but at least 2 seconds
        // This ensures emergency mode doesn't trigger too early in fast time controls
        var emergencyThreshold = Math.Max(2000, initialTimeMs / 20);
        // Second condition: only trigger if we have less than 1 second per move remaining
        // But only apply this when we have very few moves left (last 5 moves of the game)
        var isEmergency = timeRemainingMs < emergencyThreshold ||
                         (movesToEnd > 0 && movesToEnd <= 5 && timeRemainingMs < movesToEnd * 1000);

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
        // Arrays are indexed by AIDifficulty enum value directly
        // Braindead=1, Easy=2, Medium=3, Hard=4, Grandmaster=5, Experimental=6, BookGeneration=7
        // Index 0 is unused
        return index >= 0 && index < array.Length ? array[index] : array[array.Length - 1];
    }
}
