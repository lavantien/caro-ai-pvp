using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic.TimeManagement;

/// <summary>
/// Intelligent time management for 7+5 time control
///
/// Uses chess-engine inspired time allocation:
/// - Base formula: remaining_time / moves_to_end + (increment × 0.6)
/// - Two-level time control: soft bound (optimal) + hard bound (maximum)
/// - Game phase awareness: opening/early-mid/late-mid/endgame modifiers
/// - Position complexity: 0.5x to 2.0x multiplier based on threats and candidates
/// - Emergency mode: panic when low on time
/// - Difficulty-based allocation: higher difficulties get more time per move
/// </summary>
public sealed class TimeManager
{
    private readonly ThreatDetector _threatDetector = new();

    // Track initial time for adaptive thresholds
    private long _inferredInitialTimeMs = 420000;  // Default to 7 minutes (7+5)

    // CRITICAL: Time multipliers for difficulty differentiation
    // Must be carefully balanced to avoid timeouts while creating separation
    // Grandmaster gets more time but not so much that it timeouts
    private static readonly Dictionary<AIDifficulty, double> DifficultyTimeMultipliers = new()
    {
        { AIDifficulty.Grandmaster, 1.3 }, // D5: 1.3x - maximum time
        { AIDifficulty.Hard, 1.0 },         // D4: 1.0x - baseline
        { AIDifficulty.Medium, 0.8 },       // D3: 0.8x
        { AIDifficulty.Easy, 0.6 },         // D2: 0.6x
        { AIDifficulty.Braindead, 0.4 },    // D1: 0.4x
    };

    /// <summary>
    /// Get adaptive difficulty multiplier based on time control
    /// The base multipliers are now conservative enough that we don't need aggressive reduction
    /// </summary>
    private static double GetAdaptiveDifficultyMultiplier(AIDifficulty difficulty, long initialTimeMs)
    {
        double baseMultiplier = DifficultyTimeMultipliers.GetValueOrDefault(difficulty, 1.0);

        // For very short time controls (< 3 minutes), use a modest reduction
        if (initialTimeMs < 180_000) // Less than 3 minutes
        {
            return baseMultiplier * 0.8;
        }

        return baseMultiplier;
    }

    /// <summary>
    /// Calculate time allocation for a move based on game state
    /// </summary>
    /// <param name="timeRemainingMs">Time remaining on clock in milliseconds</param>
    /// <param name="moveNumber">Current move number (1-indexed)</param>
    /// <param name="candidateCount">Number of candidate moves to consider</param>
    /// <param name="board">Current board position</param>
    /// <param name="player">Player to move</param>
    /// <param name="difficulty">AI difficulty level (affects time allocation)</param>
    /// <param name="initialTimeSeconds">Initial time control in seconds (for adaptive thresholds)</param>
    /// <param name="incrementSeconds">Time increment per move in seconds</param>
    /// <returns>Time allocation with soft/hard bounds and game phase info</returns>
    public TimeAllocation CalculateMoveTime(
        long timeRemainingMs,
        int moveNumber,
        int candidateCount,
        Board board,
        Player player,
        AIDifficulty difficulty = AIDifficulty.Medium,
        int initialTimeSeconds = 420,  // Default to 7+5, but tests may use different time controls
        int incrementSeconds = 5)       // Default increment for 7+5
    {
        // Validate inputs
        if (timeRemainingMs <= 0)
            return GetEmergencyAllocation(timeRemainingMs, GamePhase.Endgame, incrementSeconds);

        // Infer initial time on first move (move 1-3)
        // Assume timeRemainingMs ≈ initial time at game start
        if (moveNumber <= 3 && timeRemainingMs > _inferredInitialTimeMs * 0.9)
        {
            _inferredInitialTimeMs = timeRemainingMs;
        }

        // Determine game phase
        var phase = DetermineGamePhase(moveNumber);

        // Estimate moves remaining until game end
        int movesToEnd = GetMovesToGameEnd(phase, moveNumber);

        // Check for emergency mode FIRST (before any calculations)
        // Use adaptive threshold: 10% of initial time, or 10s minimum (for 7+5)
        long adaptiveEmergencyThreshold = Math.Max(10000, _inferredInitialTimeMs / 10);
        bool isEmergency = ShouldUsePanicMode(timeRemainingMs, movesToEnd, adaptiveEmergencyThreshold);
        if (isEmergency)
        {
            return GetEmergencyAllocation(timeRemainingMs, phase, incrementSeconds);
        }

        // Base time: remaining / moves_left + 60% of increment
        double baseTimeMs = (timeRemainingMs / (double)movesToEnd) + (incrementSeconds * 1000 * 0.6);

        // Position complexity: 0.5x to 2.0x multiplier
        double complexity = CalculateComplexity(board, candidateCount, player);

        // Apply phase modifier
        double phaseMultiplier = GetPhaseModifier(phase);

        // Apply difficulty multiplier for higher difficulties to reach full depth
        // Use adaptive multiplier based on time control to prevent timeouts in short TC
        double difficultyMultiplier = GetAdaptiveDifficultyMultiplier(difficulty, _inferredInitialTimeMs);

        double adjustedTimeMs = baseTimeMs * complexity * phaseMultiplier * difficultyMultiplier;

        // Calculate bounds with 1s minimum reserve
        long maxAllocatableMs = Math.Max(0, timeRemainingMs - TimeControl.MinimumReserveMs);
        long softBoundMs = (long)Math.Clamp(adjustedTimeMs, 500, maxAllocatableMs);

        // Hard bound: soft bound × 1.5, but ensure min ≤ max to avoid Math.Clamp exception
        // When time is tight (softBoundMs + 1000 would exceed max), we can't add the full 1s buffer
        long minHardBoundMs = Math.Min(softBoundMs + 1000, maxAllocatableMs);
        long desiredHardBoundMs = (long)(softBoundMs * 1.5);

        // If min equals max (edge case: very low on time), use max directly
        long hardBoundMs;
        if (minHardBoundMs >= maxAllocatableMs)
        {
            hardBoundMs = maxAllocatableMs;
        }
        else
        {
            hardBoundMs = (long)Math.Clamp(desiredHardBoundMs, minHardBoundMs, maxAllocatableMs);
        }

        // Optimal time: 80% of soft bound
        long optimalTimeMs = softBoundMs * 8 / 10;

        return new TimeAllocation
        {
            SoftBoundMs = softBoundMs,
            HardBoundMs = hardBoundMs,
            OptimalTimeMs = optimalTimeMs,
            IsEmergency = false,
            Phase = phase,
            ComplexityMultiplier = complexity
        };
    }

