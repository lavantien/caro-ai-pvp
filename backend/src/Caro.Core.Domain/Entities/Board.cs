using System.Runtime.CompilerServices;
using Caro.Core.Domain.ValueObjects;
using Caro.Core.Domain.Exceptions;

namespace Caro.Core.Domain.Entities;

/// <summary>
/// Immutable Board representing the 19x19 Caro game board.
/// All operations return new instances - the original board is never modified.
/// Uses hybrid representation: Player[,] for cell access and BitBoard for fast AI operations.
/// Equality is based solely on the Zobrist hash for fast comparison.
/// </summary>
public sealed class Board : IEquatable<Board>
{
    private const int Size = 19;
    private readonly ulong _hash;
    private readonly Player[,] _cells;
    private readonly BitBoard _redBitBoard;
    private readonly BitBoard _blueBitBoard;

    /// <summary>
    /// Private constructor with all parameters
    /// </summary>
    private Board(
        ulong hash,
        Player[,] cells,
        BitBoard redBitBoard,
        BitBoard blueBitBoard)
    {
        _hash = hash;
        _cells = cells;
        _redBitBoard = redBitBoard;
        _blueBitBoard = blueBitBoard;
    }

    /// <summary>
    /// Board size (19x19)
    /// </summary>
    public static int BoardSize => Size;

    /// <summary>
    /// Total number of cells
    /// </summary>
    public static int TotalCells => Size * Size;

    /// <summary>
    /// Get the Zobrist hash of the board position
    /// </summary>
    public ulong Hash => _hash;

    /// <summary>
    /// Get the Red BitBoard (for fast AI operations)
    /// </summary>
    public BitBoard RedBitBoard => _redBitBoard;

    /// <summary>
    /// Get the Blue BitBoard (for fast AI operations)
    /// </summary>
    public BitBoard BlueBitBoard => _blueBitBoard;

    /// <summary>
    /// Get the BitBoard for a specific player
    /// </summary>
    public BitBoard GetBitBoard(Player player) => player switch
    {
        Player.Red => _redBitBoard,
        Player.Blue => _blueBitBoard,
        _ => BitBoard.Empty
    };

    /// <summary>
    /// Get the combined BitBoard (all stones)
    /// </summary>
    public BitBoard AllStones => _redBitBoard | _blueBitBoard;

    /// <summary>
    /// Create an empty board
    /// </summary>
    public static Board CreateEmpty()
    {
        return new Board(
            ZobristTables.GetInitialHash(),
            new Player[Size, Size],
            BitBoard.Empty,
            BitBoard.Empty
        );
    }

    /// <summary>
    /// Get the player at the given position
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public Player GetCell(int x, int y)
    {
        if (x < 0 || x >= Size || y < 0 || y >= Size)
            return Player.None;

        return _cells[x, y];
    }

    /// <summary>
    /// Get the player at the given position
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public Player GetCell(Position position) =>
        position.IsValid ? _cells[position.X, position.Y] : Player.None;

    /// <summary>
    /// Check if a cell is empty
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public bool IsEmpty(int x, int y)
    {
        return x >= 0 && x < Size && y >= 0 && y < Size && _cells[x, y] == Player.None;
    }

    /// <summary>
    /// Check if a cell is empty
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public bool IsEmpty(Position position) =>
        position.IsValid && _cells[position.X, position.Y] == Player.None;

    /// <summary>
    /// Place a stone and return a new board (immutable operation)
    /// </summary>
    public Board PlaceStone(int x, int y, Player player)
    {
        if (!player.IsValid())
            throw new InvalidMoveException($"Player must be Red or Blue, got {player}");

        if (x < 0 || x >= Size || y < 0 || y >= Size)
            throw new InvalidMoveException($"Position ({x}, {y}) is out of bounds");

        if (_cells[x, y] != Player.None)
            throw new InvalidMoveException($"Cell ({x}, {y}) is already occupied");

        // Create new cell array
        var newCells = (Player[,])_cells.Clone();
        newCells[x, y] = player;

        // Update bitboards
        var newRedBitBoard = _redBitBoard;
        var newBlueBitBoard = _blueBitBoard;

        if (player == Player.Red)
            newRedBitBoard = _redBitBoard.SetBit(x, y);
        else
            newBlueBitBoard = _blueBitBoard.SetBit(x, y);

        // Update hash incrementally
        var newHash = Hash ^ ZobristTables.GetKey(x, y, player);

        return new Board(newHash, newCells, newRedBitBoard, newBlueBitBoard);
    }

