using Caro.Domain;

namespace Caro.Engine;

public static partial class SearchEngine
{
    public static (int X, int Y, SearchStats Stats) SearchPosition(
        Board b,
        Player player,
        SearchConfig config,
        TranspositionTable tt,
        SearchHeuristics heuristics,
        CancellationToken ctx)
    {
        SearchBoard sb = new(b);
        List<Position> candidates = Candidates.GetCandidates(sb, Constants.MaxSearchRadius);
        candidates = Candidates.FilterOpenRule(candidates, sb, player);

        if (candidates.Count == 0)
        {
            return (-1, -1, default(SearchStats));
        }
        if (candidates.Count == 1)
        {
            return (candidates[0].X, candidates[0].Y, default(SearchStats));
        }

        int bestX = candidates[0].X;
        int bestY = candidates[0].Y;
        using TimeMonitor monitor = new(config.TimeLimitMs, ctx);

        tt.ResetStats();
        int bestScore = -Constants.Infinity;
        int completedDepth = 0;
        int fullAlpha = -Constants.Infinity;
        int fullBeta = Constants.Infinity;

        if (config.UseVCF)
        {
            SearchBoard oppSB = new(b);
            if (!Vcf.OpponentHasImmediateWin(oppSB, player.Opponent()))
            {
                long vcfTime = (long)(config.TimeLimitMs * Constants.VCFTimeFraction);
                (int vx, int vy, VCFResult result) = Vcf.SolveVCFWithDepth(b, player, config.VCFMaxDepth, vcfTime, ctx);
                if (result == VCFResult.Win)
                {
                    return (vx, vy, new SearchStats
                    {
                        DepthAchieved = 0,
                        SearchScore = Constants.WinScore,
                        AllocatedTimeMs = config.TimeLimitMs,
                        MoveType = MoveTypes.Vcf,
                    });
                }
            }
        }

        Position? vcfPreferred = null;
        if (config.UseVCF)
        {
            long oppVcfTime = (long)(config.TimeLimitMs * Constants.VCFBlockFraction);
            (int vx, int vy, VCFResult result) = Vcf.SolveVCFWithDepth(b, player.Opponent(), config.VCFMaxDepth, oppVcfTime, ctx);
            if (result == VCFResult.Win)
            {
                Board blocked = b.PlaceStone(vx, vy, player);
                long blockCheckTime = (long)(config.TimeLimitMs * Constants.VCFBlockCheckFraction);
                (_, _, VCFResult checkResult) = Vcf.SolveVCFWithDepth(blocked, player.Opponent(), config.VCFMaxDepth, blockCheckTime, ctx);
                if (checkResult != VCFResult.Win)
                {
                    vcfPreferred = new Position(vx, vy);
                }
            }
        }

        long lastIterMs = 0;
        long prevIterMs = 0;
        for (int depth = 1; depth <= config.MaxDepth; depth++)
        {
            if (monitor.ShouldStop())
            {
                break;
            }
            // Do not start an iteration that cannot finish inside the soft
            // budget; the hard bound stays as the emergency stop.
            if (depth > 1 && config.SoftLimitMs > 0 && monitor.ElapsedMs() >= config.SoftLimitMs)
            {
                break;
            }
            if (depth > 1 && !IterationBudget.NextIterationFits(monitor.ElapsedMs(), lastIterMs, prevIterMs, config.SoftLimitMs))
            {
                break;
            }
            long iterStart = monitor.ElapsedMs();

            int delta = Constants.AspirationWindowSize;
            int a = fullAlpha;
            int betaBound = fullBeta;
            if (depth > 1)
            {
                a = Math.Max(bestScore - delta, fullAlpha);
                betaBound = Math.Min(bestScore + delta, fullBeta);
            }

            int x = 0;
            int y = 0;
            int score = 0;
            bool found = false;
            for (int attempt = 0; attempt < Constants.MaxAspirationAttempts; attempt++)
            {
                (x, y, score) = SearchRoot(sb, player, depth, a, betaBound, tt, heuristics, candidates, monitor, vcfPreferred);
                if (x < 0 || monitor.ShouldStop())
                {
                    break;
                }
                if (score <= a && a > fullAlpha)
                {
                    a = Math.Max(a - delta, fullAlpha);
                    delta *= 2;
                    continue;
                }
                if (score >= betaBound && betaBound < fullBeta)
                {
                    betaBound = Math.Min(betaBound + delta, fullBeta);
                    delta *= 2;
                    continue;
                }
                found = true;
                break;
            }

            if (!found && !monitor.ShouldStop())
            {
                (x, y, score) = SearchRoot(sb, player, depth, fullAlpha, fullBeta, tt, heuristics, candidates, monitor, vcfPreferred);
                if (x >= 0)
                {
                    found = true;
                }
            }

            if (found)
            {
                bestX = x;
                bestY = y;
                bestScore = score;
                completedDepth = depth;
                prevIterMs = lastIterMs;
                lastIterMs = monitor.ElapsedMs() - iterStart;
                if (MateScore.IsForcedWinScore(score))
                {
                    break;
                }
            }
        }

        if (completedDepth == 0)
        {
            // No depth finished in time. Fall back to the best-ordered move,
            // never the raw scan-order candidate list head.
            List<Position> ordered = MoveOrdering.OrderMoves(candidates, sb, player, 0, null, heuristics);
            if (ordered.Count > 0)
            {
                bestX = ordered[0].X;
                bestY = ordered[0].Y;
            }
            bestScore = 0;
        }

        long elapsed = monitor.ElapsedMs();
        (long probes, long hits) = tt.Stats();
        long nodes = monitor.NodesCount;
        double hitRate = 0;
        if (probes > 0)
        {
            hitRate = (double)hits / probes;
        }
        double nps = 0;
        if (elapsed > 0)
        {
            nps = (double)nodes / elapsed * 1000;
        }

        string moveType = "";
        if (completedDepth == 0)
        {
            moveType = MoveTypes.TimeoutFallback;
        }
        return (bestX, bestY, new SearchStats
        {
            DepthAchieved = completedDepth,
            NodesSearched = nodes,
            NodesPerSecond = nps,
            SearchScore = bestScore,
            TableHitRate = hitRate,
            AllocatedTimeMs = config.TimeLimitMs,
            ThreadCount = 1,
            MoveType = moveType,
        });
    }
}
