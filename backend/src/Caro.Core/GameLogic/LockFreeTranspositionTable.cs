using System.Runtime.CompilerServices;
using System.Threading;
using Caro.Core.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Lock-free transposition table for parallel search (Lazy SMP)
/// Uses atomic operations and memory barriers for thread safety without locks
/// </summary>
public sealed class LockFreeTranspositionTable
{
    /// <summary>
    /// Entry flags for transposition table
    /// </summary>
    public enum EntryFlag : byte
    {
        Exact = 0,       // Score is exact (alpha < score < beta)
        LowerBound = 1,  // Score is at least this value (beta cutoff)
        UpperBound = 2   // Score is at most this value (alpha cutoff)
    }

    /// <summary>
    /// Transposition table entry (class for thread-safe operations)
    /// Uses Interlocked.Exchange for lock-free updates
    /// </summary>
    public sealed class TranspositionEntry
    {
        // Layout optimized for cache efficiency
        public ulong Hash;
        public sbyte Depth;
        public short Score;
        public sbyte MoveX;
        public sbyte MoveY;
        public EntryFlag Flag;
        public byte Age;
        public byte ThreadIndex;  // Track which thread wrote this entry (0=master, 1+=helpers)

        public TranspositionEntry(ulong hash, sbyte depth, short score, sbyte moveX, sbyte moveY, EntryFlag flag, byte age, byte threadIndex = 0)
        {
            Hash = hash;
            Depth = depth;
            Score = score;
            MoveX = moveX;
            MoveY = moveY;
            Flag = flag;
            Age = age;
            ThreadIndex = threadIndex;
        }

        /// <summary>
        /// Check if this entry has a valid move stored
        /// </summary>
        public bool HasMove => MoveX >= 0 && MoveY >= 0;

        /// <summary>
        /// Check if this entry is valid (non-zero hash)
        /// </summary>
        public bool IsValid => Hash != 0;

        /// <summary>
        /// Get the best move as a tuple
        /// </summary>
        public (int x, int y)? GetMove() => HasMove ? ((int x, int y)?)(MoveX, MoveY) : null;
    }

    private readonly TranspositionEntry?[] _entries;
    private readonly int _size;
    private int _currentAge;
    private int _hitCount;
    private int _lookupCount;

    /// <summary>
    /// Create a lock-free transposition table
    /// </summary>
    /// <param name="sizeMB">Size in MB (default 256MB = ~8M entries)</param>
    public LockFreeTranspositionTable(int sizeMB = 256)
    {
        // Each entry is 16 bytes (8 hash + 8 data)
        _size = (sizeMB * 1024 * 1024) / 16;
        _entries = new TranspositionEntry[_size];
        _currentAge = 1;
    }

    /// <summary>
    /// Calculate table index from hash
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private int GetIndex(ulong hash) => (int)(hash % (ulong)_size);

    /// <summary>
    /// Store a position in the transposition table (thread-safe)
    /// Uses atomic Interlocked.Exchange for lock-free write
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public void Store(ulong hash, sbyte depth, short score, sbyte moveX, sbyte moveY, int alpha, int beta, byte threadIndex = 0)
    {
        int index = GetIndex(hash);

        // Determine flag based on score relative to alpha/beta
        EntryFlag flag;
        if (score <= alpha)
            flag = EntryFlag.UpperBound;
        else if (score >= beta)
            flag = EntryFlag.LowerBound;
        else
            flag = EntryFlag.Exact;

        var newEntry = new TranspositionEntry(hash, depth, score, moveX, moveY, flag, (byte)_currentAge, threadIndex);
        var existing = Volatile.Read(ref _entries[index]);

        // Deep replacement strategy (lock-free) with MASTER PRIORITY:
        // MASTER THREAD (threadIndex=0) entries are protected from helper overwrites
        // This ensures master's high-quality entries are not lost
        //
        // Replace if:
        // 1. Empty slot
        // 2. Same position with deeper search
        // 3. Master thread overwriting helper (same or greater depth)
        // 4. New entry is significantly deeper (depth diff >= 2)
        // 5. Old entry is from a previous search age

        bool shouldStore = existing is null || existing.Hash == 0 || existing.Hash == hash;

        if (!shouldStore && existing is not null && existing.Hash != 0 && existing.Hash != hash)
        {
            // Different position - check deep replacement criteria
            sbyte depthDiff = (sbyte)(depth - existing.Depth);

            // MASTER PRIORITY: Protect master entries from helper overwrites
            if (existing.ThreadIndex == 0 && threadIndex > 0)
            {
                // Helper thread CANNOT overwrite master's entry unless going much deeper
                // This preserves master's high-quality search results
                shouldStore = depthDiff >= 3;
            }
            else if (threadIndex == 0 && existing.ThreadIndex > 0)
            {
                // Master thread CAN overwrite helper's entry (same depth or deeper)
                // Master's results are more reliable
                shouldStore = depthDiff >= 0;
            }
            else
            {
                // Same thread type (both master or both helper) - use depth/age criteria
                shouldStore = depthDiff >= 2 || existing.Age != _currentAge;
            }
        }

        if (shouldStore)
        {
            // Atomic write - may overwrite another thread's write, but that's acceptable
            Interlocked.Exchange(ref _entries[index], newEntry);
        }
    }

