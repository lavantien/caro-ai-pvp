using Caro.Domain;

namespace Caro.Engine;

/// <summary>
/// Iteration-cost prediction keeps per-move spend inside the soft budget. An
/// iterative-deepening loop that only checks elapsed time when between depths
/// will start an iteration it cannot finish and then burn to the hard bound,
/// so nearly every move costs the hard bound instead of the soft target.
/// </summary>
internal static class IterationBudget
{
    /// <summary>
    /// Estimates how much costlier the next depth is than the last completed
    /// one. A warm TT can make an iteration cheaper, but predictions never
    /// assume shrinkage; one noisy re-search must not predict runaway.
    /// </summary>
    internal static double IterationGrowth(long lastMs, long prevMs)
    {
        if (lastMs <= 0 || prevMs <= 0)
        {
            return Constants.Iteration.GrowthDefault;
        }
        double ratio = (double)lastMs / prevMs;
        return Math.Min(Math.Max(ratio, Constants.Iteration.GrowthMin), Constants.Iteration.GrowthMax);
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
