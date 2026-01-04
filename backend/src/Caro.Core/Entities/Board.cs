using Caro.Core.GameLogic;

namespace Caro.Core.Entities;

public enum Player { None, Red, Blue }

public class Cell
{
    internal Board? _board; // Reference to parent board for hash updates
    internal int _x;
    internal int _y;

    private Player _player = Player.None;
    public Player Player
    {
        get => _player;
        set
        {
            if (_player != value && _board != null)
            {
                // Update board hash when player changes
                _board.UpdateHashForCell(_x, _y, _player, value);
            }
            _player = value;
        }
    }

    public bool IsEmpty => Player == Player.None;

    /// <summary>
    /// Set player directly without updating hash (used internally by Clone)
    /// </summary>
    internal void SetPlayerDirect(Player player) => _player = player;
}

public class Board
{
    private const int Size = 15;
    private readonly Cell[,] _cells;
    private ulong _hash;

    // Hybrid representation: BitBoards for fast AI operations
    // These are kept in sync with the Cell[,] array
    private BitBoard _redBitBoard;
    private BitBoard _blueBitBoard;

    public Board()
    {
        _cells = new Cell[Size, Size];
        for (int i = 0; i < Size; i++)
        {
            for (int j = 0; j < Size; j++)
            {
                _cells[i, j] = new Cell { _board = this, _x = i, _y = j };
            }
        }
    }

    public int BoardSize => Size;

    public IEnumerable<Cell> Cells => _cells.Cast<Cell>();

    /// <summary>
    /// Get the current Zobrist hash of the board position
    /// This is maintained incrementally for O(1) access
    /// </summary>
    public ulong Hash => _hash;

    /// <summary>
    /// Get the BitBoard representation for Red stones
    /// Used for fast AI operations (pattern matching, threat detection)
    /// </summary>
    public BitBoard GetRedBitBoard() => _redBitBoard;

    /// <summary>
    /// Get the BitBoard representation for Blue stones
    /// Used for fast AI operations (pattern matching, threat detection)
    /// </summary>
    public BitBoard GetBlueBitBoard() => _blueBitBoard;

    /// <summary>
    /// Get the BitBoard for a specific player
    /// </summary>
    public BitBoard GetBitBoard(Player player) =>
        player == Player.Red ? _redBitBoard : _blueBitBoard;

    /// <summary>
    /// Internal method to update hash and BitBoard when a cell's player changes
    /// Called by Cell.Player setter
    /// </summary>
    internal void UpdateHashForCell(int x, int y, Player oldPlayer, Player newPlayer)
    {
        // Update BitBoard representation
        if (oldPlayer == Player.Red)
        {
            _redBitBoard.ClearBit(x, y);
        }
        else if (oldPlayer == Player.Blue)
        {
            _blueBitBoard.ClearBit(x, y);
        }

        if (newPlayer == Player.Red)
        {
            _redBitBoard.SetBit(x, y);
        }
        else if (newPlayer == Player.Blue)
        {
            _blueBitBoard.SetBit(x, y);
        }

        // Remove old player from hash
        if (oldPlayer != Player.None)
        {
            _hash ^= ZobristTables.GetKey(x, y, oldPlayer);
        }
        // Add new player to hash
        if (newPlayer != Player.None)
        {
            _hash ^= ZobristTables.GetKey(x, y, newPlayer);
        }
    }

    public void PlaceStone(int x, int y, Player player)
    {
        if (x < 0 || x >= Size || y < 0 || y >= Size)
            throw new ArgumentOutOfRangeException(nameof(x), "Position must be within board bounds");

        if (!_cells[x, y].IsEmpty)
            throw new InvalidOperationException("Cell is already occupied");

        _cells[x, y].Player = player;
    }

    public Cell GetCell(int x, int y)
    {
        if (x < 0 || x >= Size || y < 0 || y >= Size)
            throw new ArgumentOutOfRangeException(nameof(x), "Position must be within board bounds");

        return _cells[x, y];
    }

    /// <summary>
    /// Create a deep copy of the board for parallel search
    /// Each thread in Lazy SMP needs its own board copy to avoid concurrent modification
    /// </summary>
    public Board Clone()
    {
        var clone = new Board();

        // Copy cell states and bitboards
        for (int x = 0; x < Size; x++)
        {
            for (int y = 0; y < Size; y++)
            {
                var player = _cells[x, y].Player;
                if (player != Player.None)
                {
                    clone._cells[x, y].SetPlayerDirect(player);
                    // Update bitboards manually
                    if (player == Player.Red)
                        clone._redBitBoard.SetBit(x, y);
                    else if (player == Player.Blue)
                        clone._blueBitBoard.SetBit(x, y);
                }
            }
        }

        // Copy hash directly
        clone._hash = _hash;

        return clone;
    }
}
