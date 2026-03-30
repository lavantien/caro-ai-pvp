namespace Caro.Core.Domain.Configuration;

/// <summary>
/// Centralized search algorithm constants for AI engine.
/// Single source of truth for all search-related parameters.
/// </summary>
public static class SearchConstants
{
    /// <summary>
    /// Maximum search radius around existing stones for candidate generation.
    /// Ensures detection of all winning moves within range.
    /// </summary>
    public const int MaxSearchRadius = 7;

    /// <summary>
    /// Maximum number of killer moves tracked per depth.
    /// </summary>
    public const int MaxKillerMoves = 2;

    /// <summary>
    /// Maximum depth for killer move tracking array.
    /// Effectively unlimited for practical game play.
    /// </summary>
    public const int MaxKillerDepth = 512;

    /// <summary>
    /// Check time every N nodes for search timeout detection.
    /// Power of 2 for efficient masking.
    /// </summary>
    public const int TimeCheckInterval = 16;

    /// <summary>
    /// Absolute maximum search depth (safeguard, not a target).
    /// </summary>
    public const int AbsoluteMaxDepth = 50;

    /// <summary>
    /// Initial aspiration window size (in centipawns).
    /// </summary>
    public const int AspirationWindowSize = 50;

    /// <summary>
    /// Maximum number of re-searches with wider aspiration windows.
    /// </summary>
    public const int MaxAspirationAttempts = 3;

    /// <summary>
    /// Default transposition table size in megabytes.
    /// </summary>
    public const int DefaultTTSizeMb = 256;

    /// <summary>
    /// Minimum depth to apply null-move pruning.
    /// </summary>
    public const int NullMoveMinDepth = 3;

    /// <summary>
    /// Depth reduction for null-move verification search.
    /// </summary>
    public const int NullMoveDepthReduction = 3;

    /// <summary>
    /// Maximum depth for quiescence search beyond depth 0.
    /// </summary>
    public const int MaxQuiescenceDepth = 4;
}
