using System.Collections.Concurrent;
using System.Diagnostics;
using System.Runtime.CompilerServices;
using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic.Search;
using Caro.Core.GameLogic.TimeManagement;
using TMC = Caro.Core.Domain.Configuration.TimeManagementConstants;
using SHC = Caro.Core.Domain.Configuration.SearchHeuristicConstants;
using TC = Caro.Core.Domain.Configuration.TimeConstants;

namespace Caro.Core.GameLogic;

/// <summary>
/// Result from parallel search including move and statistics
/// </summary>
public record ParallelSearchResult(
    int X,
    int Y,
    int DepthAchieved,
    long NodesSearched,
    int ThreadCount = 0,
    string? ParallelDiagnostics = null,
    long AllocatedTimeMs = 0,
    int TableHits = 0,
    int TableLookups = 0,
    int Score = 0,
    double FirstMoveCutoffPercent = 0,  // FMC%: % of beta-cutoffs on 1st move
    double EffectiveBranchingFactor = 0  // EBF: average branching factor during search
);

/// <summary>
/// Parallel Minimax search using Lazy SMP (Shared Memory Parallelism)
/// Multiple threads search independently with shared transposition table
/// Provides 4-8× speedup on multi-core systems
/// Time-aware iterative deepening with move stability detection
///
/// OPTIMIZATIONS:
/// - Lazy SMP with conservative thread count (processorCount/2)-1
/// - MDAP (Move-Dependent Adaptive Pruning) / Late Move Reduction
/// - Iterative deepening with aspiration windows
/// - Killer moves and history heuristic
/// - Lock-free transposition table
/// </summary>
public sealed partial class ParallelMinimaxSearch
{
    // Debug flag for verbose search logging - set to true only during development
    private static readonly bool DebugLogging = false;

    private readonly LockFreeTranspositionTable _transpositionTable;
    private readonly BoardEvaluator _evaluator;
    private readonly WinDetector _winDetector;
    private readonly ThreatSpaceSearch _vcfSolver;
    private readonly ContinuationHistory _continuationHistory = new();
    private readonly CounterMoveHistory _counterMoveHistory = new();
    private readonly Random _random;
    private readonly int _maxThreads;
    private readonly TimeBudgetDepthManager _depthManager = new();

    // Search constants
    private const int MaxSearchRadius = SearchConstants.MaxSearchRadius;
    private const int NullMoveMinDepth = SearchConstants.NullMoveMinDepth;
    private const int NullMoveDepthReduction = SearchConstants.NullMoveDepthReduction;


    // Time management - CancellationTokenSource for proper cross-thread cancellation
    private CancellationTokenSource? _searchCts;
    private TimeMonitor? _timeMonitor;
    private long _hardTimeBoundMs;

    // Per-thread data (not shared between threads)
    private sealed class ThreadData
    {
        public int ThreadIndex; // Identifies master (0) vs helper (1+) threads for diversity logic
        public int SearchRadius; // Candidate generation radius
        public (int x, int y)[,] KillerMoves = new (int x, int y)[20, 2];
        public int[,] HistoryRed = new int[BitBoard.Size, BitBoard.Size];
        public int[,] HistoryBlue = new int[BitBoard.Size, BitBoard.Size];
        public int TableHits;
        public int TableLookups;

        // Diagnostic counters for TT provenance tracking
        public int TTReadsFromMaster;    // Entries from master thread (ThreadIndex=0)
        public int TTReadsFromHelpers;   // Entries from helper threads (ThreadIndex>0)
        public int TTScoresUsed;         // How many TT entries actually returned scores

        // CRITICAL FIX: Thread-local node counting to eliminate cache contention
        // All 9 threads incrementing a shared Interlocked counter on every node causes
        // severe performance degradation. Each thread now counts locally and we aggregate.
        public long LocalNodesSearched;

        // Continuation history: tracks move history for up to 6 previous plies
        // Uses cell indices for efficient lookup
        public int[] MoveHistory = new int[ContinuationHistory.TrackedPlyCount];
        public int MoveHistoryCount;

        // Counter-move history: tracks opponent's last move for response scoring
        // Updated on each move to enable counter-move heuristic
        public int LastOpponentCell = -1;

        // FMC% tracking: First Move Cutoff percentage for move ordering quality
        public long TotalCutoffs;      // Total beta cutoffs
        public long FirstMoveCutoffs;  // Cutoffs on first move (index 0)

        public Random Random = new();

        public void Reset()
        {
            // Clear killer moves
            for (int i = 0; i < 20; i++)
            {
                KillerMoves[i, 0] = (-1, -1);
                KillerMoves[i, 1] = (-1, -1);
            }
            // Clear move history
            Array.Clear(MoveHistory, 0, MoveHistory.Length);
            MoveHistoryCount = 0;
            LastOpponentCell = -1;
        }
    }

    /// <summary>
    /// Create parallel search instance
    /// </summary>
    /// <param name="sizeMB">Transposition table size in MB</param>
    /// <param name="maxThreads">Maximum threads to use (default: uses Lazy SMP formula (n/2)-1)</param>
    public ParallelMinimaxSearch(int sizeMB = SearchConstants.DefaultTTSizeMb, int? maxThreads = null)
    {
        _transpositionTable = new LockFreeTranspositionTable(sizeMB);
        _evaluator = new BoardEvaluator();
        _winDetector = new WinDetector();
        _vcfSolver = new ThreatSpaceSearch();
        _random = Random.Shared;
        // Use Lazy SMP formula (processorCount/2)-1 by default for better stability
        _maxThreads = maxThreads ?? ThreadPoolConfig.GetLazySMPThreadCount();

        // Configure thread pool for CPU-bound work
        ThreadPoolConfig.ConfigureForSearch();
    }

