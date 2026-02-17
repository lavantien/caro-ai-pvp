using System.Collections.Concurrent;
using System.Diagnostics;
using System.Runtime.CompilerServices;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic.TimeManagement;

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
    int TableLookups = 0
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
public sealed class ParallelMinimaxSearch
{
    // Debug flag for verbose search logging - set to true only during development
    private const bool DebugLogging = false;

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
    private const int SearchRadius = 2;
    private const int NullMoveMinDepth = 3;
    private const int NullMoveDepthReduction = 3;

    // MDAP (Move-Dependent Adaptive Pruning) constants
    private const int LMRMinDepth = 3;           // Minimum depth to apply LMR
    private const int LMRFullDepthMoves = 4;     // Number of moves searched at full depth
    private const int LMRBaseReduction = 1;      // Base depth reduction for late moves

    // Time management - CancellationTokenSource for proper cross-thread cancellation
    private CancellationTokenSource? _searchCts;
    private Stopwatch? _searchStopwatch;
    private long _hardTimeBoundMs;

    // Per-thread data (not shared between threads)
    private sealed class ThreadData
    {
        public int ThreadIndex; // Identifies master (0) vs helper (1+) threads for diversity logic
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
    public ParallelMinimaxSearch(int sizeMB = 256, int? maxThreads = null)
    {
        _transpositionTable = new LockFreeTranspositionTable(sizeMB);
        _evaluator = new BoardEvaluator();
        _winDetector = new WinDetector();
        _vcfSolver = new ThreatSpaceSearch();
        _random = new Random();
        // Use Lazy SMP formula (processorCount/2)-1 by default for better stability
        _maxThreads = maxThreads ?? ThreadPoolConfig.GetLazySMPThreadCount();

        // Configure thread pool for CPU-bound work
        ThreadPoolConfig.ConfigureForSearch();
    }

    /// <summary>
    /// Check if a move is valid per the Open Rule for Red's second move (move #3)
    /// The Open Rule requires Red's second move to be at least 3 intersections away
    /// from the first red stone (Chebyshev distance >= 3)
    /// </summary>
    private bool IsValidPerOpenRule(Board board, int x, int y)
    {
        // Count stones to verify this is move #3 (2 stones on board)
        int stoneCount = 0;
        (int firstX, int firstY) firstRed = (-1, -1);

        for (int bx = 0; bx < board.BoardSize; bx++)
        {
            for (int by = 0; by < board.BoardSize; by++)
            {
                var cell = board.GetCell(bx, by);
                if (cell.Player != Player.None)
                {
                    stoneCount++;
                    if (cell.Player == Player.Red && firstRed.firstX < 0)
                    {
                        firstRed = (bx, by);
                    }
                }
            }
        }

        // Only applies to move #3 (exactly 2 stones on board)
        if (stoneCount != 2)
            return true;

        // No first red found (shouldn't happen), allow move
        if (firstRed.firstX < 0)
            return true;

        // Check if move is at least 3 intersections away from first red
        // Chebyshev distance: max(|dx|, |dy|) >= 3
        int dx = Math.Abs(x - firstRed.firstX);
        int dy = Math.Abs(y - firstRed.firstY);
        return Math.Max(dx, dy) >= 3;
    }

    /// <summary>
    /// Get best move using parallel search (Lazy SMP)
    /// </summary>
    public (int x, int y) GetBestMove(
        Board board,
        Player player,
        AIDifficulty difficulty,
        long? timeRemainingMs = null,
        TimeAllocation? timeAlloc = null,
        int moveNumber = 0)
    {
        if (player == Player.None)
            throw new ArgumentException("Player cannot be None");

        var baseDepth = AdaptiveDepthCalculator.GetDepth(difficulty, board);
        var candidates = GetCandidateMoves(board);

        // Apply Open Rule: Red's second move (move #3) must be at least 3 intersections
        // away from the first red stone (5x5 exclusion zone centered on first move)
        if (player == Player.Red && moveNumber == 3)
        {
            candidates = candidates.Where(c => IsValidPerOpenRule(board, c.x, c.y)).ToList();
        }

        if (candidates.Count == 0)
        {
            // No valid candidates - board is empty or all filtered out
            if (player == Player.Red && moveNumber == 3)
            {
                // Open rule applies - find first valid cell outside exclusion zone
                int boardSize = board.BoardSize;
                for (int x = 0; x < boardSize; x++)
                {
                    for (int y = 0; y < boardSize; y++)
                    {
                        if (board.GetCell(x, y).Player == Player.None && IsValidPerOpenRule(board, x, y))
                            return (x, y);
                    }
                }
            }
            int center = board.BoardSize / 2;
            return (center, center); // Center move
        }

        // Use provided time allocation or create default
        var alloc = timeAlloc ?? GetDefaultTimeAllocation(difficulty, timeRemainingMs);

        // CRITICAL FIX: Calibrate NPS for difficulty to ensure proper depth scaling
        // Easy targets 50k nps, Medium targets 100k nps, Hard targets 200k nps, Grandmaster targets 500k nps
        _depthManager.CalibrateNpsForDifficulty(difficulty);

        // Try VCF first for higher difficulties
        // CRITICAL FIX: Skip VCF for BookGeneration - full search is sufficient and VCF consumes time budget
        var settings = AIDifficultyConfig.Instance.GetSettings(difficulty);
        if (settings.VCFEnabled && difficulty != AIDifficulty.BookGeneration)
        {
            var vcfTimeLimit = CalculateVCFTimeLimit(alloc);
            var vcfResult = _vcfSolver.SolveVCF(board, player, vcfTimeLimit, maxDepth: 30);

            if (vcfResult.IsSolved && vcfResult.IsWin && vcfResult.BestMove.HasValue)
            {
                return vcfResult.BestMove.Value;
            }
        }

        // Check for opponent's immediate threats that must be blocked
        // CRITICAL FIX: Do NOT return immediately. Filter candidates to ONLY blocking moves and let the engine search.
        // This ensures if there are multiple blocks, we pick the best one (or the one that doesn't lead to a loss later).
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var opponentThreatMoves = GetOpponentThreatMoves(board, opponent);
        if (opponentThreatMoves.Count > 0)
        {
            //Console.WriteLine($"[AI] Threat detected! Considering {opponentThreatMoves.Count} blocking moves.");
            // Filter candidates to only blocking moves
            var forcingSet = new HashSet<(int x, int y)>(opponentThreatMoves);
            candidates = candidates.Where(c => forcingSet.Contains((c.x, c.y))).ToList();

            // If candidates ended up empty (edge case), fallback to original threat list
            if (candidates.Count == 0)
                candidates = opponentThreatMoves;
        }

        // Calibrate NPS for difficulty
        _depthManager.CalibrateNpsForDifficulty(difficulty);

        // NOTE: Error rate is handled in MinimaxAI.GetBestMove() - do not apply again here

        // Multi-threaded Lazy SMP - thread count is determined by difficulty internally
        var parallelResult = SearchLazySMP(board, player, candidates, difficulty, alloc);
        return (parallelResult.X, parallelResult.Y);
    }

    /// <summary>
    /// Get best move using parallel search with full statistics reporting
    /// Returns move coordinates along with depth achieved and nodes searched
    /// </summary>
    public ParallelSearchResult GetBestMoveWithStats(
        Board board,
        Player player,
        AIDifficulty difficulty,
        long? timeRemainingMs = null,
        TimeAllocation? timeAlloc = null,
        int moveNumber = 0,
        int fixedThreadCount = -1,
        List<(int x, int y)>? candidates = null)
    {
        if (player == Player.None)
            throw new ArgumentException("Player cannot be None");

        candidates ??= GetCandidateMoves(board);

        // Apply Open Rule: Red's second move (move #3) must be at least 3 intersections
        // away from the first red stone (5x5 exclusion zone centered on first move)
        if (player == Player.Red && moveNumber == 3)
        {
            candidates = candidates.Where(c => IsValidPerOpenRule(board, c.x, c.y)).ToList();
        }

        if (candidates.Count == 0)
        {
            // Empty board - return center move with depth 1 (not 0, which is misleading)
            // For empty board, center is the only reasonable move
            int center = board.BoardSize / 2;
            return new ParallelSearchResult(center, center, 1, 1, 0, null, 0, 0, 0);
        }

        // Use provided time allocation or create default
        var alloc = timeAlloc ?? GetDefaultTimeAllocation(difficulty, timeRemainingMs);

        // CRITICAL FIX: Calibrate NPS for difficulty to ensure proper depth scaling
        // Easy targets 50k nps, Medium targets 100k nps, Hard targets 200k nps, Grandmaster targets 500k nps
        _depthManager.CalibrateNpsForDifficulty(difficulty);

        // Try VCF first for higher difficulties
        // CRITICAL FIX: Skip VCF for BookGeneration - full search is sufficient and VCF consumes time budget
        var settings = AIDifficultyConfig.Instance.GetSettings(difficulty);
        if (settings.VCFEnabled && difficulty != AIDifficulty.BookGeneration)
        {
            var vcfTimeLimit = CalculateVCFTimeLimit(alloc);
            var vcfResult = _vcfSolver.SolveVCF(board, player, vcfTimeLimit, maxDepth: 30);

            if (vcfResult.IsSolved && vcfResult.IsWin && vcfResult.BestMove.HasValue)
            {
                return new ParallelSearchResult(vcfResult.BestMove.Value.x, vcfResult.BestMove.Value.y,
                    vcfResult.DepthAchieved, vcfResult.NodesSearched, 0, null, vcfTimeLimit, 0, 0);
            }
        }

        // Check for opponent's immediate threats that must be blocked
        // CRITICAL FIX: Do NOT return immediately. Filter candidates to ONLY blocking moves and let the engine search.
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var opponentThreatMoves = GetOpponentThreatMoves(board, opponent);
        if (opponentThreatMoves.Count > 0)
        {
            // Filter candidates to only blocking moves
            var forcingSet = new HashSet<(int x, int y)>(opponentThreatMoves);
            candidates = candidates.Where(c => forcingSet.Contains((c.x, c.y))).ToList();

            // If candidates ended up empty (edge case), fallback to original threat list
            if (candidates.Count == 0)
                candidates = opponentThreatMoves;
        }

        // NOTE: Error rate is handled in MinimaxAI.GetBestMove() - do not apply again here
        // Duplicate error rate checks would effectively double the error rate

        // PURE TIME-BASED: Always use SearchLazySMP which will internally decide thread count
        // based on difficulty. No depth-based decision making.
        return SearchLazySMP(board, player, candidates, difficulty, alloc, fixedThreadCount);
    }

