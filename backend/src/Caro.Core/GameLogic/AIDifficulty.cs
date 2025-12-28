namespace Caro.Core.GameLogic;

/// <summary>
/// AI difficulty levels for Minimax algorithm
/// Optimized for 3+2 time control (3 minutes + 2 second increment)
/// </summary>
public enum AIDifficulty
{
    /// <summary>
    /// Easy: Depth 1 with some randomness
    /// </summary>
    Easy = 1,

    /// <summary>
    /// Medium: Depth 2
    /// </summary>
    Medium = 2,

    /// <summary>
    /// Hard: Depth 3
    /// </summary>
    Hard = 3,

    /// <summary>
    /// Expert: Depth 5 (strongest, with time-aware depth adjustment)
    /// </summary>
    Expert = 5,

    /// <summary>
    /// Master: Depth 7 (maximum strength, requires significant thinking time)
    /// Uses all advanced optimizations: PVS, LMR, Quiescence, TT, History, Aspiration Windows
    /// </summary>
    Master = 7
}
