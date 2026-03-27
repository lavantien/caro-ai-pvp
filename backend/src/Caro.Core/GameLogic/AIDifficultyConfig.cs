namespace Caro.Core.GameLogic;

/// <summary>
/// Centralized AI difficulty configuration
/// Defines all parameters for each difficulty level in one place
/// Integrates with pub-sub stats system for consistent behavior
///
/// Design principle: All depth/speed is determined by machine capability and time allotted.
/// No hardcoded targets - the AI naturally reaches whatever depth it can within its time budget.
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
                ErrorRate = 0.10,              // 10% error rate per README.md spec
                Description = "10% error rate, absolute beginners"
            },

            AIDifficulty.Easy => new AIDifficultySettings
            {
                Difficulty = AIDifficulty.Easy,
                DisplayName = "Easy",
                ThreadCount = GetEasyThreadCount(),        // (n/5)-1 threads
                PonderingThreadCount = GetEasyThreadCount(), // Same as active threads
                TimeMultiplier = 0.20,         // 20% of allocated time (per README)
                TimeBudgetPercent = 0.20,     // 20% time budget
                ParallelSearchEnabled = true,
                PonderingEnabled = true,       // Enabled since using multiple threads
                VCFEnabled = false,
                ErrorRate = 0.0,                // No intentional errors
                Description = "Parallel search + pondering"
            },

            AIDifficulty.Medium => new AIDifficultySettings
            {
                Difficulty = AIDifficulty.Medium,
                DisplayName = "Medium",
                ThreadCount = GetMediumThreadCount(),      // (n/4)-1 threads
                PonderingThreadCount = GetMediumThreadCount(), // Same as active threads
                TimeMultiplier = 0.50,         // 50% of allocated time
                TimeBudgetPercent = 0.50,     // 50% time budget
                ParallelSearchEnabled = true,
                PonderingEnabled = true,
                VCFEnabled = false,
                ErrorRate = 0.0,                // No intentional errors
                Description = "Parallel + pondering"
            },

            AIDifficulty.Hard => new AIDifficultySettings
            {
                Difficulty = AIDifficulty.Hard,
                DisplayName = "Hard",
                ThreadCount = GetHardThreadCount(),        // (n/3)-1 threads
                PonderingThreadCount = GetHardThreadCount(), // Same as active threads
                TimeMultiplier = 0.75,         // 75% of allocated time
                TimeBudgetPercent = 0.75,     // 75% time budget
                ParallelSearchEnabled = true,
                PonderingEnabled = true,
                VCFEnabled = false,            // DISABLED: VCF allocates heavily (HashSet, List, LINQ) causing NPS collapse
                ErrorRate = 0.0,                // No intentional errors
                Description = "Parallel + pondering (VCF disabled until zero-allocation)"
            },

            AIDifficulty.Grandmaster => new AIDifficultySettings
            {
                Difficulty = AIDifficulty.Grandmaster,
                DisplayName = "Grandmaster",
                ThreadCount = GetGrandmasterThreadCount(),    // max(5,(N/2)-1) per README
                PonderingThreadCount = GetGrandmasterPonderThreadCount(),
                TimeMultiplier = 1.0,          // 100% of allocated time
                TimeBudgetPercent = 1.0,     // 100% time budget
                ParallelSearchEnabled = true,    // Max parallel per README
                PonderingEnabled = true,         // Pondering per README
                VCFEnabled = false,              // DISABLED: VCF allocates heavily (HashSet, List, LINQ) causing NPS collapse
                ErrorRate = 0.0,                // No intentional errors
                Description = "Max parallel + pondering (VCF disabled until zero-allocation)"
            },

            AIDifficulty.Experimental => new AIDifficultySettings
            {
                Difficulty = AIDifficulty.Experimental,
                DisplayName = "Experimental",
                ThreadCount = GetGrandmasterThreadCount(),
                PonderingThreadCount = GetGrandmasterPonderThreadCount(),
                TimeMultiplier = 1.0,          // 100% of allocated time
                TimeBudgetPercent = 1.0,     // 100% time budget
                ParallelSearchEnabled = true,
                PonderingEnabled = true,
                VCFEnabled = true,
                ErrorRate = 0.0,                // No intentional errors
                Description = "Full features for testing"
            }
        };
    }

    /// <summary>
    /// Get Easy thread count using (processorCount/5)-1 formula
    /// Minimum 2 threads to ensure parallel search works
    /// </summary>
    private static int GetEasyThreadCount()
    {
        int processorCount = Environment.ProcessorCount;
        return Math.Max(2, (processorCount / 5) - 1);
    }

    /// <summary>
    /// Get Medium thread count using (processorCount/4)-1 formula
    /// Minimum 3 threads to ensure more than Easy
    /// </summary>
    private static int GetMediumThreadCount()
    {
        int processorCount = Environment.ProcessorCount;
        return Math.Max(3, (processorCount / 4) - 1);
    }

    /// <summary>
    /// Get Hard thread count using (processorCount/3)-1 formula
    /// Minimum 4 threads to ensure more than Medium
    /// </summary>
    private static int GetHardThreadCount()
    {
        int processorCount = Environment.ProcessorCount;
        return Math.Max(4, (processorCount / 3) - 1);
    }

    /// <summary>
    /// Get grandmaster thread count using (processorCount/2)-1 formula
    /// This is calculated dynamically to adapt to host machine
    /// CRITICAL FIX: Ensure Grandmaster always has MORE threads than Hard (4)
    /// Formula: max(Hard+1, (processorCount/2)-1) ensures GM >= 5 threads
    /// </summary>
    private static int GetGrandmasterThreadCount()
    {
        // Ensure Grandmaster has more threads than Hard (which uses (n/3)-1, minimum 4)
        // Formula: max(5, (processorCount/2)-1) ensures at least 5 threads
        return Math.Max(5, (Environment.ProcessorCount / 2) - 1);
    }

    /// <summary>
    /// Get grandmaster pondering thread count
    /// Uses same thread count as main search for consistent performance
    /// </summary>
    private static int GetGrandmasterPonderThreadCount()
    {
        // Same as main search - no reason to use fewer threads for pondering
        return GetGrandmasterThreadCount();
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
    public required string Description { get; init; }

    /// <summary>
    /// Check if this difficulty supports pondering (Easy+)
    /// </summary>
    public bool SupportsPondering => PonderingEnabled && Difficulty >= AIDifficulty.Easy;

    /// <summary>
    /// Check if this difficulty supports parallel search (Easy+)
    /// </summary>
    public bool SupportsParallelSearch => ParallelSearchEnabled;

    /// <summary>
    /// Check if this difficulty supports VCF (Hard and above)
    /// </summary>
    public bool SupportsVCF => VCFEnabled;
}