    // Orchestration methods (GetBestMove, SearchLazySMP, etc.) -> ParallelMinimaxSearch.Orchestration.cs
    // Move ordering methods -> ParallelMinimaxSearch.MoveOrdering.cs
    // Pondering methods -> ParallelMinimaxSearch.Pondering.cs


    /// <summary>
    /// Calculate VCF time limit based on time allocation
    /// </summary>
    private int CalculateVCFTimeLimit(TimeAllocation timeAlloc)
    {
        // Emergency mode - very quick VCF check
        if (timeAlloc.IsEmergency)
        {
            return 50;
        }

        // Use a fraction of the soft bound for VCF
        var vcfTime = Math.Max(50, timeAlloc.SoftBoundMs / 10);

        // Cap at reasonable values
        return (int)Math.Min(vcfTime, 500);
    }

    /// <summary>
    /// Get default time allocation when no time limit is specified
    /// Uses full (Grandmaster-level) time budget
    /// </summary>
    private static TimeAllocation GetDefaultTimeAllocation(long? timeRemainingMs)
    {
        // If time remaining is provided but no TimeAllocation, create a simple one
        if (timeRemainingMs.HasValue)
        {
            var timeLeft = timeRemainingMs.Value;

            long softBound;
            if (timeLeft < 60000) // Less than 60 seconds - short time control
            {
                // Use 80% of remaining time for maximum search depth
                const double timePercentage = 0.80;
                softBound = Math.Max(500, (long)(timeLeft * timePercentage));
            }
            else
            {
                const double divisor = 40.0;
                softBound = Math.Max(500, (long)(timeLeft / divisor));
            }
            long hardBound = Math.Min(softBound * 2, timeLeft - 500);

            return new TimeAllocation
            {
                SoftBoundMs = softBound,
                HardBoundMs = hardBound,
                OptimalTimeMs = softBound * 8 / 10,
                IsEmergency = timeLeft < 10000,
                Phase = GamePhase.EarlyMid,
                ComplexityMultiplier = 1.0
            };
        }

        // No time info - use full budget defaults
        return new TimeAllocation
        {
            SoftBoundMs = 5000,
            HardBoundMs = 20000,
            OptimalTimeMs = 4000,
            IsEmergency = false
        };
    }

    /// <summary>
    /// Clear transposition table
    /// </summary>
    public void Clear()
    {
        _transpositionTable.Clear();
        _continuationHistory.Clear();
        _counterMoveHistory.Clear();
    }

    /// <summary>
    /// Get statistics
    /// </summary>
    public (int used, double usagePercent, int hitCount, int lookupCount, double hitRate) GetStats()
    {
        return _transpositionTable.GetStats();
    }


    /// <summary>
    /// Get the shared transposition table (for ponderer access)
    /// </summary>
    public LockFreeTranspositionTable GetTranspositionTable() => _transpositionTable;

    /// <summary>
    /// Stop any ongoing search (used when pondering needs to stop)
    /// </summary>
    public void StopSearch() => _searchCts?.Cancel();

    /// <summary>
    /// Check if search is currently running
    /// </summary>
    public bool IsSearching => _timeMonitor != null && !_timeMonitor.IsTimeUp;

    #region Test Helpers

    /// <summary>
    /// Public test wrapper for OrderMoves to allow testing continuation history integration.
    /// This method is internal for testing purposes only.
    /// </summary>
    internal List<(int x, int y)> OrderMovesPublic(
        List<(int x, int y)> candidates,
        int depth,
        Board board,
        Player player,
        (int x, int y)? cachedMove,
        int[] moveHistory,
        (int x, int y)? killerMove = null)
    {
        // Create a test ThreadData with the provided move history
        var threadData = new ThreadData
        {
            ThreadIndex = 0,
            SearchRadius = MaxSearchRadius
        };

        // Set move history
        for (int i = 0; i < Math.Min(moveHistory.Length, ContinuationHistory.TrackedPlyCount); i++)
        {
            threadData.MoveHistory[i] = moveHistory[i];
        }
        threadData.MoveHistoryCount = Math.Min(moveHistory.Length, ContinuationHistory.TrackedPlyCount);

        // Set killer move if provided
        if (killerMove.HasValue && depth < 20)
        {
            threadData.KillerMoves[depth, 0] = killerMove.Value;
        }

        return OrderMovesLegacyForTesting(candidates, depth, board, player, cachedMove, threadData);
    }

    /// <summary>
    /// Public test wrapper for GetCandidateMoves.
    /// </summary>
    internal static List<(int x, int y)> GetMoveCandidates(Board board)
    {
        // This is a simplified version for testing
        var search = new ParallelMinimaxSearch();
        var searchBoard = new SearchBoard(board);
        return search.GetCandidateMoves(searchBoard, MaxSearchRadius);
    }

    /// <summary>
    /// Get the shared continuation history for testing.
    /// </summary>
    internal ContinuationHistory GetContinuationHistory() => _continuationHistory;
    internal CounterMoveHistory GetCounterMoveHistory() => _counterMoveHistory;

    #endregion
}
