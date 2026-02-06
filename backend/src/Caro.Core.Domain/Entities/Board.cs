namespace Caro.Core.Domain.Entities;

/// <summary>
/// Pure domain representation of the game board.
/// Immutable - operations return new instances.
/// </summary>
public sealed class Board
{
    private const int Size = 19;
    private readonly Cell[,] _cells;

    // Private constructor for internal use
    private Board(Cell[,] cells)
    {
        _cells = cells;
    }

    /// <summary>
    /// Create an empty board.
    /// </summary>
    public Board()
    {
        _cells = new Cell[Size, Size];
        for (int x = 0; x < Size; x++)
        {
            for (int y = 0; y < Size; y++)
            {
                _cells[x, y] = new Cell(x, y, Player.None);
            }
        }
    }

    /// <summary>
    /// Board size (always 19 for Caro).
    /// </summary>
    public int BoardSize => Size;

    /// <summary>
    /// Get all cells on the board.
    /// </summary>
    public IEnumerable<Cell> Cells => _cells.Cast<Cell>();

    /// <summary>
    /// Get the cell at the given position.
    /// </summary>
    public Cell GetCell(int x, int y)
    {
        if (x < 0 || x >= Size || y < 0 || y >= Size)
            throw new ArgumentOutOfRangeException(nameof(x), "Position must be within board bounds");

        return _cells[x, y];
    }

    /// <summary>
    /// Get the cell at the given position.
    /// </summary>
    public Cell GetCell(Position pos) => GetCell(pos.X, pos.Y);

    /// <summary>
    /// Place a stone at the given position.
    /// Immutable - returns a new board with the stone placed.
    /// </summary>
    public Board PlaceStone(int x, int y, Player player)
    {
        if (x < 0 || x >= Size || y < 0 || y >= Size)
            throw new ArgumentOutOfRangeException(nameof(x), "Position must be within board bounds");

        if (!_cells[x, y].IsEmpty)
            throw new InvalidOperationException("Cell is already occupied");

        // Create a new board with the move placed
        var newCells = new Cell[Size, Size];
        for (int i = 0; i < Size; i++)
        {
            for (int j = 0; j < Size; j++)
            {
                if (i == x && j == y)
                {
                    newCells[i, j] = _cells[i, j].WithPlayer(player);
                }
                else
                {
                    newCells[i, j] = _cells[i, j];
                }
            }
        }

        return new Board(newCells);
    }

    /// <summary>
    /// Place a stone at the given position.
    /// Immutable - returns a new board with the stone placed.
    /// </summary>
    public Board PlaceStone(Position pos, Player player) => PlaceStone(pos.X, pos.Y, player);

    /// <summary>
    /// Check if a position is empty.
    /// </summary>
    public bool IsEmpty(int x, int y)
    {
        if (x < 0 || x >= Size || y < 0 || y >= Size)
            return false;
        return _cells[x, y].IsEmpty;
    }

    /// <summary>
    /// Check if a position is empty.
    /// </summary>
    public bool IsEmpty(Position pos) => IsEmpty(pos.X, pos.Y);

    /// <summary>
    /// Get the player at the given position.
    /// </summary>
    public Player GetPlayerAt(int x, int y)
    {
        if (x < 0 || x >= Size || y < 0 || y >= Size)
            return Player.None;
        return _cells[x, y].Player;
    }

    /// <summary>
    /// Check if the entire board is empty.
    /// </summary>
    public bool IsEmpty()
    {
        foreach (var cell in _cells)
        {
            if (!cell.IsEmpty)
                return false;
        }
        return true;
    }
}

/// <summary>
/// Represents a single cell on the game board.
/// Fully immutable - use WithPlayer() to create a new cell with a different player.
/// </summary>
public readonly record struct Cell(int X, int Y, Player Player)
{
    /// <summary>
    /// Check if this cell is empty.
    /// </summary>
    public bool IsEmpty => Player == Player.None;

    /// <summary>
    /// Get the position of this cell.
    /// </summary>
    public Position Position => new(X, Y);

    /// <summary>
    /// Create a new cell with a different player.
    /// </summary>
    public Cell WithPlayer(Player player) => new(X, Y, player);
}
