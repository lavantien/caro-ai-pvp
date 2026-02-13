namespace Caro.Core.Domain.Entities;

/// <summary>
/// Symmetry transformations for canonical position reduction.
/// Used to map equivalent board positions to a single canonical form.
/// </summary>
public enum SymmetryType
{
    /// <summary>No transformation applied</summary>
    Identity = 0,

    /// <summary>90-degree clockwise rotation</summary>
    Rotate90 = 1,

    /// <summary>180-degree rotation</summary>
    Rotate180 = 2,

    /// <summary>270-degree clockwise rotation</summary>
    Rotate270 = 3,

    /// <summary>Horizontal mirror (flip across vertical axis)</summary>
    FlipHorizontal = 4,

    /// <summary>Vertical mirror (flip across horizontal axis)</summary>
    FlipVertical = 5,

    /// <summary>Main diagonal reflection (y = x)</summary>
    DiagonalA = 6,

    /// <summary>Anti-diagonal reflection (y = -x)</summary>
    DiagonalB = 7
}

/// <summary>
/// Single entry in the opening book representing a position and its recommended moves.
/// Uses canonical hash for symmetry-reduced position identification.
/// </summary>
public sealed record OpeningBookEntry
{
    /// <summary>
    /// Canonical position hash after symmetry reduction.
    /// Used for initial lookup to find candidate entries.
    /// </summary>
    public required ulong CanonicalHash { get; init; }

    /// <summary>
    /// Direct hash of the board (Board.GetHash()).
    /// Used to uniquely identify the exact board position among those sharing the same canonical hash.
    /// </summary>
    public required ulong DirectHash { get; init; }

    /// <summary>
    /// Ply depth of this position (0 = empty board, 1 = after first move, etc.)
    /// </summary>
    public required int Depth { get; init; }

    /// <summary>
    /// Recommended moves for this position.
    /// Moves are stored relative to the canonical position.
    /// </summary>
    public required BookMove[] Moves { get; init; }

    /// <summary>
    /// Symmetry transformation applied to create the canonical form.
    /// Required to transform relative moves back to actual board coordinates.
    /// </summary>
    public required SymmetryType Symmetry { get; init; }

    /// <summary>
    /// Whether this position uses absolute coordinates (near edge) or relative (center).
    /// Edge positions don't use symmetry reduction due to board boundary effects.
    /// </summary>
    public required bool IsNearEdge { get; init; }

    /// <summary>
    /// Player whose turn it is at this position.
    /// </summary>
    public required Player Player { get; init; }
}

/// <summary>
/// Single book move with evaluation metadata.
/// Moves are stored relative to the canonical position.
/// </summary>
public sealed record BookMove
{
    /// <summary>
    /// X coordinate relative to canonical position (0-18).
    /// </summary>
    public required int RelativeX { get; init; }

    /// <summary>
    /// Y coordinate relative to canonical position (0-18).
    /// </summary>
    public required int RelativeY { get; init; }

    /// <summary>
    /// Expected win rate percentage (0-100).
    /// 50 = even position, >50 = advantage for current player.
    /// </summary>
    public required int WinRate { get; init; }

    /// <summary>
    /// Search depth achieved during book generation (in plies).
    /// </summary>
    public required int DepthAchieved { get; init; }

    /// <summary>
    /// Number of nodes searched during evaluation.
    /// </summary>
    public required long NodesSearched { get; init; }

    /// <summary>
    /// Evaluation score in centipawns from the search.
    /// Positive = advantage for current player.
    /// </summary>
    public required int Score { get; init; }

    /// <summary>
    /// Whether this move is a forcing move (VCF - Victory by Continuous Four).
    /// VCF moves have higher priority when selecting from multiple candidates.
    /// </summary>
    public required bool IsForcing { get; init; }

    /// <summary>
    /// Move priority for selection when multiple moves exist.
    /// Higher values are preferred. Used for variety in book play.
    /// </summary>
    public required int Priority { get; init; }

    /// <summary>
    /// Whether this move passed blunder verification.
    /// Verified moves have been checked for tactical responses.
    /// </summary>
    public required bool IsVerified { get; init; }
}

/// <summary>
/// Result of opening book generation.
/// </summary>
public sealed record BookGenerationResult(
    int PositionsGenerated,
    int PositionsVerified,
    TimeSpan GenerationTime,
    int BlundersFound,
    int TotalMovesStored
);

/// <summary>
/// Result of opening book verification.
/// </summary>
public sealed record VerificationResult(
    int EntriesChecked,
    int BlundersFound,
    string[] BlunderDetails,
    int VerifiedCount
);

/// <summary>
/// Statistics about the opening book.
/// </summary>
public sealed record BookStatistics(
    int TotalEntries,
    int MaxDepth,
    int[] CoverageByDepth,
    int TotalMoves,
    DateTime GeneratedAt,
    string Version
);

/// <summary>
/// Represents a canonical position after symmetry reduction.
/// </summary>
public sealed record CanonicalPosition(
    ulong CanonicalHash,
    SymmetryType SymmetryApplied,
    bool IsNearEdge,
    Player Player
);

/// <summary>
/// Represents a move with its source and target coordinates for symmetry transformation.
/// </summary>
public sealed record TransformResult(
    int SourceX,
    int SourceY,
    int TargetX,
    int TargetY,
    SymmetryType Transform
);
