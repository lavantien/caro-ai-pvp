using Caro.Domain;

namespace Caro.Engine;

public static partial class SearchEngine
{
    internal static (int X, int Y, int Score) SearchRoot(
        SearchBoard sb,
        Player player,
        int depth,
        int alpha,
        int beta,
        TranspositionTable tt,
        SearchHeuristics heuristics,
        List<Position> candidates,
        TimeMonitor monitor,
        Position? preferredMove)
    {
        monitor.AddNode();
        Position? ttMove;
        if (preferredMove.HasValue)
        {
            ttMove = preferredMove;
        }
        else if (tt.Lookup(sb.Hash(), out TTEntry entry))
        {
            ttMove = new Position(entry.MoveX, entry.MoveY);
        }
        else
        {
            ttMove = null;
        }

        List<Position> ordered = MoveOrdering.OrderMoves(candidates, sb, player, depth, ttMove, heuristics);

        int bestScore = -Constants.Infinity;
        int bestX = -1;
        int bestY = -1;
        int origAlpha = alpha;

        for (int i = 0; i < ordered.Count; i++)
        {
            Position move = ordered[i];
            if (monitor.ShouldStop())
            {
                break;
            }

            sb.MakeMove(move.X, move.Y, player);

            int score;
            if (MoveOrdering.WouldWin(sb, move.X, move.Y, player))
            {
                score = Constants.WinScore - 1;
            }
            else if (i == 0)
            {
                score = -AlphaBeta(sb, player.Opponent(), depth - 1, -beta, -alpha, tt, heuristics, monitor, move, 1);
            }
            else
            {
                score = -AlphaBeta(sb, player.Opponent(), depth - 1, -alpha - 1, -alpha, tt, heuristics, monitor, move, 1);
                if (score > alpha && score < beta)
                {
                    score = -AlphaBeta(sb, player.Opponent(), depth - 1, -beta, -alpha, tt, heuristics, monitor, move, 1);
                }
            }

            sb.UnmakeMove();

            if (score > bestScore)
            {
                bestScore = score;
                bestX = move.X;
                bestY = move.Y;
            }
            if (score > alpha)
            {
                alpha = score;
            }
        }

        if (bestX >= 0 && !monitor.ShouldStop())
        {
            TTEntryType flag = TTEntryType.Exact;
            if (bestScore <= origAlpha)
            {
                flag = TTEntryType.UpperBound;
            }
            else if (bestScore >= beta)
            {
                flag = TTEntryType.LowerBound;
            }
            tt.Store(new TTEntry
            {
                Hash = sb.Hash(),
                Score = MateScore.AdjustForStore(bestScore, 0),
                Depth = (byte)depth,
                MoveX = (sbyte)bestX,
                MoveY = (sbyte)bestY,
                Type = flag,
            });
            heuristics.RecordKiller(depth, new Position(bestX, bestY));
        }

        return (bestX, bestY, bestScore);
    }

    /// <summary>
    /// Returns the late-move-reduction ply count for a move. Forcing moves
    /// (winning completions, must-blocks, threats) are never reduced: a
    /// reduced scout of a forcing move hides the refutation unless it
    /// happens to beat alpha, which is exactly the blunder the guard exists
    /// to prevent.
    /// </summary>
    internal static int LmrReduction(int depth, int moveIdx, bool tactical, int histScore)
    {
        if (tactical || depth < Constants.LMRMinDepth || moveIdx < Constants.LMRFullDepthMoves)
        {
            return 0;
        }
        int reduction = 1;
        if (moveIdx > Constants.LMRDeepMoveThreshold)
        {
            reduction = 2;
        }
        if (histScore < 0)
        {
            reduction++;
        }
        if (reduction >= depth)
        {
            reduction = depth - 1;
        }
        return reduction;
    }

