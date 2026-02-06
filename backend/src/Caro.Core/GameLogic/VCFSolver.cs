using System.Collections.Concurrent;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// VCF (Victory by Continuous Fours) Solver
///
/// Runs BEFORE alpha-beta search to detect forcing sequences.
/// - Offensive VCF: We can force a win through continuous threats
/// - Defensive VCF: Opponent can force a win, we must find defenses
///
/// Thread-safe design for Lazy SMP parallel search.
/// </summary>
public sealed class VCFSolver
{
    private readonly ThreatSpaceSearch _threatSearch;
    private readonly WinDetector _winDetector = new();

    // VCF result cache (thread-safe)
    private readonly ConcurrentDictionary<ulong, VCFCacheEntry> _vcfCache = new();
    private byte _currentAge = 1;
    private const int MaxCacheEntries = 10000;

    // Configuration
    private const int MaxVCFTimeMs = 100;      // Maximum time per VCF search
    // Note: No depth caps - only time limits search, per algorithmic principles

    /// <summary>
    /// Create a new VCF solver
    /// </summary>
    public VCFSolver(ThreatSpaceSearch threatSearch)
    {
        _threatSearch = threatSearch ?? throw new ArgumentNullException(nameof(threatSearch));
    }

    /// <summary>
    /// Check for VCF at a node during alpha-beta search
    /// Runs BEFORE move generation to detect forcing sequences
    /// </summary>
    /// <param name="board">Current position</param>
    /// <param name="attacker">Player to move</param>
    /// <param name="depth">Current search depth</param>
    /// <param name="alpha">Current alpha bound</param>
    /// <param name="timeRemainingMs">Time remaining in search</param>
    /// <returns>VCF result if found, null if not (use VCFNodeResult.None for explicit "no VCF")</returns>
    public VCFNodeResult? CheckNodeVCF(
        Board board,
        Player attacker,
        int depth,
        int alpha,
        long timeRemainingMs)
    {
        // Skip VCF if insufficient time
        if (timeRemainingMs < 10)
            return null;

        // Fast pre-check: is position quiet enough to skip VCF?
        if (!HasVCFPotential(board, attacker))
            return null;

        // Check cache first
        var boardHash = board.GetHash();
        if (_vcfCache.TryGetValue(boardHash, out var cached) && cached.Age == _currentAge)
        {
            return cached.ResultType == VCFResultType.NoVCF
                ? null
                : CreateResultFromCache(cached);
        }

        // Run VCF search with time limit
        var stopwatch = System.Diagnostics.Stopwatch.StartNew();
        var result = SolveVCF(board, attacker, depth, alpha, MaxVCFTimeMs, stopwatch);
        stopwatch.Stop();

        // Cache result
        if (result != null)
        {
            CacheResult(boardHash, result);
        }

        return result;
    }

    /// <summary>
    /// Detect if opponent has VCF threat (defensive VCF)
    /// Returns blocking moves if VCF detected
    /// </summary>
    public List<(int x, int y)> DetectOpponentVCF(
        Board board,
        Player defender,
        Player attacker)
    {
        var opponentThreats = _threatSearch.GetThreatMoves(board, attacker);

        // If opponent has 2+ threats, they may have VCF
        if (opponentThreats.Count >= 2)
        {
            var defenses = _threatSearch.GetDefenseMoves(board, attacker, defender);
            return defenses.Where(m => m.x >= 0 && m.x < 19 && m.y >= 0 && m.y < 19).ToList();
        }

        return new List<(int x, int y)>();
    }

    /// <summary>
    /// Fast pre-check to determine if VCF is worth searching
    /// Avoids expensive VCF search in quiet positions
    /// </summary>
    public bool HasVCFPotential(Board board, Player player)
    {
        var opponent = player == Player.Red ? Player.Blue : Player.Red;

        // Check for immediate win
        var winResult = _winDetector.CheckWin(board);
        if (winResult.HasWinner && winResult.Winner == player)
            return true;  // VCF will find the win immediately

        // Check if opponent has threats (defensive VCF potential)
        var opponentThreats = _threatSearch.GetThreatMoves(board, opponent);
        if (opponentThreats.Count >= 2)
            return true;

        // Check if we have threats (offensive VCF potential)
        var ourThreats = _threatSearch.GetThreatMoves(board, player);
        if (ourThreats.Count >= 2)
            return true;

        // Quiet position - skip VCF
        return false;
    }

