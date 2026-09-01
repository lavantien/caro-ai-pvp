using System.Runtime.CompilerServices;
using Caro.Domain;

namespace Caro.Engine;

public enum TTEntryType : byte
{
    Exact = 0,
    LowerBound = 1,
    UpperBound = 2,
}

public struct TTEntry
{
    public ulong Hash { get; set; }
    public int Score { get; set; }
    public byte Depth { get; set; }
    public sbyte MoveX { get; set; }
    public sbyte MoveY { get; set; }
    public TTEntryType Type { get; set; }
    public byte Age { get; set; }
}

internal struct TtSlot
{
    public ulong Hash { get; set; }
    public int Score { get; set; }
    public byte Depth { get; set; }
    public sbyte MoveX { get; set; }
    public sbyte MoveY { get; set; }
    public byte Type { get; set; }
    public byte Age { get; set; }
}

// The critical sections are a handful of instructions, so a plain monitor
// beats a reader-writer lock: RWLS bookkeeping costs more than the exclusive
// section itself, and readers barely block each other in practice.
internal sealed class TtShard
{
    public readonly object Gate = new();
    public TtSlot[] Slots { get; set; } = [];
    public ulong Mask { get; set; }
}

public sealed class TranspositionTable : IDisposable
{
    private readonly TtShard[] _shards = new TtShard[Constants.TTShardCount];
    private readonly int _sizeMB;
    private int _age;
    private long _probes;
    private long _hits;

    public TranspositionTable(int sizeMB)
    {
        _sizeMB = sizeMB;
        int entriesPerShard = (sizeMB * 1024 * 1024 / Constants.TTShardCount) / Unsafe.SizeOf<TtSlot>();
        ulong mask = 1;
        while (mask < (ulong)entriesPerShard)
        {
            mask <<= 1;
        }
        mask--;

        for (int i = 0; i < _shards.Length; i++)
        {
            _shards[i] = new TtShard { Slots = new TtSlot[mask + 1], Mask = mask };
        }
    }

    private static int ShardIndex(ulong hash) => (int)((hash >> 32) & (Constants.TTShardCount - 1));

    public void Store(TTEntry entry)
    {
        TtShard shard = _shards[ShardIndex(entry.Hash)];
        ulong idx = entry.Hash & shard.Mask;

        byte currentAge = (byte)Volatile.Read(ref _age);
        // Stamp the write with the current age so the depth-age replacement
        // policy can prefer fresh entries over stale ones.
        entry.Age = currentAge;

        int entryPrio = entry.Depth - 8 * (currentAge - entry.Age);

        lock (shard.Gate)
        {
            ref TtSlot slot = ref shard.Slots[idx];
            ulong existingHash = slot.Hash;
            byte existingDepth = slot.Depth;
            byte existingAge = slot.Age;

            int existingPrio = existingDepth - 8 * (currentAge - existingAge);

            if (existingHash == entry.Hash)
            {
                if (existingDepth > entry.Depth)
                {
                    return;
                }
            }
            else if (existingHash != 0 && existingPrio >= entryPrio)
            {
                return;
            }
            slot.Hash = entry.Hash;
            slot.Score = entry.Score;
            slot.Depth = entry.Depth;
            slot.MoveX = entry.MoveX;
            slot.MoveY = entry.MoveY;
            slot.Type = (byte)entry.Type;
            slot.Age = entry.Age;
        }
    }

    public bool Lookup(ulong hash, out TTEntry entry)
    {
        Interlocked.Increment(ref _probes);
        TtShard shard = _shards[ShardIndex(hash)];
        ulong idx = hash & shard.Mask;

        TTEntry found;
        lock (shard.Gate)
        {
            found = new TTEntry
            {
                Hash = shard.Slots[idx].Hash,
                Score = shard.Slots[idx].Score,
                Depth = shard.Slots[idx].Depth,
                MoveX = shard.Slots[idx].MoveX,
                MoveY = shard.Slots[idx].MoveY,
                Type = (TTEntryType)shard.Slots[idx].Type,
                Age = shard.Slots[idx].Age,
            };
        }

        if (found.Hash != hash)
        {
            entry = default;
            return false;
        }
        Interlocked.Increment(ref _hits);
        entry = found;
        return true;
    }

    public void Clear()
    {
        foreach (TtShard shard in _shards)
        {
            lock (shard.Gate)
            {
                Array.Clear(shard.Slots);
            }
        }
    }

    public void Dispose()
    {
        foreach (TtShard shard in _shards)
        {
            shard.Slots = [];
            shard.Mask = 0;
        }
    }

    public void IncrementAge() => Interlocked.Increment(ref _age);

    internal ulong ShardStrideForTest() => (ulong)_shards[0].Slots.Length;

    public (long Probes, long Hits) Stats() => (Volatile.Read(ref _probes), Volatile.Read(ref _hits));

    public void ResetStats()
    {
        Volatile.Write(ref _probes, 0);
        Volatile.Write(ref _hits, 0);
    }
}
