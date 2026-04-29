using System.Numerics;
using System.Threading;

namespace Caro.Core.GameLogic;

/// <summary>
/// Centralized thread pool configuration for CPU-bound parallel search.
/// All thread count decisions flow from <see cref="MaxEngineThreads"/>.
/// Optimizes thread pool settings for recursive minimax/alpha-beta search.
/// </summary>
public static class ThreadPoolConfig
{
    private static bool _configured;

    /// <summary>
    /// Maximum threads for both engine search and pondering.
    /// Largest power of 2 that does not exceed ProcessorCount / 2.
    /// Leaves half the cores for OS, GC, SignalR, and other host work.
    ///
    /// Formula: <c>1 &lt;&lt; (63 - LeadingZeroCount(total/2))</c>
    /// Examples: 20 cores → 8 threads, 16 cores → 8 threads, 8 cores → 4 threads
    /// </summary>
    public static int MaxEngineThreads
    {
        get
        {
            int total = Environment.ProcessorCount;
            if (total <= 2) return 1;
            return (int)(1UL << (63 - BitOperations.LeadingZeroCount((ulong)(total / 2))));
        }
    }

    /// <summary>
    /// Configure thread pool for CPU-bound search workload.
    /// Should be called once at application startup.
    /// </summary>
    public static void ConfigureForSearch()
    {
        if (_configured) return;

        int processorCount = Environment.ProcessorCount;
        if (processorCount < 1) processorCount = 2;

        ThreadPool.SetMinThreads(processorCount, processorCount);
        ThreadPool.SetMaxThreads(processorCount * 2, processorCount * 2);

        _configured = true;
    }

    /// <summary>
    /// Get the optimal number of threads for parallel search.
    /// Returns processor count - 1 to leave one core for system/UI.
    /// </summary>
    public static int GetOptimalThreadCount()
    {
        return Math.Max(1, Environment.ProcessorCount - 1);
    }

    /// <summary>
    /// Get the optimal degree of parallelism for Parallel.For/ForEach.
    /// </summary>
    public static int GetMaxDegreeOfParallelism() => GetOptimalThreadCount();

    /// <summary>
    /// Check if thread pool has been configured.
    /// </summary>
    public static bool IsConfigured => _configured;

    /// <summary>
    /// Get thread count for Lazy SMP parallel search.
    /// Returns <see cref="MaxEngineThreads"/>: the largest power of 2
    /// that does not exceed ProcessorCount / 2.
    /// Used by both main search and pondering.
    /// </summary>
    public static int GetLazySMPThreadCount() => MaxEngineThreads;

    /// <summary>
    /// Get the engine thread count scaled by active game count.
    /// Divides <see cref="MaxEngineThreads"/> across concurrent games
    /// so total thread usage stays bounded.
    /// </summary>
    public static int GetEngineThreadsForLoad(int activeGameCount)
        => Math.Max(1, MaxEngineThreads / Math.Max(1, activeGameCount));
}
