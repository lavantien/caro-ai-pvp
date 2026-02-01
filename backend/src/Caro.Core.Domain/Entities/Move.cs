namespace Caro.Core.Domain.Entities;

/// <summary>
/// Represents a single move in the game.
/// Immutable record for value semantics.
/// </summary>
public readonly record struct Move(Position Position, Player Player)
{
    /// <summary>
    /// X coordinate of the move
    /// </summary>
    public readonly int X => Position.X;

    /// <summary>
    /// Y coordinate of the move
    /// </summary>
    public readonly int Y => Position.Y;

    /// <summary>
    /// Check if the move is valid (position is on board and player is valid)
    /// </summary>
    public readonly bool IsValid => Position.IsValid && Player.IsValid();

    /// <summary>
    /// Create a move from coordinates
    /// </summary>
    public Move(int x, int y, Player player) : this(new Position(x, y), player)
    {
    }

    /// <summary>
    /// Deconstruct for tuple pattern matching
    /// </summary>
    public readonly void Deconstruct(out int x, out int y, out Player player)
    {
        x = X;
        y = Y;
        player = Player;
    }

    public override readonly string ToString() => $"{Player} @ {Position}";
}

/// <summary>
/// Represents a move with additional context
/// </summary>
public readonly record struct AnnotatedMove
{
    /// <summary>The move position and player</summary>
    public required Move Move { get; init; }

    /// <summary>Move number (1-indexed)</summary>
    public required int MoveNumber { get; init; }

    /// <summary>Time remaining for the player after this move (ms)</summary>
    public required long TimeRemainingMs { get; init; }

    /// <summary>Time taken to make this move (ms)</summary>
    public required long TimeTakenMs { get; init; }

    /// <summary>AI statistics for this move (if AI-generated)</summary>
    public AIStats? Stats { get; init; }
}

/// <summary>
/// AI search statistics for a move
/// </summary>
public readonly record struct AIStats(
    int DepthAchieved,
    long NodesSearched,
    double ElapsedSeconds,
    int TableHits,
    int TableLookups,
    int Score,
    int ThreadCount
)
{
    /// <summary>Nodes per second</summary>
    public readonly double NodesPerSecond => ElapsedSeconds > 0
        ? NodesSearched / ElapsedSeconds
        : 0;

    /// <summary>Transposition table hit rate</summary>
    public readonly double HitRate => TableLookups > 0
        ? (double)TableHits / TableLookups
        : 0;
}
