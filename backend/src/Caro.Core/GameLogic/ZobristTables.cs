using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Static Zobrist hash tables for board position hashing
/// Uses fixed seed (42) for reproducibility across TranspositionTable and Board
/// Each board position (x, y) has two random 64-bit numbers: one for Red, one for Blue
/// </summary>
public static class ZobristTables
{
    private const int BoardSize = 32;
    private static readonly ulong[,] _redKeys = new ulong[BoardSize, BoardSize];
    private static readonly ulong[,] _blueKeys = new ulong[BoardSize, BoardSize];
    private static readonly Random _random = new(42); // Fixed seed for reproducibility

    static ZobristTables()
    {
        InitializeTables();
    }

    /// <summary>
    /// Initialize Zobrist tables with random 64-bit numbers
    /// </summary>
    private static void InitializeTables()
    {
        var buffer = new byte[8];
        for (int x = 0; x < BoardSize; x++)
        {
            for (int y = 0; y < BoardSize; y++)
            {
                _redKeys[x, y] = RandomUInt64();
                _blueKeys[x, y] = RandomUInt64();
            }
        }
    }

    /// <summary>
    /// Generate a random 64-bit integer
    /// </summary>
    private static ulong RandomUInt64()
    {
        var bytes = new byte[8];
        _random.NextBytes(bytes);
        return BitConverter.ToUInt64(bytes, 0);
    }

    /// <summary>
    /// Get the Zobrist key for placing a stone at (x, y) for the given player
    /// </summary>
    public static ulong GetKey(int x, int y, Player player)
    {
        if (x < 0 || x >= BoardSize || y < 0 || y >= BoardSize)
            throw new ArgumentOutOfRangeException(nameof(x), "Position must be within board bounds");

        return player == Player.Red ? _redKeys[x, y] : _blueKeys[x, y];
    }

    /// <summary>
    /// Calculate the Zobrist hash for a complete board position
    /// XOR of random numbers for each occupied cell
    /// </summary>
    public static ulong CalculateHash(Board board)
    {
        ulong hash = 0;
        for (int x = 0; x < BoardSize; x++)
        {
            for (int y = 0; y < BoardSize; y++)
            {
                var cell = board.GetCell(x, y);
                if (cell.Player == Player.Red)
                {
                    hash ^= _redKeys[x, y];
                }
                else if (cell.Player == Player.Blue)
                {
                    hash ^= _blueKeys[x, y];
                }
            }
        }
        return hash;
    }
}
