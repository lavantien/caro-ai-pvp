using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic.TimeManagement;
using SHC = Caro.Core.Domain.Configuration.SearchHeuristicConstants;
using EC = Caro.Core.Domain.Configuration.EvaluationConstants;

namespace Caro.Core.GameLogic;

/// <summary>
/// Pondering support for ParallelMinimaxSearch.
/// Contains methods for background searching while the opponent is thinking.
/// Results are stored in the shared transposition table for main search benefit.
/// Uses the same <see cref="ThreadPoolConfig.MaxEngineThreads"/> cap as main search.
/// </summary>
public sealed partial class ParallelMinimaxSearch
{
    /// <summary>
    /// Pondering variant of Lazy SMP - uses same thread count as main search
    /// Searches with predicted opponent move already made on the board
    /// Results are stored in the shared transposition table for main search benefit
    /// </summary>
    /// <param name="board">Board with predicted opponent move already made</param>
    /// <param name="player">Player to move (us, after opponent's predicted move)</param>
    /// <param name="maxPonderTimeMs">Maximum time to spend pondering</param>
    /// <param name="cancellationToken">Token to cancel pondering</param>
    /// <param name="progressCallback">Optional callback for progress updates</param>
    /// <param name="ponderingFor">Player doing the pondering (for debug logging)</param>
    /// <returns>Best move found, depth reached, score, and nodes searched</returns>
    public ((int x, int y)? bestMove, int depth, int score, long nodesSearched) PonderLazySMP(
        Board board,
        Player player,
        long maxPonderTimeMs,
        CancellationToken cancellationToken,
        Action<(int x, int y, int depth, int score)>? progressCallback = null,
        Player ponderingFor = Player.None)
    {
        if (player == Player.None)
            return (null, 0, 0, 0);

        var searchBoard = new SearchBoard(board);
        var candidates = GetCandidateMoves(searchBoard, MaxSearchRadius);



        if (candidates.Count == 0)
            return (null, 0, 0, 0);

        // Use time-budget calculation for pondering depth
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

        // NPS is learned from actual search performance - no hardcoded targets

        // Use same thread count cap as main search (MaxEngineThreads)
        int ponderThreadCount = ThreadPoolConfig.MaxEngineThreads;

        _transpositionTable.IncrementAge();

        // Set up time management for pondering
        // Use the provided CancellationToken combined with our own for time-based cancellation
        _searchCts?.Cancel(); // Cancel any previous search
        _searchCts = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);

        // Create timer-based time monitor for pondering
        _timeMonitor?.Dispose();
        _timeMonitor = new TimeMonitor(maxPonderTimeMs, _searchCts);
        _hardTimeBoundMs = maxPonderTimeMs;

        // Create thread-local copies of SearchBoard and candidates for each thread
        var boardsArray = new SearchBoard[ponderThreadCount];
        var candidatesArray = new List<(int x, int y)>[ponderThreadCount];
        var diagnosticsArray = new ThreadData[ponderThreadCount];
        for (int i = 0; i < ponderThreadCount; i++)
        {
            boardsArray[i] = searchBoard.Clone();
            candidatesArray[i] = new List<(int x, int y)>(candidates);
        }

        var linkedToken = _searchCts.Token;

        // Dispatch pondering to persistent worker pool (zero thread-startup overhead)
        var rawResults = _workerPool!.Search(threadId =>
        {
            var threadData = new ThreadData
            {
                ThreadIndex = threadId,
                SearchRadius = MaxSearchRadius,
                Random = new Random((int)(Environment.TickCount64 + (long)threadId * 0x9E3779B9L))
            };

            try
            {
                var result = SearchPonderIteration(
                    boardsArray[threadId],
                    player,
                    candidatesArray[threadId],
                    threadData,
                    ponderTimeAlloc,
                    linkedToken,
                    progressCallback,
                    ponderingFor);

                diagnosticsArray[threadId] = threadData;
                return result;
            }
            catch (OperationCanceledException)
            {
                diagnosticsArray[threadId] = threadData;
                return default;
            }
        }, (int)(maxPonderTimeMs + 1000));

        _searchCts?.Cancel();

        // Convert raw results to list for merging
        var results = new List<(int x, int y, int score, int depth, long nodes, int threadIndex)>();
        for (int i = 0; i < rawResults.Length; i++)
        {
            var r = rawResults[i];
            if (r.depth > 0 || r.nodes > 0)
                results.Add((r.x, r.y, r.score, r.depth, r.nodes, i));
        }

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
        SearchBoard board,
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

        // PURE TIME-BASED SEARCH with TT inflation guards
        const int MaxSearchDepth = SHC.MaxSearchDepth;
        int currentDepth = 2;
        while (true)
        {
            // MAX DEPTH CHECK: Prevent runaway depth from TT inflation
            if (currentDepth > MaxSearchDepth)
                break;

            // PRE-ITERATION TT INFLATION GUARD: Same as SearchWithIterationTimeAware
            // When TT hit rate is high, depth can increment thousands of times without real search.
            // Require: total_nodes >= (depth-5)^2 * 200 for depth > 10
            if (currentDepth > 10)
            {
                long minimumNodesForDepth = (long)(currentDepth - SHC.DepthEstimationBaseline) * (currentDepth - SHC.DepthEstimationBaseline) * SHC.DepthEstimationMultiplier;
                int threadCount = _maxThreads > 0 ? _maxThreads : 1;
                long perThreadMinimum = minimumNodesForDepth / threadCount;
                if (threadData.LocalNodesSearched < perThreadMinimum)
                    break;
            }

            if (cancellationToken.IsCancellationRequested)
                break;

            var elapsed = _timeMonitor?.ElapsedMs ?? 0;

            if (elapsed >= _hardTimeBoundMs)
            {
                _searchCts?.Cancel();
                break;
            }

            double remainingTime = _hardTimeBoundMs - elapsed;
            if (elapsed >= timeAlloc.SoftBoundMs && lastIterationElapsedMs > remainingTime * 0.25)
                break;

            var iterationStartTime = _timeMonitor?.ElapsedMs ?? 0;
            long nodesBeforeIteration = threadData.LocalNodesSearched;
            iterationCount++;

            int alpha = int.MinValue + SHC.AlphaBetaMargin;
            int beta = int.MaxValue - SHC.AlphaBetaMargin;
            if (bestScore > int.MinValue + 2000 && bestScore < int.MaxValue - 2000)
            {
                alpha = Math.Max(int.MinValue + SHC.AlphaBetaMargin, bestScore - SHC.AspirationWindow);
                beta = Math.Min(int.MaxValue - SHC.AlphaBetaMargin, bestScore + SHC.AspirationWindow);
            }

            var result = SearchRoot(board, player, currentDepth, candidates, threadData, alpha, beta, cancellationToken);

            lastIterationElapsedMs = (_timeMonitor?.ElapsedMs ?? 0) - iterationStartTime;

            // POST-ITERATION GUARD: If search didn't actually do work, stop
            long nodesThisIteration = threadData.LocalNodesSearched - nodesBeforeIteration;
            if (nodesThisIteration == 0 || result.score == int.MinValue)
                break;

            if (cancellationToken.IsCancellationRequested)
                break;

            if (result.score > bestScore || bestMove == (-1, -1))
            {
                bestScore = result.score;
                bestMove = (result.x, result.y);
            }

            bestDepth = currentDepth;

            progressCallback?.Invoke((bestMove.x, bestMove.y, bestDepth, bestScore));

            if (result.score > EC.MaxCorrectedEval)
                break;

            currentDepth++;
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
}
