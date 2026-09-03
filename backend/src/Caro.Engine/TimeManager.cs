using Caro.Domain;

namespace Caro.Engine;

public readonly record struct TimeAllocation(long SoftBoundMs, long HardBoundMs, long OptimalMs);

public static class TimeManager
{
    public static TimeAllocation AllocateTime(long timeRemainingMs, long incrementMs, int moveNumber)
    {
        double phaseDivisor = Constants.TimeManagement.PhaseDivisorEarly;
        if (moveNumber > Constants.TimeManagement.PhaseSwitchMove)
        {
            phaseDivisor = Constants.TimeManagement.PhaseDivisorLate;
        }

        double baseMs = timeRemainingMs / phaseDivisor;
        double incContrib = incrementMs * Constants.TimeManagement.IncContribFactor;

        long optimal = (long)(baseMs + incContrib);
        if (optimal < Constants.TimeManagement.MinOptimalMs)
        {
            optimal = Constants.TimeManagement.MinOptimalMs;
        }

        long maxTime = (long)(timeRemainingMs * Constants.TimeManagement.MaxFraction);
        if (optimal > maxTime)
        {
            optimal = maxTime;
        }

        long hardBound = (long)(optimal * Constants.TimeManagement.HardBoundMultiplier);
        long buffer = (long)(timeRemainingMs * Constants.TimeManagement.BufferFraction);
        if (buffer < Constants.TimeManagement.MinBufferMs)
        {
            buffer = Constants.TimeManagement.MinBufferMs;
        }
        hardBound += buffer;
        if (hardBound > timeRemainingMs - Constants.TimeManagement.ReserveMs)
        {
            hardBound = timeRemainingMs - Constants.TimeManagement.ReserveMs;
        }
        // Any live clock still deserves a usable budget: a zero or negative
        // hard bound makes the search abort instantly and fall back to move
        // ordering.
        if (timeRemainingMs > 0 && hardBound < Constants.TimeManagement.MinBufferMs)
        {
            hardBound = Constants.TimeManagement.MinBufferMs;
        }
        if (hardBound < 0)
        {
            hardBound = 0;
        }

        long softBound = (long)(optimal * Constants.TimeManagement.SoftBoundFraction);

        return new TimeAllocation(softBound, hardBound, optimal);
    }
}
