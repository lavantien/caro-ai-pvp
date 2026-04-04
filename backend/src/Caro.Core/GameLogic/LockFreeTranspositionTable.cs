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
public sealed partial class LockFreeTranspositionTable
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

    /// Increment age for replacement strategy
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

    /// Clear the entire table
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

    public int Size => _sizePerShard * _shardCount;

    public int CurrentAge => _currentAge;
}
