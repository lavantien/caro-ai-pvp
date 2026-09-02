using System.Collections.Concurrent;
using Caro.Domain;

namespace Caro.Engine;

internal readonly struct ParallelResult(int x, int y, int score, int depth)
{
    public int X { get; } = x;
    public int Y { get; } = y;
    public int Score { get; } = score;
    public int Depth { get; } = depth;
}

public static class ParallelSearch
{
    // Odd workers start one ply deeper so iterations interleave and the
    // lazy SMP majority vote sees more distinct depths.
    private const int StartDepthStagger = 2;

    public static (int X, int Y, SearchStats Stats) Run(
        Board b,
        Player player,
        SearchConfig config,
        TranspositionTable tt,
        SearchHeuristics heuristics,
        CancellationToken ctx)
    {
        int numWorkers = config.Threads;
        if (numWorkers <= 1)
        {
            return SearchEngine.SearchPosition(b, player, config, tt, heuristics, ctx);
        }

        SearchBoard sb = new(b);
        List<Position> candidates = Candidates.GetCandidates(sb, Constants.MaxSearchRadius);
        candidates = Candidates.FilterOpenRule(candidates, sb, player);
        if (candidates.Count <= 1)
        {
            if (candidates.Count == 1)
            {
                return (candidates[0].X, candidates[0].Y, new SearchStats { ThreadCount = numWorkers });
            }
            return (-1, -1, new SearchStats { ThreadCount = numWorkers });
        }

        using TimeMonitor monitor = new(config.TimeLimitMs, ctx);

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
                        ThreadCount = numWorkers,
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

        tt.ResetStats();
        ConcurrentBag<ParallelResult> results = [];

        // Worker 0 evolves the shared heuristics so ordering knowledge
        // persists across moves; the other workers start from a pre-search
        // snapshot. The snapshots must be taken before any worker runs:
        // worker 0 writes while the clones are only read.
        SearchHeuristics[] workerHeuristics = new SearchHeuristics[numWorkers];
        workerHeuristics[0] = heuristics;
        for (int w = 1; w < numWorkers; w++)
        {
            workerHeuristics[w] = heuristics.Clone();
        }

        Task[] workers = new Task[numWorkers];
        for (int w = 0; w < numWorkers; w++)
        {
            int workerID = w;
            workers[w] = Task.Factory.StartNew(() =>
            {
                SearchBoard workerSB = new(b);
                SearchHeuristics workerH = workerHeuristics[workerID];

                int prevScore = -Constants.Infinity;
                int completedDepth = 0;
                int startDepth = 1 + workerID % StartDepthStagger;
                long lastIterMs = 0;
                long prevIterMs = 0;

                for (int depth = startDepth; depth <= config.MaxDepth; depth++)
                {
                    if (monitor.ShouldStop())
                    {
                        break;
                    }
                    // Do not start an iteration that cannot finish inside the
                    // soft budget; the hard bound stays as the emergency stop.
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
                    int a = -Constants.Infinity;
                    int bnd = Constants.Infinity;
                    if (completedDepth > 0)
                    {
                        a = Math.Max(prevScore - delta, -Constants.Infinity);
                        bnd = Math.Min(prevScore + delta, Constants.Infinity);
                    }

                    int x = 0;
                    int y = 0;
                    int score = 0;
                    bool found = false;
                    for (int attempt = 0; attempt < Constants.MaxAspirationAttempts; attempt++)
                    {
                        (x, y, score) = SearchEngine.SearchRoot(workerSB, player, depth, a, bnd, tt, workerH, candidates, monitor, vcfPreferred);
                        if (x < 0 || monitor.ShouldStop())
                        {
                            break;
                        }
                        if (score <= a && a > -Constants.Infinity)
                        {
                            a = Math.Max(a - delta, -Constants.Infinity);
                            delta *= 2;
                            continue;
                        }
                        if (score >= bnd && bnd < Constants.Infinity)
                        {
                            bnd = Math.Min(bnd + delta, Constants.Infinity);
                            delta *= 2;
                            continue;
                        }
                        found = true;
                        break;
                    }

                    if (!found && !monitor.ShouldStop())
                    {
                        (x, y, score) = SearchEngine.SearchRoot(workerSB, player, depth,
                            -Constants.Infinity, Constants.Infinity, tt, workerH, candidates, monitor, vcfPreferred);
                        if (x >= 0)
                        {
                            found = true;
                        }
                    }

                    if (!found)
                    {
                        break;
                    }

                    prevScore = score;
                    completedDepth = depth;
                    prevIterMs = lastIterMs;
                    lastIterMs = monitor.ElapsedMs() - iterStart;
                    results.Add(new ParallelResult(x, y, score, depth));

                    if (MateScore.IsForcedWinScore(score))
                    {
                        break;
                    }
                }
            }, CancellationToken.None, TaskCreationOptions.LongRunning, TaskScheduler.Default);
        }

        Task.WaitAll(workers, CancellationToken.None);

        int bestX = candidates[0].X;
        int bestY = candidates[0].Y;
        int bestScore = -Constants.Infinity;
        int bestDepth = 0;

        foreach (ParallelResult r in results)
        {
            if (r.Depth > bestDepth || (r.Depth == bestDepth && r.Score > bestScore))
            {
                bestScore = r.Score;
                bestX = r.X;
                bestY = r.Y;
                bestDepth = r.Depth;
            }
        }

        string moveType = "";
        if (bestDepth == 0)
        {
            // No worker finished a depth in time. Fall back to the
            // best-ordered move, never the raw scan-order candidate head.
            SearchBoard freshSB = new(b);
            List<Position> ordered = MoveOrdering.OrderMoves(candidates, freshSB, player, 0, null, heuristics);
            if (ordered.Count > 0)
            {
                bestX = ordered[0].X;
                bestY = ordered[0].Y;
            }
            bestScore = 0;
            moveType = MoveTypes.TimeoutFallback;
        }

        long elapsed = monitor.ElapsedMs();
        long nodes = monitor.NodesCount;
        (long probes, long hits) = tt.Stats();
        double nps = 0;
        if (elapsed > 0)
        {
            nps = (double)nodes / elapsed * 1000;
        }
        double ttHitRate = 0;
        if (probes > 0)
        {
            ttHitRate = (double)hits / probes;
        }

        return (bestX, bestY, new SearchStats
        {
            DepthAchieved = bestDepth,
            NodesSearched = nodes,
            NodesPerSecond = nps,
            SearchScore = bestScore,
            TableHitRate = ttHitRate,
            AllocatedTimeMs = config.TimeLimitMs,
            ThreadCount = numWorkers,
            MoveType = moveType,
        });
    }
}
