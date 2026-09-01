namespace Caro.Engine;

/// <summary>
/// Iteration-cost prediction keeps per-move spend inside the soft budget. An
/// iterative-deepening loop that only checks elapsed time when between depths
/// will start an iteration it cannot finish and then burn to the hard bound,
/// so nearly every move costs the hard bound instead of the soft target.
/// </summary>
internal static class IterationBudget
{
    private const double IterGrowthMin = 1.5;
    private const double IterGrowthMax = 6.0;
    private const double IterGrowthDefault = 4.0;

    /// <summary>
    /// Estimates how much costlier the next depth is than the last completed
    /// one. A warm TT can make an iteration cheaper, but predictions never
    /// assume shrinkage; one noisy re-search must not predict runaway.
    /// </summary>
    internal static double IterationGrowth(long lastMs, long prevMs)
    {
        if (lastMs <= 0 || prevMs <= 0)
        {
            return IterGrowthDefault;
        }
        double ratio = (double)lastMs / prevMs;
        return Math.Min(Math.Max(ratio, IterGrowthMin), IterGrowthMax);
    }

    /// <summary>
    /// Reports whether starting another depth is predicted to finish inside
    /// the soft budget. softMs <= 0 disables the gate (hard bound still
    /// applies through the TimeMonitor).
    /// </summary>
    internal static bool NextIterationFits(long elapsedMs, long lastIterMs, long prevIterMs, long softMs)
    {
        if (softMs <= 0 || lastIterMs <= 0)
        {
            return true;
        }
        long predicted = (long)(lastIterMs * IterationGrowth(lastIterMs, prevIterMs));
        return elapsedMs + predicted <= softMs;
    }
}
