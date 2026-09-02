using Caro.Domain;

namespace Caro.Engine;

/// <summary>Shapes the background ponder search.</summary>
public struct PonderConfig
{
    public int Threads { get; set; }
    public int MaxDepth { get; set; }
    public bool UseVCF { get; set; }
    public int VCFDepth { get; set; }
    public long TimeCapMs { get; set; }
}

/// <summary>
/// The result of a ponder search: the position searched (bot to move), the
/// predicted reply that led to it, and the best move found there.
/// Completed reports whether at least one depth iteration finished. The
/// outcome is observability: the real move always comes from a fresh search
/// over the TT the ponder warmed.
/// </summary>
public struct PonderOutcome
{
    public Player Player { get; set; }
    public Position PredictedReply { get; set; }
    public ulong BoardHash { get; set; }
    public int BestX { get; set; }
    public int BestY { get; set; }
    public SearchStats Stats { get; set; }
    public bool Completed { get; set; }
}

public sealed partial class MinimaxAI
{
    private readonly object _ponderGate = new();
    private CancellationTokenSource? _ponderCts;
    private Task? _ponderTask;
    private PonderOutcome? _ponderOutcome;

    /// <summary>
    /// Reads the TT entry the previous search stored for the position b
    /// (opponent to move) and returns its best move as the predicted
    /// opponent reply. The stored move came from the search's filtered
    /// candidate list (open rule included), so legality is inherent; the
    /// depth and emptiness checks guard against zeroed, stale, or colliding
    /// entries.
    /// </summary>
    public (Position Reply, bool Ok) PredictReply(Board b)
    {
        if (!_tt.Lookup(b.Hash, out TTEntry entry) || entry.Depth == 0)
        {
            return (default, false);
        }
        Position p = new(entry.MoveX, entry.MoveY);
        if (!p.IsValid() || !b.IsEmptyAt(p.X, p.Y))
        {
            return (default, false);
        }
        return (p, true);
    }

    /// <summary>
    /// Launches a background search on b (player to move), the position
    /// reached after the bot's own move and the predicted reply were
    /// applied. It shares the AI's TT, uses its own heuristics, never bumps
    /// the TT age, and never touches the AI's stats. Returns false if a
    /// ponder is already running.
    /// </summary>
    public bool StartPonder(Board b, Player player, Position predictedReply, PonderConfig cfg) =>
        StartPonderWithContext(b, player, predictedReply, cfg, CancellationToken.None);

    internal bool StartPonderWithContext(Board b, Player player, Position predictedReply, PonderConfig cfg, CancellationToken externalToken)
    {
        CancellationTokenSource cts = CancellationTokenSource.CreateLinkedTokenSource(externalToken);
        lock (_ponderGate)
        {
            if (_ponderTask is { IsCompleted: false })
            {
                cts.Dispose();
                return false;
            }
            _ponderCts = cts;
            _ponderOutcome = null;
            CancellationToken token = cts.Token;
            // CancellationToken.None on StartNew: a task created with an
            // already-cancelled token would transition to Canceled and make
            // the StopPonder join throw; the loop polls the token itself.
            _ponderTask = Task.Factory.StartNew(() =>
            {
                PonderOutcome outcome = RunPonder(b, player, predictedReply, cfg, token);
                lock (_ponderGate)
                {
                    _ponderOutcome = outcome;
                }
            }, CancellationToken.None, TaskCreationOptions.LongRunning, TaskScheduler.Default);
        }
        return true;
    }

    /// <summary>
    /// Cancels and joins any running ponder and consumes its outcome exactly
    /// once. Returns the outcome if a ponder had been started, even when it
    /// already self-stopped at the time cap. Idempotent: a second call
    /// returns false.
    /// </summary>
    public (PonderOutcome Outcome, bool Ok) StopPonder()
    {
        CancellationTokenSource? cts;
        Task? task;
        lock (_ponderGate)
        {
            cts = _ponderCts;
            task = _ponderTask;
        }

        if (task == null)
        {
            return (default, false);
        }
        cts?.Cancel();
        task.Wait();

        PonderOutcome? outcome;
        lock (_ponderGate)
        {
            outcome = _ponderOutcome;
            _ponderCts = null;
            _ponderTask = null;
            _ponderOutcome = null;
        }

        if (outcome == null)
        {
            return (default, false);
        }
        return (outcome.Value, true);
    }

    /// <summary>
    /// Reports whether a ponder search is still running. A ponder that hit
    /// its time cap reports false even before the outcome is consumed.
    /// </summary>
    public bool PonderActive()
    {
        lock (_ponderGate)
        {
            return _ponderTask is { IsCompleted: false };
        }
    }

    private PonderOutcome RunPonder(Board b, Player player, Position predictedReply, PonderConfig cfg, CancellationToken ctx)
    {
        int maxDepth = cfg.MaxDepth;
        if (maxDepth <= 0 || maxDepth > Constants.AbsoluteMaxDepth)
        {
            maxDepth = Constants.AbsoluteMaxDepth;
        }
        int threads = Math.Min(cfg.Threads, _maxThreads);
        if (threads < 1)
        {
            threads = 1;
        }

        // SoftLimitMs 0 disables the soft budget: ponder has no clock
        // pressure, so the ID loop runs until the cap or MaxDepth.
        (int x, int y, SearchStats stats) = ParallelSearch.Run(b, player, new SearchConfig
        {
            MaxDepth = maxDepth,
            TimeLimitMs = cfg.TimeCapMs,
            SoftLimitMs = 0,
            Threads = threads,
            UseVCF = cfg.UseVCF,
            VCFMaxDepth = cfg.VCFDepth,
        }, _tt, new SearchHeuristics(), ctx);

        return new PonderOutcome
        {
            Player = player,
            PredictedReply = predictedReply,
            BoardHash = b.Hash,
            BestX = x,
            BestY = y,
            Stats = stats,
            Completed = PonderCompleted(stats),
        };
    }

    internal static bool PonderCompleted(SearchStats stats)
    {
        // VCF results report DepthAchieved 0 but are solver-verified wins.
        return stats.DepthAchieved >= Constants.PonderMinCompletedDepth || stats.MoveType == MoveTypes.Vcf;
    }
}
