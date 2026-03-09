using Caro.Core.Domain.Configuration;

namespace Caro.Core.Domain.Entities;

/// <summary>
/// Zobrist hashing table for board position hashing.
/// Used by both immutable Board and mutable SearchBoard.
///
/// Zobrist hashing provides excellent distribution with ~50% bit flip probability per piece,
/// making collisions extremely unlikely (1 in 2^64 for random positions).
/// </summary>
public static class Zobrist
{
    private const int Size = GameConstants.BoardSize;

    // Zobrist hashing table: [x, y, player] -> random 64-bit key
    // player: 0 = Red, 1 = Blue
    private static readonly ulong[,,] Table = InitializeTable();

    /// <summary>
    /// Get the Zobrist key for a piece at the given position.
    /// </summary>
    /// <param name="x">X coordinate (0-15)</param>
    /// <param name="y">Y coordinate (0-15)</param>
    /// <param name="player">Player (Red or Blue)</param>
    /// <returns>64-bit Zobrist key</returns>
    public static ulong GetKey(int x, int y, Player player)
    {
        int playerIndex = player == Player.Red ? 0 : 1;
        return Table[x, y, playerIndex];
    }

    /// <summary>
    /// Initialize Zobrist table with deterministic pseudo-random values.
    /// Uses SplitMix64 PRNG for reproducible high-quality random numbers.
    /// </summary>
    private static ulong[,,] InitializeTable()
    {
        var table = new ulong[Size, Size, 2];
        var rng = new SplitMix64(0x58A2C43F5A3B7E91UL);  // Fixed seed for reproducibility

        for (int x = 0; x < Size; x++)
        {
            for (int y = 0; y < Size; y++)
            {
                table[x, y, 0] = rng.Next();  // Red
                table[x, y, 1] = rng.Next();  // Blue
            }
        }

        return table;
    }

    /// <summary>
    /// SplitMix64 PRNG for generating Zobrist keys.
    /// High-quality 64-bit output with good bit distribution.
    /// </summary>
    private sealed class SplitMix64
    {
        private ulong _state;

        public SplitMix64(ulong seed)
        {
            _state = seed;
        }

        public ulong Next()
        {
            _state += 0x9E3779B97F4A7C15UL;
            var z = _state;
            z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9UL;
            z = (z ^ (z >> 27)) * 0x94D049BB133111EBUL;
            return z ^ (z >> 31);
        }
    }
}
