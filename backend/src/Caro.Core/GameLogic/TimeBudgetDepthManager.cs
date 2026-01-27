using Caro.Core.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Dynamic depth manager based on time budget and machine capability.
/// Uses iterative deepening with Effective Branching Factor (EBF) tracking.
/// No hardcoded depths - scales automatically with server performance.
/// </summary>
public sealed class TimeBudgetDepthManager
{
    private readonly object _lock = new();

    // Track recent searches for EBF calculation
    private readonly CircularBuffer<int> _recentDepths = new(10);
    private readonly CircularBuffer<long> _recentNodes = new(10);

    // Estimated nodes per second (updated from actual searches)
    private double _estimatedNps = 100_000; // Conservative default
    private double _effectiveBranchingFactor = 2.5; // Alpha-beta with good move ordering

    // Minimum NPS for different difficulty tiers (for calibration)
    private const double MinNpsBraindead = 10_000;
    private const double MinNpsEasy = 50_000;
    private const double MinNpsMedium = 100_000;
    private const double MinNpsHard = 200_000;
    private const double MinNpsGrandmaster = 500_000;

    /// <summary>
    /// Get the time multiplier for a difficulty level.
    /// Higher difficulties use more of their allocated time.
    /// </summary>
    public static double GetTimeMultiplier(AIDifficulty difficulty)
    {
        return difficulty switch
        {
            AIDifficulty.Braindead => 0.01,   // Barely thinks
            AIDifficulty.Easy => 0.1,          // 10% of allocated time
            AIDifficulty.Medium => 0.3,        // 30% of allocated time
            AIDifficulty.Hard => 0.7,          // 70% of allocated time
            AIDifficulty.Grandmaster => 1.0,   // Full allocated time
            _ => 0.5
        };
    }

    /// <summary>
    /// Get the minimum depth for a difficulty.
    /// Even with zero time, AI should search at least this deep.
    /// </summary>
    public static int GetMinimumDepth(AIDifficulty difficulty)
    {
        return difficulty switch
        {
            AIDifficulty.Braindead => 1,
            AIDifficulty.Easy => 2,
            AIDifficulty.Medium => 3,
            AIDifficulty.Hard => 4,
            AIDifficulty.Grandmaster => 5,
            _ => 3
        };
    }

    /// <summary>
    /// Update NPS estimate from actual search performance.
    /// Called after each search completes.
    /// </summary>
    public void UpdateNpsEstimate(long nodesSearched, double elapsedSeconds)
    {
        if (elapsedSeconds <= 0 || nodesSearched <= 0)
            return;

        double actualNps = nodesSearched / elapsedSeconds;

        lock (_lock)
        {
            // Exponential moving average with 0.3 weight for new value
            _estimatedNps = _estimatedNps * 0.7 + actualNps * 0.3;
        }
    }

    /// <summary>
    /// Update EBF estimate from iterative deepening results.
    /// EBF = nodes(depth) / nodes(depth-1)
    /// </summary>
    public void UpdateEbfEstimate(long nodesAtDepth, long nodesAtPreviousDepth)
    {
        if (nodesAtPreviousDepth <= 0 || nodesAtDepth <= 0)
            return;

        double ebf = (double)nodesAtDepth / nodesAtPreviousDepth;

        lock (_lock)
        {
            // Clamp EBF to reasonable bounds
            ebf = Math.Clamp(ebf, 1.5, 5.0);

            // Exponential moving average
            _effectiveBranchingFactor = _effectiveBranchingFactor * 0.8 + ebf * 0.2;
        }
    }

    /// <summary>
    /// Get current NPS estimate.
    /// </summary>
    public double GetEstimatedNps()
    {
        lock (_lock)
        {
            return _estimatedNps;
        }
    }

    /// <summary>
    /// Get current EBF estimate.
    /// </summary>
    public double GetEstimatedEbf()
    {
        lock (_lock)
        {
            return _effectiveBranchingFactor;
        }
    }

