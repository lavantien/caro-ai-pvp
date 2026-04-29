using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Domain.Interfaces;
using ZobristHash = Caro.Core.Domain.ValueObjects.ZobristHash;

namespace Caro.Core.Infrastructure.AI;

/// <summary>
/// Per-game state container for AI search
/// Encapsulates all state that was previously global/shared
/// Ensures state isolation between games
/// </summary>
public sealed class AIGameState : IDisposable
{
    /// <summary>
    /// Transposition table specific to this game
    /// </summary>
    public ITranspositionTable TranspositionTable { get; }

    /// <summary>
    /// Killer move tables per depth [maxDepth][slot]
    /// </summary>
    private readonly Position[,] _killerMoves;

    /// <summary>
    /// History heuristic scores [position]
    /// </summary>
    private readonly int[,] _historyScores;

    /// <summary>
    /// Butterfly heuristic scores [position]
    /// </summary>
    private readonly int[,] _butterflyScores;

    /// <summary>
    /// Search statistics
    /// </summary>
    public long NodesSearched { get; set; }
    public int TableHits { get; set; }
    public int TableLookups { get; set; }
    public int MaxDepthReached { get; set; }

    /// <summary>
    /// Current search depth
    /// </summary>
    public int CurrentDepth { get; set; }

    /// <summary>
    /// Last calculated principal variation
    /// </summary>
    public Position[] LastPV { get; set; } = Array.Empty<Position>();

    /// <summary>
    /// Current age for transposition table entries
    /// </summary>
    public byte Age { get; private set; }

    /// <summary>
    /// Create a new AI game state
    /// </summary>
    public AIGameState(int maxDepth = 20, int tableSizeMB = 128)
    {
        TranspositionTable = new PerGameTranspositionTable(tableSizeMB);
        _killerMoves = new Position[maxDepth + 1, 2];
        _historyScores = new int[Position.BoardSize, Position.BoardSize];
        _butterflyScores = new int[Position.BoardSize, Position.BoardSize];
        Age = 0;
    }

    /// <summary>
    /// Get killer move for depth and slot
    /// </summary>
    public Position GetKillerMove(int depth, int slot)
    {
        if (depth <= 0 || depth >= _killerMoves.GetLength(0) || slot < 0 || slot >= 2)
            return new Position(-1, -1);
        return _killerMoves[depth, slot];
    }

    /// <summary>
    /// Set killer move for depth and slot
    /// </summary>
    public void SetKillerMove(int depth, int slot, Position move)
    {
        if (depth > 0 && depth < _killerMoves.GetLength(0) && slot >= 0 && slot < 2)
        {
            _killerMoves[depth, slot] = move;
        }
    }

    /// <summary>
    /// Get history score for a position
    /// </summary>
    public int GetHistoryScore(Position position)
    {
        if (!position.IsValid()) return 0;
        return _historyScores[position.X, position.Y];
    }

    /// <summary>
    /// Update history score for a position (bonus for causing cutoffs)
    /// </summary>
    public void UpdateHistoryScore(Position position, int depth)
    {
        if (!position.IsValid()) return;
        _historyScores[position.X, position.Y] += depth * depth;
    }

    /// <summary>
    /// Get butterfly score for a position
    /// </summary>
    public int GetButterflyScore(Position position)
    {
        if (!position.IsValid()) return 0;
        return _butterflyScores[position.X, position.Y];
    }

    /// <summary>
    /// Update butterfly score for a position
    /// </summary>
    public void UpdateButterflyScore(Position position, int delta)
    {
        if (!position.IsValid()) return;
        _butterflyScores[position.X, position.Y] += delta;
    }

    /// <summary>
    /// Reset statistics for a new search
    /// </summary>
    public void ResetStatistics()
    {
        NodesSearched = 0;
        TableHits = 0;
        TableLookups = 0;
        MaxDepthReached = 0;
    }