    /// <summary>
    /// Single-threaded search (fallback for low depths)
    /// Note: TranspositionTable age is incremented by caller
    /// Returns node count via out parameter for accurate reporting
    /// </summary>
    private (int x, int y, long nodes) SearchSingleThreaded(Board board, Player player, int depth, List<(int x, int y)> candidates)
    {
        var threadData = new ThreadData();
        var cts = new CancellationTokenSource();
        var token = cts.Token;

        var bestMove = candidates[0];
        var bestScore = int.MinValue;

        foreach (var (x, y) in candidates)
        {
            var newBoard = board.PlaceStone(x, y, player);
            var score = Minimax(newBoard, depth - 1, int.MinValue, int.MaxValue, false, player, depth, threadData, token);

            if (score > bestScore)
            {
                bestScore = score;
                bestMove = (x, y);
            }
        }

        return (bestMove.x, bestMove.y, threadData.LocalNodesSearched);
    }

    /// <summary>
    /// Lazy SMP: Multiple threads search independently with shared TT
    /// Each thread has slight variation to explore different parts of tree
    /// PURE TIME-BASED: No depth caps - search continues until time runs out.
    /// Thread count is based on difficulty, not estimated depth.
    /// </summary>
    private ParallelSearchResult SearchLazySMP(
        Board board,
        Player player,
        List<(int x, int y)> candidates,
        AIDifficulty difficulty,
        TimeAllocation timeAlloc,
        int fixedThreadCount = -1)
    {
        _transpositionTable.IncrementAge();

        // Set up time management with new CancellationTokenSource
        _searchCts?.Cancel(); // Cancel any previous search
        _searchCts = new CancellationTokenSource();
        _searchStopwatch = Stopwatch.StartNew();
        _hardTimeBoundMs = timeAlloc.HardBoundMs;

        // Thread count based on difficulty, not estimated depth
        // fixedThreadCount = 0 means single-threaded, -1 means use difficulty-based
        int threadCount = fixedThreadCount >= 0
            ? fixedThreadCount  // 0 = single-threaded, >0 = use that many threads
            : ThreadPoolConfig.GetThreadCountForDifficulty(difficulty);

        // If threadCount is 0 or 1, fall back to single-threaded search
        if (threadCount <= 1)
        {
            _transpositionTable.IncrementAge();

            // Use time-based single-threaded search (no depth cap)
            var threadData = new ThreadData { ThreadIndex = 0 };
            var (x, y, score, depth, nodes) = SearchWithIterationTimeAware(
                board, player, candidates, threadData, timeAlloc, difficulty, _searchCts.Token);
            return new ParallelSearchResult(x, y, depth, nodes, 1, null, _hardTimeBoundMs, 0, 0);
        }

        // Use thread-safe collections with Task-based parallelism
        var results = new ConcurrentBag<(int x, int y, int score, int depth, long nodes, int threadIndex)>();
        var diagnosticsList = new ConcurrentBag<ThreadData>();

        // Create thread-local copies of board and candidates for each thread
        var boardsArray = new Board[threadCount];
        var candidatesArray = new List<(int x, int y)>[threadCount];
        for (int i = 0; i < threadCount; i++)
        {
            boardsArray[i] = board;
            candidatesArray[i] = new List<(int x, int y)>(candidates);
        }

        // Launch parallel searches using Task.Run with LongRunning option for true parallelism
        // This fixes the memory visibility issue with Thread+ConcurrentBag
        var token = _searchCts.Token;
        var tasks = new Task[threadCount];

        for (int i = 0; i < threadCount; i++)
        {
            int threadId = i;

            // Use Task.Factory.StartNew with LongRunning for dedicated threads
            // This ensures true parallelism similar to the original Thread approach
            tasks[i] = Task.Factory.StartNew(() =>
            {
                // CRITICAL FIX: Create threadData OUTSIDE try block so it's available
                // in finally block for diagnostics collection, even when cancelled
                var threadData = new ThreadData
                {
                    ThreadIndex = threadId,
                    Random = new Random(threadId + (int)DateTime.UtcNow.Ticks)
                };

                try
                {
                    var result = SearchWithIterationTimeAware(
                        boardsArray[threadId], player, candidatesArray[threadId],
                        threadData, timeAlloc, difficulty, token);

                    // Add threadIndex to identify master vs helper thread results
                    var (x, y, score, depthAchieved, nodes) = result;
                    results.Add((x, y, score, depthAchieved, nodes, threadId));
                }
                catch (OperationCanceledException)
                {
                    // Expected when time runs out - not an error
                }
                catch (Exception)
                {
                    // Thread exception - search will continue with available results
                }
                finally
                {
                    // CRITICAL FIX: Always collect diagnostics, even when cancelled
                    // This ensures node counts are available for the fallback calculation
                    diagnosticsList.Add(threadData);
                }
            }, CancellationToken.None, TaskCreationOptions.LongRunning, TaskScheduler.Default);
        }

        // Wait for all tasks to complete with proper synchronization
        // Task.WhenAll ensures all results are visible to this thread
        // CRITICAL FIX: Reduced timeout from HardBoundMs+1000 to HardBoundMs+200
        // The old timeout caused 2x time overrun (wait + fallback both used full allocation)
        try
        {
            Task.WaitAll(tasks, (int)(timeAlloc.HardBoundMs + 200));
        }
        catch (AggregateException)
        {
            // Some tasks may have thrown - continue with available results
        }

        // CRITICAL FIX: Cancel parallel tasks before checking results
        // Without this, timed-out tasks continue running and cause CPU contention
        // with the fallback search, resulting in extremely slow NPS (300-400 vs 5000+)
        _searchCts?.Cancel();

        // Brief wait for tasks to acknowledge cancellation and release resources
        try
        {
            Task.WaitAll(tasks, 100);
        }
        catch (AggregateException)
        {
            // Tasks may throw on cancellation - ignore
        }

        _searchStopwatch.Stop();

        // INTELLIGENT MERGING: Aggregate results from ALL threads
        // Lazy SMP works best when we consider all thread results, not just master
        // Master thread is more reliable (less cancellation), but helpers can find better moves
        //
        // Selection priority:
        // 1. Depth (deeper is always better)
        // 2. Score (at same depth, higher score wins)
        // 3. Thread reliability (master > helper as tiebreaker)
        //
        // This is the MERGER's job - aggregate intelligently, not authoritarian rejection

        // CRITICAL FIX: Calculate remaining time for fallback
        // Parallel search already used time waiting for tasks
        // Fallback should only use remaining time to avoid 2x time overrun
        long elapsedMs = _searchStopwatch?.ElapsedMilliseconds ?? 0;
        long remainingHardBoundMs = Math.Max(50, _hardTimeBoundMs - elapsedMs / 2);  // At least 50ms, account for parallel overhead
        var fallbackTimeAlloc = new TimeAllocation
        {
            SoftBoundMs = Math.Max(25, remainingHardBoundMs / 2),
            HardBoundMs = remainingHardBoundMs,
            OptimalTimeMs = Math.Max(25, remainingHardBoundMs / 4),
            IsEmergency = timeAlloc.IsEmergency,
            Phase = timeAlloc.Phase
        };

        // Group results by depth, then select best within each depth
        if (results.IsEmpty)
        {
            // CRITICAL FIX: Parallel search failed - fall back to single-threaded search
            // Use remaining time allocation, not original
            if (DebugLogging) Console.WriteLine($"[PARALLEL] Falling back to single-threaded (no results) for {difficulty}");
            _searchStopwatch?.Restart();
            _hardTimeBoundMs = fallbackTimeAlloc.HardBoundMs;  // Update hard bound for fallback
            var fallbackThreadData = new ThreadData { ThreadIndex = 0 };
            var (fx, fy, fscore, fdepth, fnodes) = SearchWithIterationTimeAware(
                board, player, candidates, fallbackThreadData, fallbackTimeAlloc, difficulty, CancellationToken.None);
            return new ParallelSearchResult(fx, fy, fdepth, fnodes, 1, null, _hardTimeBoundMs, 0, 0);
        }

        var maxDepth = results.Max(r => r.depth);
        if (maxDepth <= 0)
        {
            // CRITICAL FIX: Parallel search returned invalid depth - fall back to single-threaded
            if (DebugLogging) Console.WriteLine($"[PARALLEL] Falling back to single-threaded (invalid depth) for {difficulty}");
            _searchStopwatch?.Restart();
            _hardTimeBoundMs = fallbackTimeAlloc.HardBoundMs;  // Update hard bound for fallback
            var fallbackThreadData = new ThreadData { ThreadIndex = 0 };
            var (fx, fy, fscore, fdepth, fnodes) = SearchWithIterationTimeAware(
                board, player, candidates, fallbackThreadData, fallbackTimeAlloc, difficulty, CancellationToken.None);
            return new ParallelSearchResult(fx, fy, fdepth, fnodes, 1, null, _hardTimeBoundMs, 0, 0);
        }

        // CRITICAL FIX: Select the best valid result, avoiding int.MinValue scores
        // int.MinValue EXACTLY indicates search failure (cancellation, no moves, etc.)
        // Scores close to int.MinValue (like int.MinValue + 1000) indicate losing positions
        // Strategy: Try maxDepth first, but prefer lower depths with reasonable scores
        // over higher depths with extremely negative scores
        (int x, int y, int score, int depth, long nodes, int threadIndex) bestResult = default;
        bool foundValidResult = false;

        // Score threshold: below this, consider the position "effectively lost"
        // int.MinValue + 1000000 = -2147482648, which is still a valid but terrible score
        // We want to fall back to lower depths if all scores at higher depths are this bad
        const int ReasonableScoreThreshold = int.MinValue + 100000000;  // -2147383648

        // First, try to find results at maxDepth with reasonable scores
        var reasonableAtMaxDepth = results
            .Where(r => r.depth == maxDepth && r.score > ReasonableScoreThreshold)
            .ToList();

        if (reasonableAtMaxDepth.Count > 0)
        {
            // Found reasonable results at max depth - pick the best one
            bestResult = reasonableAtMaxDepth
                .OrderByDescending(r => r.score)  // Highest score first
                .ThenBy(r => r.threadIndex == 0 ? 0 : 1)  // Master thread as tiebreaker
                .First();
            foundValidResult = true;
        }
        else
        {
            // All scores at maxDepth are extremely negative or int.MinValue
            // Try lower depths for better results
            for (int tryDepth = maxDepth - 1; tryDepth >= 1 && !foundValidResult; tryDepth--)
            {
                var validAtDepth = results
                    .Where(r => r.depth == tryDepth && r.score != int.MinValue)
                    .ToList();

                if (validAtDepth.Count > 0)
                {
                    // Found valid results at this depth - pick the best one
                    bestResult = validAtDepth
                        .OrderByDescending(r => r.score)
                        .ThenBy(r => r.threadIndex == 0 ? 0 : 1)
                        .First();
                    foundValidResult = true;
                }
            }

            // If still no valid results, use maxDepth results (even if very negative)
            if (!foundValidResult)
            {
                var anyAtMaxDepth = results
                    .Where(r => r.depth == maxDepth && r.score != int.MinValue)
                    .ToList();

                if (anyAtMaxDepth.Count > 0)
                {
                    bestResult = anyAtMaxDepth
                        .OrderByDescending(r => r.score)
                        .First();
                    foundValidResult = true;
                }
            }
        }

        // If no valid results at any depth, use master thread's result as last resort
        if (!foundValidResult)
        {
            // All threads returned int.MinValue - this should be extremely rare
            // Prefer master thread's result as it has the most reliable search
            var masterResult = results.FirstOrDefault(r => r.threadIndex == 0);
            if (!masterResult.Equals(default))
            {
                bestResult = masterResult;
            }
            else
            {
                // Last resort: pick any result
                bestResult = results.First();
            }
        }

        // DEBUG: Log all thread results and selection
        if (DebugLogging)
        {
            Console.WriteLine($"[PARALLEL DEBUG] Thread results for {difficulty}:");
            foreach (var r in results.OrderByDescending(r => r.depth).ThenBy(r => r.threadIndex))
            {
                Console.WriteLine($"  Thread {r.threadIndex}: move=({r.x},{r.y}), depth={r.depth}, score={r.score}, nodes={r.nodes}");
            }
            Console.WriteLine($"  SELECTED: move=({bestResult.x},{bestResult.y}), depth={bestResult.depth}, score={bestResult.score}, from thread {bestResult.threadIndex}");
        }

        // CRITICAL FIX: Aggregate local node counts from all threads (no Interlocked contention)
        // Each thread counted locally, now sum them up for accurate total
        long totalNodesFinal = results.Sum(r => r.nodes);

        // Build parallel diagnostics string
        var diagBuilder = new System.Text.StringBuilder();

        // Helper thread depths
        var helperResults = results.Where(r => r.threadIndex > 0).ToList();
        if (helperResults.Count > 0)
        {
            var helperDepths = helperResults.Select(r => r.depth).ToList();
            var maxHelperDepth = helperDepths.Max();
            var minHelperDepth = helperDepths.Min();
            var avgHelperDepth = helperDepths.Average();
            diagBuilder.Append($"Helpers: {helperResults.Count} threads, ");
            diagBuilder.Append($"Depths: min={minHelperDepth}, max={maxHelperDepth}, avg={avgHelperDepth:F1}");
        }

        // TT provenance diagnostics
        var masterDiag = diagnosticsList.FirstOrDefault(d => d.ThreadIndex == 0);
        if (masterDiag != null)
        {
            var totalReads = masterDiag.TTReadsFromMaster + masterDiag.TTReadsFromHelpers;
            var masterRate = totalReads > 0 ? (double)masterDiag.TTReadsFromMaster / totalReads * 100 : 0;

            if (diagBuilder.Length > 0)
                diagBuilder.Append("; ");

            diagBuilder.Append($"TT: {masterDiag.TTReadsFromMaster}M/{masterDiag.TTReadsFromHelpers}H reads, ");
            diagBuilder.Append($"{masterRate:F0}% from master");
        }

        string? diagnostics = diagBuilder.Length > 0 ? diagBuilder.ToString() : null;

        // Aggregate TT stats from all threads
        int totalTableHits = diagnosticsList.Sum(d => d.TableHits);
        int totalTableLookups = diagnosticsList.Sum(d => d.TableLookups);

        // DEFENSIVE: Validate the best move is actually in the candidates list and is empty
        // This catches any bugs where the search might return an invalid move
        var bestMoveInCandidates = candidates.Any(c => c.x == bestResult.x && c.y == bestResult.y);
        if (!bestMoveInCandidates)
        {
            // Search returned a move not in candidates - this is a bug
            // Fall back to first candidate
            Console.WriteLine($"[SEARCH ERROR] Best move ({bestResult.x},{bestResult.y}) not in candidates list - using first candidate");
            bestResult = (candidates[0].x, candidates[0].y, bestResult.score, bestResult.depth, bestResult.nodes, bestResult.threadIndex);
        }
        else if (!board.GetCell(bestResult.x, bestResult.y).IsEmpty)
        {
            // Search returned an occupied cell - this is a critical bug
            // Find the first empty candidate
            var emptyCandidate = candidates.FirstOrDefault(c => board.GetCell(c.x, c.y).IsEmpty, candidates[0]);
            Console.WriteLine($"[SEARCH ERROR] Best move ({bestResult.x},{bestResult.y}) is occupied - using fallback ({emptyCandidate.x},{emptyCandidate.y})");
            bestResult = (emptyCandidate.x, emptyCandidate.y, bestResult.score, bestResult.depth, bestResult.nodes, bestResult.threadIndex);
        }

        return new ParallelSearchResult(bestResult.x, bestResult.y, bestResult.depth, totalNodesFinal, threadCount, diagnostics, _hardTimeBoundMs, totalTableHits, totalTableLookups);
    }

