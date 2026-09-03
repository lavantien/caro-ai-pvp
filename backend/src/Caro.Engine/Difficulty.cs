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
    public static DifficultyProfile GetDifficultyProfile(int level) => GetDifficultyProfile(level, null);

    /// <summary>
    /// Same mapping against a CaroConfig's (validated) profile overrides;
    /// a null config reads the compiled table.
    /// </summary>
    public static DifficultyProfile GetDifficultyProfile(int level, CaroConfig? config)
    {
        int n = Environment.ProcessorCount;
        int l5Threads = Pow2Floor((n - ReservedCores) / 2);

        if (config is not null
            && level >= Constants.Difficulty.MinLevel && level <= Constants.Difficulty.MaxLevel
            && config.DifficultyProfiles.TryGetValue(level, out DifficultyProfileOptions? o))
        {
            return new DifficultyProfile(o.Name, o.TimeFraction, o.MaxDepth, ThreadsFor(o.ThreadsMode, l5Threads),
                o.UseVCF, o.VCFDepth, o.Ponder, o.TTSizeMB);
        }

        Constants.DifficultyProfileData data = Constants.DifficultyProfiles.FirstOrDefault(p => p.Level == level);
        if (data.Level < Constants.Difficulty.MinLevel)
        {
            data = Constants.DifficultyProfiles[^1];
        }
        return new DifficultyProfile(data.Name, data.TimeFraction, data.MaxDepth, ThreadsFor(data.Threads.ToString(), l5Threads),
            data.UseVCF, data.VCFDepth, data.Ponder, data.TTSizeMB);
    }

    private static int ThreadsFor(string mode, int l5Threads) => mode switch
    {
        nameof(Constants.ProfileThreads.One) => 1,
        nameof(Constants.ProfileThreads.Two) => 2,
        nameof(Constants.ProfileThreads.HalfL5) => Math.Max(Pow2Floor(l5Threads / 2), 1),
        _ => Math.Max(l5Threads, 1),
    };

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