    /// <summary>
    /// Get emergency mode time allocation
    /// In time scramble, use the increment time to ensure we can keep playing
    /// CRITICAL: Timeout must NEVER happen - only win/lose/draw should end games
    /// </summary>
    private static TimeAllocation GetEmergencyAllocation(long timeRemainingMs, GamePhase phase = GamePhase.Endgame, int incrementSeconds = 5)
    {
        long incrementMs = incrementSeconds * 1000;

        // In time scramble, we rely on VCF + heuristics to make quick moves
        // Hard bound: 75% of increment (leaves 25% safety margin)
        // For 7+5: 3750ms hard bound, 1250ms safety margin
        long hardBoundMs = (long)(incrementMs * 0.75);

        // Soft bound: 50% of increment (encourages faster moves)
        // For 7+5: 2500ms soft bound
        long softBoundMs = (long)(incrementMs * 0.5);

        // Minimum guarantees regardless of increment size
        softBoundMs = Math.Max(softBoundMs, 1000);  // At least 1 second
        hardBoundMs = Math.Max(hardBoundMs, 2000);  // At least 2 seconds

        return new TimeAllocation
        {
            SoftBoundMs = softBoundMs,
            HardBoundMs = hardBoundMs,
            OptimalTimeMs = softBoundMs * 8 / 10,
            IsEmergency = true,
            Phase = phase,
            ComplexityMultiplier = 0.5 // Lowest complexity in emergency
        };
    }

    /// <summary>
    /// Determine game phase based on move number
    /// </summary>
    private static GamePhase DetermineGamePhase(int moveNumber) => moveNumber switch
    {
        <= 10 => GamePhase.Opening,
        <= 25 => GamePhase.EarlyMid,
        <= 45 => GamePhase.LateMid,
        _ => GamePhase.Endgame
    };

    /// <summary>
    /// Get phase modifier for time allocation
    /// Opening saves time, late-mid uses more
    /// </summary>
    private static double GetPhaseModifier(GamePhase phase) => phase switch
    {
        GamePhase.Opening => 0.5,   // Save time early
        GamePhase.EarlyMid => 0.8,
        GamePhase.LateMid => 1.2,   // Peak complexity
        GamePhase.Endgame => 1.0,
        _ => 1.0  // Default for any unknown values
    };

    /// <summary>
    /// Calculate position complexity multiplier (0.5x to 2.0x)
    /// Factors: candidate count, threat density, board congestion
    /// </summary>
    private double CalculateComplexity(Board board, int candidateCount, Player player)
    {
        double score = 1.0;

        // Candidate count factor: more candidates = more complex
        // Base expectation is ~30-40 candidates in midgame
        score += Math.Clamp((candidateCount - 30) / 40.0, -0.3, 0.5);

        // Threat density: more threats = higher complexity
        var threats = _threatDetector.DetectThreats(board, player);
        score += Math.Clamp(threats.Count / 15.0, 0.0, 0.5);

        // Board congestion: more stones = more complex tactics
        var redBoard = board.GetBitBoard(Player.Red);
        var blueBoard = board.GetBitBoard(Player.Blue);
        int stoneCount = redBoard.CountBits() + blueBoard.CountBits();
        if (stoneCount > 100)
            score += 0.2;
        else if (stoneCount < 20)
            score -= 0.1; // Early game, less complex

        return Math.Clamp(score, 0.5, 2.0);
    }

    /// <summary>
    /// Estimate number of moves until game end based on phase
    /// </summary>
    private static int GetMovesToGameEnd(GamePhase phase, int currentMove) => phase switch
    {
        GamePhase.Opening => 50,   // Expect ~50 total moves
        GamePhase.EarlyMid => 40,  // Expect ~40 more moves
        GamePhase.LateMid => 20,   // Expect ~20 more moves
        GamePhase.Endgame => 10,   // Expect ~10 more moves
        _ => 20  // Default for any unknown values
    };

    /// <summary>
    /// Check if panic mode should be activated
    /// Panic when: < adaptive threshold (10% of initial time, or 10s minimum) OR < 1s per move remaining
    /// </summary>
    private static bool ShouldUsePanicMode(long timeRemainingMs, int movesEstimate, long emergencyThresholdMs)
    {
        // Hard threshold: less than emergency threshold (adaptive based on time control)
        if (timeRemainingMs < emergencyThresholdMs)
            return true;

        // Per-move threshold: less than 1 second per move
        if (movesEstimate > 0 && timeRemainingMs < movesEstimate * 1000)
            return true;

        return false;
    }
}