    /// <summary>
    /// Iterative deepening search for a single thread with time awareness
    /// Implements move stability detection for early termination
    /// Note: Node counting is done globally via Interlocked in Minimax()
    /// CRITICAL FIX: Only master thread (ThreadIndex=0) can trigger cancellation
    /// Helper threads must NOT cancel as they complete early and would interrupt deeper searches
    ///
    /// PURE TIME-BASED: No depth caps - search continues until time runs out.
    /// Different machines will naturally reach different depths based on their performance.
    /// </summary>
    private (int x, int y, int score, int depth, long nodes) SearchWithIterationTimeAware(
        Board board,
        Player player,
        List<(int x, int y)> candidates,
        ThreadData threadData,
        TimeAllocation timeAlloc,
        AIDifficulty difficulty,
        CancellationToken cancellationToken)
    {
        var bestMove = candidates[0];
        var bestScore = int.MinValue;
        int bestDepth = 1;
        int stableCount = 0;
        long lastIterationElapsedMs = 0;
        long iterationStartMs = 0;  // Track start time of current iteration
        long nodesAtStart = threadData.LocalNodesSearched;  // Track nodes at iteration start

        bool isMasterThread = threadData.ThreadIndex == 0;

        // PURE TIME-BASED SEARCH
        // Search continues until time runs out
        // LAZY SMP: Per Chessprogramming Wiki, helper threads should search at different
        // depths to exploit nondeterminism. Cheng uses: current depth + (1 for each even helper)
        // This provides:
        // 1. Some threads complete deeper searches (D2) before master completes D1
        // 2. Shared hash table benefits from different search paths
        // 3. Nondeterminism from depth diversity + move ordering + timing
        //
        // Implementation:
        // - Master (ThreadIndex=0): Start at depth 1
        // - Helper odd (ThreadIndex=1,3,...): Start at depth 2
        // - Helper even (ThreadIndex=2,4,...): Start at depth 1
        //
        // This ensures at least some threads attempt D2 even at blitz time controls,
        // which is critical because D2 can see immediate threats that D1 cannot.
        int depthOffset = threadData.ThreadIndex % 2 == 1 ? 1 : 0;
        int currentDepth = 1 + depthOffset;
        while (true)
        {
            // Record iteration start time BEFORE any work
            iterationStartMs = _searchStopwatch?.ElapsedMilliseconds ?? 0;

            // TIME BOUND ENFORCEMENT
            // CRITICAL: Always check hard bound, even at D1-D2, to prevent massive time overruns
            // At blitz time controls, D2 can take 2+ seconds which exceeds the 900ms budget
            var elapsedForCheck = _searchStopwatch?.ElapsedMilliseconds ?? 0;
            long remainingTimeMs = _hardTimeBoundMs - elapsedForCheck;

            // Hard bound check - ALL threads must stop when time is up
            if (elapsedForCheck >= _hardTimeBoundMs)
            {
                if (isMasterThread) _searchCts?.Cancel();
                break;
            }

            // PRE-ITERATION TIME ESTIMATE
            // Estimate if the next iteration can complete in time.
            // Each depth iteration typically takes 2-4x the previous iteration.
            // Only apply for D3+ to ensure at least D2 is attempted.
            if (currentDepth > 2 && lastIterationElapsedMs > 0 && remainingTimeMs < lastIterationElapsedMs * 2)
            {
                // Not enough time to complete the next iteration - stop now
                break;
            }

            // Check cancellation
            if (cancellationToken.IsCancellationRequested)
                break;

            // For depth 3+, use soft bound and optimal time checks
            if (currentDepth > 2)
            {
                // SOFT BOUND: Stop early if we're approaching time limit
                if (elapsedForCheck >= _hardTimeBoundMs * 0.9)
                {
                    if (isMasterThread) _searchCts?.Cancel();
                    break;
                }

                // PURE TIME-BASED: Check if we should continue based on iteration time
                if (isMasterThread && elapsedForCheck >= timeAlloc.SoftBoundMs)
                {
                    if (lastIterationElapsedMs > remainingTimeMs * 0.5)
                        break;
                }

                // Optimal time check - very stable moves can stop earlier
                if (isMasterThread && elapsedForCheck >= timeAlloc.OptimalTimeMs && stableCount >= 3)
                {
                    if (lastIterationElapsedMs > remainingTimeMs * 0.4)
                        break;
                }
            }

            int alpha = int.MinValue + 1000;
            int beta = int.MaxValue - 1000;

            if (bestScore > int.MinValue + 2000 && bestScore < int.MaxValue - 2000)
            {
                alpha = Math.Max(int.MinValue + 1000, bestScore - 50);
                beta = Math.Min(int.MaxValue - 1000, bestScore + 50);
            }

            // Track nodes before this iteration to detect if search actually happened
            long nodesBeforeIteration = threadData.LocalNodesSearched;

            var result = SearchRoot(board, player, currentDepth, candidates, threadData, alpha, beta, cancellationToken);

            var elapsedNow = _searchStopwatch?.ElapsedMilliseconds ?? 0;
            lastIterationElapsedMs = elapsedNow - iterationStartMs;  // Time for THIS iteration only

            // CRITICAL FIX: Only update bestMove/bestDepth if search actually happened
            // SearchRoot may return early with (candidates[0], 0) if time is up
            // We detect this by checking if any nodes were searched
            long nodesSearchedThisIteration = threadData.LocalNodesSearched - nodesBeforeIteration;
            if (nodesSearchedThisIteration == 0)
            {
                // No actual search happened - SearchRoot returned early
                // Don't update bestDepth, continue to next iteration or break
                if (currentDepth > 2 && elapsedNow >= _hardTimeBoundMs * 0.8)
                {
                    // Running out of time and no search completed - stop trying deeper
                    break;
                }
                currentDepth++;
                continue;
            }

            // CRITICAL FIX: Update bestMove/bestScore BEFORE checking cancellation
            // If we completed the search, we should use the result even if cancellation is requested
            // BUT only update if the score is valid (not int.MinValue from aborted search)
            if (result.score == int.MinValue)
            {
                // Search was aborted - don't update anything, keep previous iteration's result
            }
            else if (result.x == bestMove.Item1 && result.y == bestMove.Item2)
                stableCount++;
            else
            {
                stableCount = 1;
                bestMove = (result.x, result.y);
            }

            // CRITICAL FIX: Only update bestMove/bestDepth when score is valid
            // SearchRoot returns int.MinValue when search was aborted (timeout/cancellation)
            // We should NOT update bestMove with garbage results
            if (result.score > bestScore || bestMove == (-1, -1))
            {
                // Only update bestScore if it's a real search result (not int.MinValue)
                // int.MinValue means the search was aborted and returned first candidate
                if (result.score != int.MinValue)
                {
                    bestScore = result.score;
                    bestMove = (result.x, result.y);
                    bestDepth = currentDepth;
                }
                else if (bestMove == (-1, -1))
                {
                    // First iteration with aborted search - use result but keep bestScore at int.MinValue
                    // This is a fallback for the very first search when even D1 times out
                    bestMove = (result.x, result.y);
                    bestDepth = currentDepth;
                }
            }

            // NOW check cancellation - after saving the result
            if (cancellationToken.IsCancellationRequested)
                break;

            if (result.score >= 100000)
                break;

            currentDepth++;  // Increment depth for next iteration
        }

        return (bestMove.x, bestMove.y, bestScore, bestDepth, threadData.LocalNodesSearched);
    }