    /// <summary>
    /// Main VCF solving logic
    /// </summary>
    private VCFNodeResult? SolveVCF(
        Board board,
        Player attacker,
        int depth,
        int alpha,
        int timeLimitMs,
        System.Diagnostics.Stopwatch stopwatch)
    {
        // First, check for immediate win
        var winResult = _winDetector.CheckWin(board);
        if (winResult.HasWinner && winResult.Winner == attacker)
        {
            return VCFNodeResult.Winning(new List<(int x, int y)>(), 0, 1);
        }

        // Get forcing moves (threats)
        var forcingMoves = _threatSearch.GetThreatMoves(board, attacker);

        if (forcingMoves.Count == 0)
        {
            // No forcing moves - no VCF possible
            CacheNoResult(board.GetHash());
            return null;
        }

        // Try each forcing move recursively
        var bestSequence = new List<(int x, int y)>();
        long nodesSearched = 0;

        foreach (var (x, y) in forcingMoves)
        {
            if (stopwatch.ElapsedMilliseconds > timeLimitMs)
                break;

            // Make move
            var newBoard = board.PlaceStone(x, y, attacker);

            // Check if this leads to win
            var sequence = new List<(int x, int y)> { (x, y) };
            var (found, seq, nodes) = SolveVCFRecursive(
                newBoard, attacker, depth - 1, 1, sequence, timeLimitMs, stopwatch, ref nodesSearched);

            if (found && seq.Count > bestSequence.Count)
            {
                bestSequence = seq;
            }
        }

        if (bestSequence.Count > 0)
        {
            return VCFNodeResult.Winning(bestSequence, bestSequence.Count, nodesSearched);
        }

        CacheNoResult(board.GetHash());
        return null;
    }

    /// <summary>
    /// Recursive VCF search
    /// Returns: (found, sequence, nodes)
    /// </summary>
    private (bool found, List<(int x, int y)> sequence, long nodes) SolveVCFRecursive(
        Board board,
        Player attacker,
        int remainingDepth,
        int currentDepth,
        List<(int x, int y)> currentSequence,
        int timeLimitMs,
        System.Diagnostics.Stopwatch stopwatch,
        ref long totalNodes)
    {
        totalNodes++;

        // Time limit only (no depth cap per algorithmic principles)
        if (stopwatch.ElapsedMilliseconds > timeLimitMs)
            return (false, currentSequence, totalNodes);

        // Check for win
        var winResult = _winDetector.CheckWin(board);
        if (winResult.HasWinner && winResult.Winner == attacker)
        {
            return (true, currentSequence, totalNodes);
        }

        // Get forcing moves
        var forcingMoves = _threatSearch.GetThreatMoves(board, attacker);

        if (forcingMoves.Count == 0)
        {
            return (false, currentSequence, totalNodes);
        }

        // Recurse on each forcing move
        foreach (var (x, y) in forcingMoves)
        {
            var recurseBoard = board.PlaceStone(x, y, attacker);
            var newSequence = new List<(int x, int y)>(currentSequence) { (x, y) };

            var (found, seq, nodes) = SolveVCFRecursive(
                recurseBoard, attacker, remainingDepth - 1, currentDepth + 1,
                newSequence, timeLimitMs, stopwatch, ref totalNodes);

            if (found)
            {
                return (true, seq, nodes);
            }
        }

        return (false, currentSequence, totalNodes);
    }

    /// <summary>
    /// Increment cache age and clean old entries
    /// </summary>
    public void IncrementAge()
    {
        _currentAge++;
        if (_currentAge == 0)
        {
            _currentAge = 1;
            _vcfCache.Clear();
        }

        // Lazy cleanup: remove 10% of old entries
        if (_vcfCache.Count > MaxCacheEntries)
        {
            var toRemove = _vcfCache
                .Where(kvp => kvp.Value.Age != _currentAge)
                .Take(_vcfCache.Count / 10)
                .Select(kvp => kvp.Key);

            foreach (var key in toRemove)
                _vcfCache.TryRemove(key, out _);
        }
    }

    /// <summary>
    /// Clear the cache (called at start of new search)
    /// </summary>
    public void ClearCache()
    {
        _vcfCache.Clear();
        _currentAge = 1;
    }

    private void CacheResult(ulong hash, VCFNodeResult result)
    {
        var entry = new VCFCacheEntry
        {
            Hash = hash,
            ResultType = result.Type,
            Score = result.Score,
            Depth = (byte)result.Depth,
            Age = _currentAge
        };
        _vcfCache.TryAdd(hash, entry);
    }

    private void CacheNoResult(ulong hash)
    {
        var entry = new VCFCacheEntry
        {
            Hash = hash,
            ResultType = VCFResultType.NoVCF,
            Score = 0,
            Depth = 0,
            Age = _currentAge
        };
        _vcfCache.TryAdd(hash, entry);
    }

    private VCFNodeResult CreateResultFromCache(VCFCacheEntry cached)
    {
        return cached.ResultType switch
        {
            VCFResultType.WinningSequence => VCFNodeResult.Winning(
                new List<(int x, int y)>(), cached.Depth, 0),
            VCFResultType.LosingSequence => VCFNodeResult.Losing(
                new List<(int x, int y)>(), cached.Depth, 0),
            _ => VCFNodeResult.None
        };
    }
}
