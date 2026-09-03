using Caro.Domain;

namespace Caro.Engine;

public readonly record struct TimeAllocation(long SoftBoundMs, long HardBoundMs, long OptimalMs);

public static class TimeManager
{
    public static TimeAllocation AllocateTime(long timeRemainingMs, long incrementMs, int moveNumber,
        TimeManagementOptions? options = null)
    {
        TimeManagementOptions o = options ?? CaroConfig.Default.TimeManagement;

        double phaseDivisor = o.PhaseDivisorEarly;
        if (moveNumber > o.PhaseSwitchMove)
        {
            phaseDivisor = o.PhaseDivisorLate;
        }

        double baseMs = timeRemainingMs / phaseDivisor;
        double incContrib = incrementMs * o.IncContribFactor;

        long optimal = (long)(baseMs + incContrib);
        if (optimal < o.MinOptimalMs)
        {
            optimal = o.MinOptimalMs;
        }

        long maxTime = (long)(timeRemainingMs * o.MaxFraction);
        if (optimal > maxTime)
        {
            optimal = maxTime;
        }

        long hardBound = (long)(optimal * o.HardBoundMultiplier);
        long buffer = (long)(timeRemainingMs * o.BufferFraction);
        if (buffer < o.MinBufferMs)
        {
            buffer = o.MinBufferMs;
        }
        hardBound += buffer;
        if (hardBound > timeRemainingMs - o.ReserveMs)
        {
            hardBound = timeRemainingMs - o.ReserveMs;
        }
        // Any live clock still deserves a usable budget: a zero or negative
        // hard bound makes the search abort instantly and fall back to move
        // ordering.
        if (timeRemainingMs > 0 && hardBound < o.MinBufferMs)
        {
            hardBound = o.MinBufferMs;
        }
        if (hardBound < 0)
        {
            hardBound = 0;
        }

        long softBound = (long)(optimal * o.SoftBoundFraction);

        return new TimeAllocation(softBound, hardBound, optimal);
    }
}
