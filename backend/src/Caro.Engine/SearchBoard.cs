using Caro.Domain;

namespace Caro.Engine;

/// <summary>
/// Mutable search board with incremental Zobrist hashing and an undo stack.
/// The search makes and unmakes moves in place instead of copying boards.
/// </summary>
public sealed class SearchBoard
{
    private readonly Player[] _cells = new Player[Constants.Board.Size * Constants.Board.Size];
    private BitBoard _redBits;
    private BitBoard _blueBits;
    private ulong _hash;
    private int _stones;
    private UndoEntry[] _undoStack = new UndoEntry[Constants.Capacity.InitialUndoCapacity];
    private int _undoCount;

    private struct UndoEntry(int x, int y, Player player, ulong hash)
    {
        public int X = x;
        public int Y = y;
        public Player Player = player;
        public ulong Hash = hash;
    }

    public SearchBoard(Board b)
    {
        (_redBits, _blueBits) = BitBoard.BitBoardsFromDomain(b);
        _hash = b.Hash;

        for (int x = 0; x < Constants.Board.Size; x++)
        {
            for (int y = 0; y < Constants.Board.Size; y++)
            {
                Player p = b.GetPlayerAt(x, y);
                _cells[x * Constants.Board.Size + y] = p;
                if (p != Player.None)
                {
                    _stones++;
                }
            }
        }
    }

    /// <summary>Returns the number of stones on the board.</summary>
    public int StoneCount() => _stones;

    public ulong Hash() => _hash;

    public Player PlayerAt(int x, int y)
    {
        if (x < 0 || x >= Constants.Board.Size || y < 0 || y >= Constants.Board.Size)
        {
            return Player.None;
        }
        return _cells[x * Constants.Board.Size + y];
    }

    public BitBoard BitBoardFor(Player player) => player == Player.Red ? _redBits : _blueBits;

    public BitBoard Occupied() => _redBits.Or(_blueBits);

    public bool IsEmpty(int x, int y)
    {
        if (x < 0 || x >= Constants.Board.Size || y < 0 || y >= Constants.Board.Size)
        {
            return false;
        }
        return _cells[x * Constants.Board.Size + y] == Player.None;
    }

    public void MakeMove(int x, int y, Player player)
    {
        if (_undoCount == _undoStack.Length)
        {
            Array.Resize(ref _undoStack, _undoStack.Length * 2);
        }
        _undoStack[_undoCount++] = new UndoEntry(x, y, _cells[x * Constants.Board.Size + y], _hash);

        _cells[x * Constants.Board.Size + y] = player;
        if (player == Player.Red)
        {
            _redBits.Set(x, y);
        }
        else
        {
            _blueBits.Set(x, y);
        }
        _hash ^= Zobrist.ZobristKey(x, y, player);
        _stones++;
    }

    public void UnmakeMove()
    {
        UndoEntry entry = _undoStack[--_undoCount];

        Player currentPlayer = _cells[entry.X * Constants.Board.Size + entry.Y];
        if (currentPlayer == Player.Red)
        {
            _redBits.Clear(entry.X, entry.Y);
        }
        else if (currentPlayer == Player.Blue)
        {
            _blueBits.Clear(entry.X, entry.Y);
        }

        _cells[entry.X * Constants.Board.Size + entry.Y] = entry.Player;
        _hash = entry.Hash;
        _stones--;
    }

    public void MakeNullMove()
    {
        if (_undoCount == _undoStack.Length)
        {
            Array.Resize(ref _undoStack, _undoStack.Length * 2);
        }
        _undoStack[_undoCount++] = new UndoEntry(-1, -1, Player.None, _hash);
        _hash ^= Zobrist.ZobristNullMove();
    }

    public void UnmakeNullMove()
    {
        UndoEntry entry = _undoStack[--_undoCount];
        _hash = entry.Hash;
    }
}
