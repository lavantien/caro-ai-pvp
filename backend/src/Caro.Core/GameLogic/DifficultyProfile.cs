namespace Caro.Core.GameLogic;

public static class DifficultyProfile
{
    public static string GetName(int level) => level switch
    {
        1 => "Novice",
        2 => "Beginner",
        3 => "Intermediate",
        4 => "Advanced",
        5 => "Grandmaster",
        _ => throw new ArgumentOutOfRangeException(nameof(level), $"Must be 1-5, got {level}")
    };

    /// <summary>
    /// Get thread count for a given difficulty level.
    /// All counts are capped at <see cref="ThreadPoolConfig.MaxEngineThreads"/>.
    /// </summary>
    public static int GetThreadCount(int level)
    {
        int cap = ThreadPoolConfig.MaxEngineThreads;
        return level switch
        {
            1 => 1,
            2 => 1,
            3 => Math.Min(2, cap),
            4 => Math.Min(Math.Max(2, Environment.ProcessorCount / 2), cap),
            5 => cap,
            _ => throw new ArgumentOutOfRangeException(nameof(level), $"Must be 1-5, got {level}")
        };
    }

    public static double GetTimeFraction(int level) => level switch
    {
        1 => 0.05,
        2 => 0.15,
        3 => 0.40,
        4 => 0.70,
        5 => 1.00,
        _ => throw new ArgumentOutOfRangeException(nameof(level), $"Must be 1-5, got {level}")
    };

    public static bool GetPonderingEnabled(int level) => level == 5;

    public static bool GetUseVCF(int level) => level >= 3;

    public static bool GetParallelSearchEnabled(int level) => level >= 3;
}