    /// <summary>
    /// Look up a position in the transposition table (thread-safe)
    /// Returns entry provenance (threadIndex) for selective reading
    /// For Lazy SMP: Also returns entries found at shallower depths for move ordering
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public (bool found, bool hasExactDepth, short score, (int x, int y)? move, byte threadIndex) Lookup(ulong hash, sbyte depth, int alpha, int beta)
    {
        Interlocked.Increment(ref _lookupCount);
        int index = GetIndex(hash);
        var entry = Volatile.Read(ref _entries[index]);

        if (entry is null || entry.Hash != hash)
            return (false, false, 0, null, 0);

        // Check if entry has sufficient depth
        bool hasExactDepth = entry.Depth >= depth;
        byte threadIndex = entry.ThreadIndex;

        // If not at sufficient depth, only return for move ordering (not for cutoff)
        if (!hasExactDepth)
        {
            Interlocked.Increment(ref _hitCount);
            // Return found=true but hasExactDepth=false - caller can use move but not score
            return (true, false, entry.Score, entry.GetMove(), threadIndex);
        }

        Interlocked.Increment(ref _hitCount);

        // Check if we can use the cached score based on flag
        switch (entry.Flag)
        {
            case EntryFlag.Exact:
                return (true, true, entry.Score, entry.GetMove(), threadIndex);

            case EntryFlag.LowerBound:
                if (entry.Score >= beta)
                    return (true, true, entry.Score, entry.GetMove(), threadIndex);
                break;

            case EntryFlag.UpperBound:
                if (entry.Score <= alpha)
                    return (true, true, entry.Score, entry.GetMove(), threadIndex);
                break;
        }

        // Can't use the score, but can use the move for ordering
        return (true, false, entry.Score, entry.GetMove(), threadIndex);
    }

    /// <summary>
    /// Increment age for replacement strategy
    /// Should be called at the start of each new search
    /// </summary>
    public void IncrementAge()
    {
        Interlocked.Increment(ref _currentAge);
        if (_currentAge == 255)
        {
            Interlocked.Exchange(ref _currentAge, 1);
        }

        // Reset stats
        Interlocked.Exchange(ref _hitCount, 0);
        Interlocked.Exchange(ref _lookupCount, 0);
    }

    /// <summary>
    /// Clear the entire table
    /// </summary>
    public void Clear()
    {
        Array.Clear(_entries, 0, _entries.Length);
        _currentAge = 1;
        Interlocked.Exchange(ref _hitCount, 0);
        Interlocked.Exchange(ref _lookupCount, 0);
    }

    /// <summary>
    /// Get transposition table statistics
    /// </summary>
    public (int used, double usagePercent, int hitCount, int lookupCount, double hitRate) GetStats()
    {
        int used = 0;
        for (int i = 0; i < _size; i++)
        {
            var entry = Volatile.Read(ref _entries[i]);
            if (entry is not null && entry.IsValid && entry.Age == _currentAge)
                used++;
        }

        // Use Interlocked.CompareExchange to get current value without volatile read
        int hits = Interlocked.CompareExchange(ref _hitCount, 0, 0);
        int lookups = Interlocked.CompareExchange(ref _lookupCount, 0, 0);
        double hitRate = lookups > 0 ? (double)hits / lookups * 100 : 0;

        return (used, (double)used / _size * 100, hits, lookups, hitRate);
    }

    /// <summary>
    /// Get the table size in entries
    /// </summary>
    public int Size => _size;

    /// <summary>
    /// Get current age counter
    /// </summary>
    public int CurrentAge => _currentAge;
}
