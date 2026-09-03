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
    // Cores kept away from the L4/L5 thread formulas for the host and the
    // API layer.
    private const int ReservedCores = 2;

    /// <summary>
    /// Maps a level onto the central profile table in
    /// Constants.DifficultyProfiles; levels outside MinLevel..MaxLevel play
    /// the top profile.
    /// </summary>
    public static DifficultyProfile GetDifficultyProfile(int level)
    {
        int n = Environment.ProcessorCount;
        int l5Threads = Pow2Floor((n - ReservedCores) / 2);

        Constants.DifficultyProfileData data = Constants.DifficultyProfiles[^1];
        foreach (Constants.DifficultyProfileData candidate in Constants.DifficultyProfiles)
        {
            if (candidate.Level == level)
            {
                data = candidate;
                break;
            }
        }

        int threads = data.Threads switch
        {
            Constants.ProfileThreads.One => 1,
            Constants.ProfileThreads.Two => 2,
            Constants.ProfileThreads.HalfL5 => Math.Max(Pow2Floor(l5Threads / 2), 1),
            _ => Math.Max(l5Threads, 1),
        };

        return new DifficultyProfile(data.Name, data.TimeFraction, data.MaxDepth, threads,
            data.UseVCF, data.VCFDepth, data.Ponder, data.TTSizeMB);
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
}