    /// <summary>
    /// Estimate nodes searched for a given depth and candidate count
    /// This is a rough approximation for statistics reporting
    /// </summary>
    private long EstimateNodes(int depth, int candidateCount)
    {
        // Approximate: candidate_count * (branching_factor^(depth-1))
        // Where branching factor is capped at a reasonable value
        long nodes = candidateCount;
        int branching = Math.Min(candidateCount, 25); // Cap effective branching
        for (int i = 1; i < depth && i < 6; i++) // Limit estimation depth
        {
            nodes *= branching;
        }
        return nodes;
    }

    /// <summary>
    /// Root search with aspiration window
    /// </summary>
    private (int x, int y, int score) SearchRoot(
        Board board, Player player, int depth, List<(int x, int y)> candidates,
        ThreadData threadData, int alpha, int beta, CancellationToken cancellationToken)
    {
        // Quick time check before starting search at this depth
        if (_searchStopwatch != null && _hardTimeBoundMs > 0)
        {
            var elapsed = _searchStopwatch.ElapsedMilliseconds;
            if (elapsed >= _hardTimeBoundMs)
            {
                // CRITICAL FIX: Return int.MinValue to indicate invalid/timeout result
                // Score 0 was being picked by result merging as a "valid" result
                // int.MinValue signals "this search did not complete"
                return (candidates[0].x, candidates[0].y, int.MinValue);
            }
        }

        var bestMove = candidates[0];
        var bestScore = int.MinValue;

        var orderedMoves = OrderMovesStaged(candidates, depth, board, player, null, threadData);

        int moveIndex = 0;
        foreach (var (x, y) in orderedMoves)
        {
            // CRITICAL: Check time before each move evaluation
            // This catches timeout during long candidate loops
            if (_searchStopwatch != null && _hardTimeBoundMs > 0)
            {
                if (_searchStopwatch.ElapsedMilliseconds >= _hardTimeBoundMs)
                {
                    if (threadData.ThreadIndex == 0) _searchCts?.Cancel();
                    break;
                }
            }

            var newBoard = board.PlaceStone(x, y, player);
            var score = Minimax(newBoard, depth - 1, alpha, beta, false, player, depth, threadData, cancellationToken);

            // DEBUG: Log score for each move
            if (DebugLogging && threadData.ThreadIndex == 0 && moveIndex < 5)
            {
                Console.WriteLine($"  [SearchRoot] Thread {threadData.ThreadIndex}: move=({x},{y}), score={score}");
            }

            // CRITICAL FIX: Update bestScore/bestMove BEFORE checking cancellation
            // If Minimax completed successfully (score != int.MinValue), we should use the result
            // even if cancellation was requested during the search.
            // Only skip updating if the score is int.MinValue (which means Minimax was cancelled)
            if (score != int.MinValue && score > bestScore)
            {
                bestScore = score;
                bestMove = (x, y);
            }

            // NOW check cancellation - after saving any valid result
            if (cancellationToken.IsCancellationRequested)
            {
                break;
            }

            // Additional time check every 8 moves to catch timeout faster
            if ((++moveIndex & 7) == 0 && _searchStopwatch != null && _hardTimeBoundMs > 0)
            {
                if (_searchStopwatch.ElapsedMilliseconds >= _hardTimeBoundMs)
                {
                    if (threadData.ThreadIndex == 0) _searchCts?.Cancel();
                    break;
                }
            }

            alpha = Math.Max(alpha, score);
            if (beta <= alpha)
            {
                RecordKillerMove(threadData, depth, x, y);
                break;
            }
        }

        // LAZY SMP TT WRITING: Allow helper threads to write with quality criteria
        // See Minimax() function for detailed explanation
        bool shouldStore = threadData.ThreadIndex == 0 || depth >= 3;
        if (shouldStore)
        {
            _transpositionTable.Store(board.GetHash(), (sbyte)depth, (short)bestScore, (sbyte)bestMove.x, (sbyte)bestMove.y, alpha, beta, (byte)threadData.ThreadIndex, rootDepth: depth);
        }

        return (bestMove.x, bestMove.y, bestScore);
    }

