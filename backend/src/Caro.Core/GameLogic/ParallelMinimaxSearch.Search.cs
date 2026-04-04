using System.Runtime.CompilerServices;
using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic.Search;
using Caro.Core.GameLogic.TimeManagement;
using TMC = Caro.Core.Domain.Configuration.TimeManagementConstants;
using SHC = Caro.Core.Domain.Configuration.SearchHeuristicConstants;
using EC = Caro.Core.Domain.Configuration.EvaluationConstants;

namespace Caro.Core.GameLogic;

public sealed partial class ParallelMinimaxSearch
{
    // Mate scores are near WinScore. Adjust when storing/retrieving from TT
    // so scores are position-relative, not root-relative.
    private const int MateScoreThreshold = SHC.WinScore - SHC.MaxSearchDepth - SHC.MaxQuiescenceDepth;

    /// Adjust a score for TT storage: convert root-relative mate scores to position-relative.
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static short ScoreToTT(int score, int plyFromRoot)
    {
        if (score > MateScoreThreshold)
            return (short)(score + plyFromRoot);
        if (score < -MateScoreThreshold)
            return (short)(score - plyFromRoot);
        return (short)score;
    }

    /// Adjust a TT score for retrieval: convert position-relative back to root-relative.
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static int ScoreFromTT(short ttScore, int plyFromRoot)
    {
        int score = ttScore;
        if (score > MateScoreThreshold)
            return score - plyFromRoot;
        if (score < -MateScoreThreshold)
            return score + plyFromRoot;
        return score;
    }

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
            return isMaximizing ? alpha : beta;

        // Terminal check with mate-distance scoring
        var winner = ParallelNodeEvaluator.CheckWinner(board);
        if (winner != null)
        {
            int plyFromRoot = rootDepth - depth;
            return winner == aiPlayer
                ? SHC.WinScore - plyFromRoot
                : -(SHC.WinScore - plyFromRoot);
        }

        if (depth == 0)
        {
            // Use quiescence search to resolve tactical positions
            // This extends search in positions with active threats to avoid horizon effect
            int plyFromRoot = rootDepth - depth;
            return Quiesce(board, alpha, beta, isMaximizing, aiPlayer, plyFromRoot, threadData, cancellationToken);
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
            int currentPly = rootDepth - depth;
            return ScoreFromTT(cachedScore, currentPly);
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
                return isMaximizing ? alpha : beta;
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

            int storePly = rootDepth - depth;
            _transpositionTable.Store(boardHash, (sbyte)depth, ScoreToTT(bestScore, storePly),
                (sbyte)bestMove.Value.x, (sbyte)bestMove.Value.y, alpha, beta, (byte)threadData.ThreadIndex, rootDepth);
        }

        return bestScore;
    }
}
