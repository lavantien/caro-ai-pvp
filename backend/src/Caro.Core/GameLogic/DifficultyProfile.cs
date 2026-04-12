using System.Numerics;

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

    public static int GetThreadCount(int level)
    {
        int n = Environment.ProcessorCount;
        return level switch
        {
            1 => 1,
            2 => 1,
            3 => 2,
            4 => Math.Max(2, n / 2),
            5 => Math.Max(1, 1 << (int)BitOperations.Log2((uint)Math.Max(1, n - 2))),
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
