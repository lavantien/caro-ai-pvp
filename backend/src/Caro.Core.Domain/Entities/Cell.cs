namespace Caro.Core.Domain.Entities;

/// <summary>
/// Represents a single cell on the board.
/// Immutable record struct for value semantics.
/// </summary>
public readonly record struct Cell(Position Position, Player Player)
{
    /// <summary>X coordinate</summary>
    public readonly int X => Position.X;

    /// <summary>Y coordinate</summary>
    public readonly int Y => Position.Y;

    /// <summary>Check if the cell is empty</summary>
    public readonly bool IsEmpty => Player == Player.None;

    /// <summary>Check if the cell is occupied</summary>
    public readonly bool IsOccupied => !IsEmpty;

    /// <summary>Check if the cell has a Red stone</summary>
    public readonly bool IsRed => Player == Player.Red;

    /// <summary>Check if the cell has a Blue stone</summary>
    public readonly bool IsBlue => Player == Player.Blue;

    /// <summary>
    /// Create a cell from coordinates
    /// </summary>
    public Cell(int x, int y, Player player) : this(new Position(x, y), player)
    {
    }

    /// <summary>
    /// Create an empty cell at the given position
    /// </summary>
    public static Cell Empty(Position position) => new(position, Player.None);

    /// <summary>
    /// Create an empty cell at the given coordinates
    /// </summary>
    public static Cell Empty(int x, int y) => new(new Position(x, y), Player.None);

    /// <summary>
    /// Return a new cell with the given player
    /// </summary>
    public readonly Cell WithPlayer(Player player) => new(Position, player);

    /// <summary>
    /// Return a new cell with a Red stone
    /// </summary>
    public readonly Cell WithRed() => new(Position, Player.Red);

    /// <summary>
    /// Return a new cell with a Blue stone
    /// </summary>
    public readonly Cell WithBlue() => new(Position, Player.Blue);

    /// <summary>
    /// Return a new empty cell
    /// </summary>
    public readonly Cell Cleared() => new(Position, Player.None);

    public override readonly string ToString() => IsEmpty ? $"." : Player == Player.Red ? "R" : "B";
}
