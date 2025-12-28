using Caro.Core.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Transposition table with Zobrist hashing for caching search results
/// Provides 2-5x speedup by avoiding re-computation of identical positions
/// </summary>
public class TranspositionTable
{
    private enum EntryFlag
    {
        Exact,       // Score is exact (alpha < score < beta)
        LowerBound,  // Score is at least this value (beta cutoff)
        UpperBound   // Score is at most this value (alpha cutoff)
    }

    private class TableEntry
    {
        public ulong Hash { get; set; }
        public int Depth { get; set; }
        public int Score { get; set; }
        public (int x, int y)? BestMove { get; set; }
        public EntryFlag Flag { get; set; }
        public byte Age { get; set; } // For replacement strategy
    }

    private readonly TableEntry[] _table;
    private readonly ulong[,] _zobristTableRed;   // Zobrist keys for Red pieces
    private readonly ulong[,] _zobristTableBlue;  // Zobrist keys for Blue pieces
    private readonly Random _random;
    private readonly int _size;
    private byte _currentAge;

    public TranspositionTable(int sizeMB = 64)
    {
        // Calculate number of entries (each entry is ~32 bytes)
        _size = (sizeMB * 1024 * 1024) / 32;
        _table = new TableEntry[_size];
        _random = new Random(42); // Fixed seed for reproducibility

        // Initialize Zobrist tables for 15x15 board
        _zobristTableRed = new ulong[15, 15];
        _zobristTableBlue = new ulong[15, 15];
        InitializeZobristTables();
    }

    /// <summary>
    /// Initialize Zobrist hash tables with random 64-bit numbers
    /// Each board position has two random numbers: one for Red, one for Blue
    /// </summary>
    private void InitializeZobristTables()
    {
        var buffer = new byte[8];
        for (int x = 0; x < 15; x++)
        {
            for (int y = 0; y < 15; y++)
            {
                _zobristTableRed[x, y] = RandomUInt64();
                _zobristTableBlue[x, y] = RandomUInt64();
            }
        }
    }

    /// <summary>
    /// Generate a random 64-bit integer
    /// </summary>
    private ulong RandomUInt64()
    {
        var bytes = new byte[8];
        _random.NextBytes(bytes);
        return BitConverter.ToUInt64(bytes, 0);
    }

    /// <summary>
    /// Calculate Zobrist hash for the current board position
    /// XOR of random numbers for each occupied cell
    /// </summary>
    public ulong CalculateHash(Board board)
    {
        ulong hash = 0;
        for (int x = 0; x < 15; x++)
        {
            for (int y = 0; y < 15; y++)
            {
                var cell = board.GetCell(x, y);
                if (cell.Player == Player.Red)
                {
                    hash ^= _zobristTableRed[x, y];
                }
                else if (cell.Player == Player.Blue)
                {
                    hash ^= _zobristTableBlue[x, y];
                }
            }
        }
        return hash;
    }

    /// <summary>
    /// Store a search result in the transposition table
    /// </summary>
    public void Store(ulong hash, int depth, int score, (int x, int y)? bestMove, int alpha, int beta)
    {
        var index = hash % (ulong)_size;
        var entry = _table[index];

        // Determine flag based on score relative to alpha/beta
        EntryFlag flag;
        if (score <= alpha)
            flag = EntryFlag.UpperBound;
        else if (score >= beta)
            flag = EntryFlag.LowerBound;
        else
            flag = EntryFlag.Exact;

        // Replace strategy:
        // 1. Empty slot
        // 2. Same position with deeper depth
        // 3. Old entry (different age)
        bool shouldReplace = entry == null ||
                            entry.Hash == hash && depth > entry.Depth ||
                            entry.Age != _currentAge;

        if (shouldReplace)
        {
            _table[index] = new TableEntry
            {
                Hash = hash,
                Depth = depth,
                Score = score,
                BestMove = bestMove,
                Flag = flag,
                Age = _currentAge
            };
        }
    }

    /// <summary>
    /// Look up a position in the transposition table
    /// Returns (found, score, bestMove)
    /// </summary>
    public (bool found, int score, (int x, int y)? bestMove) Lookup(ulong hash, int depth, int alpha, int beta)
    {
        var index = hash % (ulong)_size;
        var entry = _table[index];

        if (entry == null || entry.Hash != hash || entry.Depth < depth)
        {
            return (false, 0, null);
        }

        // Check if we can use the cached score
        switch (entry.Flag)
        {
            case EntryFlag.Exact:
                return (true, entry.Score, entry.BestMove);

            case EntryFlag.LowerBound:
                if (entry.Score >= beta)
                    return (true, entry.Score, entry.BestMove);
                break;

            case EntryFlag.UpperBound:
                if (entry.Score <= alpha)
                    return (true, entry.Score, entry.BestMove);
                break;
        }

        // Can't use the score, but can use the best move for move ordering
        return (false, 0, entry.BestMove);
    }

    /// <summary>
    /// Increment age counter for replacement strategy
    /// Should be called at the start of each search
    /// </summary>
    public void IncrementAge()
    {
        _currentAge++;
        if (_currentAge == 255)
        {
            _currentAge = 1;
        }
    }

    /// <summary>
    /// Clear the entire table
    /// </summary>
    public void Clear()
    {
        Array.Clear(_table, 0, _table.Length);
        _currentAge = 0;
    }

    /// <summary>
    /// Get table statistics for debugging
    /// </summary>
    public (int used, double usagePercent) GetStats()
    {
        int used = 0;
        for (int i = 0; i < _size; i++)
        {
            if (_table[i] != null && _table[i].Age == _currentAge)
                used++;
        }
        return (used, (double)used / _size * 100);
    }
}
