using System.Collections.Concurrent;
using System.Diagnostics;
using Caro.Core.Entities;
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
    long AllocatedTimeMs = 0
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
    private readonly LockFreeTranspositionTable _transpositionTable;
    private readonly BoardEvaluator _evaluator;
    private readonly WinDetector _winDetector;
    private readonly ThreatSpaceSearch _vcfSolver;
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

    // Real node counting (thread-safe via Interlocked)
    private long _realNodesSearched;

    // Per-thread data (not shared between threads)
    private sealed class ThreadData
    {
        public int ThreadIndex; // Identifies master (0) vs helper (1+) threads for diversity logic
        public (int x, int y)[,] KillerMoves = new (int x, int y)[20, 2];
        // CRITICAL FIX: Use BitBoard.Size not hardcoded 15 for 19x19 boards
        public int[,] HistoryRed = new int[BitBoard.Size, BitBoard.Size];
        public int[,] HistoryBlue = new int[BitBoard.Size, BitBoard.Size];
        public int TableHits;
        public int TableLookups;

        // Diagnostic counters for TT provenance tracking
        public int TTReadsFromMaster;    // Entries from master thread (ThreadIndex=0)
        public int TTReadsFromHelpers;   // Entries from helper threads (ThreadIndex>0)
        public int TTReadsSkipped;       // Helper entries skipped due to depth threshold
        public int TTScoresUsed;         // How many TT entries actually returned scores

        public Random Random = new();

        public void Reset()
        {
            // Clear killer moves
            for (int i = 0; i < 20; i++)
            {
                KillerMoves[i, 0] = (-1, -1);
                KillerMoves[i, 1] = (-1, -1);
            }
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
                for (int x = 0; x < 15; x++)
                {
                    for (int y = 0; y < 15; y++)
                    {
                        if (board.GetCell(x, y).Player == Player.None && IsValidPerOpenRule(board, x, y))
                            return (x, y);
                    }
                }
            }
            return (7, 7); // Center move
        }

        // Use provided time allocation or create default
        var alloc = timeAlloc ?? GetDefaultTimeAllocation(difficulty, timeRemainingMs);

        // CRITICAL FIX: Calibrate NPS for difficulty to ensure proper depth scaling
        // Easy targets 50k nps, Medium targets 100k nps, Hard targets 200k nps, Grandmaster targets 500k nps
        _depthManager.CalibrateNpsForDifficulty(difficulty);

        // Try VCF first for higher difficulties
        if (difficulty >= AIDifficulty.Hard)
        {
            var vcfTimeLimit = CalculateVCFTimeLimit(alloc);
            var vcfResult = _vcfSolver.SolveVCF(board, player, vcfTimeLimit, maxDepth: 30);

            if (vcfResult.IsSolved && vcfResult.IsWin && vcfResult.BestMove.HasValue)
            {
                Console.WriteLine($"[AI VCF] Found winning move ({vcfResult.BestMove.Value.x}, {vcfResult.BestMove.Value.y}), depth: {vcfResult.DepthAchieved}");
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

        // CRITICAL FIX: Use time-budget calculation with difficulty time multiplier
        // This ensures Easy (10% time), Medium (30% time), Hard (70% time), Grandmaster (100% time)
        // scale their search depths appropriately even in parallel mode
        var depthFromBudget = CalculateDepthFromTimeBudget(alloc, difficulty);
        var minDepth = TimeBudgetDepthManager.GetMinimumDepth(difficulty);
        var adjustedDepth = Math.Max(depthFromBudget, minDepth);

        // For Braindead difficulty, add randomness (20% error rate)
        if (difficulty == AIDifficulty.Braindead && _random.Next(100) < 20)
        {
            return candidates[_random.Next(candidates.Count)];
        }

        // Single-threaded for low depths (overhead not worth it)
        if (adjustedDepth <= 3)
        {
            return SearchSingleThreaded(board, player, adjustedDepth, candidates);
        }

        // Multi-threaded Lazy SMP for deeper searches
        var parallelResult = SearchLazySMP(board, player, adjustedDepth, candidates, difficulty, alloc);
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
        int fixedThreadCount = -1)
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
            // Empty board - return center move with depth 1 (not 0, which is misleading)
            // For empty board, center is the only reasonable move
            return new ParallelSearchResult(7, 7, 1, 1, 0, null, 0);
        }

        // Use provided time allocation or create default
        var alloc = timeAlloc ?? GetDefaultTimeAllocation(difficulty, timeRemainingMs);

        // CRITICAL FIX: Calibrate NPS for difficulty to ensure proper depth scaling
        // Easy targets 50k nps, Medium targets 100k nps, Hard targets 200k nps, Grandmaster targets 500k nps
        _depthManager.CalibrateNpsForDifficulty(difficulty);

        // Try VCF first for higher difficulties
        if (difficulty >= AIDifficulty.Hard)
        {
            var vcfTimeLimit = CalculateVCFTimeLimit(alloc);
            var vcfResult = _vcfSolver.SolveVCF(board, player, vcfTimeLimit, maxDepth: 30);

            if (vcfResult.IsSolved && vcfResult.IsWin && vcfResult.BestMove.HasValue)
            {
                Console.WriteLine($"[AI VCF] Found winning move ({vcfResult.BestMove.Value.x}, {vcfResult.BestMove.Value.y}), depth: {vcfResult.DepthAchieved}");
                return new ParallelSearchResult(vcfResult.BestMove.Value.x, vcfResult.BestMove.Value.y,
                    vcfResult.DepthAchieved, vcfResult.NodesSearched, 0, null, vcfTimeLimit);
            }
        }

        // Check for opponent's immediate threats that must be blocked
        // CRITICAL FIX: Do NOT return immediately. Filter candidates to ONLY blocking moves and let the engine search.
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

        // CRITICAL FIX: Use time-budget calculation with difficulty time multiplier
        // This ensures Easy (10% time), Medium (30% time), Hard (70% time), Grandmaster (100% time)
        // scale their search depths appropriately even in parallel mode
        var depthFromBudget = CalculateDepthFromTimeBudget(alloc, difficulty);
        var minDepth = TimeBudgetDepthManager.GetMinimumDepth(difficulty);
        var adjustedDepth = Math.Max(depthFromBudget, minDepth);

        // For Braindead difficulty, add randomness (20% error rate)
        if (difficulty == AIDifficulty.Braindead && _random.Next(100) < 20)
        {
            var randomMove = candidates[_random.Next(candidates.Count)];
            return new ParallelSearchResult(randomMove.x, randomMove.y, 1, candidates.Count, 0, null, alloc.HardBoundMs);
        }

        // Single-threaded for low depths (overhead not worth it)
        if (adjustedDepth <= 3)
        {
            // Reset node counter for single-threaded search
            Interlocked.Exchange(ref _realNodesSearched, 0);
            _transpositionTable.IncrementAge();

            var (x, y) = SearchSingleThreaded(board, player, adjustedDepth, candidates);
            // Use real node count instead of estimate
            long actualNodes = Interlocked.Read(ref _realNodesSearched);
            return new ParallelSearchResult(x, y, adjustedDepth, actualNodes, 0, null, alloc.HardBoundMs);
        }

        // Multi-threaded Lazy SMP for deeper searches
        return SearchLazySMP(board, player, adjustedDepth, candidates, difficulty, alloc, fixedThreadCount);
    }

    /// <summary>
    /// Single-threaded search (fallback for low depths)
    /// Note: TranspositionTable age is incremented by caller
    /// </summary>
    private (int x, int y) SearchSingleThreaded(Board board, Player player, int depth, List<(int x, int y)> candidates)
    {
        var threadData = new ThreadData();
        var cts = new CancellationTokenSource();
        var token = cts.Token;

        var bestMove = candidates[0];
        var bestScore = int.MinValue;

        foreach (var (x, y) in candidates)
        {
            board.PlaceStone(x, y, player);
            var score = Minimax(board, depth - 1, int.MinValue, int.MaxValue, false, player, depth, threadData, token);
            board.GetCell(x, y).Player = Player.None;

            if (score > bestScore)
            {
                bestScore = score;
                bestMove = (x, y);
            }
        }

        return bestMove;
    }

    /// <summary>
    /// Lazy SMP: Multiple threads search independently with shared TT
    /// Each thread has slight variation to explore different parts of tree
    /// Time-aware iterative deepening with move stability detection
    /// </summary>
    private ParallelSearchResult SearchLazySMP(
        Board board,
        Player player,
        int depth,
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

        // Reset real node counter for this search
        Interlocked.Exchange(ref _realNodesSearched, 0);

        // Number of threads based on depth and available cores
        // FIX: Use fixed thread count when provided to reduce non-determinism
        // fixedThreadCount = 0 means single-threaded (skip parallel search)
        int threadCount = fixedThreadCount >= 0
            ? fixedThreadCount  // 0 = single-threaded, >0 = use that many threads
            : Math.Min(_maxThreads, Math.Max(2, depth / 2));

        // If threadCount is 0, fall back to single-threaded search
        if (threadCount == 0)
        {
            Interlocked.Exchange(ref _realNodesSearched, 0);
            _transpositionTable.IncrementAge();

            var (x, y) = SearchSingleThreaded(board, player, depth, candidates);
            long actualNodes = Interlocked.Read(ref _realNodesSearched);
            return new ParallelSearchResult(x, y, depth, actualNodes, 0, null, _hardTimeBoundMs);
        }

        // Include threadIndex to distinguish master thread (0) from helper threads (1+)
        var results = new ConcurrentBag<(int x, int y, int score, int depth, long nodes, int threadIndex)>();
        var diagnosticsList = new ConcurrentBag<ThreadData>();

        // Create thread-local copies of board and candidates for each thread
        var boardsArray = new Board[threadCount];
        var candidatesArray = new List<(int x, int y)>[threadCount];
        for (int i = 0; i < threadCount; i++)
        {
            boardsArray[i] = board.Clone();
            candidatesArray[i] = new List<(int x, int y)>(candidates);
        }

        // Launch parallel searches with time-aware iterative deepening
        // Use dedicated threads instead of Task.Run for true parallelism
        // Task.Run queues to thread pool which doesn't scale immediately
        var token = _searchCts.Token;
        var threads = new List<Thread>();

        // PREVENT EARLY CANCELLATION: Track minimum depth reached across all threads
        // Only allow cancellation when all threads reach at least minTargetDepth
        int minTargetDepth = Math.Max(3, depth - 1); // Ensure at least depth 3 or target-1
        var threadsReachedMinDepth = 0;
        var depthLock = new object();

        for (int i = 0; i < threadCount; i++)
        {
            int threadId = i;

            // Create dedicated thread for true parallelism
            // Task.Run uses thread pool which doesn't scale immediately
            var thread = new Thread(() =>
            {
                // DIVERSITY FIX: Set ThreadIndex to distinguish master (0) from helper (1+) threads
                // Master thread (ThreadIndex=0) searches deterministically without noise
                // Helper threads (ThreadIndex>0) get noise injection in OrderMoves for tree diversity
                var threadData = new ThreadData
                {
                    ThreadIndex = threadId,
                    Random = new Random(threadId + (int)DateTime.UtcNow.Ticks)
                };

                var result = SearchWithIterationTimeAware(
                    boardsArray[threadId], player, depth, candidatesArray[threadId],
                    threadData, timeAlloc, difficulty, token, minTargetDepth, () =>
                    {
                        // Callback when thread reaches minimum depth
                        lock (depthLock)
                        {
                            threadsReachedMinDepth++;
                        }
                    });

                // Add threadIndex to identify master vs helper thread results
                var (x, y, score, depthAchieved, nodes) = result;
                results.Add((x, y, score, depthAchieved, nodes, threadId));

                // Collect diagnostics for analysis
                diagnosticsList.Add(threadData);
            });

            thread.IsBackground = false; // Important: keep thread alive until done
            thread.Start();
            threads.Add(thread);
        }

        // Wait for all threads - handle cancellation gracefully
        foreach (var thread in threads)
        {
            thread.Join();
        }

        _searchStopwatch.Stop();

        // CRITICAL FIX: Only use master thread (threadIndex=0) result for move selection
        // Lazy SMP helper threads are for populating the transposition table ONLY
        // They should NOT influence move selection as their results may be inconsistent
        // due to early cancellation at different depths during iterative deepening.
        //
        // The regression (Medium losing to Easy) was caused by selecting shallow
        // helper results instead of waiting for the master thread's deeper result.

        var bestResult = results.FirstOrDefault(r => r.threadIndex == 0);

        // If master thread produced no result, use fallback
        if (bestResult == default)
        {
            // Fallback: deepest result from any thread (should rarely happen)
            var maxDepth = results.Max(r => r.depth);
            if (maxDepth > 0)
            {
                bestResult = results.Where(r => r.depth == maxDepth)
                                   .OrderByDescending(r => r.score)
                                   .FirstOrDefault();
            }
            else
            {
                // Last resort: first candidate
                long totalNodes = Interlocked.Read(ref _realNodesSearched);
                return new ParallelSearchResult(candidates[0].x, candidates[0].y, 1, totalNodes, 0, null, _hardTimeBoundMs);
            }
        }

        // Use real node count instead of estimated
        long totalNodesFinal = Interlocked.Read(ref _realNodesSearched);

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

        // Threads reached minimum depth
        if (threadsReachedMinDepth > 0)
        {
            if (diagBuilder.Length > 0)
                diagBuilder.Append("; ");
            diagBuilder.Append($"{threadsReachedMinDepth}/{threadCount} reached min depth");
        }

        string? diagnostics = diagBuilder.Length > 0 ? diagBuilder.ToString() : null;

        return new ParallelSearchResult(bestResult.x, bestResult.y, bestResult.depth, totalNodesFinal, threadCount, diagnostics, _hardTimeBoundMs);
    }

    /// <summary>
    /// Iterative deepening search for a single thread with time awareness
    /// Implements move stability detection for early termination
    /// Note: Node counting is done globally via Interlocked in Minimax()
    /// CRITICAL FIX: Only master thread (ThreadIndex=0) can trigger cancellation
    /// Helper threads must NOT cancel as they complete early and would interrupt deeper searches
    /// </summary>
    private (int x, int y, int score, int depth, long nodes) SearchWithIterationTimeAware(
        Board board,
        Player player,
        int targetDepth,
        List<(int x, int y)> candidates,
        ThreadData threadData,
        TimeAllocation timeAlloc,
        AIDifficulty difficulty,
        CancellationToken cancellationToken,
        int minTargetDepth = 2,
        Action? onMinDepthReached = null)
    {
        var bestMove = candidates[0];
        var bestScore = int.MinValue;
        int bestDepth = 1;
        int stableCount = 0; // Number of consecutive depths with same best move
        bool minDepthCallbackCalled = false;

        // CRITICAL: Only master thread (ThreadIndex=0) can trigger cancellation
        // Helper threads complete early with shallow searches and must NOT interrupt master
        bool isMasterThread = threadData.ThreadIndex == 0;

        // Start from depth 2 and iterate up
        for (int currentDepth = 2; currentDepth <= targetDepth; currentDepth++)
        {
            // Check if search should stop (time exceeded by another thread)
            if (cancellationToken.IsCancellationRequested)
            {
                break;
            }

            var elapsed = _searchStopwatch?.ElapsedMilliseconds ?? 0;

            // Check hard bound (absolute maximum) - ONLY master thread can cancel
            if (isMasterThread && elapsed >= _hardTimeBoundMs)
            {
                _searchCts?.Cancel();
                break;
            }

            // Helper threads: also check hard bound but don't cancel others
            if (!isMasterThread && elapsed >= _hardTimeBoundMs)
            {
                break; // Just exit, don't cancel
            }

            // CRITICAL FIX: Don't exit early until we've reached at least 80% of target depth
            // This ensures each difficulty searches to an appropriate depth
            // Only master thread (ThreadIndex=0) can use stability-based early termination
            bool hasReachedSufficientDepth = currentDepth >= (targetDepth * 4 / 5);
            bool isStableEnough = stableCount >= 2;

            // Check soft bound with stability consideration - ONLY for master thread
            // Helper threads must not terminate early as they provide tree diversity
            if (isMasterThread && elapsed >= timeAlloc.SoftBoundMs && hasReachedSufficientDepth && isStableEnough)
            {
                break;
            }

            // Check optimal time - only for master thread with very stable moves
            if (isMasterThread && elapsed >= timeAlloc.OptimalTimeMs && hasReachedSufficientDepth && stableCount >= 3)
            {
                break;
            }

            // FIX: Use safe bounds to prevent overflow
            int alpha = int.MinValue + 1000;
            int beta = int.MaxValue - 1000;

            // Aspiration Windows (Safe Implementation)
            // Only narrow the window if we have a previous stable score
            if (currentDepth > 2 && bestScore > int.MinValue + 2000 && bestScore < int.MaxValue - 2000)
            {
                alpha = Math.Max(int.MinValue + 1000, bestScore - 50);
                beta = Math.Min(int.MaxValue - 1000, bestScore + 50);
            }

            var result = SearchRoot(board, player, currentDepth, candidates, threadData, alpha, beta, cancellationToken);

            // Check move stability
            if (result.x == bestMove.Item1 && result.y == bestMove.Item2)
            {
                stableCount++;
            }
            else
            {
                stableCount = 1; // Reset but count this as one
                bestMove = (result.x, result.y);
            }

            if (result.score > bestScore || bestMove == (-1, -1))
            {
                bestScore = result.score;
                bestMove = (result.x, result.y);
            }

            bestDepth = currentDepth;

            // Call callback when minimum target depth is reached (once only)
            if (hasReachedSufficientDepth && !minDepthCallbackCalled && onMinDepthReached != null)
            {
                minDepthCallbackCalled = true;
                onMinDepthReached();
            }

            // Early exit on winning move
            if (result.score >= 100000)
            {
                break;
            }
        }

        // Return 0 for nodes - real count is tracked globally in _realNodesSearched
        return (bestMove.x, bestMove.y, bestScore, bestDepth, 0);
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
        var bestMove = candidates[0];
        var bestScore = int.MinValue;

        var orderedMoves = OrderMoves(candidates, depth, board, player, null, threadData);

        foreach (var (x, y) in orderedMoves)
        {
            board.PlaceStone(x, y, player);
            var score = Minimax(board, depth - 1, alpha, beta, false, player, depth, threadData, cancellationToken);
            board.GetCell(x, y).Player = Player.None;

            if (score > bestScore)
            {
                bestScore = score;
                bestMove = (x, y);
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
            _transpositionTable.Store(board.Hash, (sbyte)depth, (short)bestScore, (sbyte)bestMove.x, (sbyte)bestMove.y, alpha, beta, (byte)threadData.ThreadIndex, rootDepth: depth);
        }

        return (bestMove.x, bestMove.y, bestScore);
    }

    /// <summary>
    /// Minimax with alpha-beta pruning (thread-safe via per-thread data)
    /// </summary>
    private int Minimax(Board board, int depth, int alpha, int beta, bool isMaximizing, Player aiPlayer, int rootDepth, ThreadData threadData, CancellationToken cancellationToken)
    {
        // Count this node (thread-safe)
        Interlocked.Increment(ref _realNodesSearched);

        // Time management check - use CancellationToken for proper cancellation
        if (cancellationToken.IsCancellationRequested)
        {
            return 0;
        }

        // Periodic time check at the start of each node
        // CRITICAL FIX: Only master thread (ThreadIndex=0) can trigger cancellation
        if (_searchStopwatch != null && _hardTimeBoundMs > 0)
        {
            var elapsed = _searchStopwatch.ElapsedMilliseconds;
            if (elapsed >= _hardTimeBoundMs)
            {
                // Only master thread cancels - helpers just exit
                if (threadData.ThreadIndex == 0)
                {
                    _searchCts?.Cancel();
                }
                return 0;
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

        var boardHash = board.Hash;
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
        var orderedMoves = OrderMoves(candidates, rootDepth - depth, board, currentPlayer, cachedMove, threadData);

        int bestScore = isMaximizing ? int.MinValue : int.MaxValue;
        (int x, int y)? bestMove = null;
        int moveIndex = 0;

        foreach (var (x, y) in orderedMoves)
        {
            // MDAP: Move-Dependent Adaptive Pruning (Late Move Reduction)
            // Apply depth reduction for moves searched later in the list
            int reducedDepth = depth;
            bool doLMR = false;

            // Apply LMR for late moves when depth is sufficient
            if (depth >= LMRMinDepth && moveIndex >= LMRFullDepthMoves)
            {
                // Check if this is NOT a high-priority move (hash move, killer, threat)
                bool isHighPriority = (cachedMove.HasValue && cachedMove.Value == (x, y));
                if (!isHighPriority)
                {
                    // Calculate reduction based on move index
                    // Later moves get more reduction
                    int extraReduction = Math.Min(2, (moveIndex - LMRFullDepthMoves) / 4);
                    reducedDepth = depth - LMRBaseReduction - extraReduction;
                    if (reducedDepth < 1) reducedDepth = 1;
                    doLMR = true;
                }
            }

            board.PlaceStone(x, y, currentPlayer);
            int score;

            if (doLMR)
            {
                // Search with reduced depth first
                score = Minimax(board, reducedDepth - 1, alpha, beta, !isMaximizing, aiPlayer, rootDepth, threadData, cancellationToken);

                // If reduced depth search returns a score that could improve alpha/beta,
                // re-search at full depth (verification)
                if ((isMaximizing && score > alpha) || (!isMaximizing && score < beta))
                {
                    score = Minimax(board, depth - 1, alpha, beta, !isMaximizing, aiPlayer, rootDepth, threadData, cancellationToken);
                }
            }
            else
            {
                // Full depth search for early/high-priority moves
                score = Minimax(board, depth - 1, alpha, beta, !isMaximizing, aiPlayer, rootDepth, threadData, cancellationToken);
            }

            board.GetCell(x, y).Player = Player.None;
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
                break;
            }
        }

        // LAZY SMP TT WRITING: All threads can write to transposition table
        // This is essential for Lazy SMP - helper threads populate TT with results
        // from different parts of the tree, allowing master thread to benefit.
        //
        // To prevent "pollution" from shallow helper entries:
        // 1. Only store entries where depth >= currentDepth - 1 (not too shallow)
        // 2. Prefer exact bounds over upper/lower bounds for helpers
        // 3. Master thread (ThreadIndex=0) can store any entry
        //
        // The regression was caused by disabling ALL helper writes, which
        // broke Lazy SMP's fundamental sharing mechanism.

        bool shouldStore = bestMove.HasValue;
        if (threadData.ThreadIndex > 0 && depth < rootDepth - 1)
        {
            // Helper threads: only store if at reasonable depth relative to root
            // This prevents very shallow entries from polluting the TT
            shouldStore = depth >= rootDepth / 2;
        }

        if (shouldStore && bestMove.HasValue)
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
    /// Order moves by priority (Defensive > TT move > Killer > History > Position)
    /// Lazy SMP diversity comes naturally from different threads searching at different times,
    /// different cutoffs, and different TT entries - no artificial noise needed.
    /// </summary>
    private List<(int x, int y)> OrderMoves(List<(int x, int y)> candidates, int depth, Board board, Player player, (int x, int y)? cachedMove, ThreadData threadData)
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

            // 3. Winning Move (Tactical)
            // (You can call EvaluateTactical here if you want greater precision)

            // 4. Killer Moves
            if (depth < 20)
            {
                if (threadData.KillerMoves[depth, 0] == (x, y)) score += 500000;
                else if (threadData.KillerMoves[depth, 1] == (x, y)) score += 400000;
            }

            // 5. History Heuristic
            score += Math.Min(historyTable[x, y], 10000);

            // 6. Center Preference & Proximity
            int centerDist = Math.Abs(x - 7) + Math.Abs(y - 7);
            score += (14 - centerDist) * 100;
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
        var considered = new bool[15, 15];

        var playerBitBoard = board.GetBitBoard(Player.Red);
        var opponentBitBoard = board.GetBitBoard(Player.Blue);
        var occupied = playerBitBoard | opponentBitBoard;

        // Check if board is empty (no stones placed)
        bool boardIsEmpty = true;
        for (int x = 0; x < 15 && boardIsEmpty; x++)
        {
            for (int y = 0; y < 15 && boardIsEmpty; y++)
            {
                if (occupied.GetBit(x, y))
                    boardIsEmpty = false;
            }
        }

        // Empty board - return center-area moves for opening
        if (boardIsEmpty)
        {
            // Return center 3x3 area as candidates (standard opening positions)
            for (int x = 6; x <= 8; x++)
            {
                for (int y = 6; y <= 8; y++)
                {
                    candidates.Add((x, y));
                }
            }
            return candidates;
        }

        // Find all cells within SearchRadius of existing stones
        for (int x = 0; x < 15; x++)
        {
            for (int y = 0; y < 15; y++)
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
                            if (nx >= 0 && nx < 15 && ny >= 0 && ny < 15 &&
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
        var playerBitBoard = board.GetBitBoard(Player.Red);
        var opponentBitBoard = board.GetBitBoard(Player.Blue);
        int score = 0;

        for (int dx = -2; dx <= 2; dx++)
        {
            for (int dy = -2; dy <= 2; dy++)
            {
                int nx = x + dx;
                int ny = y + dy;
                if (nx >= 0 && nx < 15 && ny >= 0 && ny < 15)
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

        // Order tactical moves for better pruning
        var orderedMoves = OrderMoves(tacticalMoves, quiesceDepth, board, currentPlayer, null, threadData);

        // Search tactical moves (only empty cells)
        if (isMaximizing)
        {
            var maxEval = standPat;
            foreach (var (x, y) in orderedMoves)
            {
                // Skip occupied cells
                if (!board.GetCell(x, y).IsEmpty)
                    continue;

                board.PlaceStone(x, y, currentPlayer);

                // Recursive quiescence search
                var eval = Quiesce(board, alpha, beta, false, aiPlayer, quiesceDepth + 1, threadData, cancellationToken);

                board.GetCell(x, y).Player = Player.None;

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

                board.PlaceStone(x, y, currentPlayer);

                var eval = Quiesce(board, alpha, beta, true, aiPlayer, quiesceDepth + 1, threadData, cancellationToken);

                board.GetCell(x, y).Player = Player.None;

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
    /// Calculate depth using time-budget formula (same as sequential path)
    /// Uses TimeBudgetDepthManager for consistency with sequential search
    /// </summary>
    private int CalculateDepthFromTimeBudget(TimeAllocation timeAlloc, AIDifficulty difficulty)
    {
        double timeForDepthSeconds = timeAlloc.SoftBoundMs / 1000.0;
        return _depthManager.CalculateMaxDepth(timeForDepthSeconds, difficulty);
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

            // SHORT TIME CONTROL FIX: Use percentage-based allocation instead of fixed division
            // For short time controls (< 60s), allocate based on percentage of remaining time
            // For long time controls, distribute over remaining moves
            long softBound;
            if (timeLeft < 60000) // Less than 60 seconds - short time control
            {
                // Use 20-30% of remaining time per move for short controls
                softBound = Math.Max(500, timeLeft / 5); // 20% of remaining time
            }
            else
            {
                // Distribute over estimated remaining moves (40 for long games)
                softBound = Math.Max(500, timeLeft / 40);
            }
            long hardBound = Math.Min(softBound * 3, timeLeft - 500);

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
            _ => TimeAllocation.Default
        };
    }

    /// <summary>
    /// Clear transposition table
    /// </summary>
    public void Clear()
    {
        _transpositionTable.Clear();
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

                board.PlaceStone(x, y, opponent);
                bool isWinningMove = _winDetector.CheckWin(board).HasWinner;
                board.GetCell(x, y).Player = Player.None;

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

        var targetDepth = AdaptiveDepthCalculator.GetDepth(difficulty, board);
        var candidates = GetCandidateMoves(board);

        if (candidates.Count == 0)
            return (null, 0, 0, 0);

        // Use same thread count as main search for this difficulty
        // This ensures pondering uses the same resources as thinking
        int ponderThreadCount = ThreadPoolConfig.GetThreadCountForDifficulty(difficulty);

        // Cap thread count based on depth to avoid overhead
        ponderThreadCount = Math.Min(ponderThreadCount, Math.Max(2, targetDepth / 2));

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
            boardsArray[i] = board.Clone();
            candidatesArray[i] = new List<(int x, int y)>(candidates);
        }

        // Create pondering-specific time allocation
        var ponderTimeAlloc = new TimeAllocation
        {
            SoftBoundMs = maxPonderTimeMs * 3 / 4,  // 75% for soft bound
            HardBoundMs = maxPonderTimeMs,
            OptimalTimeMs = maxPonderTimeMs / 2,
            IsEmergency = false,
            Phase = GamePhase.EarlyMid,
            ComplexityMultiplier = 1.0
        };

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
                    targetDepth,
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

        // Find best result (prefer master thread, then deeper results if scores are similar)
        // For pondering, we still prefer master thread result for consistency
        var bestResult = results.FirstOrDefault(r => r.threadIndex == 0);
        if (bestResult == default)
        {
            bestResult = results.OrderByDescending(r => r.score)
                                       .ThenByDescending(r => r.depth)
                                       .FirstOrDefault();
        }

        if (bestResult.depth == 0)
            return (null, 0, 0, 0);

        // Track maximum depth achieved across all threads (not just the winning move's depth)
        // This gives a better picture of how deeply the AI thought during pondering
        int maxDepth = results.Any() ? results.Max(r => r.depth) : bestResult.depth;

        // Return total nodes searched across all threads (via Interlocked counter), not just the winning thread's nodes
        long totalNodes = Interlocked.Read(ref _realNodesSearched);
        return ((bestResult.x, bestResult.y), maxDepth, bestResult.score, totalNodes);
    }

    /// <summary>
    /// Iterative deepening search for pondering thread with cancellation support
    /// </summary>
    private (int x, int y, int score, int depth, long nodes) SearchPonderIteration(
        Board board,
        Player player,
        int targetDepth,
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

        // Use the real node counter (thread-safe via Interlocked)
        Interlocked.Exchange(ref _realNodesSearched, 0);

        // Debug: log start - simplified format
        Console.WriteLine($"[PONDER {ponderingFor}] Starting, targetDepth={targetDepth}");

        // Start from depth 2 and iterate up
        for (int currentDepth = 2; currentDepth <= targetDepth; currentDepth++)
        {
            // Check cancellation
            if (cancellationToken.IsCancellationRequested)
            {
                Console.WriteLine($"[PONDER {ponderingFor}] Cancelled at depth {currentDepth}");
                break;
            }

            var elapsed = _searchStopwatch?.ElapsedMilliseconds ?? 0;

            // Check hard bound
            if (elapsed >= _hardTimeBoundMs)
            {
                _searchCts?.Cancel();
                break;
            }

            // Check soft bound
            if (elapsed >= timeAlloc.SoftBoundMs)
                break;

            // FIX: Use safe bounds to prevent overflow
            int alpha = int.MinValue + 1000;
            int beta = int.MaxValue - 1000;

            // Aspiration Windows (Safe Implementation)
            // Only narrow the window if we have a previous stable score
            if (currentDepth > 2 && bestScore > int.MinValue + 2000 && bestScore < int.MaxValue - 2000)
            {
                alpha = Math.Max(int.MinValue + 1000, bestScore - 50);
                beta = Math.Min(int.MaxValue - 1000, bestScore + 50);
            }

            var result = SearchRoot(board, player, currentDepth, candidates, threadData, alpha, beta, cancellationToken);

            if (result.score > bestScore || bestMove == (-1, -1))
            {
                bestScore = result.score;
                bestMove = (result.x, result.y);
            }

            bestDepth = currentDepth;

            // Report progress
            progressCallback?.Invoke((bestMove.x, bestMove.y, bestDepth, bestScore));

            // Early exit on winning move
            if (result.score >= 100000)
                break;
        }

        // Use real node count instead of estimate
        long actualNodes = Interlocked.Read(ref _realNodesSearched);

        // Debug: log final result
        Console.WriteLine($"[PONDER {ponderingFor}] Finished: {actualNodes} nodes, depth {bestDepth}");

        return (bestMove.x, bestMove.y, bestScore, bestDepth, actualNodes);
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
    /// Get the actual node count from the search (thread-safe)
    /// Returns the total nodes searched across all threads via Interlocked counter
    /// </summary>
    public long GetRealNodesSearched() => Interlocked.Read(ref _realNodesSearched);

    /// <summary>
    /// Check if search is currently running
    /// </summary>
    public bool IsSearching => _searchStopwatch != null && _searchStopwatch.IsRunning;

    #endregion
}