    /// <summary>
    /// Calculate maximum sustainable depth for given time budget.
    /// Formula: max_depth = log(time * nps) / log(ebf)
    ///
    /// This ensures we never timeout while maximizing depth reached.
    /// </summary>
    public int CalculateMaxDepth(double timeSeconds, AIDifficulty difficulty)
    {
        if (timeSeconds <= 0.001)
            return GetMinimumDepth(difficulty);

        lock (_lock)
        {
            // Apply time multiplier - lower difficulties use less time
            double effectiveTime = timeSeconds * GetTimeMultiplier(difficulty);

            // Minimum time to ensure at least minimum depth
            effectiveTime = Math.Max(effectiveTime, 0.01);

            // Calculate max depth from formula
            // nodes = time * nps
            // nodes = ebf^depth
            // depth = log(nodes) / log(ebf)
            double totalNodes = effectiveTime * _estimatedNps;

            // Calculate depth with NO hardcoded safety factor
            // The time multiplier is the ONLY difficulty differentiator
            // Different machines will naturally achieve different depths based on their NPS
            double maxDepth = Math.Log(totalNodes) / Math.Log(_effectiveBranchingFactor);
            int calculatedDepth = Math.Max((int)maxDepth, 1);

            // Only minimum depth constraint is to ensure at least 1 ply was searched
            // All difficulty separation comes from time multiplier, not hardcoded depths
            int minDepth = 1;

            // Clamp to reasonable bounds (1-15)
            return Math.Clamp(calculatedDepth, minDepth, 15);
        }
    }

    /// <summary>
    /// Calculate if we should start another iteration.
    /// Returns true if we have time for at least one more ply at current EBF.
    /// </summary>
    public bool ShouldContinueIterating(double elapsedSeconds, double softBoundSeconds, int currentDepth)
    {
        // Must not exceed soft bound
        if (elapsedSeconds >= softBoundSeconds)
            return false;

        // Estimate time for next iteration: current_nodes * EBF
        double timeForNextIteration = elapsedSeconds * _effectiveBranchingFactor;
        double remainingTime = softBoundSeconds - elapsedSeconds;

        // Continue only if we have time for at least 80% of next iteration
        return remainingTime >= timeForNextIteration * 0.8;
    }

    /// <summary>
    /// Calibrate NPS based on difficulty tier.
    /// Called on startup to set reasonable baseline.
    /// </summary>
    public void CalibrateNpsForDifficulty(AIDifficulty difficulty)
    {
        double targetNps = difficulty switch
        {
            AIDifficulty.Braindead => MinNpsBraindead,
            AIDifficulty.Easy => MinNpsEasy,
            AIDifficulty.Medium => MinNpsMedium,
            AIDifficulty.Hard => MinNpsHard,
            AIDifficulty.Grandmaster => MinNpsGrandmaster,
            _ => 100_000
        };

        lock (_lock)
        {
            // Only update if current estimate is below target
            // This allows actual performance to exceed baseline
            if (_estimatedNps < targetNps)
            {
                _estimatedNps = targetNps;
            }
        }
    }

    /// <summary>
    /// Reset tracking state (call at start of each game).
    /// </summary>
    public void Reset()
    {
        lock (_lock)
        {
            _recentDepths.Clear();
            _recentNodes.Clear();
            // Keep NPS and EBF estimates across games - they're machine-specific
        }
    }
}

/// <summary>
/// Simple circular buffer for tracking recent values.
/// </summary>
internal sealed class CircularBuffer
{
    private readonly int[] _depthBuffer;
    private readonly long[] _nodeBuffer;
    private int _index = 0;
    private int _count = 0;

    public CircularBuffer(int capacity)
    {
        _depthBuffer = new int[capacity];
        _nodeBuffer = new long[capacity];
    }

    public void AddDepth(int depth)
    {
        _depthBuffer[_index] = depth;
        _count = Math.Min(_count + 1, _depthBuffer.Length);
        _index = (_index + 1) % _depthBuffer.Length;
    }

    public void AddNodes(long nodes)
    {
        _nodeBuffer[_index] = nodes;
    }

    public void Clear()
    {
        _index = 0;
        _count = 0;
        Array.Clear(_depthBuffer, 0, _depthBuffer.Length);
        Array.Clear(_nodeBuffer, 0, _nodeBuffer.Length);
    }
}

/// <summary>
/// Generic circular buffer for tracking recent values.
/// </summary>
internal sealed class CircularBuffer<T>
{
    private readonly T[] _buffer;
    private int _index = 0;
    private int _count = 0;

    public CircularBuffer(int capacity)
    {
        _buffer = new T[capacity];
    }

    public void Add(T item)
    {
        _buffer[_index] = item;
        _count = Math.Min(_count + 1, _buffer.Length);
        _index = (_index + 1) % _buffer.Length;
    }

    public void Clear()
    {
        _index = 0;
        _count = 0;
        Array.Clear(_buffer, 0, _buffer.Length);
    }

    public int Count => _count;
}
