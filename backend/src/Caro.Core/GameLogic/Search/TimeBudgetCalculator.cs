using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic.TimeManagement;

namespace Caro.Core.GameLogic.Search;

/// <summary>
/// Stateless time budget calculations for search, VCF, and pondering.
/// Pure functions that compute time limits based on game allocation.
/// </summary>
public static class TimeBudgetCalculator
{
    /// <summary>
    /// Calculate appropriate time limit for VCF search based on time allocation.
    /// </summary>
    public static (int timeLimitMs, int maxDepth) CalculateVCFTimeLimit(TimeAllocation timeAlloc)
    {
        if (timeAlloc.IsEmergency)
            return (50, 15);
        var baseVcfTime = Math.Max(50, timeAlloc.SoftBoundMs / 10);
        var vcfTime = (int)(baseVcfTime * 2.5);
        var maxDepth = 40;
        var finalVcfTime = (int)Math.Clamp(vcfTime, 50, 2000);
        return (finalVcfTime, maxDepth);
    }

    /// <summary>
    /// Calculate pondering time based on remaining time.
    /// Pondering uses a portion of the opponent's thinking time.
    /// </summary>
    public static long CalculatePonderTime(long? timeRemainingMs)
    {
        var baseTimeMs = timeRemainingMs ?? 5000;
        return baseTimeMs / 2;
    }

    /// <summary>
    /// Get default time allocation when no time limit is specified.
    /// </summary>
    public static TimeAllocation GetDefaultTimeAllocation() => new()
    {
        SoftBoundMs = 5000,
        HardBoundMs = 20000,
        OptimalTimeMs = 4000,
        IsEmergency = false
    };
}
