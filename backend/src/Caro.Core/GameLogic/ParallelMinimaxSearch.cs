using System.Collections.Concurrent;
using System.Diagnostics;
using Caro.Core.Entities;
using Caro.Core.GameLogic.TimeManagement;

namespace Caro.Core.GameLogic;

/// <summary>
/// Result from parallel search including move and statistics
/// </summary>
public record ParallelSearchResult(int X, int Y, int DepthAchieved, long NodesSearched, int ThreadCount = 0);

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
        public int[,] HistoryRed = new int[15, 15];
        public int[,] HistoryBlue = new int[15, 15];
        public int TableHits;
        public int TableLookups;
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

        // Apply Open Rule: Red's second move (move #3) cannot be in center 5x5 zone
        if (player == Player.Red && moveNumber == 3)
        {
            candidates = candidates.Where(c => !(c.x >= 5 && c.x <= 9 && c.y >= 5 && c.y <= 9)).ToList();
        }

        if (candidates.Count == 0)
        {
            // No valid candidates - board is empty or all filtered out
            if (player == Player.Red && moveNumber == 3)
            {
                // Open rule applies - find first valid cell outside center 5x5
                for (int x = 0; x < 15; x++)
                {
                    for (int y = 0; y < 15; y++)
                    {
                        if (board.GetCell(x, y).Player == Player.None && !(x >= 5 && x <= 9 && y >= 5 && y <= 9))
                            return (x, y);
                    }
                }
            }
            return (7, 7); // Center move
        }

        // Use provided time allocation or create default
        var alloc = timeAlloc ?? GetDefaultTimeAllocation(difficulty, timeRemainingMs);

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
            Console.WriteLine($"[AI] Threat detected! Considering {opponentThreatMoves.Count} blocking moves.");
            // Filter candidates to only blocking moves
            var forcingSet = new HashSet<(int x, int y)>(opponentThreatMoves);
            candidates = candidates.Where(c => forcingSet.Contains((c.x, c.y))).ToList();

            // If candidates ended up empty (edge case), fallback to original threat list
            if (candidates.Count == 0)
                candidates = opponentThreatMoves;
        }

        // Adjust depth based on time allocation
        var adjustedDepth = CalculateDepthForTime(baseDepth, alloc, candidates.Count);

        // For Braindead difficulty, add randomness
        if (difficulty == AIDifficulty.Braindead && _random.Next(100) < 50)
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

        // Apply Open Rule: Red's second move (move #3) cannot be in center 5x5 zone
        if (player == Player.Red && moveNumber == 3)
        {
            candidates = candidates.Where(c => !(c.x >= 5 && c.x <= 9 && c.y >= 5 && c.y <= 9)).ToList();
        }

        if (candidates.Count == 0)
        {
            // No valid candidates - return center with minimal stats
            return new ParallelSearchResult(7, 7, 0, 0);
        }

        // Use provided time allocation or create default
        var alloc = timeAlloc ?? GetDefaultTimeAllocation(difficulty, timeRemainingMs);

        // Try VCF first for higher difficulties
        if (difficulty >= AIDifficulty.Hard)
        {
            var vcfTimeLimit = CalculateVCFTimeLimit(alloc);
            var vcfResult = _vcfSolver.SolveVCF(board, player, vcfTimeLimit, maxDepth: 30);

            if (vcfResult.IsSolved && vcfResult.IsWin && vcfResult.BestMove.HasValue)
            {
                Console.WriteLine($"[AI VCF] Found winning move ({vcfResult.BestMove.Value.x}, {vcfResult.BestMove.Value.y}), depth: {vcfResult.DepthAchieved}");
                return new ParallelSearchResult(vcfResult.BestMove.Value.x, vcfResult.BestMove.Value.y,
                    vcfResult.DepthAchieved, vcfResult.NodesSearched);
            }
        }

        // Check for opponent's immediate threats that must be blocked
        // CRITICAL FIX: Do NOT return immediately. Filter candidates to ONLY blocking moves and let the engine search.
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var opponentThreatMoves = GetOpponentThreatMoves(board, opponent);
        if (opponentThreatMoves.Count > 0)
        {
            Console.WriteLine($"[AI] Threat detected! Considering {opponentThreatMoves.Count} blocking moves.");
            // Filter candidates to only blocking moves
            var forcingSet = new HashSet<(int x, int y)>(opponentThreatMoves);
            candidates = candidates.Where(c => forcingSet.Contains((c.x, c.y))).ToList();

            // If candidates ended up empty (edge case), fallback to original threat list
            if (candidates.Count == 0)
                candidates = opponentThreatMoves;
        }

        // Adjust depth based on time allocation
        var adjustedDepth = CalculateDepthForTime(baseDepth, alloc, candidates.Count);

        // For Braindead difficulty, add randomness
        if (difficulty == AIDifficulty.Braindead && _random.Next(100) < 50)
        {
            var randomMove = candidates[_random.Next(candidates.Count)];
            return new ParallelSearchResult(randomMove.x, randomMove.y, 1, candidates.Count);
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
            return new ParallelSearchResult(x, y, adjustedDepth, actualNodes);
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
            return new ParallelSearchResult(x, y, depth, actualNodes, 0);
        }

        // Include threadIndex to distinguish master thread (0) from helper threads (1+)
        var results = new ConcurrentBag<(int x, int y, int score, int depth, long nodes, int threadIndex)>();

        // Create thread-local copies of board and candidates for each thread
        var boardsArray = new Board[threadCount];
        var candidatesArray = new List<(int x, int y)>[threadCount];
        for (int i = 0; i < threadCount; i++)
        {
            boardsArray[i] = board.Clone();
            candidatesArray[i] = new List<(int x, int y)>(candidates);
        }

        // Launch parallel searches with time-aware iterative deepening
        var token = _searchCts.Token;
        var tasks = new List<Task>();
        for (int i = 0; i < threadCount; i++)
        {
            int threadId = i;
            var task = Task.Run(() =>
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
                    threadData, timeAlloc, difficulty, token);

                // Add threadIndex to identify master vs helper thread results
                var (x, y, score, depthAchieved, nodes) = result;
                results.Add((x, y, score, depthAchieved, nodes, threadId));
            }, token);
            tasks.Add(task);
        }

        // Wait for all threads - handle cancellation gracefully
        try
        {
            Task.WaitAll(tasks.ToArray());
        }
        catch (AggregateException ae)
        {
            // TaskCanceledException is expected when time runs out
            // Filter out expected cancellation exceptions
            var unexpectedExceptions = ae.InnerExceptions.Where(e => e is not TaskCanceledException).ToList();
            if (unexpectedExceptions.Any())
            {
                throw new AggregateException("Unexpected exceptions during parallel search", unexpectedExceptions);
            }
            // If all are TaskCanceledException, that's expected - just continue
        }

        _searchStopwatch.Stop();

        // Select best result (prefer deeper results if scores are similar)
        // If no results were produced (all tasks cancelled), use a fallback move
        if (results.IsEmpty)
        {
            // Fallback to first candidate if no results
            long totalNodes = Interlocked.Read(ref _realNodesSearched);
            return new ParallelSearchResult(candidates[0].x, candidates[0].y, 1, totalNodes);
        }

        // RELIABILITY FIX: Always trust the Master Thread (ThreadIndex=0) result
        // The master thread searches deterministically without noise injection.
        // Helper threads populate the transposition table for speedup, but their
        // result selection may be affected by timing and cutoff differences.
        //
        // First try to get the master thread result at acceptable depth
        int minAcceptableDepth = Math.Max(1, depth - 2);
        var masterThreadResult = results.FirstOrDefault(r => r.threadIndex == 0 && r.depth >= minAcceptableDepth);

        // Fallback 1: Any master thread result (regardless of depth)
        if (masterThreadResult == default)
        {
            masterThreadResult = results.FirstOrDefault(r => r.threadIndex == 0);
        }

        // Fallback 2: Best result from any thread at acceptable depth
        if (masterThreadResult == default)
        {
            var acceptableResults = results.Where(r => r.depth >= minAcceptableDepth).ToList();
            if (acceptableResults.Count > 0)
            {
                masterThreadResult = acceptableResults.OrderByDescending(r => r.score).First();
            }
        }

        // Fallback 3: Any result at all
        if (masterThreadResult == default)
        {
            masterThreadResult = results.OrderByDescending(r => r.score).FirstOrDefault();
        }

        var bestResult = (masterThreadResult.x, masterThreadResult.y, masterThreadResult.score, masterThreadResult.depth, masterThreadResult.nodes);

        // Use real node count instead of estimated
        long totalNodesFinal = Interlocked.Read(ref _realNodesSearched);

        Console.WriteLine($"[TIME] Depth: {bestResult.depth}, Time: {_searchStopwatch.ElapsedMilliseconds}ms/" +
                         $"{timeAlloc.SoftBoundMs}ms, Phase: {timeAlloc.Phase}, " +
                         $"Complexity: {timeAlloc.ComplexityMultiplier:F2}, Nodes: {totalNodesFinal:N0}");

        return new ParallelSearchResult(bestResult.x, bestResult.y, bestResult.depth, totalNodesFinal, threadCount);
    }

    /// <summary>
    /// Iterative deepening search for a single thread with time awareness
    /// Implements move stability detection for early termination
    /// Note: Node counting is done globally via Interlocked in Minimax()
    /// </summary>
    private (int x, int y, int score, int depth, long nodes) SearchWithIterationTimeAware(
        Board board,
        Player player,
        int targetDepth,
        List<(int x, int y)> candidates,
        ThreadData threadData,
        TimeAllocation timeAlloc,
        AIDifficulty difficulty,
        CancellationToken cancellationToken)
    {
        var bestMove = candidates[0];
        var bestScore = int.MinValue;
        int bestDepth = 1;
        int stableCount = 0; // Number of consecutive depths with same best move

        // Start from depth 2 and iterate up
        for (int currentDepth = 2; currentDepth <= targetDepth; currentDepth++)
        {
            // Check if search should stop (time exceeded by another thread)
            if (cancellationToken.IsCancellationRequested)
            {
                break;
            }

            var elapsed = _searchStopwatch?.ElapsedMilliseconds ?? 0;

            // Check hard bound (absolute maximum) - cancel for all threads
            if (elapsed >= _hardTimeBoundMs)
            {
                _searchCts?.Cancel();
                break;
            }

            // Check soft bound with stability consideration
            if (elapsed >= timeAlloc.SoftBoundMs)
            {
                // If move is stable (same for 2+ consecutive depths), we can stop early
                if (stableCount >= 2)
                {
                    break;
                }
            }

            // Check optimal time - if move is very stable, exit
            if (elapsed >= timeAlloc.OptimalTimeMs && stableCount >= 3)
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

        // Store in shared TT
        _transpositionTable.Store(board.Hash, (sbyte)depth, (short)bestScore, bestMove, alpha, beta);

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
        if (_searchStopwatch != null && _hardTimeBoundMs > 0)
        {
            var elapsed = _searchStopwatch.ElapsedMilliseconds;
            if (elapsed >= _hardTimeBoundMs)
            {
                _searchCts?.Cancel();
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
            return Evaluate(board, aiPlayer);
        }

        var candidates = GetCandidateMoves(board);
        if (candidates.Count == 0)
        {
            return 0; // Draw
        }

        // TT lookup
        var boardHash = board.Hash;
        threadData.TableLookups++;
        var (found, cachedScore, cachedMove) = _transpositionTable.Lookup(boardHash, (sbyte)depth, alpha, beta);
        if (found)
        {
            threadData.TableHits++;
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

        // Store result
        if (bestMove.HasValue)
        {
            _ = (bestScore <= alpha)
                ? LockFreeTranspositionTable.EntryFlag.UpperBound
                : (bestScore >= beta ? LockFreeTranspositionTable.EntryFlag.LowerBound : LockFreeTranspositionTable.EntryFlag.Exact);

            _transpositionTable.Store(boardHash, (sbyte)depth, (short)bestScore,
                (sbyte)bestMove.Value.x, (sbyte)bestMove.Value.y, alpha, beta);
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
    /// </summary>
    private List<(int x, int y)> GetCandidateMoves(Board board)
    {
        var candidates = new List<(int x, int y)>(64);
        var considered = new bool[15, 15];

        var playerBitBoard = board.GetBitBoard(Player.Red);
        var opponentBitBoard = board.GetBitBoard(Player.Blue);
        var occupied = playerBitBoard | opponentBitBoard;

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
    private int CalculateDepthForTime(int baseDepth, TimeAllocation timeAlloc, int candidateCount)
    {
        // Emergency mode - reduce depth significantly
        if (timeAlloc.IsEmergency)
        {
            return Math.Max(1, baseDepth - 3);
        }

        // Adjust based on time available
        var softBoundSeconds = timeAlloc.SoftBoundMs / 1000.0;

        // Very tight time (< 2s)
        if (softBoundSeconds < 2)
        {
            return Math.Max(1, baseDepth - 2);
        }

        // Tight time (< 5s)
        if (softBoundSeconds < 5)
        {
            return Math.Max(2, baseDepth - 1);
        }

        // Moderate time (< 10s)
        if (softBoundSeconds < 10)
        {
            if (candidateCount > 20) // Complex position
            {
                return Math.Max(2, baseDepth - 1);
            }
            return baseDepth;
        }

        // Good time availability: use full depth
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
            long softBound = Math.Max(500, timeLeft / 40); // Distribute over 40 moves
            long hardBound = Math.Min(softBound * 2, timeLeft - 1000);

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
        for (int x = 0; x < 15; x++)
        {
            for (int y = 0; y < 15; y++)
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

        // Check for StraightFour (XXXX_ pattern) - semi-open four
        // GainSquares contains the winning square(s) - for StraightFour, this is the block position
        foreach (var threat in opponentThreats)
        {
            if (threat.Type == ThreatType.StraightFour)
            {
                // StraightFour with exactly 1 gain square = semi-open four (one end blocked)
                // This is critical and must be blocked immediately
                if (threat.GainSquares.Count == 1)
                {
                    var blockSquare = threat.GainSquares[0];
                    if (board.GetCell(blockSquare.x, blockSquare.y).IsEmpty)
                    {
                        threats.Add(blockSquare);
                        return threats; // Critical threat - return immediately
                    }
                }
                // StraightFour with 2+ gain squares = open four (both ends open)
                // Also critical - add the first gain square
                else if (threat.GainSquares.Count > 1)
                {
                    foreach (var gainSquare in threat.GainSquares)
                    {
                        if (board.GetCell(gainSquare.x, gainSquare.y).IsEmpty && !threats.Contains(gainSquare))
                        {
                            threats.Add(gainSquare);
                        }
                    }
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
    /// Pondering variant of Lazy SMP - uses reduced thread count to avoid starving main search
    /// Searches with predicted opponent move already made on the board
    /// Results are stored in the shared transposition table for main search benefit
    /// </summary>
    /// <param name="board">Board with predicted opponent move already made</param>
    /// <param name="player">Player to move (us, after opponent's predicted move)</param>
    /// <param name="difficulty">AI difficulty level</param>
    /// <param name="maxPonderTimeMs">Maximum time to spend pondering</param>
    /// <param name="cancellationToken">Token to cancel pondering</param>
    /// <param name="progressCallback">Optional callback for progress updates</param>
    /// <returns>Best move found, depth reached, score, and nodes searched</returns>
    public ((int x, int y)? bestMove, int depth, int score, long nodesSearched) PonderLazySMP(
        Board board,
        Player player,
        AIDifficulty difficulty,
        long maxPonderTimeMs,
        CancellationToken cancellationToken,
        Action<(int x, int y, int depth, int score)>? progressCallback = null)
    {
        if (player == Player.None)
            return (null, 0, 0, 0);

        var targetDepth = AdaptiveDepthCalculator.GetDepth(difficulty, board);
        var candidates = GetCandidateMoves(board);

        if (candidates.Count == 0)
            return (null, 0, 0, 0);

        // Use dedicated pondering thread count (conservative to avoid system responsiveness issues)
        // This prevents pondering from starving the main search when it starts
        int ponderThreadCount = ThreadPoolConfig.GetPonderingThreadCount();

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
                var threadData = new ThreadData { Random = new Random(threadId + (int)DateTime.UtcNow.Ticks) };

                var result = SearchPonderIteration(
                    boardsArray[threadId],
                    player,
                    targetDepth,
                    candidatesArray[threadId],
                    threadData,
                    ponderTimeAlloc,
                    linkedToken,
                    progressCallback);

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

        return ((bestResult.x, bestResult.y), bestResult.depth, bestResult.score, bestResult.nodes);
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
        Action<(int x, int y, int depth, int score)>? progressCallback)
    {
        var bestMove = candidates[0];
        var bestScore = int.MinValue;
        int bestDepth = 1;
        long nodesSearched = 0;

        // Start from depth 2 and iterate up
        for (int currentDepth = 2; currentDepth <= targetDepth; currentDepth++)
        {
            // Check cancellation
            if (cancellationToken.IsCancellationRequested)
                break;

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
            nodesSearched += CountNodes(currentDepth, candidates.Count);

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

        return (bestMove.x, bestMove.y, bestScore, bestDepth, nodesSearched);
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
}
