namespace Caro.Core.GameLogic;

/// <summary>
/// AI difficulty levels for Minimax algorithm
/// </summary>
public enum AIDifficulty
{
    /// <summary>
    /// Easy: Depth 3 with some randomness
    /// </summary>
    Easy = 3,

    /// <summary>
    /// Medium: Depth 5
    /// </summary>
    Medium = 5,

    /// <summary>
    /// Hard: Depth 7
    /// </summary>
    Hard = 7,

    /// <summary>
    /// Expert: Depth 7 with enhanced heuristics
    /// </summary>
    Expert = 7
}
