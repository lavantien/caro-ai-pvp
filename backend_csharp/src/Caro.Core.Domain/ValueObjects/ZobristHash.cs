using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;

namespace Caro.Core.Domain.ValueObjects;

/// <summary>
/// Zobrist hash for board position identification.
/// Provides XOR-based incremental hashing for fast position comparison.
/// </summary>
public readonly record struct ZobristHash(ulong Value)
{
    /// <summary>
    /// Empty hash (all zeros)
    /// </summary>
    public static readonly ZobristHash Empty = new(0);

    /// <summary>
    /// XOR with another hash (for incremental updates)
    /// </summary>
    [System.Runtime.CompilerServices.MethodImpl(System.Runtime.CompilerServices.MethodImplOptions.AggressiveInlining)]
    public readonly ZobristHash Xor(ZobristHash other) => new(Value ^ other.Value);

    /// <summary>
    /// XOR with a ulong value
    /// </summary>
    [System.Runtime.CompilerServices.MethodImpl(System.Runtime.CompilerServices.MethodImplOptions.AggressiveInlining)]
    public readonly ZobristHash Xor(ulong other) => new(Value ^ other);

    /// <summary>
    /// Combine multiple hashes using XOR
    /// </summary>
    public static ZobristHash Combine(params ZobristHash[] hashes)
    {
        ulong result = 0;
        foreach (var hash in hashes)
        {
            result ^= hash.Value;
        }
        return new ZobristHash(result);
    }

    /// <summary>
    /// Check if the hash is empty
    /// </summary>
    public readonly bool IsEmpty => Value == 0;

    /// <summary>
    /// Implicit conversion to ulong
    /// </summary>
    public static implicit operator ulong(ZobristHash hash) => hash.Value;

    /// <summary>
    /// Implicit conversion from ulong
    /// </summary>
    public static implicit operator ZobristHash(ulong value) => new(value);

    public override readonly string ToString() => $"0x{Value:X16}";
}

/// <summary>
/// Zobrist table for generating random keys for board positions.
/// Uses a fixed seed for reproducibility.
/// </summary>
public sealed class ZobristTable
{
    private const int BoardSize = GameConstants.BoardSize;
    private readonly ulong[,] _redKeys;
    private readonly ulong[,] _blueKeys;
    private readonly ulong _initialHash;

    /// <summary>
    /// Create a new Zobrist table with the given seed.
    /// Use a fixed seed (e.g., 42) for reproducibility.
    /// </summary>
    public ZobristTable(int seed = 42)
    {
        var random = new Random(seed);
        _redKeys = new ulong[BoardSize, BoardSize];
        _blueKeys = new ulong[BoardSize, BoardSize];

        var buffer = new byte[8];
        for (int x = 0; x < BoardSize; x++)
        {
            for (int y = 0; y < BoardSize; y++)
            {
                _redKeys[x, y] = RandomUInt64(random);
                _blueKeys[x, y] = RandomUInt64(random);
            }
        }

        // Initial hash for empty board
        _initialHash = RandomUInt64(random);
    }

    /// <summary>
    /// Get the Zobrist key for placing a stone at (x, y) for the given player
    /// </summary>
    public ulong GetKey(int x, int y, Player player)
    {
        if (x < 0 || x >= BoardSize || y < 0 || y >= BoardSize)
            throw new ArgumentOutOfRangeException(nameof(x), "Position must be within board bounds");

        return player == Player.Red ? _redKeys[x, y] : _blueKeys[x, y];
    }

    /// <summary>
    /// Get the initial hash for an empty board
    /// </summary>
    public ulong GetInitialHash() => _initialHash;

    /// <summary>
    /// Calculate the Zobrist hash for a complete board position
    /// </summary>
    public ulong CalculateHash(IBoard board)
    {
        ulong hash = _initialHash;
        for (int x = 0; x < BoardSize; x++)
        {
            for (int y = 0; y < BoardSize; y++)
            {
                var player = board.GetCell(x, y).Player;
                if (player == Player.Red)
                {
                    hash ^= _redKeys[x, y];
                }
                else if (player == Player.Blue)
                {
                    hash ^= _blueKeys[x, y];
                }
            }
        }
        return hash;
    }

    private static ulong RandomUInt64(Random random)
    {
        var bytes = new byte[8];
        random.NextBytes(bytes);
        return BitConverter.ToUInt64(bytes, 0);
    }
}

/// <summary>
/// Default Zobrist table instance using seed 42
/// </summary>
public static class ZobristTables
{
    private static readonly Lazy<ZobristTable> _instance = new(() => new ZobristTable(42));

    /// <summary>
    /// Get the default Zobrist table instance
    /// </summary>
    public static ZobristTable Instance => _instance.Value;

    /// <summary>
    /// Get the Zobrist key for placing a stone at (x, y) for the given player
    /// </summary>
    public static ulong GetKey(int x, int y, Player player) => Instance.GetKey(x, y, player);

    /// <summary>
    /// Get the initial hash for an empty board
    /// </summary>
    public static ulong GetInitialHash() => Instance.GetInitialHash();
}
