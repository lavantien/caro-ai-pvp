using Caro.Domain;

namespace Caro.Engine;

public readonly record struct TimeAllocation(long SoftBoundMs, long HardBoundMs, long OptimalMs);

public static class TimeManager
{
    public static TimeAllocation AllocateTime(long timeRemainingMs, long incrementMs, int moveNumber)
    {
        double phaseDivisor = Constants.TimePhaseDivisorEarly;
        if (moveNumber > Constants.TimePhaseSwitchMove)
        {
            phaseDivisor = Constants.TimePhaseDivisorLate;
        }

        double baseMs = timeRemainingMs / phaseDivisor;
        double incContrib = incrementMs * Constants.TimeIncContribFactor;

        long optimal = (long)(baseMs + incContrib);
        if (optimal < Constants.TimeMinOptimalMs)
        {
            optimal = Constants.TimeMinOptimalMs;
        }

        long maxTime = (long)(timeRemainingMs * Constants.TimeMaxFraction);
        if (optimal > maxTime)
        {
            optimal = maxTime;
        }

        long hardBound = (long)(optimal * Constants.TimeHardBoundMultiplier);
        long buffer = (long)(timeRemainingMs * Constants.TimeBufferFraction);
        if (buffer < Constants.TimeMinBufferMs)
        {
            buffer = Constants.TimeMinBufferMs;
        }
        hardBound += buffer;
        if (hardBound > timeRemainingMs - Constants.TimeReserveMs)
        {
            hardBound = timeRemainingMs - Constants.TimeReserveMs;
        }
        // Any live clock still deserves a usable budget: a zero or negative
        // hard bound makes the search abort instantly and fall back to move
        // ordering.
        if (timeRemainingMs > 0 && hardBound < Constants.TimeMinBufferMs)
        {
            hardBound = Constants.TimeMinBufferMs;
        }
        if (hardBound < 0)
        {
            hardBound = 0;
        }

        long softBound = (long)(optimal * Constants.TimeSoftBoundFraction);

        return new TimeAllocation(softBound, hardBound, optimal);
    }
}