    /// <summary>
    /// Minimax with alpha-beta pruning (thread-safe via per-thread data)
    /// </summary>
    private int Minimax(Board board, int depth, int alpha, int beta, bool isMaximizing, Player aiPlayer, int rootDepth, ThreadData threadData, CancellationToken cancellationToken)
    {
        // CRITICAL FIX: Count nodes locally to avoid cache contention from Interlocked
        // All 9 threads incrementing shared counter on every node = severe bottleneck
        threadData.LocalNodesSearched++;

        // Time management check - use CancellationToken for proper cancellation
        // CRITICAL FIX: Return int.MinValue on cancellation to signal invalid result
        // Returning 0 was corrupting results - score 0 looks valid but is meaningless
        if (cancellationToken.IsCancellationRequested)
        {
            // Return a value that won't be selected as "best" by either player
            // For maximizing: int.MinValue is worst possible, won't be selected
            // For minimizing: int.MinValue is best possible, but we handle this by
            // checking if bestScore stayed at int.MinValue in the caller
            return int.MinValue;
        }

        // Periodic time check - every 16 nodes to catch timeouts faster
        // CRITICAL: Check both time AND cancellation token for immediate abort
        if ((_searchStopwatch != null && _hardTimeBoundMs > 0 && (threadData.LocalNodesSearched & 15) == 0) ||
            ((threadData.LocalNodesSearched & 15) == 0 && cancellationToken.IsCancellationRequested))
        {
            // Check cancellation token first for immediate abort
            if (cancellationToken.IsCancellationRequested)
                return int.MinValue;

            var elapsed = _searchStopwatch!.ElapsedMilliseconds;
            if (elapsed >= _hardTimeBoundMs)
            {
                // Master thread triggers cancellation for all other threads
                if (threadData.ThreadIndex == 0)
                {
                    _searchCts?.Cancel();
                }
                return int.MinValue;
            }
        }

        // Terminal check
        var winner = CheckWinner(board);
        if (winner != null)
        {
            return winner == aiPlayer ? 100000 : -100000;
        }

        if (depth == 0)
        {
            // Use quiescence search to resolve tactical positions
            // This extends search in positions with active threats to avoid horizon effect
            return Quiesce(board, alpha, beta, isMaximizing, aiPlayer, rootDepth, threadData, cancellationToken);
        }

        var candidates = GetCandidateMoves(board);
        if (candidates.Count == 0)
        {
            return 0; // Draw
        }

        // TT lookup with provenance-based selective reading
        // MASTER THREAD (ThreadIndex=0): Ignores ALL helper entries for score
        // HELPER THREADS (ThreadIndex>0): Can use any TT entry for diversity
        //
        // The root cause of the regression was that helper threads write entries
        // with inconsistent bounds due to early cancellation during iterative deepening.
        // The master thread would then use these entries and make suboptimal decisions.
        //
        // FIX: Master thread completely ignores helper-written entries for scoring.
        // Helper entries can still be used for move ordering (cachedMove), which is safe.

        var boardHash = board.GetHash();
        threadData.TableLookups++;
        var (found, hasExactDepth, cachedScore, cachedMove, ttThreadIndex) = _transpositionTable.Lookup(boardHash, (sbyte)depth, alpha, beta);

        // Track lookups for diagnostics (even if we don't use the result)
        if (found)
        {
            if (ttThreadIndex == 0)
                threadData.TTReadsFromMaster++;
            else
                threadData.TTReadsFromHelpers++;
        }

        // Master thread TT reading policy for Lazy SMP:
        // Helper write policy ensures quality: depth >= rootDepth/2 AND exact scores only.
        // Master thread uses all valid helper entries for proper Lazy SMP operation.
        // The write policy is the quality gate - if helper stored it, we can use it.
        bool shouldUseScore = found && hasExactDepth;

        // Use the score if we have a valid exact-depth entry
        if (shouldUseScore)
        {
            threadData.TableHits++;
            threadData.TTScoresUsed++;
            return cachedScore;
        }

        var currentPlayer = isMaximizing ? aiPlayer : (aiPlayer == Player.Red ? Player.Blue : Player.Red);
        var orderedMoves = OrderMovesStaged(candidates, rootDepth - depth, board, currentPlayer, cachedMove, threadData);

        int bestScore = isMaximizing ? int.MinValue : int.MaxValue;
        (int x, int y)? bestMove = null;
        int moveIndex = 0;

        foreach (var (x, y) in orderedMoves)
        {
            // MDAP: Move-Dependent Adaptive Pruning (Adaptive Late Move Reduction)
            // Apply dynamic depth reduction based on position characteristics
            int reducedDepth = depth;
            bool doLMR = false;

            // Get history score for this move
            var historyTable = currentPlayer == Player.Red ? threadData.HistoryRed : threadData.HistoryBlue;
            int historyScore = historyTable[x, y];

            // Determine move characteristics for adaptive LMR
            bool isImproving = IsImproving(board, currentPlayer);
            bool isPvNode = beta - alpha <= 1;
            bool isCutNode = !isPvNode && beta - alpha > 1;
            bool isTTMove = cachedMove.HasValue && cachedMove.Value == (x, y);

            // Calculate adaptive reduction based on multiple factors
            int adaptiveReduction = GetAdaptiveReduction(
                depth, moveIndex, isImproving, isPvNode, isCutNode, isTTMove, historyScore);

            if (adaptiveReduction > 0)
            {
                reducedDepth = depth - adaptiveReduction;
                if (reducedDepth < 1) reducedDepth = 1;
                doLMR = true;
            }

            // Push current move to history for continuation tracking
            int currentCell = y * BitBoard.Size + x;

            // Save opponent's last move for counter-move history before updating
            // MoveHistory[0] contains opponent's last move (from 1 ply ago)
            if (threadData.MoveHistoryCount > 0)
            {
                threadData.LastOpponentCell = threadData.MoveHistory[0];
            }

            if (threadData.MoveHistoryCount < ContinuationHistory.TrackedPlyCount)
            {
                // Shift existing history to make room at the front
                for (int j = Math.Min(threadData.MoveHistoryCount, ContinuationHistory.TrackedPlyCount - 1); j > 0; j--)
                {
                    threadData.MoveHistory[j] = threadData.MoveHistory[j - 1];
                }
                threadData.MoveHistory[0] = currentCell;
                threadData.MoveHistoryCount = Math.Min(threadData.MoveHistoryCount + 1, ContinuationHistory.TrackedPlyCount);
            }

            var newBoard = board.PlaceStone(x, y, currentPlayer);
            int score;

            if (doLMR)
            {
                // Search with reduced depth first
                score = Minimax(newBoard, reducedDepth - 1, alpha, beta, !isMaximizing, aiPlayer, rootDepth, threadData, cancellationToken);

                // If reduced depth search returns a score that could improve alpha/beta,
                // re-search at full depth (verification)
                if ((isMaximizing && score > alpha) || (!isMaximizing && score < beta))
                {
                    score = Minimax(newBoard, depth - 1, alpha, beta, !isMaximizing, aiPlayer, rootDepth, threadData, cancellationToken);
                }
            }
            else
            {
                // Full depth search for early/high-priority moves
                score = Minimax(newBoard, depth - 1, alpha, beta, !isMaximizing, aiPlayer, rootDepth, threadData, cancellationToken);
            }
            moveIndex++;

            // Check if search was stopped during recursion
            if (cancellationToken.IsCancellationRequested)
            {
                return bestScore; // Return best we found so far
            }

            if (isMaximizing)
            {
                if (score > bestScore)
                {
                    bestScore = score;
                    bestMove = (x, y);
                }
                alpha = Math.Max(alpha, score);
            }
            else
            {
                if (score < bestScore)
                {
                    bestScore = score;
                    bestMove = (x, y);
                }
                beta = Math.Min(beta, score);
            }

            if (beta <= alpha)
            {
                // Cutoff
                if (depth >= 2 && depth < 20)
                {
                    RecordKillerMove(threadData, rootDepth - depth, x, y);
                }
                RecordHistoryMove(threadData, currentPlayer, x, y, depth);

                // Update continuation history for this successful move
                // Use move history to update continuation scores
                int bonus = depth * depth * 4;
                for (int j = 1; j < threadData.MoveHistoryCount && j <= ContinuationHistory.TrackedPlyCount; j++)
                {
                    int prevCell = threadData.MoveHistory[j];
                    _continuationHistory.Update(currentPlayer, prevCell, currentCell, bonus);
                }

                // Update counter-move history for this successful response
                // Tracks: opponent's last move -> our response (current move)
                if (threadData.LastOpponentCell >= 0)
                {
                    _counterMoveHistory.Update(currentPlayer, threadData.LastOpponentCell, currentCell, bonus);
                }

                break;
            }

            // Pop move from history (shift back)
            if (threadData.MoveHistoryCount > 0)
            {
                for (int j = 0; j < threadData.MoveHistoryCount - 1; j++)
                {
                    threadData.MoveHistory[j] = threadData.MoveHistory[j + 1];
                }
                threadData.MoveHistoryCount--;
            }
        }

        // LAZY SMP TT WRITING: All threads (master and helper) use identical write policy
        // This is essential for Lazy SMP - helper threads populate TT with results
        // from different parts of the tree, allowing master thread to benefit.
        //
        // ALL THREADS use the same logic - no helper restrictions.
        // Only difference is threadIndex tracking for diagnostics.
        // The TT replacement strategy handles quality naturally via depth-based replacement.

        if (bestMove.HasValue)
        {
            var flag = (bestScore <= alpha)
                ? LockFreeTranspositionTable.EntryFlag.UpperBound
                : (bestScore >= beta ? LockFreeTranspositionTable.EntryFlag.LowerBound : LockFreeTranspositionTable.EntryFlag.Exact);

            _transpositionTable.Store(boardHash, (sbyte)depth, (short)bestScore,
                (sbyte)bestMove.Value.x, (sbyte)bestMove.Value.y, alpha, beta, (byte)threadData.ThreadIndex, rootDepth);
        }

        return bestScore;
    }

    /// <summary>
    /// Order moves using staged move picker for better alpha-beta cutoff rates.
    /// </summary>
    private List<(int x, int y)> OrderMovesStaged(
        List<(int x, int y)> candidates,
        int depth,
        Board board,
        Player player,
        (int x, int y)? cachedMove,
        ThreadData threadData)
    {
        // Convert ThreadData to MovePicker.ThreadData
        var pickerThreadData = ConvertToPickerThreadData(threadData);

        var picker = new MovePicker(
            candidates,
            board,
            player,
            depth,
            cachedMove,
            pickerThreadData,
            _continuationHistory,
            _counterMoveHistory);

        return picker.GetRemainingMoves();
    }

    /// <summary>
    /// Convert ParallelMinimaxSearch.ThreadData to MovePicker.ThreadData.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static MovePicker.ThreadData ConvertToPickerThreadData(ThreadData source)
    {
        var target = new MovePicker.ThreadData
        {
            ThreadIndex = source.ThreadIndex,
            MoveHistoryCount = source.MoveHistoryCount,
            LastOpponentCell = source.LastOpponentCell
        };

        // Copy killer moves
        for (int i = 0; i < 20; i++)
        {
            target.KillerMoves[i, 0] = source.KillerMoves[i, 0];
            target.KillerMoves[i, 1] = source.KillerMoves[i, 1];
        }

        // Copy history tables
        for (int x = 0; x < BitBoard.Size; x++)
        {
            for (int y = 0; y < BitBoard.Size; y++)
            {
                target.HistoryRed[x, y] = source.HistoryRed[x, y];
                target.HistoryBlue[x, y] = source.HistoryBlue[x, y];
            }
        }

        // Copy move history
        for (int i = 0; i < source.MoveHistoryCount && i < source.MoveHistory.Length; i++)
        {
            target.MoveHistory[i] = source.MoveHistory[i];
        }

        return target;
    }

    /// <summary>
    /// <summary>
    /// Legacy move ordering for testing continuation history integration.
    /// Production code uses OrderMovesStaged with MovePicker for better performance.
    /// </summary>
    private List<(int x, int y)> OrderMovesLegacyForTesting(List<(int x, int y)> candidates, int depth, Board board, Player player, (int x, int y)? cachedMove, ThreadData threadData)
    {
        int count = candidates.Count;
        if (count == 0) return candidates; // Safety check
        if (count == 1) return candidates;

        Span<int> scores = stackalloc int[count];
        var historyTable = player == Player.Red ? threadData.HistoryRed : threadData.HistoryBlue;

        // Check filtering for threats (Optimization from previous fix)
        // Note: For Lazy SMP to work best, we should usually search ALL candidates,
        // but high priority moves must come first.
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var threatMoves = GetOpponentThreatMoves(board, opponent);

        for (int i = 0; i < count; i++)
        {
            var (x, y) = candidates[i];
            int score = 0;

            // 1. Mandatory Blocks (Highest Priority - Deterministic)
            if (threatMoves.Contains((x, y)))
                score += 2000000;

            // 2. Hash Move (High Priority)
            if (cachedMove.HasValue && cachedMove.Value == (x, y))
                score += 1000000;

            // 3. Continuation History (Higher priority than killer moves)
            // Weight formula: 2 * mainHistory + sum(continuationHistory[0..2])
            // Continuation history tracks which moves have been good after previous moves
            int currentCell = y * BitBoard.Size + x;
            int continuationScore = 0;
            for (int j = 0; j < threadData.MoveHistoryCount && j < ContinuationHistory.TrackedPlyCount; j++)
            {
                int prevCell = threadData.MoveHistory[j];
                continuationScore += _continuationHistory.GetScore(player, prevCell, currentCell);
            }
            // Weight: continuation history gets bonus up to 300000
            score += Math.Min(continuationScore * 3, 300000);

            // 3b. Counter-Move History (Response to opponent's last move)
            // Tracks which responses have been good after opponent's specific moves
            int counterMoveScore = _counterMoveHistory.GetScore(player, threadData.LastOpponentCell, currentCell);
            // Weight: counter-move history gets bonus up to 150000 (half of continuation)
            score += Math.Min(counterMoveScore * 2, 150000);

            // 4. Winning Move (Tactical)
            // (You can call EvaluateTactical here if you want greater precision)

            // 5. Killer Moves
            if (depth < 20)
            {
                if (threadData.KillerMoves[depth, 0] == (x, y)) score += 500000;
                else if (threadData.KillerMoves[depth, 1] == (x, y)) score += 400000;
            }

            // 6. History Heuristic (weighted 2x as part of composite score)
            score += Math.Min(historyTable[x, y] * 2, 20000);

            // 7. Center Preference & Proximity
            int center = board.BoardSize / 2;
            int centerDist = Math.Abs(x - center) + Math.Abs(y - center);
            score += ((board.BoardSize * 2 - 4) - centerDist) * 100;
            score += GetProximityScore(x, y, board) * 10;

            // NOISE REMOVED: Lazy SMP diversity comes naturally from:
            // - Different threads starting at slightly different times
            // - Different cutoffs during search
            // - Different TT entries from shared table
            // Artificial noise injection was causing helper threads to explore
            // bad branches and corrupt the result selection.

            scores[i] = score;
        }

        return InsertionSort(candidates, scores);
    }

