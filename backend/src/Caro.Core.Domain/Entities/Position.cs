namespace Caro.Core.Domain.Entities;

/// <summary>
/// Represents a position on the game board using X, Y coordinates.
/// Value object - immutable and identified by its values.
/// </summary>
/// <param name="X">X coordinate (0-18)</param>
/// <param name="Y">Y coordinate (0-18)</param>
public readonly record struct Position(int X, int Y)
{
    /// <summary>
    /// Board size constant (19x19).
    /// </summary>
    public static readonly int BoardSize = 32;

    /// <summary>
    /// Check if this position is within valid board bounds (0-18).
    /// </summary>
    public readonly bool IsValid => X >= 0 && X < BoardSize && Y >= 0 && Y < BoardSize;

    /// <summary>
    /// Create a position with offset applied.
    /// </summary>
    public readonly Position Offset(int dx, int dy) => new(X + dx, Y + dy);

    /// <summary>
    /// Convert to tuple for convenience.
    /// </summary>
    public readonly (int x, int y) ToTuple() => (X, Y);

    /// <summary>
    /// Create position from tuple.
    /// </summary>
    public static Position FromTuple((int x, int y) tuple) => new(tuple.x, tuple.y);
}
