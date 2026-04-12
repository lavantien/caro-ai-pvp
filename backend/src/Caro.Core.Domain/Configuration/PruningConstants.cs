namespace Caro.Core.Domain.Configuration;

/// <summary>
/// Centralized pruning constants for AI search algorithms.
/// Controls futility pruning, late move reduction, and PVS thresholds.
/// </summary>
public static class PruningConstants
{
    /// <summary>
    /// Base margin for futility pruning (in centipawns).
    /// Moves whose static eval + this margin cannot exceed alpha are pruned.
    /// </summary>
    public const int FutilityMarginBase = 300;

    /// <summary>
    /// Additional futility margin per depth remaining.
    /// Total margin = FutilityMarginBase + depth * FutilityMarginPerDepth.
    /// </summary>
    public const int FutilityMarginPerDepth = 100;

    /// <summary>
    /// Minimum depth to apply futility pruning.
    /// </summary>
    public const int FutilityMinDepth = 3;

    /// <summary>
    /// Minimum depth to apply Late Move Reduction (LMR).
    /// </summary>
    public const int LMRMinDepth = 3;

    /// <summary>
    /// Number of moves searched at full depth before LMR kicks in.
    /// </summary>
    public const int LMRFullDepthMoves = 4;

    /// <summary>
    /// Base depth reduction for late moves in LMR.
    /// </summary>
    public const int LMRBaseReduction = 1;

    /// <summary>
    /// Minimum depth to enable Principal Variation Search (PVS).
    /// </summary>
    public const int PvsEnabledDepth = 2;
}
