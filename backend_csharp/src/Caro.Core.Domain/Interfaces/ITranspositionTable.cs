using Caro.Core.Domain.ValueObjects;

namespace Caro.Core.Domain.Interfaces;

/// <summary>
/// Interface for transposition table (position cache) in alpha-beta search.
/// Provides per-game scoped caching to prevent cross-game pollution.
/// </summary>
public interface ITranspositionTable : IDisposable
{
    /// <summary>
    /// Size of the transposition table in megabytes
    /// </summary>
    int SizeInMB { get; }

    /// <summary>
    /// Current age counter (for replacement strategy)
    /// </summary>
    byte Age { get; }

    /// <summary>
    /// Number of entries currently stored
    /// </summary>
    int EntryCount { get; }

    /// <summary>
    /// Total number of lookup operations
    /// </summary>
    int LookupCount { get; }

    /// <summary>
    /// Number of successful lookups (cache hits)
    /// </summary>
    int HitCount { get; }

    /// <summary>
    /// Hit rate (0-1)
    /// </summary>
    double HitRate => LookupCount > 0 ? (double)HitCount / LookupCount : 0;

    /// <summary>
    /// Lookup an entry in the transposition table
    /// </summary>
    TTEntry? Lookup(ZobristHash hash, int depth);

    /// <summary>
    /// Store an entry in the transposition table
    /// </summary>
    void Store(ZobristHash hash, int depth, int score, TTMove bestMove, TTFlag flag, byte age);

    /// <summary>
    /// Increment the age counter (call at start of each search)
    /// </summary>
    void IncrementAge();

    /// <summary>
    /// Clear all entries from the table
    /// </summary>
    void Clear();

    /// <summary>
    /// Get statistics about the table
    /// </summary>
    TTStats GetStats();
}

/// <summary>
/// Transposition table entry
/// </summary>
public sealed record TTEntry(
    ZobristHash Hash,
    int Depth,
    int Score,
    TTMove BestMove,
    TTFlag Flag,
    byte Age
);

/// <summary>
/// Move stored in transposition table (compact representation)
/// </summary>
public readonly record struct TTMove(int X, int Y)
{
    public static readonly TTMove None = new(-1, -1);

    public readonly bool IsValid => X >= 0 && Y >= 0;

    public readonly bool IsNone => X < 0 || Y < 0;

    public readonly void Deconstruct(out int x, out int y) => (x, y) = (X, Y);
}

/// <summary>
/// Transposition table entry flag
/// </summary>
public enum TTFlag : byte
{
    /// <summary>Exact score within the window</summary>
    Exact,

    /// <summary>Lower bound (score >= entry score)</summary>
    LowerBound,

    /// <summary>Upper bound (score <= entry score)</summary>
    UpperBound
}

/// <summary>
/// Transposition table statistics
/// </summary>
public sealed record TTStats(
    int SizeInMB,
    int EntryCount,
    int LookupCount,
    int HitCount,
    double HitRate,
    byte Age
);