    /// <summary>
    /// Insertion sort with zero allocations
    /// </summary>
    private List<(int x, int y)> InsertionSort(List<(int x, int y)> moves, Span<int> scores)
    {
        for (int i = 1; i < moves.Count; i++)
        {
            int j = i;
            while (j > 0 && scores[j] > scores[j - 1])
            {
                // Swap moves
                var tempMove = moves[j];
                moves[j] = moves[j - 1];
                moves[j - 1] = tempMove;

                // Swap scores
                int tempScore = scores[j];
                scores[j] = scores[j - 1];
                scores[j - 1] = tempScore;

                j--;
            }
        }
        return moves;
    }

    /// <summary>
    /// Get candidate moves near existing stones
    /// For empty board, returns center-area moves for the opening
    /// </summary>
    private List<(int x, int y)> GetCandidateMoves(Board board)
    {
        var candidates = new List<(int x, int y)>(64);
        int boardSize = board.BoardSize;
        var considered = new bool[boardSize, boardSize];

        var playerBitBoard = board.GetBitBoard(Player.Red);
        var opponentBitBoard = board.GetBitBoard(Player.Blue);
        var occupied = playerBitBoard | opponentBitBoard;

        // Check if board is empty (no stones placed)
        bool boardIsEmpty = true;
        for (int x = 0; x < boardSize && boardIsEmpty; x++)
        {
            for (int y = 0; y < boardSize && boardIsEmpty; y++)
            {
                if (occupied.GetBit(x, y))
                    boardIsEmpty = false;
            }
        }

        // Empty board - return center-area moves for opening
        if (boardIsEmpty)
        {
            // Return center 3x3 area as candidates (standard opening positions)
            int center = boardSize / 2;
            for (int x = center - 1; x <= center + 1; x++)
            {
                for (int y = center - 1; y <= center + 1; y++)
                {
                    candidates.Add((x, y));
                }
            }
            return candidates;
        }

        // Find all cells within SearchRadius of existing stones
        for (int x = 0; x < boardSize; x++)
        {
            for (int y = 0; y < boardSize; y++)
            {
                if (occupied.GetBit(x, y))
                {
                    // Add neighbors as candidates
                    for (int dx = -SearchRadius; dx <= SearchRadius; dx++)
                    {
                        for (int dy = -SearchRadius; dy <= SearchRadius; dy++)
                        {
                            int nx = x + dx;
                            int ny = y + dy;
                            if (nx >= 0 && nx < boardSize && ny >= 0 && ny < boardSize &&
                                !occupied.GetBit(nx, ny) && !considered[nx, ny])
                            {
                                candidates.Add((nx, ny));
                                considered[nx, ny] = true;
                            }
                        }
                    }
                }
            }
        }

        return candidates;
    }

    /// <summary>
    /// Calculate proximity score (prefer moves near existing stones)
    /// </summary>
    private int GetProximityScore(int x, int y, Board board)
    {
        int boardSize = board.BoardSize;
        var playerBitBoard = board.GetBitBoard(Player.Red);
        var opponentBitBoard = board.GetBitBoard(Player.Blue);
        int score = 0;

        for (int dx = -2; dx <= 2; dx++)
        {
            for (int dy = -2; dy <= 2; dy++)
            {
                int nx = x + dx;
                int ny = y + dy;
                if (nx >= 0 && nx < boardSize && ny >= 0 && ny < boardSize)
                {
                    if (playerBitBoard.GetBit(nx, ny)) score += 3;
                    if (opponentBitBoard.GetBit(nx, ny)) score += 2;
                }
            }
        }

        return score;
    }

    /// <summary>
    /// Record killer move
    /// </summary>
    private void RecordKillerMove(ThreadData threadData, int depth, int x, int y)
    {
        if (depth >= 0 && depth < 20)
        {
            // Shift existing killers
            threadData.KillerMoves[depth, 1] = threadData.KillerMoves[depth, 0];
            threadData.KillerMoves[depth, 0] = (x, y);
        }
    }

    /// <summary>
    /// Record history move
    /// </summary>
    private void RecordHistoryMove(ThreadData threadData, Player player, int x, int y, int depth)
    {
        var table = player == Player.Red ? threadData.HistoryRed : threadData.HistoryBlue;
        table[x, y] += depth * depth;
    }

    /// <summary>
    /// Calculate adaptive late move reduction based on position and move characteristics.
    /// Uses multiple factors to determine optimal reduction:
    /// - Depth: Deeper searches can reduce more
    /// - Move count: Later moves get more reduction
    /// - Improving: Positions with better static eval get less reduction
    /// - PV node: Principal variation nodes get less reduction
    /// - Cut node: Nodes that are likely to cutoff get more reduction
    /// - TT move: Transposition table moves get no reduction
    /// - History score: Moves with good history get less reduction
    ///
    /// Expected ELO gain: +25-40 through better search efficiency.
    /// </summary>
    [System.Runtime.CompilerServices.MethodImpl(System.Runtime.CompilerServices.MethodImplOptions.AggressiveInlining)]
    private int GetAdaptiveReduction(
        int depth,
        int moveCount,
        bool improving,
        bool isPvNode,
        bool isCutNode,
        bool isTTMove,
        int historyScore)
    {
        // Early moves get no reduction
        if (moveCount < LMRFullDepthMoves)
            return 0;

        // Minimum depth must be met
        if (depth < LMRMinDepth)
            return 0;

        int reduction = LMRBaseReduction;

        // Depth-based adjustment: deeper searches can reduce more
        // For each 3 plies beyond minimum, add 1 to reduction
        reduction += (depth - LMRMinDepth) / 3;

        // Move count adjustment: later moves get more reduction
        // For every 4 moves beyond LMRFullDepthMoves, add 1 to reduction
        reduction += (moveCount - LMRFullDepthMoves) / 4;

        // Improving positions get less reduction (more valuable to search accurately)
        if (improving)
            reduction -= 1;

        // PV nodes get less reduction (more important for accuracy)
        if (isPvNode)
            reduction -= 1;

        // Cut nodes get more reduction (likely to cutoff anyway)
        if (isCutNode)
            reduction += 1;

        // TT moves get no reduction (highest priority move)
        if (isTTMove)
            reduction = 0;

        // High history scores get less reduction (these moves have been good)
        // Scale: historyScore up to 30000, divide by 10000 = up to 3 reduction bonus
        int historyBonus = Math.Min(3, historyScore / 10000);
        reduction -= historyBonus;

        // Ensure reduction is valid: non-negative and less than depth
        reduction = Math.Max(0, reduction);
        reduction = Math.Min(depth - 1, reduction);

        return reduction;
    }

    /// <summary>
    /// Check if a position is improving (better than previous evaluation).
    /// This is a simplified check that uses material balance as a proxy.
    /// In a full implementation, this would track the evaluation from previous plies.
    /// </summary>
    [System.Runtime.CompilerServices.MethodImpl(System.Runtime.CompilerServices.MethodImplOptions.AggressiveInlining)]
    private bool IsImproving(Board board, Player player)
    {
        // Simplified: a position is "improving" if the current player has equal or more material
        // This is a basic heuristic; a full implementation would track eval across plies
        var redBitBoard = board.GetBitBoard(Player.Red);
        var blueBitBoard = board.GetBitBoard(Player.Blue);

        int redCount = redBitBoard.CountBits();
        int blueCount = blueBitBoard.CountBits();

        // Current player is improving if they have equal or more stones
        if (player == Player.Red)
            return redCount >= blueCount;
        else
            return blueCount >= redCount;
    }

    /// <summary>
    /// Check for winner
    /// </summary>
    private Player? CheckWinner(Board board)
    {
        var result = _winDetector.CheckWin(board);
        return result.HasWinner ? result.Winner : null;
    }

