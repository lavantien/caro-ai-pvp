using System.Runtime.CompilerServices;
using System.Runtime.InteropServices;
using System.Threading;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Lock-free transposition table for parallel search (Lazy SMP)
/// Uses SeqLock pattern for atomic reads of entries with version-based protection
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
    /// Transposition table entry using SeqLock pattern for torn-read protection.
    ///
    /// Layout (20 bytes total):
    /// - Hash (8 bytes): 64-bit Zobrist hash
    /// - Data (4 bytes): Packed Score(16) + Depth(8) + MoveX(4) + MoveY(4)
    /// - Meta (4 bytes): Age(8) + Flag(8) + ThreadIndex(8) - simplified byte fields
    /// - Version (4 bytes): SeqLock version counter (odd=writing, even=stable)
    ///
    /// SeqLock protocol:
    /// - Writer: Increment Version to odd, write all fields, increment Version to even
    /// - Reader: Read Version, copy entry, verify Version unchanged (retry if changed)
    ///
    /// This guarantees consistent reads without locks.
    /// </summary>
    [StructLayout(LayoutKind.Explicit, Size = 20)]
    public struct TranspositionEntry
    {
        [FieldOffset(0)] public ulong Hash;
        [FieldOffset(8)] public uint Data;         // Packed: Score(16) + Depth(8) + MoveX(4) + MoveY(4)
        [FieldOffset(12)] public uint Meta;        // Age(8) + Flag(8) + ThreadIndex(8) + Reserved(8)
        [FieldOffset(16)] public uint Version;     // SeqLock version (odd=writing, even=stable)

        // Bit positions for Data field packing
        private const int ScoreShift = 16;
        private const int DepthShift = 8;
        private const int MoveXShift = 4;
        private const int MoveYShift = 0;

        // Bit positions for Meta field packing
        private const int ThreadIndexShift = 0;
        private const int FlagShift = 8;
        private const int AgeShift = 16;

        public TranspositionEntry(ulong hash, sbyte depth, short score, sbyte moveX, sbyte moveY, EntryFlag flag, byte age, byte threadIndex = 0)
        {
            Hash = hash;
            Version = 0; // Start at 0 (even = stable)
            Data = PackData(score, depth, moveX, moveY);
            Meta = PackMeta(age, flag, threadIndex);
        }

        /// <summary>
        /// Pack Score, Depth, MoveX, MoveY into 32-bit Data field
        /// Layout: Score(16) | Depth(8) | MoveX(4) | MoveY(4)
        /// </summary>
        [MethodImpl(MethodImplOptions.AggressiveInlining)]
        private static uint PackData(short score, sbyte depth, sbyte moveX, sbyte moveY)
        {
            uint packed = 0;
            packed |= ((uint)(ushort)score) << ScoreShift;
            packed |= ((uint)(byte)depth) << DepthShift;
            packed |= ((uint)(moveX & 0x0F)) << MoveXShift;
            packed |= ((uint)(moveY & 0x0F)) << MoveYShift;
            return packed;
        }

        /// <summary>
        /// Pack Age, Flag, ThreadIndex into 32-bit Meta field
        /// Layout: Reserved(8) | Age(8) | Flag(8) | ThreadIndex(8)
        /// </summary>
        [MethodImpl(MethodImplOptions.AggressiveInlining)]
        private static uint PackMeta(byte age, EntryFlag flag, byte threadIndex)
        {
            uint packed = 0;
            packed |= ((uint)age) << AgeShift;
            packed |= ((uint)flag) << FlagShift;
            packed |= ((uint)threadIndex) << ThreadIndexShift;
            return packed;
        }

        /// <summary>
        /// Unpack score from Data field (16-bit signed)
        /// </summary>
        public short Score
        {
            [MethodImpl(MethodImplOptions.AggressiveInlining)]
            get => (short)((Data >> ScoreShift) & 0xFFFF);
        }

        /// <summary>
        /// Unpack depth from Data field (8-bit signed)
        /// </summary>
        public sbyte Depth
        {
            [MethodImpl(MethodImplOptions.AggressiveInlining)]
            get => (sbyte)((Data >> DepthShift) & 0xFF);
        }

        /// <summary>
        /// Unpack MoveX from Data field (4-bit, 0-15)
        /// </summary>
        public byte MoveX
        {
            [MethodImpl(MethodImplOptions.AggressiveInlining)]
            get => (byte)((Data >> MoveXShift) & 0x0F);
        }

        /// <summary>
        /// Unpack MoveY from Data field (4-bit, 0-15)
        /// </summary>
        public byte MoveY
        {
            [MethodImpl(MethodImplOptions.AggressiveInlining)]
            get => (byte)((Data >> MoveYShift) & 0x0F);
        }

        /// <summary>
        /// Unpack age from Meta field
        /// </summary>
        public byte Age
        {
            [MethodImpl(MethodImplOptions.AggressiveInlining)]
            get => (byte)((Meta >> AgeShift) & 0xFF);
        }

        /// <summary>
        /// Unpack flag from Meta field
        /// </summary>
        public EntryFlag Flag
        {
            [MethodImpl(MethodImplOptions.AggressiveInlining)]
            get => (EntryFlag)((Meta >> FlagShift) & 0xFF);
        }

        /// <summary>
        /// Unpack thread index from Meta field
        /// </summary>
        public byte ThreadIndex
        {
            [MethodImpl(MethodImplOptions.AggressiveInlining)]
            get => (byte)((Meta >> ThreadIndexShift) & 0xFF);
        }

        /// <summary>
        /// Check if this entry has a valid move stored
        /// </summary>
        public bool HasMove => MoveX < 16 && MoveY < 16;

        /// <summary>
        /// Check if this entry is valid (non-zero hash)
        /// </summary>
        public bool IsValid => Hash != 0;

        /// <summary>
        /// Get the best move as a tuple
        /// </summary>
        public (int x, int y)? GetMove() => HasMove ? ((int x, int y)?)(MoveX, MoveY) : null;

        /// <summary>
        /// Fast hash verification using high 32 bits comparison.
        /// Catches >99.99% of mismatches with minimal overhead.
        /// </summary>
        [MethodImpl(MethodImplOptions.AggressiveInlining)]
        public readonly bool MatchesHash(ulong hash)
        {
            return (Hash >> 32) == (hash >> 32);
        }
    }

    // TT SHARDING: Multiple segments to reduce cache line contention
    private readonly TranspositionEntry[][] _shards;
    private readonly int _shardCount;
    private readonly int _shardMask;
    private readonly int _sizePerShard;
    private int _currentAge;
    private int _hitCount;
    private int _lookupCount;

    /// <summary>
    /// Create a lock-free transposition table with sharding for reduced contention
    /// </summary>
    public LockFreeTranspositionTable(int sizeMB = 256, int shardCount = 16)
    {
        if ((shardCount & (shardCount - 1)) != 0)
            shardCount = 16;

        _shardCount = shardCount;
        _shardMask = shardCount - 1;

        // Each entry is 20 bytes
        int totalEntries = (sizeMB * 1024 * 1024) / 20;
        _sizePerShard = totalEntries / shardCount;

        _shards = new TranspositionEntry[shardCount][];
        for (int i = 0; i < shardCount; i++)
        {
            _shards[i] = new TranspositionEntry[_sizePerShard];
        }

        _currentAge = 1;
    }

    /// <summary>
    /// Calculate shard and index from hash
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private (int shardIndex, int entryIndex) GetShardAndIndex(ulong hash)
    {
        int shardIndex = (int)(hash >> 32) & _shardMask;
        int entryIndex = (int)(hash % (ulong)_sizePerShard);
        return (shardIndex, entryIndex);
    }

    /// <summary>
    /// Store a position in the transposition table using SeqLock pattern.
    /// Thread-safe without explicit locks.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public void Store(ulong hash, sbyte depth, short score, sbyte moveX, sbyte moveY, int alpha, int beta, byte threadIndex = 0, int rootDepth = 1)
    {
        var (shardIndex, entryIndex) = GetShardAndIndex(hash);
        var shard = _shards[shardIndex];

        // Determine flag based on score relative to alpha/beta
        EntryFlag entryFlag;
        if (score <= alpha)
            entryFlag = EntryFlag.UpperBound;
        else if (score >= beta)
            entryFlag = EntryFlag.LowerBound;
        else
            entryFlag = EntryFlag.Exact;

        // Read existing entry with SeqLock protection
        TranspositionEntry existing = ReadEntryWithSeqLock(shard, entryIndex);

        // Deep replacement strategy
        bool existingMatchesHash = existing.Hash != 0 && existing.MatchesHash(hash);
        bool shouldStore = existing.Hash == 0;

        if (existing.Hash != 0)
        {
            if (existingMatchesHash)
            {
                bool isDeeper = depth > existing.Depth;
                bool isSameDepthMaster = depth == existing.Depth && threadIndex == 0;
                bool isSameDepthBetterFlag = depth == existing.Depth && entryFlag == EntryFlag.Exact && existing.Flag != EntryFlag.Exact;
                shouldStore = isDeeper || isSameDepthMaster || isSameDepthBetterFlag;
            }
            else
            {
                sbyte depthDiff = (sbyte)(depth - existing.Depth);
                shouldStore = depthDiff >= 2 || existing.Age != _currentAge;
            }
        }

        if (shouldStore)
        {
            // Create new entry
            var newEntry = new TranspositionEntry(hash, depth, score, moveX, moveY, entryFlag, (byte)_currentAge, threadIndex);

            // Write with SeqLock pattern
            WriteEntryWithSeqLock(shard, entryIndex, newEntry);
        }
    }

    /// <summary>
    /// Look up a position in the transposition table using SeqLock pattern.
    /// Thread-safe without explicit locks.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public (bool found, bool hasExactDepth, short score, (int x, int y)? move, byte threadIndex) Lookup(ulong hash, sbyte depth, int alpha, int beta)
    {
        Interlocked.Increment(ref _lookupCount);
        var (shardIndex, entryIndex) = GetShardAndIndex(hash);
        var shard = _shards[shardIndex];

        // Read with SeqLock protection
        TranspositionEntry entry = ReadEntryWithSeqLock(shard, entryIndex);

        if (!entry.MatchesHash(hash))
            return (false, false, 0, null, 0);

        bool hasExactDepth = entry.Depth >= depth;
        byte threadIndex = entry.ThreadIndex;

        if (!hasExactDepth)
        {
            Interlocked.Increment(ref _hitCount);
            return (true, false, entry.Score, entry.GetMove(), threadIndex);
        }

        Interlocked.Increment(ref _hitCount);

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

        return (true, false, entry.Score, entry.GetMove(), threadIndex);
    }

    /// <summary>
    /// Read entry with SeqLock protection against torn reads.
    /// Spins if a write is in progress, retries if version changed during read.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static TranspositionEntry ReadEntryWithSeqLock(TranspositionEntry[] shard, int entryIndex)
    {
        int maxRetries = 100;
        for (int retry = 0; retry < maxRetries; retry++)
        {
            // Read version - if odd, write is in progress
            uint v1 = Volatile.Read(ref shard[entryIndex].Version);
            if ((v1 & 1) != 0)
            {
                Thread.SpinWait(1);
                continue;
            }

            // Copy entry (may still be torn if write started mid-copy)
            TranspositionEntry entry = shard[entryIndex];

            // Memory barrier to ensure read completes before version check
            Thread.MemoryBarrier();

            // Check if version changed during read
            uint v2 = Volatile.Read(ref shard[entryIndex].Version);
            if (v1 == v2)
            {
                // Consistent read
                return entry;
            }

            // Version changed - retry
            Thread.SpinWait(1);
        }

        // Fallback: return empty entry after too many retries
        return default;
    }

    /// <summary>
    /// Write entry with SeqLock pattern.
    /// Increments version to odd (writing), writes data, increments to even (done).
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static void WriteEntryWithSeqLock(TranspositionEntry[] shard, int entryIndex, TranspositionEntry newEntry)
    {
        // Get current version and increment to odd (mark as writing)
        uint currentVersion = Volatile.Read(ref shard[entryIndex].Version);
        uint writeVersion = (currentVersion & ~1u) + 1; // Ensure odd

        // Set version to odd (writing in progress)
        newEntry.Version = writeVersion;
        shard[entryIndex] = newEntry;

        // Memory barrier to ensure write completes
        Thread.MemoryBarrier();

        // Increment version to even (write complete)
        shard[entryIndex].Version = writeVersion + 1;
    }

    /// <summary>
    /// Increment age for replacement strategy
    /// </summary>
    public void IncrementAge()
    {
        int newAge = Interlocked.Increment(ref _currentAge);
        if (newAge >= 255)
        {
            Interlocked.Exchange(ref _currentAge, 1);
        }

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
                var entry = ReadEntryWithSeqLock(shard, i);
                if (entry.IsValid && entry.Age == _currentAge)
                    used++;
            }
        }

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
