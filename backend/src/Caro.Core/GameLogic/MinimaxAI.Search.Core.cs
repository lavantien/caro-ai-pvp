using System.Runtime.CompilerServices;
using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic.Search;
using SHC = Caro.Core.Domain.Configuration.SearchHeuristicConstants;

namespace Caro.Core.GameLogic;

/// <summary>
/// MinimaxAI partial class - Core search algorithms using SearchBoard (make/unmake).
/// MinimaxCore and QuiesceCore for high-performance zero-allocation search path.
/// </summary>
public partial class MinimaxAI
{
    /// <summary>
    /// Core minimax algorithm using SearchBoard with make/unmake pattern.
    /// High-performance path that avoids Board.PlaceStone allocations.
    /// </summary>
    private int MinimaxCore(SearchBoard board, int depth, int alpha, int beta, bool isMaximizing, Player aiPlayer, int rootDepth)
    {
        // Count this node
        _nodesSearched++;

        // Time control: check periodically (every N nodes) to avoid timeout
        if ((_nodesSearched & (TimeCheckInterval - 1)) == 0)
        {
            var elapsed = _searchStopwatch.ElapsedMilliseconds;
            if (elapsed >= _searchHardBoundMs)
            {
                _searchStopped = true;
                return isMaximizing ? alpha : beta;
            }
        }

        // Check terminal states
        var winner = CheckWinner(board);
        if (winner != null)
        {
            return winner == aiPlayer ? SHC.WinScore : -SHC.WinScore;
        }

        if (depth == 0)
        {
            return QuiesceCore(board, alpha, beta, isMaximizing, aiPlayer, rootDepth, qsPly: 0);
        }

        // NULL-MOVE PRUNING
        var isNullMoveEligible = (beta - alpha) <= 1;
        if (depth >= SearchConstants.NullMoveMinDepth && isNullMoveEligible && TacticalEvaluator.IsNullMoveSafe(board, isMaximizing ? aiPlayer : (aiPlayer == Player.Red ? Player.Blue : Player.Red)))
        {
            int nullMoveDepth = depth - SearchConstants.NullMoveDepthReduction;
            if (nullMoveDepth > 0)
            {
                int nullMoveScore = MinimaxCore(board, nullMoveDepth, beta - 1, beta, !isMaximizing, aiPlayer, rootDepth);
                if (nullMoveScore >= beta)
                {
                    return beta;
                }
            }
        }

        // Use Span-based candidate generation for zero allocation
        Span<(int x, int y)> candidateSpan = stackalloc (int, int)[128];
        int candidateCount = CandidateGenerator.GetCandidateMoves(board, candidateSpan, SearchConstants.MaxSearchRadius);
        if (candidateCount == 0)
        {
            return 0;
        }

        // Transposition table lookup using SearchBoard hash
        var boardHash = board.GetHash();
        _tableLookups++;
        var (found, cachedScore, cachedMove) = _transpositionTable.Lookup(boardHash, depth, alpha, beta);
        if (found)
        {
            _tableHits++;
            return cachedScore;
        }

        var currentPlayer = isMaximizing ? aiPlayer : (aiPlayer == Player.Red ? Player.Blue : Player.Red);

        // Convert span to list for move ordering (ordering requires List interface)
        var candidateList = new List<(int x, int y)>(candidateCount);
        for (int i = 0; i < candidateCount; i++)
            candidateList.Add(candidateSpan[i]);

        // Order moves
        var orderedMoves = _moveOrderer.OrderMoves(candidateList, rootDepth - depth, board, currentPlayer, cachedMove);

        int score;
        const int lmrFullDepthMoves = SHC.LMRFullDepthMoves;
        const int pvsEnabledDepth = SHC.PVSEnabledDepth;

        if (isMaximizing)
        {
            var maxEval = int.MinValue;
            var moveIndex = 0;

            foreach (var (x, y) in orderedMoves)
            {
                // Make move (mutates board in-place)
                var undo = board.MakeMove(x, y, currentPlayer);

                int eval;
                bool isPvNode = (moveIndex == 0) && (depth >= pvsEnabledDepth);

                if (isPvNode)
                {
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !TacticalEvaluator.IsTacticalPosition(board))
                    {
                        eval = MinimaxCore(board, depth - 2, alpha, beta, false, aiPlayer, rootDepth);
                        if (eval > alpha && eval < beta - 100)
                        {
                            eval = MinimaxCore(board, depth - 1, alpha, beta, false, aiPlayer, rootDepth);
                        }
                    }
                    else
                    {
                        eval = MinimaxCore(board, depth - 1, alpha, beta, false, aiPlayer, rootDepth);
                    }
                }
                else
                {
                    int searchDepth = depth - 1;
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !TacticalEvaluator.IsTacticalPosition(board))
                    {
                        searchDepth = depth - 2;
                    }

                    eval = MinimaxCore(board, searchDepth, alpha, alpha + 1, false, aiPlayer, rootDepth);

                    if (eval > alpha && eval < beta)
                    {
                        if (searchDepth == depth - 2)
                        {
                            eval = MinimaxCore(board, depth - 1, alpha, beta, false, aiPlayer, rootDepth);
                        }
                        else
                        {
                            eval = MinimaxCore(board, depth - 1, alpha, beta, false, aiPlayer, rootDepth);
                        }
                    }
                }

                // Unmake move (restores board state)
                board.UnmakeMove(undo);

                if (eval > maxEval)
                {
                    maxEval = eval;
                }

                alpha = Math.Max(alpha, eval);
                if (beta <= alpha)
                {
                    RecordKillerMove(rootDepth - depth, x, y);
                    RecordHistoryMove(currentPlayer, x, y, depth);
                    break;
                }

                moveIndex++;
            }
            score = maxEval;
        }
        else
        {
            var minEval = int.MaxValue;
            var moveIndex = 0;

            foreach (var (x, y) in orderedMoves)
            {
                // Make move (mutates board in-place)
                var undo = board.MakeMove(x, y, currentPlayer);

                int eval;
                bool isPvNode = (moveIndex == 0) && (depth >= pvsEnabledDepth);

                if (isPvNode)
                {
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !TacticalEvaluator.IsTacticalPosition(board))
                    {
                        eval = MinimaxCore(board, depth - 2, alpha, beta, true, aiPlayer, rootDepth);
                        if (eval < beta && eval > alpha + 100)
                        {
                            eval = MinimaxCore(board, depth - 1, alpha, beta, true, aiPlayer, rootDepth);
                        }
                    }
                    else
                    {
                        eval = MinimaxCore(board, depth - 1, alpha, beta, true, aiPlayer, rootDepth);
                    }
                }
                else
                {
                    int searchDepth = depth - 1;
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !TacticalEvaluator.IsTacticalPosition(board))
                    {
                        searchDepth = depth - 2;
                    }

                    eval = MinimaxCore(board, searchDepth, beta - 1, beta, true, aiPlayer, rootDepth);

                    if (eval < beta && eval > alpha)
                    {
                        if (searchDepth == depth - 2)
                        {
                            eval = MinimaxCore(board, depth - 1, alpha, beta, true, aiPlayer, rootDepth);
                        }
                        else
                        {
                            eval = MinimaxCore(board, depth - 1, alpha, beta, true, aiPlayer, rootDepth);
                        }
                    }
                }

                // Unmake move (restores board state)
                board.UnmakeMove(undo);

                if (eval < minEval)
                {
                    minEval = eval;
                }

                beta = Math.Min(beta, eval);
                if (beta <= alpha)
                {
                    RecordKillerMove(rootDepth - depth, x, y);
                    RecordHistoryMove(currentPlayer, x, y, depth);
                    break;
                }

                moveIndex++;
            }
            score = minEval;
        }

        // Store result in transposition table
        _transpositionTable.Store(boardHash, depth, score, null, alpha, beta);

        return score;
    }

    /// <summary>
    /// Quiescence search using SearchBoard with make/unmake pattern.
    /// High-performance path that avoids Board.PlaceStone allocations.
    /// </summary>
    private int QuiesceCore(SearchBoard board, int alpha, int beta, bool isMaximizing, Player aiPlayer, int rootDepth, int qsPly)
    {
        // Time control
        if ((_nodesSearched & 15) == 8)
        {
            var elapsed = _searchStopwatch.ElapsedMilliseconds;
            if (elapsed >= _searchHardBoundMs)
            {
                _searchStopped = true;
                return isMaximizing ? alpha : beta;
            }
        }

        // Get stand-pat score using SearchBoard evaluator
        var standPat = EvaluateBoard(board, aiPlayer);

        // Check for terminal states
        var winner = CheckWinner(board);
        if (winner != null)
        {
            return winner == aiPlayer ? SHC.WinScore : -SHC.WinScore;
        }

        // Generate tactical moves using Span for zero allocation
        Span<(int x, int y)> tacticalSpan = stackalloc (int, int)[128];
        int tacticalCount = CandidateGenerator.GetCandidateMoves(board, tacticalSpan, SearchConstants.MaxSearchRadius);

        // Limit quiescence depth (track quiescence ply separately from minimax depth)
        const int maxQuiescenceDepth = 4;
        if (qsPly >= maxQuiescenceDepth)
        {
            return standPat;
        }

        // Filter to only tactical (forcing) moves to prevent branching explosion
        var currentPlayer = isMaximizing ? aiPlayer : (aiPlayer == Player.Red ? Player.Blue : Player.Red);
        for (int i = tacticalCount - 1; i >= 0; i--)
        {
            if (!TacticalEvaluator.IsTacticalPosition(board))
            {
                // Not a forcing position - filter non-tactical candidates
                var (tx, ty) = tacticalSpan[i];
                var undo = board.MakeMove(tx, ty, currentPlayer);
                bool isTactical = Pattern4Evaluator.IsForcingThreat(
                    Pattern4Evaluator.EvaluatePositionBitBoard(
                        board.GetBitBoard(currentPlayer),
                        board.GetBitBoard(currentPlayer == Player.Red ? Player.Blue : Player.Red),
                        tx, ty));
                board.UnmakeMove(undo);
                if (!isTactical)
                {
                    tacticalSpan[i] = tacticalSpan[tacticalCount - 1];
                    tacticalCount--;
                }
            }
        }

        if (tacticalCount == 0)
            return standPat;

        // Stand-pat pruning: skip if in a forced response situation
        bool hasForcingThreat = TacticalEvaluator.IsTacticalPosition(board);
        if (!hasForcingThreat)
        {
            // Beta cutoff
            if (isMaximizing && standPat >= beta)
                return beta;

            // Alpha cutoff
            if (!isMaximizing && standPat <= alpha)
                return alpha;

            // Update bounds
            if (isMaximizing)
                alpha = Math.Max(alpha, standPat);
            else
                beta = Math.Min(beta, standPat);
        }

        // Convert tactical span to list for move ordering
        var tacticalList = new List<(int x, int y)>(tacticalCount);
        for (int i = 0; i < tacticalCount; i++)
            tacticalList.Add(tacticalSpan[i]);

        // Order tactical moves
        var orderedMoves = _moveOrderer.OrderMoves(tacticalList, rootDepth, board, currentPlayer, null);

        if (isMaximizing)
        {
            var maxEval = standPat;
            foreach (var (x, y) in orderedMoves)
            {
                if (!board.IsEmpty(x, y))
                    continue;

                var undo = board.MakeMove(x, y, currentPlayer);
                var eval = QuiesceCore(board, alpha, beta, false, aiPlayer, rootDepth, qsPly + 1);
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
            foreach (var (x, y) in orderedMoves)
            {
                if (!board.IsEmpty(x, y))
                    continue;

                var undo = board.MakeMove(x, y, currentPlayer);
                var eval = QuiesceCore(board, alpha, beta, true, aiPlayer, rootDepth, qsPly + 1);
                board.UnmakeMove(undo);

                minEval = Math.Min(minEval, eval);
                beta = Math.Min(beta, eval);

                if (beta <= alpha)
                    return alpha;
            }
            return minEval;
        }
    }
}
