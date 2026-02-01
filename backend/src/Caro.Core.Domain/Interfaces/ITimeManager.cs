namespace Caro.Core.Domain.Interfaces;

/// <summary>
/// Interface for time management in AI search.
/// Calculates time allocations and tracks time usage for adaptive adjustments.
/// </summary>
public interface ITimeManager
{
    /// <summary>
    /// Calculate the time allocation for the next move.
    /// </summary>
    /// <param name="difficulty">AI difficulty level</param>
    /// <param name="timeRemainingMs">Time remaining for the current player (ms)</param>
    /// <param name="moveNumber">Current move number</param>
    /// <param name="candidateCount">Number of candidate moves to consider</param>
    /// <returns>Time allocation for the search</returns>
    TimeAllocation CalculateAllocation(
        AIDifficulty difficulty,
        long timeRemainingMs,
        int moveNumber,
        int candidateCount);

    /// <summary>
    /// Calculate the maximum search depth based on time allocation and performance.
    /// </summary>
    /// <param name="difficulty">AI difficulty level</param>
    /// <param name="timeAllocationMs">Allocated time for the search (ms)</param>
    /// <param name="nodesPerSecond">Estimated nodes per second</param>
    /// <returns>Maximum search depth</returns>
    int CalculateMaxDepth(AIDifficulty difficulty, long timeAllocationMs, double nodesPerSecond);

    /// <summary>
    /// Report the actual time used for a move (for adaptive adjustment).
    /// </summary>
    /// <param name="allocatedTimeMs">Time that was allocated</param>
    /// <param name="actualTimeMs">Time actually used</param>
    /// <param name="timedOut">Whether the search timed out</param>
    void ReportTimeUsed(long allocatedTimeMs, long actualTimeMs, bool timedOut);

    /// <summary>
    /// Update the nodes-per-second estimate.
    /// </summary>
    /// <param name="nodesSearched">Number of nodes searched</param>
    /// <param name="elapsedMs">Time elapsed (ms)</param>
    void UpdateNodesPerSecond(long nodesSearched, long elapsedMs);

    /// <summary>
    /// Get the current nodes-per-second estimate.
    /// </summary>
    double GetNodesPerSecond();

    /// <summary>
    /// Reset internal state for a new game.
    /// </summary>
    void Reset();
}

/// <summary>
/// Time allocation for a single search
/// </summary>
public sealed record TimeAllocation(
    long SoftBoundMs,      // Soft time limit (target to stay under)
    long HardBoundMs,      // Hard time limit (must not exceed)
    long OptimalTimeMs,    // Optimal time to aim for
    bool IsEmergency,      // Whether this is an emergency allocation (low time)
    double ComplexityMultiplier  // Complexity factor for depth calculation
)
{
    /// <summary>
    /// Default time allocation
    /// </summary>
    public static readonly TimeAllocation Default = new(
        SoftBoundMs: 5000,
        HardBoundMs: 10000,
        OptimalTimeMs: 4000,
        IsEmergency: false,
        ComplexityMultiplier: 1.0
    );
}

/// <summary>
/// AI difficulty levels
/// </summary>
public enum AIDifficulty
{
    /// <summary>Weakest AI (beginner)</summary>
    Braindead,

    /// <summary>Easy difficulty</summary>
    Easy,

    /// <summary>Medium difficulty</summary>
    Medium,

    /// <summary>Hard difficulty</summary>
    Hard,

    /// <summary>Strongest AI (grandmaster level)</summary>
    Grandmaster
}
