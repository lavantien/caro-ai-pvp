using System.Runtime.CompilerServices;
using System.Threading;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// Lock-free transposition table IO: Store, Lookup, SeqLock read/write, and stats.
public sealed partial class LockFreeTranspositionTable
{
    /// Store a position using SeqLock pattern. Thread-safe without explicit locks.
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

    /// Look up a position using SeqLock pattern. Thread-safe without explicit locks.
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

    /// Read entry with SeqLock protection against torn reads.
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

    /// Write entry with SeqLock pattern.
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

    /// Get transposition table statistics
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
}
