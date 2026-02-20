using Caro.Core.Domain.Configuration;

namespace Caro.Core.Domain.Entities;

/// <summary>
/// Pure domain representation of the game board.
/// Immutable - operations return new instances.
/// PERFORMANCE: Pre-computed BitBoards and Hash for O(1) AI search operations.
/// </summary>
public sealed class Board
{
    private const int Size = GameConstants.BoardSize;
    private readonly Cell[,] _cells;

    // Pre-computed bitboards for O(1) access during AI search
    // 16x16 board = 256 bits = 4 ulongs
    private readonly ulong[] _redBits;
    private readonly ulong[] _blueBits;
    private readonly ulong _hash;

    // Private constructor for internal use
    private Board(Cell[,] cells, ulong[] redBits, ulong[] blueBits, ulong hash)
    {
        _cells = cells;
        _redBits = redBits;
        _blueBits = blueBits;
        _hash = hash;
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
        // Empty board has all zeros
        _redBits = new ulong[4];
        _blueBits = new ulong[4];
        _hash = 0;
    }

    /// <summary>
    /// Board size (always 32 for Caro).
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
    /// PERFORMANCE: O(n²) cell copy + O(1) bitboard/hash update
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
                newCells[i, j] = _cells[i, j];
            }
        }

        // Place the new stone
        newCells[x, y] = new Cell(x, y, player);

        // O(1) incremental bitboard update - copy arrays and set one bit
        var newRedBits = new ulong[4];
        var newBlueBits = new ulong[4];
        Array.Copy(_redBits, newRedBits, 4);
        Array.Copy(_blueBits, newBlueBits, 4);

        int bitIndex = y * Size + x;  // y * 32 + x
        int ulongIndex = bitIndex >> 6;  // bitIndex / 64
        int bitOffset = bitIndex & 63;    // bitIndex % 64
        ulong bitMask = 1UL << bitOffset;

        // Simple hash XOR (actual Zobrist handled by extension method for compatibility)
        ulong pieceKey = (ulong)((x << 8) | y) ^ (player == Player.Red ? 0xAAAAAAAAAAAAAAAAUL : 0x5555555555555555UL);
        ulong newHash = _hash ^ pieceKey;

        if (player == Player.Red)
            newRedBits[ulongIndex] |= bitMask;
        else
            newBlueBits[ulongIndex] |= bitMask;

        return new Board(newCells, newRedBits, newBlueBits, newHash);
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
        for (int i = 0; i < 4; i++)
        {
            if (_redBits[i] != 0 || _blueBits[i] != 0)
                return false;
        }
        return true;
    }

    /// <summary>
    /// Get pre-computed bitboard bits for a player.
    /// Returns array of 4 ulongs (256 bits for 16x16 board).
    /// </summary>
    public ulong[] GetBitBoardBits(Player player) => player == Player.Red ? _redBits : _blueBits;

    /// <summary>
    /// Get the pre-computed hash for this position.
    /// </summary>
    public ulong GetHash() => _hash;
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
