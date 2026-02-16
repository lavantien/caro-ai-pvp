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
                ErrorRate = 0.10,              // 10% error rate per README.md spec
                MinDepth = 1,
                TargetNps = 10_000,
                Description = "10% error rate, absolute beginners",
                OpeningBookEnabled = false,     // No opening book for beginner level
                MaxBookDepth = 0               // No opening book
            },

            AIDifficulty.Easy => new AIDifficultySettings
            {
                Difficulty = AIDifficulty.Easy,
                DisplayName = "Easy",
                ThreadCount = GetEasyThreadCount(),        // (n/5)-1 threads
                PonderingThreadCount = 1,
                TimeMultiplier = 0.20,         // 20% of allocated time (per README)
                TimeBudgetPercent = 0.20,     // 20% time budget
                ParallelSearchEnabled = true,
                PonderingEnabled = false,
                VCFEnabled = false,
                ErrorRate = 0.0,                // No intentional errors
                MinDepth = 2,                   // Natural depth from time budget
                TargetNps = 50_000,
                Description = "Parallel search from Easy",
                OpeningBookEnabled = true,      // Easy uses opening book (4 plies)
                MaxBookDepth = 4               // 4 plies = 2 moves per side
            },

            AIDifficulty.Medium => new AIDifficultySettings
            {
                Difficulty = AIDifficulty.Medium,
                DisplayName = "Medium",
                ThreadCount = GetMediumThreadCount(),      // (n/4)-1 threads
                PonderingThreadCount = 2,
                TimeMultiplier = 0.50,         // 50% of allocated time
                TimeBudgetPercent = 0.50,     // 50% time budget
                ParallelSearchEnabled = true,
                PonderingEnabled = true,
                VCFEnabled = false,
                ErrorRate = 0.0,                // No intentional errors
                MinDepth = 3,
                TargetNps = 100_000,
                Description = "Parallel + pondering",
                OpeningBookEnabled = true,      // Medium uses opening book (6 plies)
                MaxBookDepth = 6               // 6 plies = 3 moves per side
            },

            AIDifficulty.Hard => new AIDifficultySettings
            {
                Difficulty = AIDifficulty.Hard,
                DisplayName = "Hard",
                ThreadCount = GetHardThreadCount(),        // (n/3)-1 threads
                PonderingThreadCount = 3,
                TimeMultiplier = 0.75,         // 75% of allocated time
                TimeBudgetPercent = 0.75,     // 75% time budget
                ParallelSearchEnabled = true,
                PonderingEnabled = true,
                VCFEnabled = true,
                ErrorRate = 0.0,                // No intentional errors
                MinDepth = 4,
                TargetNps = 200_000,
                Description = "Parallel + pondering + VCF",
                OpeningBookEnabled = true,      // Hard uses opening book
                MaxBookDepth = 10              // 10 plies = 5 moves per side
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
                Description = "Max parallel, VCF, pondering",
                OpeningBookEnabled = true,      // Grandmaster uses opening book
                MaxBookDepth = 14              // 14 plies = 7 moves per side
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
                MinDepth = 5,
                TargetNps = 500_000,
                Description = "Full opening book + max features for testing",
                OpeningBookEnabled = true,      // Experimental uses full opening book
                MaxBookDepth = int.MaxValue    // Experimental uses all available book depth
            },

            AIDifficulty.BookGeneration => new AIDifficultySettings
            {
                Difficulty = AIDifficulty.BookGeneration,
                DisplayName = "BookGeneration",
                ThreadCount = GetBookGenerationThreadCount(),
                PonderingThreadCount = 0,
                TimeMultiplier = 1.0,
                TimeBudgetPercent = 1.0,
                ParallelSearchEnabled = true,    // Enable parallel search for maximum throughput
                PonderingEnabled = false,
                VCFEnabled = true,
                ErrorRate = 0.0,
                MinDepth = 12,
                TargetNps = 1_000_000,
                Description = "Offline book generation with (N-4) threads",
                OpeningBookEnabled = true,
                MaxBookDepth = int.MaxValue
            },

            _ => throw new ArgumentException($"Unknown difficulty: {difficulty}")
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
    /// Get book generation thread count for book generation.
    /// Uses fewer threads per search to allow more parallel searches.
    /// </summary>
    private static int GetBookGenerationThreadCount()
    {
        // With position-level parallelism (4 outer workers) and sequential candidates,
        // each search can use more threads for better core utilization
        // 4 workers x 5 threads per search = 20 threads (optimal for 20-core machine)
        return Math.Max(5, Environment.ProcessorCount / 4);
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
    /// Uses half of main search threads to avoid system issues
    /// CRITICAL FIX: Ensure at least 3 pondering threads (more than Hard's 3)
    /// </summary>
    private static int GetGrandmasterPonderThreadCount()
    {
        // Minimum 3 threads to ensure GM pondering is at least as strong as Hard's
        return Math.Max(3, GetGrandmasterThreadCount() / 2);
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
    public required bool OpeningBookEnabled { get; init; }  // Whether this difficulty uses opening book
    public required int MaxBookDepth { get; init; }  // Maximum book depth in plies (0 = no book)

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
    /// Check if this difficulty supports VCF (Hard and above)
    /// </summary>
    public bool SupportsVCF => VCFEnabled;

    /// <summary>
    /// Check if this difficulty uses the opening book (Easy, Medium, Hard, Grandmaster, Experimental)
    /// </summary>
    public bool SupportsOpeningBook => OpeningBookEnabled;
}