    /// <summary>
    /// Quiescence search: extend search in tactical positions to get accurate evaluation
    /// Only considers moves near existing stones (tactical moves)
    /// Prevents horizon effect by searching deeper when there are active threats
    /// </summary>
    private int Quiesce(Board board, int alpha, int beta, bool isMaximizing, Player aiPlayer, int quiesceDepth, ThreadData threadData, CancellationToken cancellationToken)
    {
        // Count quiescence node locally (no Interlocked contention)
        threadData.LocalNodesSearched++;

        // Check cancellation
        if (cancellationToken.IsCancellationRequested)
        {
            return isMaximizing ? alpha : beta;
        }

        // Get stand-pat score (static evaluation)
        var standPat = Evaluate(board, aiPlayer);

        // Beta cutoff (stand-pat is good enough for maximizing player)
        if (isMaximizing && standPat >= beta)
            return beta;

        // Alpha cutoff (stand-pat is good enough for minimizing player)
        if (!isMaximizing && standPat <= alpha)
            return alpha;

        // Update bounds for search
        if (isMaximizing)
            alpha = Math.Max(alpha, standPat);
        else
            beta = Math.Min(beta, standPat);

        // Check for terminal states in quiescence
        var winner = CheckWinner(board);
        if (winner != null)
        {
            return winner == aiPlayer ? 100000 : -100000;
        }

        // Limit quiescence search depth to avoid explosion
        const int maxQuiescenceDepth = 4;
        if (quiesceDepth > maxQuiescenceDepth)
        {
            return standPat;
        }

        // Generate tactical moves (only near existing stones)
        var tacticalMoves = GetCandidateMoves(board);

        // If no tactical moves, return static evaluation
        if (tacticalMoves.Count == 0)
            return standPat;

        var currentPlayer = isMaximizing ? aiPlayer : (aiPlayer == Player.Red ? Player.Blue : Player.Red);

        // Order tactical moves for better pruning using staged move picker
        var orderedMoves = OrderMovesStaged(tacticalMoves, quiesceDepth, board, currentPlayer, null, threadData);

        // Search tactical moves (only empty cells)
        if (isMaximizing)
        {
            var maxEval = standPat;
            foreach (var (x, y) in orderedMoves)
            {
                // Skip occupied cells
                if (!board.GetCell(x, y).IsEmpty)
                    continue;

                var qBoard = board.PlaceStone(x, y, currentPlayer);

                // Recursive quiescence search
                var eval = Quiesce(qBoard, alpha, beta, false, aiPlayer, quiesceDepth + 1, threadData, cancellationToken);

                maxEval = Math.Max(maxEval, eval);
                alpha = Math.Max(alpha, eval);

                if (beta <= alpha)
                    return beta;
            }
            return maxEval;
        }
        else
        {
            var minEval = standPat;
            foreach (var (x, y) in orderedMoves)
            {
                // Skip occupied cells
                if (!board.GetCell(x, y).IsEmpty)
                    continue;

                var qBoard = board.PlaceStone(x, y, currentPlayer);

                var eval = Quiesce(qBoard, alpha, beta, true, aiPlayer, quiesceDepth + 1, threadData, cancellationToken);

                minEval = Math.Min(minEval, eval);
                beta = Math.Min(beta, eval);

                if (beta <= alpha)
                    return alpha;
            }
            return minEval;
        }
    }

    /// <summary>
    /// Evaluate board position using scalar evaluator for consistency
    /// SIMD evaluator has potential bugs that cause AI strength inversion
    /// </summary>
    private int Evaluate(Board board, Player player)
    {
        return BitBoardEvaluator.Evaluate(board, player);
    }

