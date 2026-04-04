using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic.Search;
using Caro.Core.GameLogic.TimeManagement;
using TMC = Caro.Core.Domain.Configuration.TimeManagementConstants;
using SHC = Caro.Core.Domain.Configuration.SearchHeuristicConstants;

namespace Caro.Core.GameLogic;

public sealed partial class ParallelMinimaxSearch
{
    /// <summary>
    /// Minimax with alpha-beta pruning (thread-safe via per-thread data)
    /// </summary>
    private int Minimax(SearchBoard board, int depth, int alpha, int beta, bool isMaximizing, Player aiPlayer, int rootDepth, ThreadData threadData, CancellationToken cancellationToken)
    {
        // CRITICAL FIX: Count nodes locally to avoid cache contention from Interlocked
        // All 9 threads incrementing shared counter on every node = severe bottleneck
        threadData.LocalNodesSearched++;

        // Single cancellation check per Minimax call (not per-node)
        // TimeMonitor cancels via timer, this ensures we respond within one call
        if (cancellationToken.IsCancellationRequested)
            return int.MinValue;

        // Terminal check
        var winner = ParallelNodeEvaluator.CheckWinner(board);
        if (winner != null)
        {
            return winner == aiPlayer ? SHC.WinScore : -SHC.WinScore;
        }

        if (depth == 0)
        {
            // Use quiescence search to resolve tactical positions
            // This extends search in positions with active threats to avoid horizon effect
            return Quiesce(board, alpha, beta, isMaximizing, aiPlayer, rootDepth, threadData, cancellationToken);
        }

        Span<(int x, int y)> candidateBuf = stackalloc (int x, int y)[256];
        int candidateCount = board.GetCandidateMovesBitwise(candidateBuf, threadData.SearchRadius);
        if (candidateCount == 0)
        {
            return 0; // Draw
        }
        var candidates = candidateBuf.Slice(0, candidateCount);

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
        OrderMovesStagedSpan(candidates, rootDepth - depth, board, currentPlayer, cachedMove, threadData);

        int bestScore = isMaximizing ? int.MinValue : int.MaxValue;
        (int x, int y)? bestMove = null;
        int moveIndex = 0;

        foreach (var (x, y) in candidates)
        {
            // No per-move time check needed - TimeMonitor handles cancellation via timer

            // MDAP: Move-Dependent Adaptive Pruning (Adaptive Late Move Reduction)
            // Apply dynamic depth reduction based on position characteristics
            int reducedDepth = depth;
            bool doLMR = false;

            // Get history score for this move
            var historyTable = currentPlayer == Player.Red ? threadData.HistoryRed : threadData.HistoryBlue;
            int historyScore = historyTable[x, y];

            // Determine move characteristics for adaptive LMR
            bool isImproving = ParallelNodeEvaluator.IsImproving(board, currentPlayer);
            bool isPvNode = beta - alpha <= 1;
            bool isCutNode = !isPvNode && beta - alpha > 1;
            bool isTTMove = cachedMove.HasValue && cachedMove.Value == (x, y);

            // Calculate adaptive reduction based on multiple factors
            int adaptiveReduction = ParallelNodeEvaluator.GetAdaptiveReduction(
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

            var undo = board.MakeMove(x, y, currentPlayer);
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
            board.UnmakeMove(undo);
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
                // FMC% tracking: record cutoff statistics
                threadData.TotalCutoffs++;
                if (moveIndex == 1)
                {
                    // Cutoff on first move searched (best move ordering)
                    threadData.FirstMoveCutoffs++;
                }

                // Cutoff
                if (depth >= 2 && depth < 20)
                {
                    RecordKillerMove(threadData, rootDepth - depth, x, y);
                }
                RecordHistoryMove(threadData, currentPlayer, x, y, depth);

                // Update continuation history for this successful move
                // Use move history to update continuation scores
                int bonus = depth * depth * TMC.DepthBonusMultiplier;
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
    /// Quiescence search: extend search in tactical positions to get accurate evaluation
    /// Only considers moves near existing stones (tactical moves)
    /// PERFORMANCE: Simplified to match sequential search - no expensive per-candidate loops
    /// </summary>
    private int Quiesce(SearchBoard board, int alpha, int beta, bool isMaximizing, Player aiPlayer, int quiesceDepth, ThreadData threadData, CancellationToken cancellationToken)
    {
        // Count quiescence node locally (no Interlocked contention)
        threadData.LocalNodesSearched++;

        // Check cancellation
        if (cancellationToken.IsCancellationRequested)
        {
            return isMaximizing ? alpha : beta;
        }

        // Get stand-pat score (static evaluation)
        var standPat = ParallelNodeEvaluator.Evaluate(board, aiPlayer);

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
        var winner = ParallelNodeEvaluator.CheckWinner(board);
        if (winner != null)
        {
            return winner == aiPlayer ? SHC.WinScore : -SHC.WinScore;
        }

        // Limit quiescence search depth to avoid explosion
        const int maxQuiescenceDepth = 4;
        if (quiesceDepth > maxQuiescenceDepth)
        {
            return standPat;
        }

        // Generate candidate moves (near existing stones)
        Span<(int x, int y)> tacticalBuf = stackalloc (int x, int y)[256];
        int tacticalCount = board.GetCandidateMovesBitwise(tacticalBuf, threadData.SearchRadius);

        // If no tactical moves, return static evaluation
        if (tacticalCount == 0)
            return standPat;

        var currentPlayer = isMaximizing ? aiPlayer : (aiPlayer == Player.Red ? Player.Blue : Player.Red);

        // Order moves for better pruning
        var tacticalMoves = tacticalBuf.Slice(0, tacticalCount);
        OrderMovesStagedSpan(tacticalMoves, quiesceDepth, board, currentPlayer, null, threadData);

        // Search tactical moves (only empty cells)
        if (isMaximizing)
        {
            var maxEval = standPat;
            foreach (var (x, y) in tacticalMoves)
            {
                // Skip occupied cells
                if (!board.IsEmpty(x, y))
                    continue;

                var undo = board.MakeMove(x, y, currentPlayer);

                // Recursive quiescence search
                var eval = Quiesce(board, alpha, beta, false, aiPlayer, quiesceDepth + 1, threadData, cancellationToken);
                board.UnmakeMove(undo);

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
            foreach (var (x, y) in tacticalMoves)
            {
                // Skip occupied cells
                if (!board.IsEmpty(x, y))
                    continue;

                var undo = board.MakeMove(x, y, currentPlayer);

                var eval = Quiesce(board, alpha, beta, true, aiPlayer, quiesceDepth + 1, threadData, cancellationToken);
                board.UnmakeMove(undo);

                minEval = Math.Min(minEval, eval);
                beta = Math.Min(beta, eval);

                if (beta <= alpha)
                    return alpha;
            }
            return minEval;
        }
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
}
