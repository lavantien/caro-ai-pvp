using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic.Search;
using Caro.Core.GameLogic.TimeManagement;
using SHC = Caro.Core.Domain.Configuration.SearchHeuristicConstants;
using EC = Caro.Core.Domain.Configuration.EvaluationConstants;

namespace Caro.Core.GameLogic;

public sealed partial class ParallelMinimaxSearch
{
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
        SearchBoard board,
        Player player,
        List<(int x, int y)> candidates,
        ThreadData threadData,
        TimeAllocation timeAlloc,
        CancellationToken cancellationToken)
    {
        // CRITICAL FIX: Preserve priority moves (blocking squares) at the front
        // The caller may have already prioritized blocking squares for open threes
        // Pre-sorting by evaluation would undo this prioritization
        // Solution: Keep the first few candidates in their original order (they're priority moves)
        // and only sort the rest by static evaluation

        const int PriorityMoveCount = SHC.PriorityMoveCount; // First N candidates are considered "priority" and not re-sorted

        // Filter to empty cells only to prevent PlaceStone from throwing
        var emptyCandidates = candidates
            .Where(c => board.IsEmpty(c.x, c.y))
            .ToList();

        // Separate priority moves (first N) from the rest
        var priorityMoves = emptyCandidates.Take(PriorityMoveCount).ToList();
        var remainingCandidates = emptyCandidates.Skip(PriorityMoveCount).ToList();

        // Sort remaining candidates by static evaluation
        var sortedRemaining = remainingCandidates
            .Select(c =>
            {
                var undo = board.MakeMove(c.x, c.y, player);
                int eval = ParallelNodeEvaluator.Evaluate(board, player);
                board.UnmakeMove(undo);
                return (c, eval);
            })
            .OrderByDescending(x => x.eval)
            .Select(x => x.c)
            .ToList();

        // Combine: priority moves first, then sorted remaining
        var evaluatedCandidates = priorityMoves.Concat(sortedRemaining).ToList();

        // Initialize bestMove with the first candidate (highest priority - may be a blocking square)
        var bestMove = evaluatedCandidates.Count > 0 ? evaluatedCandidates[0] : candidates[0];
        var bestScore = int.MinValue;
        int bestDepth = 1;

        // FIX 1: Track best move from completed depth separately
        // This is preserved even if current iteration aborts
        int lastCompletedDepth = 0;
        (int x, int y) bestMoveFromCompletedDepth = bestMove;
        int stableCount = 0;
        long lastIterationElapsedMs = 0;
        long iterationStartMs = 0;  // Track start time of current iteration
        long nodesAtStart = threadData.LocalNodesSearched;  // Track nodes at iteration start

        bool isMasterThread = threadData.ThreadIndex == 0;

        // PURE TIME-BASED SEARCH
        // Search continues until time runs out
        // LAZY SMP: Per Chessprogramming Wiki, helper threads should search at different
        // depths to exploit nondeterminism. Cheng uses: current depth + (1 for each even helper)
        int depthOffset = threadData.ThreadIndex % 2 == 1 ? 1 : 0;
        int currentDepth = 1 + depthOffset;
        const int MaxSearchDepth = SHC.MaxSearchDepth;
        while (true)
        {
            // MAX DEPTH CHECK: Prevent runaway depth values
            if (currentDepth > MaxSearchDepth)
            {
                break;
            }

            // CRITICAL: Pre-iteration check - Total nodes must scale with depth
            if (currentDepth > 10)
            {
                long minimumTotalNodesForDepth = (long)(currentDepth - SHC.DepthEstimationBaseline) * (currentDepth - SHC.DepthEstimationBaseline) * SHC.DepthEstimationMultiplier;
                int threadCount = _maxThreads > 0 ? _maxThreads : 1;
                long perThreadMinimum = minimumTotalNodesForDepth / threadCount;
                if (threadData.LocalNodesSearched < perThreadMinimum)
                {
                    break;
                }
            }

            // Record iteration start time BEFORE any work
            iterationStartMs = _timeMonitor?.ElapsedMs ?? 0;

            // TIME BOUND ENFORCEMENT
            var elapsedForCheck = _timeMonitor?.ElapsedMs ?? 0;
            long remainingTimeMs = _hardTimeBoundMs - elapsedForCheck;

            // Hard bound check - ALL threads must stop when time is up
            if (elapsedForCheck >= _hardTimeBoundMs)
            {
                if (isMasterThread) _searchCts?.Cancel();
                break;
            }

            // PRE-ITERATION TIME ESTIMATE
            if (currentDepth >= 2)
            {
                long estimatedIterationTimeMs;
                if (currentDepth == 2 && lastIterationElapsedMs > 0)
                {
                    estimatedIterationTimeMs = lastIterationElapsedMs * SHC.IterationTimeEstimateAggressive;
                }
                else if (currentDepth > 2 && lastIterationElapsedMs > 0)
                {
                    estimatedIterationTimeMs = lastIterationElapsedMs * SHC.IterationTimeEstimateNormal;
                }
                else
                {
                    estimatedIterationTimeMs = 0;
                }

                if (estimatedIterationTimeMs > 0 && remainingTimeMs < estimatedIterationTimeMs)
                {
                    break;
                }
            }

            // Check cancellation
            if (cancellationToken.IsCancellationRequested)
                break;

            // For depth 3+, use soft bound and optimal time checks
            if (currentDepth > 2)
            {
                if (elapsedForCheck >= _hardTimeBoundMs * SHC.HardTimeCheckRatio)
                {
                    if (isMasterThread) _searchCts?.Cancel();
                    break;
                }

                if (isMasterThread && elapsedForCheck >= timeAlloc.SoftBoundMs)
                {
                    if (lastIterationElapsedMs > remainingTimeMs * SHC.IterationAbortRatio)
                        break;
                }

                if (isMasterThread && elapsedForCheck >= timeAlloc.OptimalTimeMs && stableCount >= 3)
                {
                    if (lastIterationElapsedMs > remainingTimeMs * SHC.IterationCautionRatio)
                        break;
                }
            }

            int alpha = int.MinValue + SHC.AlphaBetaMargin;
            int beta = int.MaxValue - SHC.AlphaBetaMargin;

            if (bestScore > int.MinValue + 2000 && bestScore < int.MaxValue - 2000)
            {
                alpha = Math.Max(int.MinValue + SHC.AlphaBetaMargin, bestScore - SHC.AspirationWindow);
                beta = Math.Min(int.MaxValue - SHC.AlphaBetaMargin, bestScore + SHC.AspirationWindow);
            }

            long nodesBeforeIteration = threadData.LocalNodesSearched;

            var result = SearchRoot(board, player, currentDepth, candidates, threadData, alpha, beta, cancellationToken);

            var elapsedNow = _timeMonitor?.ElapsedMs ?? 0;
            lastIterationElapsedMs = elapsedNow - iterationStartMs;

            long nodesSearchedThisIteration = threadData.LocalNodesSearched - nodesBeforeIteration;

            bool searchWasAborted = nodesSearchedThisIteration == 0 || result.score == int.MinValue;
            if (searchWasAborted)
            {
                break;
            }

            if (result.score == int.MinValue)
            {
                // Search was aborted - don't update anything
            }
            else if (result.x == bestMove.Item1 && result.y == bestMove.Item2)
                stableCount++;
            else
            {
                stableCount = 1;
                bestMove = (result.x, result.y);
            }

            if (result.score > bestScore || bestMove == (-1, -1))
            {
                if (result.score != int.MinValue)
                {
                    bestScore = result.score;
                    bestMove = (result.x, result.y);
                    bestDepth = currentDepth;

                    lastCompletedDepth = currentDepth;
                    bestMoveFromCompletedDepth = (result.x, result.y);
                }
                else if (bestMove == (-1, -1))
                {
                    bestMove = (result.x, result.y);
                    bestDepth = currentDepth;
                }
            }

            if (cancellationToken.IsCancellationRequested)
                break;

            if (result.score > EC.MaxCorrectedEval)
                break;

            currentDepth++;
        }

        // FIX 1: Return preserved best from completed depth if available
        var finalMove = lastCompletedDepth > 0 ? bestMoveFromCompletedDepth : (bestMove.x, bestMove.y);
        var finalDepth = lastCompletedDepth > 0 ? lastCompletedDepth : bestDepth;

        return (finalMove.x, finalMove.y, bestScore, finalDepth, threadData.LocalNodesSearched);
    }

    /// <summary>
    /// Estimate nodes searched for a given depth and candidate count
    /// </summary>
    private long EstimateNodes(int depth, int candidateCount)
    {
        long nodes = candidateCount;
        int branching = Math.Min(candidateCount, 25);
        for (int i = 1; i < depth && i < 6; i++)
        {
            nodes *= branching;
        }
        return nodes;
    }

    /// <summary>
    /// Root search with aspiration window
    /// </summary>
    private (int x, int y, int score) SearchRoot(
        SearchBoard board, Player player, int depth, List<(int x, int y)> candidates,
        ThreadData threadData, int alpha, int beta, CancellationToken cancellationToken)
    {
        if (cancellationToken.IsCancellationRequested)
        {
            return (candidates[0].x, candidates[0].y, int.MinValue);
        }

        var bestMove = candidates[0];
        var bestScore = int.MinValue;

        // CRITICAL FIX: Preserve priority moves (blocking squares) at the front
        const int PriorityMoveCount = SHC.PriorityMoveCount;

        var priorityMoves = candidates.Take(PriorityMoveCount).ToList();
        var remainingCandidates = candidates.Skip(PriorityMoveCount).ToList();

        var orderedRemaining = OrderMovesStaged(remainingCandidates, depth, board, player, null, threadData);

        var orderedMoves = priorityMoves.Concat(orderedRemaining).ToList();

        int moveIndex = 0;
        foreach (var (x, y) in orderedMoves)
        {
            if (!board.IsEmpty(x, y))
            {
                continue;
            }

            if (cancellationToken.IsCancellationRequested)
            {
                break;
            }

            var undo = board.MakeMove(x, y, player);
            var score = Minimax(board, depth - 1, alpha, beta, false, player, depth, threadData, cancellationToken);
            board.UnmakeMove(undo);

            if (DebugLogging && threadData.ThreadIndex == 0 && moveIndex < 5)
            {
                Console.WriteLine($"  [SearchRoot] Thread {threadData.ThreadIndex}: move=({x},{y}), score={score}");
            }

            if (score != int.MinValue && score > bestScore)
            {
                bestScore = score;
                bestMove = (x, y);
            }

            if (cancellationToken.IsCancellationRequested)
            {
                break;
            }

            moveIndex++;

            alpha = Math.Max(alpha, score);
            if (beta <= alpha)
            {
                RecordKillerMove(threadData, depth, x, y);
                break;
            }
        }

        // LAZY SMP TT WRITING: Allow helper threads to write with quality criteria
        bool shouldStore = threadData.ThreadIndex == 0 || depth >= 3;
        if (shouldStore)
        {
            // At root, plyFromRoot = 0 so ScoreToTT is a no-op for mate scores.
            // Still call it for consistency.
            _transpositionTable.Store(board.GetHash(), (sbyte)depth, ScoreToTT(bestScore, 0), (sbyte)bestMove.x, (sbyte)bestMove.y, alpha, beta, (byte)threadData.ThreadIndex, rootDepth: depth);
        }

        return (bestMove.x, bestMove.y, bestScore);
    }
}