    /// <summary>
    /// Calculate search depth based on time allocation
    /// </summary>
    private int CalculateDepthForTime(int baseDepth, TimeAllocation timeAlloc, int candidateCount, long? timeRemainingMs = null)
    {
        // Emergency mode - reduce depth significantly
        if (timeAlloc.IsEmergency)
        {
            return Math.Max(1, baseDepth - 3);
        }

        // Adjust based on time available
        var softBoundSeconds = timeAlloc.SoftBoundMs / 1000.0;

        // Infer initial time for ratio calculation (default to 7 minutes = 420s for 7+5 time control)
        var initialTimeSeconds = timeRemainingMs.HasValue ? timeRemainingMs.Value / 1000.0 : 420.0;
        var softBoundRatio = softBoundSeconds / initialTimeSeconds;

        // Very tight time (< 1.5% of initial time or < 2s)
        if ((softBoundSeconds < 2 && softBoundRatio < 0.015) || (timeRemainingMs.HasValue && timeRemainingMs.Value < initialTimeSeconds * 1000 * 0.10))
        {
            return Math.Max(1, baseDepth - 2);
        }

        // Tight time (< 3% of initial time or < 4s)
        if ((softBoundSeconds < 4 && softBoundRatio < 0.03) || (timeRemainingMs.HasValue && timeRemainingMs.Value < initialTimeSeconds * 1000 * 0.15))
        {
            if (candidateCount > 30) // Very complex position with some time pressure
            {
                return Math.Max(2, baseDepth - 1);
            }
            return baseDepth;
        }

        // Good time availability: use full depth
        // For 7+5 time control (420s initial), 5s soft bound is only 1.2% - plenty of time
        return baseDepth;
    }

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
    /// </summary>
    private static TimeAllocation GetDefaultTimeAllocation(AIDifficulty difficulty, long? timeRemainingMs)
    {
        // If time remaining is provided but no TimeAllocation, create a simple one
        if (timeRemainingMs.HasValue)
        {
            var timeLeft = timeRemainingMs.Value;

            // FIX: Use difficulty-based time allocation for short time controls
            // Higher difficulties should use a larger percentage of remaining time
            long softBound;
            if (timeLeft < 60000) // Less than 60 seconds - short time control
            {
                // CRITICAL FIX: Scale time allocation by difficulty
                // Grandmaster uses up to 80% of remaining time, Braindead uses 20%
                // BookGeneration uses 100% to maximize search depth
                double timePercentage = difficulty switch
                {
                    AIDifficulty.BookGeneration => 1.00, // 100% - use full time for max depth
                    AIDifficulty.Braindead => 0.20,   // 20% - barely thinks
                    AIDifficulty.Easy => 0.30,        // 30%
                    AIDifficulty.Medium => 0.50,      // 50%
                    AIDifficulty.Hard => 0.65,        // 65%
                    AIDifficulty.Grandmaster => 0.80, // 80% - uses most of the time
                    _ => 0.50
                };
                softBound = Math.Max(500, (long)(timeLeft * timePercentage));
            }
            else
            {
                // CRITICAL FIX: For long time controls, use difficulty-based divisor
                // BookGeneration needs full time budget - use divisor of 2 instead of 40
                // This fixes the bug where 60s budget was divided by 40 = 1.5s actual search
                double divisor = (difficulty == AIDifficulty.BookGeneration) ? 2.0 : 40.0;
                softBound = Math.Max(500, (long)(timeLeft / divisor));
            }
            // CRITICAL FIX: For BookGeneration, use full timeLeft for hard bound
            // This ensures the full budget is utilized for deep searches
            long hardBound = (difficulty == AIDifficulty.BookGeneration)
                ? timeLeft - 100  // Use almost full remaining time for BookGeneration
                : Math.Min(softBound * 2, timeLeft - 500);  // Reduced to 2x from 3x

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

        // No time info - use difficulty defaults
        return difficulty switch
        {
            AIDifficulty.Braindead => new() { SoftBoundMs = 50, HardBoundMs = 200, OptimalTimeMs = 40, IsEmergency = false },
            AIDifficulty.Easy => new() { SoftBoundMs = 200, HardBoundMs = 1000, OptimalTimeMs = 160, IsEmergency = false },
            AIDifficulty.Medium => new() { SoftBoundMs = 1000, HardBoundMs = 3000, OptimalTimeMs = 800, IsEmergency = false },
            AIDifficulty.Hard => new() { SoftBoundMs = 3000, HardBoundMs = 10000, OptimalTimeMs = 2400, IsEmergency = false },
            AIDifficulty.Grandmaster => new() { SoftBoundMs = 5000, HardBoundMs = 20000, OptimalTimeMs = 4000, IsEmergency = false },
            AIDifficulty.Experimental => new() { SoftBoundMs = 5000, HardBoundMs = 20000, OptimalTimeMs = 4000, IsEmergency = false },
            AIDifficulty.BookGeneration => new() { SoftBoundMs = 15000, HardBoundMs = 30000, OptimalTimeMs = 5000, IsEmergency = false },
            _ => TimeAllocation.Default
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
    /// Find opponent's critical threat moves that MUST be blocked
    /// Priority order:
    /// 1. Five in row (immediate win)
    /// 2. Semi-open four (XXXX_ - one end blocked, must block now)
    /// 3. Open four (XXXX - both ends open)
    /// 4. Broken four (XXX_X - can create double threat)
    /// </summary>
    private List<(int x, int y)> GetOpponentThreatMoves(Board board, Player opponent)
    {
        var threats = new List<(int x, int y)>();

        // Priority 1: Check for immediate winning moves (5-in-row completion)
        // CRITICAL FIX: Use BitBoard.Size not hardcoded 15
        for (int x = 0; x < BitBoard.Size; x++)
        {
            for (int y = 0; y < BitBoard.Size; y++)
            {
                if (!board.GetCell(x, y).IsEmpty)
                    continue;

                var testBoard = board.PlaceStone(x, y, opponent);
                bool isWinningMove = _winDetector.CheckWin(testBoard).HasWinner;

                if (isWinningMove)
                {
                    threats.Add((x, y));
                    return threats; // Immediate win - highest priority, return immediately
                }
            }
        }

        // Priority 2: Check for semi-open four using ThreatDetector
        // Semi-open four = 4 consecutive stones with ONE open end (XXXX_)
        // This is CRITICAL - opponent can win on their next turn if not blocked
        var detector = new ThreatDetector();
        var opponentThreats = detector.DetectThreats(board, opponent);

        // Check for StraightFour (XXXX_ pattern) - semi-open four and open four
        // GainSquares contains the blocking squares
        // CRITICAL FIX: For open four (2 gain squares), both must be in candidates
        // For semi-open four (1 gain square), only one blocking move exists
        foreach (var threat in opponentThreats)
        {
            if (threat.Type == ThreatType.StraightFour)
            {
                // Add all gain squares (blocking moves) for this threat
                foreach (var gainSquare in threat.GainSquares)
                {
                    if (board.GetCell(gainSquare.x, gainSquare.y).IsEmpty && !threats.Contains(gainSquare))
                    {
                        threats.Add(gainSquare);
                    }
                }

                // CRITICAL FIX: For semi-open four (1 blocking move), return immediately
                // For open four (2+ blocking moves), continue checking for other threats
                // But if we have any threats, filter candidates to only blocking moves
                if (threat.GainSquares.Count == 1 && threats.Count > 0)
                {
                    return threats; // Semi-open four - only one way to block
                }
            }
        }

        // Priority 3: BrokenFour (XXX_X pattern) - can create double threats
        foreach (var threat in opponentThreats)
        {
            if (threat.Type == ThreatType.BrokenFour)
            {
                // Add gain squares that would complete the broken four
                foreach (var gainSquare in threat.GainSquares)
                {
                    if (board.GetCell(gainSquare.x, gainSquare.y).IsEmpty && !threats.Contains(gainSquare))
                    {
                        threats.Add(gainSquare);
                    }
                }
            }
        }

        return threats;
    }

    #region Pondering Support

    /// <summary>
    /// Pondering variant of Lazy SMP - uses same thread count as main search
    /// Searches with predicted opponent move already made on the board
    /// Results are stored in the shared transposition table for main search benefit
    /// </summary>
    /// <param name="board">Board with predicted opponent move already made</param>
    /// <param name="player">Player to move (us, after opponent's predicted move)</param>
    /// <param name="difficulty">AI difficulty level</param>
    /// <param name="maxPonderTimeMs">Maximum time to spend pondering</param>
    /// <param name="cancellationToken">Token to cancel pondering</param>
    /// <param name="progressCallback">Optional callback for progress updates</param>
    /// <param name="ponderingFor">Player doing the pondering (for debug logging)</param>
    /// <returns>Best move found, depth reached, score, and nodes searched</returns>
    public ((int x, int y)? bestMove, int depth, int score, long nodesSearched) PonderLazySMP(
        Board board,
        Player player,
        AIDifficulty difficulty,
        long maxPonderTimeMs,
        CancellationToken cancellationToken,
        Action<(int x, int y, int depth, int score)>? progressCallback = null,
        Player ponderingFor = Player.None)
    {
        if (player == Player.None)
            return (null, 0, 0, 0);

        var candidates = GetCandidateMoves(board);

        if (candidates.Count == 0)
            return (null, 0, 0, 0);

        // CRITICAL FIX: Use time-budget calculation for target depth, not fixed AdaptiveDepthCalculator
        // This allows pondering to reach deeper depths when there's time and machine is fast
        var ponderTimeAlloc = new TimeAllocation
        {
            SoftBoundMs = maxPonderTimeMs * 3 / 4,  // 75% for soft bound
            HardBoundMs = maxPonderTimeMs,
            OptimalTimeMs = maxPonderTimeMs / 2,
            IsEmergency = false,
            Phase = GamePhase.EarlyMid,
            ComplexityMultiplier = 1.0
        };

        // Calibrate NPS for difficulty
        _depthManager.CalibrateNpsForDifficulty(difficulty);

        // Use same thread count as main search for this difficulty
        // This ensures pondering uses the same resources as thinking
        int ponderThreadCount = ThreadPoolConfig.GetThreadCountForDifficulty(difficulty);

        // No depth-based thread capping - use the configured thread count directly

        _transpositionTable.IncrementAge();

        // Set up time management for pondering
        // Use the provided CancellationToken combined with our own for time-based cancellation
        _searchCts?.Cancel(); // Cancel any previous search
        _searchCts = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        _searchStopwatch = Stopwatch.StartNew();
        _hardTimeBoundMs = maxPonderTimeMs;

        // Include threadIndex to distinguish master thread (0) from helper threads (1+)
        var results = new ConcurrentBag<(int x, int y, int score, int depth, long nodes, int threadIndex)>();

        // Create thread-local copies of board and candidates for each thread
        var boardsArray = new Board[ponderThreadCount];
        var candidatesArray = new List<(int x, int y)>[ponderThreadCount];
        for (int i = 0; i < ponderThreadCount; i++)
        {
            boardsArray[i] = board;
            candidatesArray[i] = new List<(int x, int y)>(candidates);
        }

        // Launch parallel pondering searches
        var linkedToken = _searchCts.Token;
        var tasks = new List<Task>();
        for (int i = 0; i < ponderThreadCount; i++)
        {
            int threadId = i;
            var task = Task.Run(() =>
            {
                var threadData = new ThreadData
                {
                    ThreadIndex = threadId,
                    Random = new Random(threadId + (int)DateTime.UtcNow.Ticks)
                };

                var result = SearchPonderIteration(
                    boardsArray[threadId],
                    player,
                    candidatesArray[threadId],
                    threadData,
                    ponderTimeAlloc,
                    linkedToken,
                    progressCallback,
                    ponderingFor);

                // Add threadIndex to identify master vs helper thread results
                var (x, y, score, depthAchieved, nodes) = result;
                results.Add((x, y, score, depthAchieved, nodes, threadId));
            }, linkedToken);
            tasks.Add(task);
        }

        // Wait for all threads or cancellation
        try
        {
            Task.WaitAll(tasks.ToArray());
        }
        catch (AggregateException)
        {
            // Cancellation occurred - this is expected during pondering
        }

        _searchStopwatch.Stop();
        _searchCts?.Cancel();

        // INTELLIGENT MERGING for pondering: Aggregate ALL thread results
        // Pondering searched predicted opponent moves - those results are valuable
        // Don't discard helper thread work - merge intelligently
        //
        // Same priority as main search: Depth > Score > Reliability(master > helper)

        if (!results.Any())
            return (null, 0, 0, 0);

        var maxDepth = results.Max(r => r.depth);
        if (maxDepth <= 0)
            return (null, 0, 0, 0);

        // At max depth, pick highest score with master as tiebreaker
        // CRITICAL FIX: Use single OrderBy with compound key to avoid replacing previous sort
        var bestResult = results
            .Where(r => r.depth == maxDepth)
            .OrderBy(r => (-r.score, r.threadIndex == 0 ? 0 : 1))  // Compound: (-score for desc, master priority)
            .FirstOrDefault();

        // Track maximum depth achieved across all threads (not just the winning move's depth)
        // This gives a better picture of how deeply the AI thought during pondering
        int overallMaxDepth = results.Any() ? results.Max(r => r.depth) : bestResult.depth;

        // CRITICAL FIX: Aggregate local node counts from all threads (no Interlocked contention)
        long totalNodes = results.Sum(r => r.nodes);
        return ((bestResult.x, bestResult.y), overallMaxDepth, bestResult.score, totalNodes);
    }

    /// <summary>
    /// Iterative deepening search for pondering thread with cancellation support
    /// PURE TIME-BASED: No depth caps - search continues until time runs out.
    /// Different machines will naturally reach different depths based on their performance.
    /// </summary>
    private (int x, int y, int score, int depth, long nodes) SearchPonderIteration(
        Board board,
        Player player,
        List<(int x, int y)> candidates,
        ThreadData threadData,
        TimeAllocation timeAlloc,
        CancellationToken cancellationToken,
        Action<(int x, int y, int depth, int score)>? progressCallback,
        Player ponderingFor = Player.None)
    {
        var bestMove = candidates[0];
        var bestScore = int.MinValue;
        int bestDepth = 1;
        long lastIterationElapsedMs = 0;  // Track time for last completed iteration
        int iterationCount = 0;  // DIAGNOSTIC: Track how many iterations actually ran

        // PURE TIME-BASED SEARCH
        // Search continues until time runs out
        int currentDepth = 2;
        while (true)  // Time-based only - depth is incidental
        {
            // Check cancellation before starting this depth
            if (cancellationToken.IsCancellationRequested)
                break;

            var elapsed = _searchStopwatch?.ElapsedMilliseconds ?? 0;

            // Hard bound check - stop when time is up
            if (elapsed >= _hardTimeBoundMs)
            {
                _searchCts?.Cancel();
                break;
            }

            // Time-based soft bound: stop if soft bound reached and last iteration was slow
            double remainingTime = _hardTimeBoundMs - elapsed;
            if (elapsed >= timeAlloc.SoftBoundMs && lastIterationElapsedMs > remainingTime * 0.25)
                break;

            var iterationStartTime = _searchStopwatch?.ElapsedMilliseconds ?? 0;
            iterationCount++;

            // Aspiration Windows - narrow window based on previous score
            int alpha = int.MinValue + 1000;
            int beta = int.MaxValue - 1000;
            if (bestScore > int.MinValue + 2000 && bestScore < int.MaxValue - 2000)
            {
                alpha = Math.Max(int.MinValue + 1000, bestScore - 50);
                beta = Math.Min(int.MaxValue - 1000, bestScore + 50);
            }

            var result = SearchRoot(board, player, currentDepth, candidates, threadData, alpha, beta, cancellationToken);

            // Track iteration time for smart continuation decisions
            lastIterationElapsedMs = (_searchStopwatch?.ElapsedMilliseconds ?? 0) - iterationStartTime;

            // If search was cancelled during this iteration, result is unreliable
            if (cancellationToken.IsCancellationRequested)
                break;

            if (result.score > bestScore || bestMove == (-1, -1))
            {
                bestScore = result.score;
                bestMove = (result.x, result.y);
            }

            // Update bestDepth to the depth we just completed
            bestDepth = currentDepth;

            // Report progress
            progressCallback?.Invoke((bestMove.x, bestMove.y, bestDepth, bestScore));

            // Early exit on winning move
            if (result.score >= 100000)
                break;

            currentDepth++;  // Increment depth for next iteration
        }

        // Use local node count (no Interlocked contention)
        long actualNodes = threadData.LocalNodesSearched;

        // Report the actual depth we achieved (not artificially inflated)
        // If actualNodes is 0 or 1 but we have a bestDepth, report bestDepth
        int reportedDepth = (actualNodes <= 1 && bestDepth < 2) ? 1 : bestDepth;

        return (bestMove.x, bestMove.y, bestScore, reportedDepth, actualNodes);
    }

    /// <summary>
    /// Estimate node count for a search at given depth
    /// </summary>
    private long CountNodes(int depth, int branchingFactor)
    {
        // Rough estimation: branching_factor ^ depth
        // This is approximate but sufficient for progress reporting
        long nodes = 1;
        for (int i = 0; i < depth && i < 6; i++) // Cap at depth 6 for estimation
        {
            nodes *= Math.Min(branchingFactor, 30); // Cap branching at 30
        }
        return nodes;
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
    public bool IsSearching => _searchStopwatch != null && _searchStopwatch.IsRunning;

    #endregion

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
            ThreadIndex = 0
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
        return search.GetCandidateMoves(board);
    }

    /// <summary>
    /// Get the shared continuation history for testing.
    /// </summary>
    internal ContinuationHistory GetContinuationHistory() => _continuationHistory;
    internal CounterMoveHistory GetCounterMoveHistory() => _counterMoveHistory;

    #endregion
}
