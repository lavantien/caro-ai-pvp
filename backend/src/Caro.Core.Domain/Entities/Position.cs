namespace Caro.Core.Domain.Entities;

/// <summary>
/// Represents a position on the 19x19 board.
/// Immutable record struct for value semantics.
/// </summary>
public readonly record struct Position(int X = -1, int Y = -1)
{
    /// <summary>
    /// Board size constant
    /// </summary>
    public const int BoardSize = 19;

    /// <summary>
    /// Total number of cells on the board
    /// </summary>
    public const int TotalCells = BoardSize * BoardSize; // 361

    /// <summary>
    /// Check if the position is valid (within board bounds)
    /// </summary>
    public readonly bool IsValid => X >= 0 && X < BoardSize && Y >= 0 && Y < BoardSize;

    /// <summary>
    /// Get the linear index of this position (0-360)
    /// </summary>
    public readonly int Index => Y * BoardSize + X;

    /// <summary>
    /// Create a position from a linear index
    /// </summary>
    public static Position FromIndex(int index)
    {
        if (index < 0 || index >= TotalCells)
            throw new ArgumentOutOfRangeException(nameof(index), $"Index must be between 0 and {TotalCells - 1}");

        int y = index / BoardSize;
        int x = index % BoardSize;
        return new Position(x, y);
    }

    /// <summary>
    /// Get the position offset by the given delta
    /// </summary>
    public readonly Position Offset(int dx, int dy) => new(X + dx, Y + dy);

    /// <summary>
    /// Get adjacent positions (up, down, left, right)
    /// </summary>
    public readonly ReadOnlyMemory<Position> GetAdjacentPositions()
    {
        var positions = new List<Position>(4);
        AddIfValid(positions, Offset(0, -1)); // Up
        AddIfValid(positions, Offset(0, 1));  // Down
        AddIfValid(positions, Offset(-1, 0)); // Left
        AddIfValid(positions, Offset(1, 0));  // Right
        return positions.ToArray();

        void AddIfValid(List<Position> list, Position pos)
        {
            if (pos.IsValid) list.Add(pos);
        }
    }

    /// <summary>
    /// Get all 8 neighboring positions (including diagonals)
    /// </summary>
    public readonly ReadOnlyMemory<Position> GetNeighbors()
    {
        var positions = new List<Position>(8);
        for (int dx = -1; dx <= 1; dx++)
        {
            for (int dy = -1; dy <= 1; dy++)
            {
                if (dx == 0 && dy == 0) continue;
                var pos = Offset(dx, dy);
                if (pos.IsValid) positions.Add(pos);
            }
        }
        return positions.ToArray();
    }

    /// <summary>
    /// Deconstruct for tuple pattern matching
    /// </summary>
    public readonly void Deconstruct(out int x, out int y)
    {
        x = X;
        y = Y;
    }

    public override readonly string ToString() => $"({X},{Y})";
}

/// <summary>
/// Direction vectors for line extraction
/// </summary>
public readonly record struct Direction(int Dx, int Dy)
{
    /// <summary>Horizontal (left-right)</summary>
    public static readonly Direction Horizontal = new(1, 0);

    /// <summary>Vertical (up-down)</summary>
    public static readonly Direction Vertical = new(0, 1);

    /// <summary>Diagonal (top-left to bottom-right)</summary>
    public static readonly Direction DiagonalDown = new(1, 1);

    /// <summary>Anti-diagonal (top-right to bottom-left)</summary>
    public static readonly Direction DiagonalUp = new(1, -1);

    /// <summary>All four directions</summary>
    public static readonly ReadOnlyMemory<Direction> All = new[]
    {
        Horizontal,
        Vertical,
        DiagonalDown,
        DiagonalUp
    };

    /// <summary>Get the opposite direction</summary>
    public readonly Direction Opposite() => new(-Dx, -Dy);
}
