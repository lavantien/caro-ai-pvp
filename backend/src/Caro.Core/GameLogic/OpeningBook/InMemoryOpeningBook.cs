using System.Collections.Concurrent;
using Caro.Core.Domain.Entities;
using Microsoft.Extensions.Logging;

namespace Caro.Core.GameLogic;

/// <summary>
/// In-memory opening book for fast lookup during games.
/// Uses lock-free concurrent reads for maximum performance.
/// Loads from SQLite at startup and provides nanosecond access times.
/// </summary>
public sealed class InMemoryOpeningBook : IDisposable
{
    private readonly ConcurrentDictionary<ulong, OpeningBookEntry> _entriesByCanonicalHash;
    private readonly ConcurrentDictionary<(ulong canonicalHash, ulong directHash, Player player), OpeningBookEntry> _entriesByExactKey;
    private readonly IPositionCanonicalizer _canonicalizer;
    private readonly ILogger<InMemoryOpeningBook> _logger;
    private bool _disposed;

    /// <summary>
    /// Number of positions loaded in the book.
    /// </summary>
    public int Count => _entriesByCanonicalHash.Count;

    public InMemoryOpeningBook(
        IOpeningBookStore store,
        IPositionCanonicalizer canonicalizer,
        ILoggerFactory? loggerFactory = null)
    {
        _entriesByCanonicalHash = new ConcurrentDictionary<ulong, OpeningBookEntry>();
        _entriesByExactKey = new ConcurrentDictionary<(ulong, ulong, Player), OpeningBookEntry>();
        _canonicalizer = canonicalizer ?? throw new ArgumentNullException(nameof(canonicalizer));
        _logger = (loggerFactory ?? Microsoft.Extensions.Logging.Abstractions.NullLoggerFactory.Instance)
            .CreateLogger<InMemoryOpeningBook>();

        // Load all entries from store
        LoadFromStore(store);
    }

    /// <summary>
    /// Load all entries from the store into memory.
    /// </summary>
    private void LoadFromStore(IOpeningBookStore store)
    {
        _logger.LogInformation("Loading opening book into memory...");

        var allEntries = store.GetAllEntries();
        int loaded = 0;

        foreach (var entry in allEntries)
        {
            // Index by canonical hash for fast lookup
            _entriesByCanonicalHash[entry.CanonicalHash] = entry;

            // Index by exact key for precise lookup
            _entriesByExactKey[(entry.CanonicalHash, entry.DirectHash, entry.Player)] = entry;
            loaded++;
        }

        _logger.LogInformation("Loaded {Count} entries into memory", loaded);
    }

    /// <summary>
    /// Look up moves for a board position.
    /// Returns null if position not in book.
    /// </summary>
    public BookMove[]? Lookup(Board board, Player player)
    {
        if (_disposed)
            return null;

        // Canonicalize the position
        var canonical = _canonicalizer.Canonicalize(board);

        // First try exact match
        if (_entriesByExactKey.TryGetValue((canonical.CanonicalHash, board.GetHash(), player), out var exactEntry))
        {
            return exactEntry.Moves;
        }

        // Try canonical hash match
        if (_entriesByCanonicalHash.TryGetValue(canonical.CanonicalHash, out var entry) && entry.Player == player)
        {
            // Transform moves back to actual coordinates if symmetry was applied
            if (canonical.SymmetryApplied != SymmetryType.Identity && !canonical.IsNearEdge)
            {
                return TransformMovesBack(entry.Moves, canonical.SymmetryApplied);
            }
            return entry.Moves;
        }

        return null;
    }

    /// <summary>
    /// Check if a position is in the book.
    /// </summary>
    public bool Contains(Board board, Player player)
    {
        if (_disposed)
            return false;

        var canonical = _canonicalizer.Canonicalize(board);
        return _entriesByCanonicalHash.ContainsKey(canonical.CanonicalHash);
    }

    /// <summary>
    /// Get the best move for a position, prioritizing:
    /// 1. Solved moves (VCF proven)
    /// 2. Learned moves (deep search)
    /// 3. Self-play moves (statistical)
    /// Then by score (higher = better).
    /// </summary>
    public BookMove? GetBestMove(Board board, Player player)
    {
        var moves = Lookup(board, player);
        if (moves == null || moves.Length == 0)
            return null;

        // Sort by source priority, then by score
        return moves
            .OrderBy(m => (int)m.Source)  // Solved=0, Learned=1, SelfPlay=2
            .ThenByDescending(m => m.Score)  // Higher score = better
            .FirstOrDefault();
    }

    /// <summary>
    /// Transform moves from canonical coordinates back to actual coordinates.
    /// </summary>
    private BookMove[] TransformMovesBack(BookMove[] moves, SymmetryType symmetry)
    {
        if (symmetry == SymmetryType.Identity)
            return moves;

        var transformed = new BookMove[moves.Length];
        for (int i = 0; i < moves.Length; i++)
        {
            var move = moves[i];
            var (actualX, actualY) = _canonicalizer.ApplyInverseSymmetry(
                move.RelativeX, move.RelativeY, symmetry);

            transformed[i] = move with
            {
                RelativeX = actualX,
                RelativeY = actualY
            };
        }

        return transformed;
    }

    /// <summary>
    /// Add or update an entry in the book.
    /// Thread-safe for concurrent access.
    /// </summary>
    public void AddEntry(OpeningBookEntry entry)
    {
        if (_disposed)
            return;

        _entriesByCanonicalHash[entry.CanonicalHash] = entry;
        _entriesByExactKey[(entry.CanonicalHash, entry.DirectHash, entry.Player)] = entry;
    }

    /// <summary>
    /// Get statistics about the in-memory book.
    /// </summary>
    public InMemoryBookStatistics GetStatistics()
    {
        int solvedMoves = 0;
        int learnedMoves = 0;
        int selfPlayMoves = 0;
        int totalMoves = 0;
        int maxDepth = 0;

        foreach (var entry in _entriesByCanonicalHash.Values)
        {
            maxDepth = Math.Max(maxDepth, entry.Depth);
            foreach (var move in entry.Moves)
            {
                totalMoves++;
                switch (move.Source)
                {
                    case MoveSource.Solved:
                        solvedMoves++;
                        break;
                    case MoveSource.Learned:
                        learnedMoves++;
                        break;
                    case MoveSource.SelfPlay:
                        selfPlayMoves++;
                        break;
                }
            }
        }

        return new InMemoryBookStatistics(
            TotalPositions: _entriesByCanonicalHash.Count,
            TotalMoves: totalMoves,
            SolvedMoves: solvedMoves,
            LearnedMoves: learnedMoves,
            SelfPlayMoves: selfPlayMoves,
            MaxDepth: maxDepth
        );
    }

    public void Dispose()
    {
        if (_disposed)
            return;

        _disposed = true;
        _entriesByCanonicalHash.Clear();
        _entriesByExactKey.Clear();
    }
}

/// <summary>
/// Statistics for the in-memory opening book.
/// </summary>
public sealed record InMemoryBookStatistics(
    int TotalPositions,
    int TotalMoves,
    int SolvedMoves,
    int LearnedMoves,
    int SelfPlayMoves,
    int MaxDepth
)
{
    public double SolvedMovePercentage => TotalMoves > 0 ? (double)SolvedMoves / TotalMoves * 100 : 0;
    public double LearnedMovePercentage => TotalMoves > 0 ? (double)LearnedMoves / TotalMoves * 100 : 0;
    public double SelfPlayMovePercentage => TotalMoves > 0 ? (double)SelfPlayMoves / TotalMoves * 100 : 0;
}
