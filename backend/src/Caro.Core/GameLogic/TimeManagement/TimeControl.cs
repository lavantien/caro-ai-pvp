namespace Caro.Core.GameLogic.TimeManagement;

/// <summary>
/// Time control configuration for 7+5 (7 minutes initial + 5 seconds increment per move)
/// This is the standard "fast" time control for online Caro/Gomoku games
/// </summary>
public static class TimeControl
{
    /// <summary>
    /// Initial time per player in milliseconds (7 minutes)
    /// </summary>
    public const long SevenPlusFiveInitialMs = 420_000;

    /// <summary>
    /// Time increment per move in milliseconds (5 seconds)
    /// </summary>
    public const long SevenPlusFiveIncrementMs = 5_000;

    /// <summary>
    /// Total estimated time budget for a typical 40-move game
    /// Initial time + (increment × estimated moves)
    /// </summary>
    public const long SevenPlusFiveTotalBudgetMs = SevenPlusFiveInitialMs + (SevenPlusFiveIncrementMs * 40);

    /// <summary>
    /// Average time available per move for 7+5 time control
    /// Based on typical 40-move game length
    /// </summary>
    public const long SevenPlusFiveAveragePerMoveMs = SevenPlusFiveTotalBudgetMs / 40;

    /// <summary>
    /// Minimum time reserve to always keep (1 second)
    /// Prevents clock flag due to system latency
    /// </summary>
    public const long MinimumReserveMs = 1_000;

    /// <summary>
    /// Emergency threshold - panic mode when time remaining is below this (10 seconds)
    /// </summary>
    public const long EmergencyThresholdMs = 10_000;
}
