using System.Threading;

namespace Caro.Core.GameLogic;

/// <summary>
/// Thread pool configuration for CPU-bound parallel search
/// Optimizes thread pool settings for recursive minimax/alpha-beta search
/// </summary>
public static class ThreadPoolConfig
{
    private static bool _configured;

    /// <summary>
    /// Configure thread pool for CPU-bound search workload
    /// Should be called once at application startup
    /// </summary>
    public static void ConfigureForSearch()
    {
        if (_configured) return;

        // Get processor count for optimal configuration
        int processorCount = Environment.ProcessorCount;
        if (processorCount < 1) processorCount = 2;

        // Set minimum threads to processor count
        // This prevents thread pool from delaying thread creation
        // For CPU-bound work, we want threads available immediately
        ThreadPool.SetMinThreads(processorCount, processorCount);

        // Set maximum threads to prevent excessive thread creation
        // For recursive search, more threads than CPUs is not beneficial
        ThreadPool.SetMaxThreads(processorCount * 2, processorCount * 2);

        _configured = true;
    }

    /// <summary>
    /// Get the optimal number of threads for parallel search
    /// Returns processor count - 1 to leave one core for system/UI
    /// </summary>
    public static int GetOptimalThreadCount()
    {
        int processorCount = Environment.ProcessorCount;
        return Math.Max(1, processorCount - 1);
    }

    /// <summary>
    /// Get the optimal degree of parallelism for Parallel.For/ForEach
    /// </summary>
    public static int GetMaxDegreeOfParallelism()
    {
        return GetOptimalThreadCount();
    }

    /// <summary>
    /// Check if thread pool has been configured
    /// </summary>
    public static bool IsConfigured => _configured;

    /// <summary>
    /// Get conservative thread count for Lazy SMP parallel search
    /// Uses (processorCount / 2) - 1 to avoid hyperthreading contention
    /// while leaving headroom for the main thread
    /// Example: 8 cores -> (8/2)-1 = 3 helper threads
    /// </summary>
    public static int GetLazySMPThreadCount()
    {
        int processorCount = Environment.ProcessorCount;
        // Formula: (total threads/2) - 1
        // Minimum 1 thread, maximum processorCount - 2
        int halfCount = processorCount / 2;
        return Math.Max(1, halfCount - 1);
    }

    /// <summary>
    /// Get thread count for pondering (background search during opponent's turn)
    /// Uses fewer threads than main search to avoid system responsiveness issues
    /// </summary>
    public static int GetPonderingThreadCount()
    {
        // Pondering uses even fewer threads to avoid impacting system responsiveness
        // and to leave resources for the main search when it starts
        int processorCount = Environment.ProcessorCount;
        return Math.Max(1, processorCount / 4);
    }
}
