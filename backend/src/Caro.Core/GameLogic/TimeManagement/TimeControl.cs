namespace Caro.Core.GameLogic.TimeManagement;

/// <summary>
/// Time control configuration for chess-clock style time management
/// Supports different time controls like 3+2 (Blitz), 7+5 (Rapid), 15+10 (Classical)
/// </summary>
public readonly record struct TimeControl(
    long InitialTimeMs,    // Initial time per player in milliseconds
    int IncrementSeconds,  // Time increment per move in seconds
    string Name            // Display name (e.g., "3+2", "7+5")
)
{
    /// <summary>
    /// Time increment per move in milliseconds
    /// </summary>
    public long IncrementMs => IncrementSeconds * 1000L;

    /// <summary>
    /// Minimum time reserve to always keep (1 second)
    /// Prevents clock flag due to system latency
    /// </summary>
    public const long MinimumReserveMs = 1_000;

    /// <summary>
    /// Standard time controls
    /// </summary>
    public static TimeControl Blitz => new(180_000, 2, "3+2");      // 3 minutes + 2 seconds per move
    public static TimeControl Rapid => new(420_000, 5, "7+5");       // 7 minutes + 5 seconds per move
    public static TimeControl Classical => new(900_000, 10, "15+10"); // 15 minutes + 10 seconds per move

    /// <summary>
    /// Default time control for production use
    /// </summary>
    public static TimeControl Default => Rapid;

    /// <summary>
    /// Calculate total estimated time budget for a typical game
    /// </summary>
    public long GetTotalBudgetMs(int estimatedMoves = 40)
    {
        return InitialTimeMs + (IncrementMs * estimatedMoves);
    }

    /// <summary>
    /// Calculate average time available per move
    /// </summary>
    public long GetAveragePerMoveMs(int estimatedMoves = 40)
    {
        return GetTotalBudgetMs(estimatedMoves) / estimatedMoves;
    }

    /// <summary>
    /// Emergency threshold - panic mode when time remaining is below this
    /// 10% of initial time, or 10 seconds minimum
    /// </summary>
    public long GetEmergencyThresholdMs()
    {
        return Math.Max(10_000, InitialTimeMs / 10);
    }
}

/// <summary>
/// Legacy constants for backward compatibility
/// TODO: Migrate to TimeControl struct usage
/// </summary>
public static class LegacyTimeControl
{
    public const long SevenPlusFiveInitialMs = 420_000;
    public const long SevenPlusFiveIncrementMs = 5_000;
    public const long MinimumReserveMs = 1_000;
    public const long EmergencyThresholdMs = 10_000;
}
