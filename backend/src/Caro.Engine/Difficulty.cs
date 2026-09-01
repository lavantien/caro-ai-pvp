using Caro.Domain;

namespace Caro.Engine;

public readonly record struct DifficultyProfile(
    string Name,
    double TimeFraction,
    int MaxDepth,
    int Threads,
    bool UseVCF,
    int VCFDepth,
    bool Ponder,
    int TTSizeMB);

public static class Difficulty
{
    /// <summary>
    /// Levels are strength-based first (depth caps, solver sight and
    /// parallel gating) and time-fraction scaled second, so L(k) is stronger
    /// than L(k-1) on any host. L3/L4 caps stay at or below 5: measured at
    /// bullet, ID depth past ~6 stops buying strength in self-play, so the
    /// ladder keeps those levels below the plateau and scales VCF sight
    /// instead.
    /// </summary>
    public static DifficultyProfile GetDifficultyProfile(int level)
    {
        int n = Environment.ProcessorCount;
        int l5Threads = Pow2Floor((n - 2) / 2);

        switch (level)
        {
            case 1:
                return new DifficultyProfile("Novice", 0.05, 2, 1, false, 0, false, 64);
            case 2:
                return new DifficultyProfile("Beginner", 0.15, 4, 1, false, 0, false, 64);
            case 3:
                return new DifficultyProfile("Intermediate", 0.40, 4, 2, true, 2, false, 256);
            case 4:
                int l4 = Pow2Floor(l5Threads / 2);
                if (l4 < 1)
                {
                    l4 = 1;
                }
                return new DifficultyProfile("Advanced", 0.70, 5, l4, true, 4, false, Constants.DefaultTTSizeMB);
            default:
                if (l5Threads < 1)
                {
                    l5Threads = 1;
                }
                return new DifficultyProfile("Grandmaster", 1.0, Constants.AbsoluteMaxDepth, l5Threads,
                    true, Constants.VCFSearchDepth, true, Constants.DefaultTTSizeMB);
        }
    }

    internal static int Pow2Floor(int n)
    {
        if (n <= 0)
        {
            return 1;
        }
        int p = 1;
        while (p * 2 <= n)
        {
            p *= 2;
        }
        return p;
    }

    public static int GetEngineThreadsForLoad(int activeGames)
    {
        if (activeGames <= 1)
        {
            return Environment.ProcessorCount;
        }
        return Environment.ProcessorCount / activeGames;
    }

    /// <summary>Bounds the table used when no difficulty level is set.</summary>
    public const int DefaultSessionTTSizeMB = 256;
}
