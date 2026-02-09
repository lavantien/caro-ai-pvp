using System.Runtime.CompilerServices;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Evaluation cache for storing correction values to position evaluations.
/// Caches the difference between static evaluation and search results,
/// allowing faster subsequent evaluations with corrected values.
/// </summary>
public sealed class EvaluationCache
{
    /// <summary>
    /// Cache entry storing correction information
    /// </summary>
    private struct Entry
    {
        public ulong Hash;           // Position hash
        public int StaticEval;       // Original static evaluation
        public int CorrectedEval;    // Corrected evaluation from search
        public sbyte Depth;          // Search depth when cached
        public sbyte Age;            // Entry age for replacement
        public byte DecayFactor;     // Correction decay factor (0-255)

        /// <summary>
        /// Calculate if this entry can be used for a given lookup depth
        /// </summary>
        [MethodImpl(MethodImplOptions.AggressiveInlining)]
        public readonly bool CanUse(sbyte lookupDepth) => Depth >= lookupDepth;

        /// <summary>
        /// Get the replacement value for age-based replacement strategy
        /// </summary>
        [MethodImpl(MethodImplOptions.AggressiveInlining)]
        public readonly int ReplacementValue() => Depth - (Age * 8);
    }

    private readonly Entry[] _table;
    private readonly int _size;
    private byte _currentAge;
    private int _hitCount;
    private int _missCount;

    // Maximum/Minimum bounds for corrected evaluations
    private const int MaxCorrectedEval = 200000;
    private const int MinCorrectedEval = -200000;

    /// <summary>
    /// Create an evaluation cache with specified size in MB
    /// </summary>
    public EvaluationCache(int sizeMB = 1)
    {
        // Each entry is approximately 24 bytes
        // ulong (8) + int (4) + int (4) + sbyte (1) + sbyte (1) + byte (1) + padding
        _size = (sizeMB * 1024 * 1024) / 24;
        _table = new Entry[_size];
        _currentAge = 1;
    }

    /// <summary>
    /// Get corrected evaluation for a position.
    /// Returns corrected evaluation if cache hit (depth sufficient), otherwise returns staticEval.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public int GetCorrectedEvaluation(ulong hash, int staticEval, sbyte depth)
    {
        var index = hash % (ulong)_size;
        var entry = _table[index];

        // Cache miss check
        if (entry.Hash != hash || !entry.CanUse(depth))
        {
            Interlocked.Increment(ref _missCount);
            return staticEval;
        }

        // Cache hit - apply correction
        Interlocked.Increment(ref _hitCount);

        // Formula: corrected = staticEval + (cachedCorrection * decayFactor)
        // where cachedCorrection = entry.CorrectedEval - entry.StaticEval
        int correction = entry.CorrectedEval - entry.StaticEval;
        int decayedCorrection = (correction * entry.DecayFactor) / 256;
        int corrected = staticEval + decayedCorrection;

        // Apply bounds
        if (corrected > MaxCorrectedEval)
            corrected = MaxCorrectedEval;
        else if (corrected < MinCorrectedEval)
            corrected = MinCorrectedEval;

        return corrected;
    }

    /// <summary>
    /// Update the cache with search result.
    /// Stores the difference between search result and static evaluation.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public void Update(ulong hash, int staticEval, int searchResult, sbyte depth)
    {
        var index = hash % (ulong)_size;
        var existingEntry = _table[index];

        // Calculate decay factor based on depth
        // Deeper searches have more reliable corrections (higher decay factor)
        // Depth 1: 64/256 (25%), Depth 10: 230/256 (90%)
        byte decayFactor = (byte)Math.Clamp(64 + (depth * 18), 64, 230);

        // Calculate bounded corrected eval
        int correctedEval = searchResult;
        if (correctedEval > MaxCorrectedEval)
            correctedEval = MaxCorrectedEval;
        else if (correctedEval < MinCorrectedEval)
            correctedEval = MinCorrectedEval;

        var newEntry = new Entry
        {
            Hash = hash,
            StaticEval = staticEval,
            CorrectedEval = correctedEval,
            Depth = depth,
            Age = (sbyte)_currentAge,
            DecayFactor = decayFactor
        };

        // Replacement strategy:
        // 1. Empty slot (Hash == 0)
        // 2. Same hash with deeper depth
        // 3. Same hash with same depth (update correction)
        // 4. Lower replacement value (depth - 8*age)
        bool shouldReplace = existingEntry.Hash == 0;

        if (!shouldReplace && existingEntry.Hash != 0)
        {
            if (existingEntry.Hash == hash)
            {
                // Same position - replace if deeper or same depth
                shouldReplace = depth >= existingEntry.Depth;
            }
            else
            {
                // Different position - use replacement value
                shouldReplace = newEntry.ReplacementValue() > existingEntry.ReplacementValue();
            }
        }

        if (shouldReplace)
        {
            _table[index] = newEntry;
        }
    }

    /// <summary>
    /// Add a new entry (alias for Update with clearer semantics)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public void NewEntry(ulong hash, int staticEval, int searchResult, sbyte depth)
    {
        Update(hash, staticEval, searchResult, depth);
    }

    /// <summary>
    /// Increment age counter for replacement strategy
    /// </summary>
    public void IncrementAge()
    {
        _currentAge++;
        if (_currentAge == 64)
        {
            _currentAge = 1;
        }
    }

    /// <summary>
    /// Clear the entire cache
    /// </summary>
    public void Clear()
    {
        Array.Clear(_table, 0, _table.Length);
        _currentAge = 1;
        _hitCount = 0;
        _missCount = 0;
    }

    /// <summary>
    /// Get cache statistics
    /// </summary>
    public (int hits, int misses, double hitRate) GetStats()
    {
        int hits = Interlocked.CompareExchange(ref _hitCount, 0, 0);
        int misses = Interlocked.CompareExchange(ref _missCount, 0, 0);
        int total = hits + misses;
        double hitRate = total > 0 ? (double)hits / total * 100 : 0;

        return (hits, misses, hitRate);
    }

    /// <summary>
    /// Get the number of entries currently stored
    /// </summary>
    public int Count
    {
        get
        {
            int count = 0;
            for (int i = 0; i < _size; i++)
            {
                if (_table[i].Hash != 0)
                    count++;
            }
            return count;
        }
    }

    /// <summary>
    /// Get the table size (number of slots)
    /// </summary>
    public int Size => _size;
}
