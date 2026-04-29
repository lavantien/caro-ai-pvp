using System.Runtime.CompilerServices;
using System.Runtime.InteropServices;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// Transposition table IO: Store, Lookup, FindSlotToReplace, and stats.
public partial class TranspositionTable
{
    /// Store a search result using depth-age replacement.
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public void Store(ulong hash, int depth, int score, (int x, int y)? bestMove, int alpha, int beta)
    {
        var index = hash % (ulong)_size;
        var cluster = _table[index];

        // Determine flag based on score relative to alpha/beta
        EntryFlag flag;
        if (score <= alpha)
            flag = EntryFlag.UpperBound;
        else if (score >= beta)
            flag = EntryFlag.LowerBound;
        else
            flag = EntryFlag.Exact;

        // Create new entry
        var key16 = (ushort)(hash >> 48);
        if (key16 == 0) key16 = 1; // Ensure non-zero to distinguish from empty

        var newEntry = new TTEntry
        {
            Key16 = key16,
            Value = (short)score,
            Depth8 = (sbyte)depth,
            BoundAndAge = TTEntry.MakeBoundAndAge(flag, _currentAge),
            Move16 = TTEntry.PackMove(bestMove?.x ?? -1, bestMove?.y ?? -1),
            Eval16 = 0 // Static eval not currently used
        };

        // Find best slot in cluster
        int replaceIndex = FindSlotToReplace(cluster, hash, newEntry);

        // Store the entry in the cluster
        cluster.SetEntry(replaceIndex, newEntry);
        _table[index] = cluster;
    }

    /// Find the best slot to replace in a cluster
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private unsafe int FindSlotToReplace(Cluster cluster, ulong hash, TTEntry newEntry)
    {
        ushort key16 = (ushort)(hash >> 48);
        if (key16 == 0) key16 = 1; // Match the Store logic
        int emptyIndex = -1;
        int lowestValueIndex = 0;
        int lowestValue = int.MaxValue;
        int sameKeyLowestDepthIndex = -1;
        int sameKeyLowestDepth = int.MaxValue;

        for (int i = 0; i < 3; i++)
        {
            TTEntry entry = cluster.GetEntry(i);

            // Check for empty slot (Key16 == 0)
            if (entry.Key16 == 0)
            {
                emptyIndex = i;
                continue;
            }

            // Check for same hash key
            if (entry.Key16 == key16)
            {
                // Same position: replace if deeper
                if (newEntry.Depth8 >= entry.Depth8)
                {
                    if (entry.Depth8 < sameKeyLowestDepth)
                    {
                        sameKeyLowestDepth = entry.Depth8;
                        sameKeyLowestDepthIndex = i;
                    }
                }
                else
                {
                    return i; // Keep existing deeper entry
                }
            }

            // Track lowest value for replacement
            int value = entry.ReplacementValue();
            if (value < lowestValue)
            {
                lowestValue = value;
                lowestValueIndex = i;
            }
        }

        if (emptyIndex >= 0)
            return emptyIndex;

        if (sameKeyLowestDepthIndex >= 0)
            return sameKeyLowestDepthIndex;

        return lowestValueIndex;
    }

    /// Look up a position. Searches all 3 entries, returns best match.
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public unsafe (bool found, int score, (int x, int y)? bestMove) Lookup(ulong hash, int depth, int alpha, int beta)
    {
        var index = hash % (ulong)_size;
        var cluster = _table[index];
        ushort key16 = (ushort)(hash >> 48);
        if (key16 == 0) key16 = 1; // Match the Store logic

        TTEntry? bestEntry = null;
        int bestMatchDepth = -1;

        // Search all 3 entries for matching hash
        for (int i = 0; i < 3; i++)
        {
            TTEntry entry = cluster.GetEntry(i);

            if (entry.Key16 == key16 && entry.Depth8 >= depth && entry.Depth8 > bestMatchDepth)
            {
                bestEntry = entry;
                bestMatchDepth = entry.Depth8;
            }
        }

        if (bestEntry == null)
        {
            // No exact match, but check if we have a matching entry for move ordering
            for (int i = 0; i < 3; i++)
            {
                TTEntry entry = cluster.GetEntry(i);
                if (entry.Key16 == key16)
                {
                    return (false, 0, entry.GetMove());
                }
            }
            return (false, 0, null);
        }

        var entryVal = bestEntry.Value;

        // Check if we can use the cached score
        switch (entryVal.GetBound())
        {
            case EntryFlag.Exact:
                return (true, entryVal.Value, entryVal.GetMove());

            case EntryFlag.LowerBound:
                if (entryVal.Value >= beta)
                    return (true, entryVal.Value, entryVal.GetMove());
                break;

            case EntryFlag.UpperBound:
                if (entryVal.Value <= alpha)
                    return (true, entryVal.Value, entryVal.GetMove());
                break;
        }

        // Can't use the score, but can use the best move for move ordering
        return (false, 0, entryVal.GetMove());
    }

    /// Get table statistics for debugging
    public (int used, double usagePercent) GetStats()
    {
        int used = 0;
        for (int i = 0; i < _size; i++)
        {
            var cluster = _table[i];
            unsafe
            {
                TTEntry* entries = cluster.GetEntriesPtr();
                for (int j = 0; j < 3; j++)
                {
                    if (entries[j].Key16 != 0 && entries[j].GetAge() == _currentAge)
                        used++;
                }
            }
        }
        return (used, (double)used / (_size * 3) * 100);
    }
}
