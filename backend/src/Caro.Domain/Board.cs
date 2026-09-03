namespace Caro.Domain;

/// <summary>
/// Immutable 16x16 board. Stone placement returns a new Board; the bitboards
/// and Zobrist hash are maintained incrementally per placement.
/// </summary>
public sealed class Board
{
    private readonly Player[] _cells;
    private readonly ulong[] _redBits;
    private readonly ulong[] _blueBits;

    private Board(Player[] cells, ulong[] redBits, ulong[] blueBits, ulong hash)
    {
        _cells = cells;
        _redBits = redBits;
        _blueBits = blueBits;
        Hash = hash;
    }

    public ulong Hash { get; }

    public static Board NewBoard() =>
        new(new Player[Constants.Board.Size * Constants.Board.Size], new ulong[4], new ulong[4], 0);

    public Cell GetCell(int x, int y)
    {
        if (x < 0 || x >= Constants.Board.Size || y < 0 || y >= Constants.Board.Size)
        {
            return new Cell(x, y, Player.None);
        }
        return new Cell(x, y, _cells[x * Constants.Board.Size + y]);
    }

    public bool IsEmpty()
    {
        for (int i = 0; i < _redBits.Length; i++)
        {
            if (_redBits[i] != 0 || _blueBits[i] != 0)
            {
                return false;
            }
        }
        return true;
    }

    public bool IsEmptyAt(int x, int y)
    {
        if (x < 0 || x >= Constants.Board.Size || y < 0 || y >= Constants.Board.Size)
        {
            return false;
        }
        return _cells[x * Constants.Board.Size + y] == Player.None;
    }

    public Player GetPlayerAt(int x, int y)
    {
        if (x < 0 || x >= Constants.Board.Size || y < 0 || y >= Constants.Board.Size)
        {
            return Player.None;
        }
        return _cells[x * Constants.Board.Size + y];
    }

    public ulong[] BitBoardBits(Player player)
    {
        ulong[] source = player == Player.Red ? _redBits : _blueBits;
        return (ulong[])source.Clone();
    }

    /// <summary>
    /// Places a stone and returns the new board. Throws
    /// <see cref="PositionBoundsException"/> or <see cref="CellOccupiedException"/>
    /// when the placement is invalid.
    /// </summary>
    public Board PlaceStone(int x, int y, Player player)
    {
        if (x < 0 || x >= Constants.Board.Size || y < 0 || y >= Constants.Board.Size)
        {
            throw new PositionBoundsException();
        }
        if (_cells[x * Constants.Board.Size + y] != Player.None)
        {
            throw new CellOccupiedException();
        }

        Player[] cells = (Player[])_cells.Clone();
        cells[x * Constants.Board.Size + y] = player;

        ulong[] redBits = (ulong[])_redBits.Clone();
        ulong[] blueBits = (ulong[])_blueBits.Clone();

        int bitIndex = y * Constants.Board.Size + x;
        int ulongIndex = bitIndex >> 6;
        int bitOffset = bitIndex & 63;
        ulong bitMask = 1UL << bitOffset;

        if (player == Player.Red)
        {
            redBits[ulongIndex] |= bitMask;
        }
        else
        {
            blueBits[ulongIndex] |= bitMask;
        }

        return new Board(cells, redBits, blueBits, Hash ^ Zobrist.ZobristKey(x, y, player));
    }
}
