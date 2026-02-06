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
                    newCells[i, j] = new Cell(i, j, player);
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

    /// <summary>
    /// Create a deep copy of the board.
    /// For compatibility with existing code.
    /// NOTE: Must deep copy Cell objects because AI uses SetPlayerUnsafe during search,
    /// which would mutate shared cells if we only do Array.Copy.
    /// </summary>
    public Board Clone()
    {
        var newCells = new Cell[Size, Size];
        for (int x = 0; x < Size; x++)
        {
            for (int y = 0; y < Size; y++)
            {
                newCells[x, y] = new Cell(x, y, _cells[x, y].Player);
            }
        }
        return new Board(newCells);
    }

    /// <summary>
    /// Place stone with a mutable board reference (for test convenience).
    /// This is a convenience method for tests that need to make multiple moves.
    /// Creates a chain of new boards efficiently.
    /// </summary>
    public Board WithStone(int x, int y, Player player) => PlaceStone(x, y, player);

    /// <summary>
    /// Create a mutable board wrapper for test scenarios.
    /// Tests that need to make multiple sequential moves can use this.
    /// </summary>
    public MutableBoard AsMutable() => new MutableBoard(this);
}

/// <summary>
/// Mutable board wrapper for test scenarios.
/// Allows convenient sequential move placement without manual return value chaining.
/// </summary>
public sealed class MutableBoard
{
    private Board _board;

    public MutableBoard(Board board)
    {
        _board = board ?? throw new ArgumentNullException(nameof(board));
    }

    public Board Board => _board;

    /// <summary>
    /// Place a stone and update the internal board reference.
    /// </summary>
    public void PlaceStone(int x, int y, Player player)
    {
        _board = _board.PlaceStone(x, y, player);
    }

    /// <summary>
    /// Get the current board.
    /// </summary>
    public Board GetCurrentBoard() => _board;

    /// <summary>
    /// Implicit conversion to Board.
    /// </summary>
    public static implicit operator Board(MutableBoard mutable) => mutable._board;
}

/// <summary>
/// Represents a single cell on the game board.
/// Mostly immutable - has unsafe mutation for AI probe operations.
/// </summary>
public sealed class Cell
{
    private Player _player;

    /// <summary>
    /// X coordinate of this cell.
    /// </summary>
    public int X { get; }

    /// <summary>
    /// Y coordinate of this cell.
    /// </summary>
    public int Y { get; }

    /// <summary>
    /// Player occupying this cell (None if empty).
    /// </summary>
    public Player Player => _player;

    /// <summary>
    /// Check if this cell is empty.
    /// </summary>
    public bool IsEmpty => _player == Player.None;

    /// <summary>
    /// Get the position of this cell.
    /// </summary>
    public Position Position => new(X, Y);

    /// <summary>
    /// Create a new cell.
    /// </summary>
    public Cell(int x, int y, Player player)
    {
        X = x;
        Y = y;
        _player = player;
    }

    /// <summary>
    /// Set player directly (for internal use - creates new cell).
    /// </summary>
    internal Cell WithPlayer(Player player) => new Cell(X, Y, player);

    /// <summary>
    /// Mutable setter for AI probe operations.
    /// WARNING: Only use for temporary mutations that are immediately reverted.
    /// </summary>
    public void SetPlayerUnsafe(Player player) => _player = player;

    /// <summary>
    /// Get the player directly (for internal use).
    /// </summary>
    public Player GetPlayerUnsafe() => _player;
}
