namespace Caro.Core.GameLogic;

/// <summary>
/// Centralized AI difficulty configuration
/// Defines all parameters for each difficulty level in one place
/// Integrates with pub-sub stats system for consistent behavior
/// </summary>
public sealed class AIDifficultyConfig
{
    /// <summary>
    /// Singleton instance
    /// </summary>
    private static readonly AIDifficultyConfig _instance = new();

    public static AIDifficultyConfig Instance => _instance;

    private AIDifficultyConfig() { }

    /// <summary>
    /// Get configuration for a specific difficulty level
    /// </summary>
    public AIDifficultySettings GetSettings(AIDifficulty difficulty)
    {
        return difficulty switch
        {
            AIDifficulty.Braindead => new AIDifficultySettings
            {
                Difficulty = AIDifficulty.Braindead,
                DisplayName = "Braindead",
                ThreadCount = 1,
                PonderingThreadCount = 0,
                TimeMultiplier = 0.05,         // 5% of allocated time
                TimeBudgetPercent = 0.05,     // 5% time budget
                ParallelSearchEnabled = false,
                PonderingEnabled = false,
                VCFEnabled = false,
                ErrorRate = 0.10,              // 10% error rate
                MinDepth = 1,
                TargetNps = 10_000,
                Description = "10% error rate, beginners"
            },

            AIDifficulty.Easy => new AIDifficultySettings
            {
                Difficulty = AIDifficulty.Easy,
                DisplayName = "Easy",
                ThreadCount = 2,
                PonderingThreadCount = 1,
                TimeMultiplier = 0.20,         // 20% of allocated time
                TimeBudgetPercent = 0.20,     // 20% time budget
                ParallelSearchEnabled = true,
                PonderingEnabled = false,
                VCFEnabled = false,
                ErrorRate = 0.0,                // No intentional errors
                MinDepth = 2,
                TargetNps = 50_000,
                Description = "Parallel search from Easy"
            },

            AIDifficulty.Medium => new AIDifficultySettings
            {
                Difficulty = AIDifficulty.Medium,
                DisplayName = "Medium",
                ThreadCount = 3,
                PonderingThreadCount = 2,
                TimeMultiplier = 0.50,         // 50% of allocated time
                TimeBudgetPercent = 0.50,     // 50% time budget
                ParallelSearchEnabled = true,
                PonderingEnabled = true,
                VCFEnabled = false,
                ErrorRate = 0.0,                // No intentional errors
                MinDepth = 3,
                TargetNps = 100_000,
                Description = "Parallel + pondering"
            },

            AIDifficulty.Hard => new AIDifficultySettings
            {
                Difficulty = AIDifficulty.Hard,
                DisplayName = "Hard",
                ThreadCount = 4,
                PonderingThreadCount = 3,
                TimeMultiplier = 0.75,         // 75% of allocated time
                TimeBudgetPercent = 0.75,     // 75% time budget
                ParallelSearchEnabled = true,
                PonderingEnabled = true,
                VCFEnabled = true,
                ErrorRate = 0.0,                // No intentional errors
                MinDepth = 4,
                TargetNps = 200_000,
                Description = "Parallel + pondering + VCF"
            },

            AIDifficulty.Grandmaster => new AIDifficultySettings
            {
                Difficulty = AIDifficulty.Grandmaster,
                DisplayName = "Grandmaster",
                ThreadCount = GetGrandmasterThreadCount(),
                PonderingThreadCount = GetGrandmasterPonderThreadCount(),
                TimeMultiplier = 1.0,          // 100% of allocated time
                TimeBudgetPercent = 1.0,     // 100% time budget
                ParallelSearchEnabled = true,
                PonderingEnabled = true,
                VCFEnabled = true,
                ErrorRate = 0.0,                // No intentional errors
                MinDepth = 5,
                TargetNps = 500_000,
                Description = "Max parallel, VCF, pondering"
            },

            _ => throw new ArgumentException($"Unknown difficulty: {difficulty}")
        };
    }

    /// <summary>
    /// Get grandmaster thread count using (processorCount/2)-1 formula
    /// This is calculated dynamically to adapt to host machine
    /// </summary>
    private static int GetGrandmasterThreadCount()
    {
        int processorCount = Environment.ProcessorCount;
        return Math.Max(4, (processorCount / 2) - 1);
    }

    /// <summary>
    /// Get grandmaster pondering thread count
    /// Uses half of main search threads to avoid system issues
    /// </summary>
    private static int GetGrandmasterPonderThreadCount()
    {
        return Math.Max(2, GetGrandmasterThreadCount() / 2);
    }
}

/// <summary>
/// Complete configuration for a single AI difficulty level
/// </summary>
public sealed record AIDifficultySettings
{
    public required AIDifficulty Difficulty { get; init; }
    public required string DisplayName { get; init; }
    public required int ThreadCount { get; init; }
    public required int PonderingThreadCount { get; init; }
    public required double TimeMultiplier { get; init; }
    public required double TimeBudgetPercent { get; init; }
    public required bool ParallelSearchEnabled { get; init; }
    public required bool PonderingEnabled { get; init; }
    public required bool VCFEnabled { get; init; }
    public required double ErrorRate { get; init; }
    public required int MinDepth { get; init; }
    public required long TargetNps { get; init; }
    public required string Description { get; init; }

    /// <summary>
    /// Check if this difficulty supports pondering (Medium+)
    /// </summary>
    public bool SupportsPondering => PonderingEnabled && Difficulty >= AIDifficulty.Medium;

    /// <summary>
    /// Check if this difficulty supports parallel search (Easy, Hard, Grandmaster)
    /// Note: Medium uses single-threaded per current config
    /// </summary>
    public bool SupportsParallelSearch => ParallelSearchEnabled;

    /// <summary>
    /// Check if this difficulty supports VCF (Grandmaster only)
    /// </summary>
    public bool SupportsVCF => VCFEnabled;
}
