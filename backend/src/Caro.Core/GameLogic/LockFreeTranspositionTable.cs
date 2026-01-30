using System.Runtime.CompilerServices;
using System.Threading;
using Caro.Core.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Lock-free transposition table for parallel search (Lazy SMP)
/// Uses atomic operations and memory barriers for thread safety without locks
/// TT SHARDING: Partitioned into segments to reduce cache line contention
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

    // TT SHARDING: Multiple segments to reduce cache line contention
    // Each segment is a separate array, reducing contention between threads
    private readonly TranspositionEntry?[][] _shards;
    private readonly int _shardCount;
    private readonly int _shardMask;
    private readonly int _sizePerShard;
    private int _currentAge;
    private int _hitCount;
    private int _lookupCount;

    /// <summary>
    /// Create a lock-free transposition table with sharding for reduced contention
    /// </summary>
    /// <param name="sizeMB">Size in MB (default 256MB = ~8M entries)</param>
    /// <param name="shardCount">Number of shards (default 16, must be power of 2)</param>
    public LockFreeTranspositionTable(int sizeMB = 256, int shardCount = 16)
    {
        // Validate shard count is power of 2
        if ((shardCount & (shardCount - 1)) != 0)
            shardCount = 16;

        _shardCount = shardCount;
        _shardMask = shardCount - 1;

        // Each entry is 16 bytes (8 hash + 8 data)
        int totalEntries = (sizeMB * 1024 * 1024) / 16;
        _sizePerShard = totalEntries / shardCount;

        // Create separate arrays for each shard
        _shards = new TranspositionEntry[shardCount][];
        for (int i = 0; i < shardCount; i++)
        {
            _shards[i] = new TranspositionEntry[_sizePerShard];
        }

        _currentAge = 1;
    }

    /// <summary>
    /// Calculate shard and index from hash
    /// TT SHARDING: Uses high bits of hash for shard selection to distribute entries evenly
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private (int shardIndex, int entryIndex) GetShardAndIndex(ulong hash)
    {
        // Use high bits for shard, low bits for index within shard
        // This distributes entries across shards to reduce contention
        int shardIndex = (int)(hash >> 32) & _shardMask;
        int entryIndex = (int)(hash % (ulong)_sizePerShard);
        return (shardIndex, entryIndex);
    }

    /// <summary>
    /// Store a position in the transposition table (thread-safe)
    /// Uses atomic Interlocked.Exchange for lock-free write
    /// IDENTICAL THREAD LOGIC: All threads (master and helper) use same write policy
    /// TT SHARDING: Uses separate shard arrays to reduce cache line contention
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public void Store(ulong hash, sbyte depth, short score, sbyte moveX, sbyte moveY, int alpha, int beta, byte threadIndex = 0, int rootDepth = 1)
    {
        var (shardIndex, entryIndex) = GetShardAndIndex(hash);
        var shard = _shards[shardIndex];

        // ALL THREADS use identical logic - only difference is threadIndex for tracking
        // No special restrictions for helper threads

        // Determine flag based on score relative to alpha/beta
        EntryFlag entryFlag;
        if (score <= alpha)
            entryFlag = EntryFlag.UpperBound;
        else if (score >= beta)
            entryFlag = EntryFlag.LowerBound;
        else
            entryFlag = EntryFlag.Exact;

        var newEntry = new TranspositionEntry(hash, depth, score, moveX, moveY, entryFlag, (byte)_currentAge, threadIndex);
        var existing = Volatile.Read(ref shard[entryIndex]);

        // Deep replacement strategy (lock-free) with EQUAL THREAD PRIORITY:
        // ALL THREADS compete equally - master has no special priority
        // Only difference is threadIndex tracks provenance for debugging
        //
        // Replace if:
        // 1. Empty slot
        // 2. Same position with deeper search
        // 3. New entry is significantly deeper (depth diff >= 2)
        // 4. Old entry is from a previous search age

        bool shouldStore = existing is null || existing.Hash == 0 || existing.Hash == hash;

        if (!shouldStore && existing is not null && existing.Hash != 0 && existing.Hash != hash)
        {
            // Different position - check deep replacement criteria
            // ALL THREADS use same criteria - no master priority
            sbyte depthDiff = (sbyte)(depth - existing.Depth);
            shouldStore = depthDiff >= 2 || existing.Age != _currentAge;
        }

        if (shouldStore)
        {
            // Atomic write - may overwrite another thread's write, but that's acceptable
            Interlocked.Exchange(ref shard[entryIndex], newEntry);
        }
    }

    /// <summary>
    /// Look up a position in the transposition table (thread-safe)
    /// Returns entry provenance (threadIndex) for selective reading
    /// For Lazy SMP: Also returns entries found at shallower depths for move ordering
    /// TT SHARDING: Uses separate shard arrays to reduce cache line contention
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public (bool found, bool hasExactDepth, short score, (int x, int y)? move, byte threadIndex) Lookup(ulong hash, sbyte depth, int alpha, int beta)
    {
        Interlocked.Increment(ref _lookupCount);
        var (shardIndex, entryIndex) = GetShardAndIndex(hash);
        var shard = _shards[shardIndex];
        var entry = Volatile.Read(ref shard[entryIndex]);

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
        for (int i = 0; i < _shardCount; i++)
        {
            Array.Clear(_shards[i], 0, _sizePerShard);
        }
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
        int totalSize = _sizePerShard * _shardCount;

        for (int s = 0; s < _shardCount; s++)
        {
            var shard = _shards[s];
            for (int i = 0; i < _sizePerShard; i++)
            {
                var entry = Volatile.Read(ref shard[i]);
                if (entry is not null && entry.IsValid && entry.Age == _currentAge)
                    used++;
            }
        }

        // Use Interlocked.CompareExchange to get current value without volatile read
        int hits = Interlocked.CompareExchange(ref _hitCount, 0, 0);
        int lookups = Interlocked.CompareExchange(ref _lookupCount, 0, 0);
        double hitRate = lookups > 0 ? (double)hits / lookups * 100 : 0;

        return (used, (double)used / totalSize * 100, hits, lookups, hitRate);
    }

    /// <summary>
    /// Get the table size in entries
    /// </summary>
    public int Size => _sizePerShard * _shardCount;

    /// <summary>
    /// Get current age counter
    /// </summary>
    public int CurrentAge => _currentAge;
}
