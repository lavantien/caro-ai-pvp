using Caro.Domain;

namespace Caro.Engine;

public static partial class SearchEngine
{
    internal static int Quiesce(
        SearchBoard sb,
        Player player,
        int alpha,
        int beta,
        int maxPly,
        SearchHeuristics heuristics,
        TimeMonitor monitor,
        int plyFromRoot)
    {
        monitor.AddNode();
        if (monitor.ShouldStop())
        {
            return 0;
        }

        int standPat = Evaluation.Evaluate(sb, player);
        int best = standPat;
        if (standPat >= beta)
        {
            return standPat;
        }
        if (standPat > alpha)
        {
            alpha = standPat;
        }
        if (maxPly <= 0)
        {
            return standPat;
        }

        List<Position> candidates = Candidates.GetTacticalCandidates(sb, player);
        candidates = Candidates.FilterOpenRule(candidates, sb, player);
        foreach (Position move in candidates)
        {
            if (monitor.ShouldStop())
            {
                break;
            }

            sb.MakeMove(move.X, move.Y, player);
            int score;
            if (MoveOrdering.WouldWin(sb, move.X, move.Y, player))
            {
                score = Constants.Score.WinScore - plyFromRoot;
            }
            else
            {
                score = -Quiesce(sb, player.Opponent(), -beta, -alpha, maxPly - 1, heuristics, monitor, plyFromRoot + 1);
            }
            sb.UnmakeMove();

            if (score > best)
            {
                best = score;
            }
            if (score >= beta)
            {
                return score;
            }
            if (score > alpha)
            {
                alpha = score;
            }
        }

        return best;
    }
}

internal static class MateScore
{
    internal static bool IsForcedWinScore(int score) =>
        score >= Constants.Score.WinScore - Constants.Search.AbsoluteMaxDepth;

    internal static int AdjustForStore(int score, int plyFromRoot)
    {
        if (score > Constants.Score.WinScore - Constants.Search.AbsoluteMaxDepth)
        {
            return score + plyFromRoot;
        }
        if (score < -Constants.Score.WinScore + Constants.Search.AbsoluteMaxDepth)
        {
            return score - plyFromRoot;
        }
        return score;
    }

    internal static int AdjustForRetrieve(int storedScore, int plyFromRoot)
    {
        if (storedScore >= Constants.Score.WinScore - Constants.Search.AbsoluteMaxDepth + 1)
        {
            return storedScore - plyFromRoot;
        }
        if (storedScore <= -(Constants.Score.WinScore - Constants.Search.AbsoluteMaxDepth) - 1)
        {
            return storedScore + plyFromRoot;
        }
        return storedScore;
    }
}