    internal static int AlphaBeta(
        SearchBoard sb,
        Player player,
        int depth,
        int alpha,
        int beta,
        TranspositionTable tt,
        SearchHeuristics heuristics,
        TimeMonitor monitor,
        Position prevMove,
        int plyFromRoot)
    {
        monitor.AddNode();
        if (monitor.ShouldStop())
        {
            return 0;
        }

        if (depth <= 0)
        {
            return Quiesce(sb, player, alpha, beta, Constants.MaxQuiescenceDepth, heuristics, monitor, plyFromRoot);
        }

        int origAlpha = alpha;

        // Null-move pruning. The static eval is only needed on this path;
        // most interior nodes skip it entirely.
        if (depth >= Constants.NullMoveMinDepth && Evaluation.Evaluate(sb, player) >= beta)
        {
            sb.MakeNullMove();
            Position nullPrev = new(-1, -1);
            int nullScore = -AlphaBeta(sb, player.Opponent(), depth - 1 - Constants.NullMoveReduction,
                -beta, -beta + 1, tt, heuristics, monitor, nullPrev, plyFromRoot + 1);
            sb.UnmakeNullMove();
            if (nullScore >= beta && !monitor.ShouldStop())
            {
                return nullScore;
            }
        }

        if (tt.Lookup(sb.Hash(), out TTEntry entry) && entry.Depth >= depth)
        {
            int ttScore = MateScore.AdjustForRetrieve(entry.Score, plyFromRoot);
            switch (entry.Type)
            {
                case TTEntryType.Exact:
                    return ttScore;
                case TTEntryType.LowerBound:
                    if (ttScore > alpha)
                    {
                        alpha = ttScore;
                    }
                    break;
                case TTEntryType.UpperBound:
                    if (ttScore < beta)
                    {
                        beta = ttScore;
                    }
                    break;
            }
            if (alpha >= beta)
            {
                return ttScore;
            }
        }

        List<Position> candidates = Candidates.GetCandidates(sb, Constants.MaxSearchRadius);
        candidates = Candidates.FilterOpenRule(candidates, sb, player);
        Position? ttMove = null;
        if (tt.Lookup(sb.Hash(), out TTEntry ttEntry))
        {
            ttMove = new Position(ttEntry.MoveX, ttEntry.MoveY);
        }

        MovePicker picker = new(candidates, sb, player, depth, ttMove, heuristics, prevMove);

        int bestScore = -Constants.Infinity;
        int bestMoveX = -1;
        int bestMoveY = -1;
        int moveIdx = 0;

        while (picker.Next(out Position move))
        {
            if (monitor.ShouldStop())
            {
                break;
            }

            int reduction = LmrReduction(depth, moveIdx, picker.LastMoveTactical(),
                heuristics.HistoryScore(player, move.X, move.Y));

            sb.MakeMove(move.X, move.Y, player);

            int score;
            if (MoveOrdering.WouldWin(sb, move.X, move.Y, player))
            {
                score = Constants.WinScore - plyFromRoot;
            }
            else
            {
                int newDepth = depth - 1 - reduction;
                if (moveIdx == 0)
                {
                    score = -AlphaBeta(sb, player.Opponent(), newDepth, -beta, -alpha, tt, heuristics, monitor, move, plyFromRoot + 1);
                }
                else
                {
                    score = -AlphaBeta(sb, player.Opponent(), newDepth, -alpha - 1, -alpha, tt, heuristics, monitor, move, plyFromRoot + 1);
                    if (score > alpha && score < beta)
                    {
                        score = -AlphaBeta(sb, player.Opponent(), depth - 1, -beta, -alpha, tt, heuristics, monitor, move, plyFromRoot + 1);
                    }
                }
            }

            sb.UnmakeMove();

            if (score > bestScore)
            {
                bestScore = score;
                bestMoveX = move.X;
                bestMoveY = move.Y;
            }
            if (score > alpha)
            {
                alpha = score;
            }
            if (alpha >= beta)
            {
                heuristics.RecordKiller(depth, move);
                heuristics.RecordHistory(player, move.X, move.Y, depth);
                heuristics.RecordContHistory(player, prevMove.X, prevMove.Y, move.X, move.Y, depth);
                if (prevMove.X >= 0)
                {
                    heuristics.RecordCounterMove(player, prevMove.X, prevMove.Y, move.X, move.Y);
                }
                break;
            }
            moveIdx++;
        }

        if (!monitor.ShouldStop())
        {
            TTEntryType flag = TTEntryType.Exact;
            if (bestScore <= origAlpha)
            {
                flag = TTEntryType.UpperBound;
            }
            else if (bestScore >= beta)
            {
                flag = TTEntryType.LowerBound;
            }
            tt.Store(new TTEntry
            {
                Hash = sb.Hash(),
                Score = MateScore.AdjustForStore(bestScore, plyFromRoot),
                Depth = (byte)depth,
                MoveX = (sbyte)bestMoveX,
                MoveY = (sbyte)bestMoveY,
                Type = flag,
            });
        }

        return bestScore;
    }
}