    /// <summary>
    /// Clear all state (end of game)
    /// </summary>
    public void Clear()
    {
        TranspositionTable.Clear();
        Array.Clear(_historyScores, 0, _historyScores.Length);
        Array.Clear(_butterflyScores, 0, _butterflyScores.Length);
        for (int i = 0; i < _killerMoves.GetLength(0); i++)
        {
            for (int j = 0; j < 2; j++)
            {
                _killerMoves[i, j] = new Position(-1, -1);
            }
        }
        ResetStatistics();
    }

    public void Dispose()
    {
        Clear();
    }
}

/// <summary>
/// Per-game transposition table implementation
/// Uses Zobrist hashing for fast position lookup
/// Implements ITranspositionTable for state isolation per game
/// </summary>
internal sealed class PerGameTranspositionTable : ITranspositionTable
{
    private const int EntrySize = 24; // bytes per entry
    private readonly TTEntry[] _table;
    private readonly int _tableSize;
    private readonly int _ageMask;
    private readonly int _sizeInMB;
    private byte _age;
    private int _lookupCount;
    private int _hitCount;

    public PerGameTranspositionTable(int sizeMB)
    {
        _sizeInMB = sizeMB;

        // Calculate table size (power of 2 for fast modulo)
        var totalBytes = sizeMB * 1024 * 1024;
        _tableSize = 1;
        while (_tableSize * EntrySize < totalBytes)
        {
            _tableSize *= 2;
        }
        _ageMask = _tableSize - 1;

        _table = new TTEntry[_tableSize];
        for (int i = 0; i < _tableSize; i++)
        {
            _table[i] = new TTEntry(
                default(ZobristHash),
                -1,
                0,
                default(TTMove),
                default(TTFlag),
                0
            );
        }
    }

    public int SizeInMB => _sizeInMB;
    public byte Age => _age;
    public int EntryCount => _entryCount;
    public int LookupCount => _lookupCount;
    public int HitCount => _hitCount;
    public double HitRate => _lookupCount > 0 ? (double)_hitCount / _lookupCount : 0;

    private int _entryCount;

    public TTEntry? Lookup(ZobristHash hash, int depth)
    {
        _lookupCount++;
        var index = hash.Value & (uint)_ageMask;
        var tableEntry = _table[index];

        if (tableEntry.Hash == hash && tableEntry.Depth >= depth)
        {
            _hitCount++;
            return tableEntry;
        }

        return null;
    }

    public void Store(ZobristHash hash, int depth, int score, TTMove bestMove, TTFlag flag, byte age)
    {
        var index = hash.Value & (uint)_ageMask;
        var existing = _table[index];

        // Replace if:
        // 1. Empty slot (depth == -1)
        // 2. New entry has higher depth
        // 3. Same hash with better depth
        if (existing.Depth == -1 || depth > existing.Depth ||
            (existing.Hash == hash && depth > existing.Depth))
        {
            _table[index] = new TTEntry(
                hash,
                depth,
                score,
                bestMove,
                flag,
                age
            );

            // Update entry count if this was previously empty
            if (existing.Depth == -1)
            {
                _entryCount++;
            }
        }
    }

    public void IncrementAge()
    {
        _age++;
        for (int i = 0; i < _tableSize; i++)
        {
            if (_table[i].Depth != -1)
            {
                // Increment age for stored entries
                var entry = _table[i];
                _table[i] = entry with { Age = (byte)(entry.Age + 1) };
            }
        }
    }

    public void Clear()
    {
        for (int i = 0; i < _tableSize; i++)
        {
            _table[i] = new TTEntry(
                default(ZobristHash),
                -1,
                0,
                default(TTMove),
                default(TTFlag),
                0
            );
        }
        _entryCount = 0;
        _age = 0;
    }

    public TTStats GetStats()
    {
        return new TTStats(
            SizeInMB,
            EntryCount,
            LookupCount,
            HitCount,
            HitRate,
            Age
        );
    }

    public void Dispose()
    {
        Clear();
    }
}

