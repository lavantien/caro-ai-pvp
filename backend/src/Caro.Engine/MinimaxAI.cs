using Caro.Domain;

namespace Caro.Engine;

public struct SearchOptions
{
    public long TimeRemainingMs { get; set; }
    public long IncrementMs { get; set; }
    public int MoveNumber { get; set; }
    public int ThreadCount { get; set; }
    public bool ParallelEnabled { get; set; }
    public double TimeFraction { get; set; }
    public bool UseVCF { get; set; }
    public int VCFMaxDepth { get; set; }
    public int MaxDepth { get; set; }
}

/// <summary>
/// The public engine facade. Owns the transposition table and the shared
/// heuristics; GetBestMove allocates the clock budget and picks the single
/// or parallel search path.
/// </summary>
public sealed partial class MinimaxAI : IDisposable
{
    private readonly TranspositionTable _tt;
    private readonly SearchHeuristics _heuristics;
    private readonly int _maxThreads;
    private SearchStats _stats;

    internal TranspositionTable TT => _tt;

    public MinimaxAI(int maxThreads, int ttSizeMB)
    {
        if (maxThreads < 1)
        {
            maxThreads = 1;
        }
        if (ttSizeMB < 1)
        {
            ttSizeMB = Constants.DefaultTTSizeMB;
        }
        _tt = new TranspositionTable(ttSizeMB);
        _heuristics = new SearchHeuristics();
        _maxThreads = maxThreads;
    }

    public (int X, int Y, SearchStats Stats) GetBestMove(
        Board b,
        Player player,
        SearchOptions opts,
        CancellationToken ctx)
    {
        // A ponder must never overlap the official search on the same AI.
        StopPonder();

        TimeAllocation timeAlloc = TimeManager.AllocateTime(opts.TimeRemainingMs, opts.IncrementMs, opts.MoveNumber);
        long hardBound = (long)(timeAlloc.HardBoundMs * opts.TimeFraction);
        if (hardBound < 0)
        {
            hardBound = 0;
        }
        long softBound = (long)(timeAlloc.SoftBoundMs * opts.TimeFraction);
        if (softBound < 0)
        {
            softBound = 0;
        }

        int maxDepth = opts.MaxDepth;
        if (maxDepth <= 0 || maxDepth > Constants.AbsoluteMaxDepth)
        {
            maxDepth = Constants.AbsoluteMaxDepth;
        }

        SearchConfig config = new()
        {
            MaxDepth = maxDepth,
            TimeLimitMs = hardBound,
            SoftLimitMs = softBound,
            Threads = Math.Min(opts.ThreadCount, _maxThreads),
            UseVCF = opts.UseVCF,
            VCFMaxDepth = opts.VCFMaxDepth,
            TimeFraction = opts.TimeFraction,
        };

        if (config.Threads < 1)
        {
            config.Threads = 1;
        }

        _heuristics.AgeForNewMove();
        _tt.IncrementAge();

        (int x, int y, SearchStats stats) = opts.ParallelEnabled && config.Threads > 1
            ? ParallelSearch.Run(b, player, config, _tt, _heuristics, ctx)
            : SearchEngine.SearchPosition(b, player, config, _tt, _heuristics, ctx);

        _stats = stats;
        return (x, y, stats);
    }

    public SearchStats GetStats() => _stats;

    public void Dispose()
    {
        // Join the ponder before freeing the table: a straggler search would
        // index the emptied shard arrays.
        StopPonder();
        _tt.Dispose();
        _heuristics.Clear();
    }
}
