using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic.Search;
using Caro.Core.GameLogic.TimeManagement;
using TMC = Caro.Core.Domain.Configuration.TimeManagementConstants;
using SHC = Caro.Core.Domain.Configuration.SearchHeuristicConstants;

namespace Caro.Core.GameLogic;

public sealed partial class ParallelMinimaxSearch
{
    /// Quiescence search: extend search in tactical positions to get accurate evaluation.
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

    /// Calculate search depth based on time allocation.
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
        return baseDepth;
    }
}