    /// <summary>
    /// Place a stone and return a new board (immutable operation)
    /// </summary>
    public Board PlaceStone(Position position, Player player) =>
        PlaceStone(position.X, position.Y, player);

    /// <summary>
    /// Place a stone from a move and return a new board
    /// </summary>
    public Board PlaceStone(Move move) =>
        PlaceStone(move.X, move.Y, move.Player);

    /// <summary>
    /// Remove a stone and return a new board (for undo operations)
    /// </summary>
    public Board RemoveStone(int x, int y)
    {
        if (x < 0 || x >= Size || y < 0 || y >= Size)
            throw new InvalidMoveException($"Position ({x}, {y}) is out of bounds");

        var player = _cells[x, y];
        if (player == Player.None)
            return this; // No change needed

        // Create new cell array
        var newCells = (Player[,])_cells.Clone();
        newCells[x, y] = Player.None;

        // Update bitboards
        var newRedBitBoard = _redBitBoard;
        var newBlueBitBoard = _blueBitBoard;

        if (player == Player.Red)
            newRedBitBoard = _redBitBoard.ClearBit(x, y);
        else
            newBlueBitBoard = _blueBitBoard.ClearBit(x, y);

        // Update hash incrementally
        var newHash = Hash ^ ZobristTables.GetKey(x, y, player);

        return new Board(newHash, newCells, newRedBitBoard, newBlueBitBoard);
    }

    /// <summary>
    /// Remove a stone and return a new board
    /// </summary>
    public Board RemoveStone(Position position) => RemoveStone(position.X, position.Y);

    /// <summary>
    /// Apply multiple moves and return a new board
    /// </summary>
    public Board ApplyMoves(IEnumerable<Move> moves)
    {
        var board = this;
        foreach (var move in moves)
        {
            board = board.PlaceStone(move);
        }
        return board;
    }

    /// <summary>
    /// Get all empty cells on the board
    /// </summary>
    public IEnumerable<Position> GetEmptyCells()
    {
        for (int x = 0; x < Size; x++)
        {
            for (int y = 0; y < Size; y++)
            {
                if (_cells[x, y] == Player.None)
                    yield return new Position(x, y);
            }
        }
    }

    /// <summary>
    /// Get all cells occupied by the given player
    /// </summary>
    public IEnumerable<Position> GetOccupiedCells(Player player)
    {
        var bitBoard = GetBitBoard(player);
        for (int y = 0; y < Size; y++)
        {
            for (int x = 0; x < Size; x++)
            {
                if (bitBoard.GetBit(x, y))
                    yield return new Position(x, y);
            }
        }
    }

    /// <summary>
    /// Count stones for a given player
    /// </summary>
    public int CountStones(Player player) => GetBitBoard(player).CountBits();

    /// <summary>
    /// Count total stones on the board
    /// </summary>
    public int TotalStones() => _redBitBoard.CountBits() + _blueBitBoard.CountBits();

    /// <summary>
    /// Check if the board is empty
    /// </summary>
    public bool IsEmpty() => TotalStones() == 0;

    /// <summary>
    /// Check if the board is full
    /// </summary>
    public bool IsFull() => TotalStones() == TotalCells;

    /// <summary>
    /// Get a string representation of the board
    /// </summary>
    public override string ToString()
    {
        var sb = new System.Text.StringBuilder();
        sb.AppendLine("  0123456789012345678");
        for (int y = 0; y < Size; y++)
        {
            sb.Append(y.ToString().PadLeft(2));
            for (int x = 0; x < Size; x++)
            {
                sb.Append(_cells[x, y] switch
                {
                    Player.None => '.',
                    Player.Red => 'R',
                    Player.Blue => 'B',
                    _ => '?'
                });
            }
            sb.AppendLine();
        }
        return sb.ToString();
    }

    /// <summary>
    /// Override equality to only compare hash (not internal arrays)
    /// </summary>
    public bool Equals(Board? other)
    {
        if (other is null) return false;
        if (ReferenceEquals(this, other)) return true;
        return Hash == other.Hash;
    }

    /// <summary>
    /// Override equality to only compare hash (not internal arrays)
    /// </summary>
    public override bool Equals(object? obj) => Equals(obj as Board);

    /// <summary>
    /// Hash code is the Zobrist hash
    /// </summary>
    public override int GetHashCode() => Hash.GetHashCode();
}
