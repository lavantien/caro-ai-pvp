using Caro.Domain;
using Caro.Engine;
using Xunit;

namespace Caro.Engine.Tests;

public class DifficultyTests
{
    public static TheoryData<int, string, double, double, int, bool, int, int> ProfileCases => new()
    {
        { 1, "Novice", 0.04, 0.06, 1, false, 0, 2 },
        { 2, "Beginner", 0.14, 0.16, 1, false, 0, 4 },
        { 3, "Intermediate", 0.39, 0.41, 2, true, 2, 4 },
        { 4, "Advanced", 0.69, 0.71, 1, true, 4, 5 },
        { 5, "Grandmaster", 0.99, 1.01, 1, true, Constants.Vcf.SearchDepth, Constants.Search.AbsoluteMaxDepth },
    };

    [Theory]
    [MemberData(nameof(ProfileCases))]
    public void DifficultyProfileLevels(int level, string name, double minFraction, double maxFraction,
        int minThreads, bool useVCF, int vcfDepth, int maxDepth)
    {
        DifficultyProfile p = Difficulty.GetDifficultyProfile(level);
        Assert.Equal(name, p.Name);
        Assert.True(p.TimeFraction >= minFraction);
        Assert.True(p.TimeFraction <= maxFraction);
        Assert.True(p.Threads >= minThreads);
        Assert.Equal(useVCF, p.UseVCF);
        Assert.Equal(vcfDepth, p.VCFDepth);
        Assert.Equal(maxDepth, p.MaxDepth);
    }

    /// <summary>
    /// Measured at bullet (1+0, 20-game duels): ID depth beyond ~6 buys no
    /// wins in self-play, so L3/L4 keep their depth caps below the
    /// saturation plateau and separate from L2 by VCF sight and time
    /// fraction instead.
    /// </summary>
    [Fact]
    public void DifficultyLadderOrdersStrengthAxes()
    {
        DifficultyProfile l3 = Difficulty.GetDifficultyProfile(3);
        DifficultyProfile l4 = Difficulty.GetDifficultyProfile(4);
        DifficultyProfile l5 = Difficulty.GetDifficultyProfile(5);
        Assert.True(l3.MaxDepth <= 5);
        Assert.True(l4.MaxDepth <= 5);
        Assert.Equal(Constants.Search.AbsoluteMaxDepth, l5.MaxDepth);

        DifficultyProfile prev = Difficulty.GetDifficultyProfile(1);
        for (int level = 2; level <= 5; level++)
        {
            DifficultyProfile p = Difficulty.GetDifficultyProfile(level);
            Assert.True(p.MaxDepth >= prev.MaxDepth);
            if (p.UseVCF && prev.UseVCF)
            {
                Assert.True(p.VCFDepth > prev.VCFDepth);
            }
            Assert.True(p.TimeFraction > prev.TimeFraction);
            prev = p;
        }
    }

    [Fact]
    public void DifficultyProfilePonderOnlyL5()
    {
        for (int level = 1; level <= 6; level++)
        {
            DifficultyProfile p = Difficulty.GetDifficultyProfile(level);
            Assert.Equal(level >= 5, p.Ponder);
        }
    }

    [Fact]
    public void DifficultyL5Threads()
    {
        int n = Environment.ProcessorCount;
        DifficultyProfile p = Difficulty.GetDifficultyProfile(5);
        Assert.Equal(Difficulty.Pow2Floor((n - 2) / 2), p.Threads);
    }

    [Fact]
    public void AllocateTime()
    {
        TimeAllocation alloc = TimeManager.AllocateTime(30_000, 1000, 5);
        Assert.True(alloc.OptimalMs > 0);
        Assert.True(alloc.OptimalMs < 30_000);
        Assert.True(alloc.HardBoundMs > alloc.OptimalMs);
        Assert.True(alloc.OptimalMs > alloc.SoftBoundMs);
    }

    [Fact]
    public void AllocateTimeMinimum()
    {
        TimeAllocation alloc = TimeManager.AllocateTime(200, 0, 1);
        Assert.True(alloc.OptimalMs > 0);
        Assert.True(alloc.OptimalMs <= 200);
    }

    [Fact]
    public void Pow2Floor()
    {
        Assert.Equal(1, Difficulty.Pow2Floor(0));
        Assert.Equal(1, Difficulty.Pow2Floor(-1));
        Assert.Equal(1, Difficulty.Pow2Floor(1));
        Assert.Equal(2, Difficulty.Pow2Floor(2));
        Assert.Equal(4, Difficulty.Pow2Floor(5));
        Assert.Equal(8, Difficulty.Pow2Floor(10));
        Assert.Equal(16, Difficulty.Pow2Floor(20));
    }

    [Fact]
    public void GetEngineThreadsForLoad()
    {
        int n = Environment.ProcessorCount;
        Assert.Equal(n, Difficulty.GetEngineThreadsForLoad(1));
        Assert.Equal(n, Difficulty.GetEngineThreadsForLoad(0));
        Assert.Equal(n / 2, Difficulty.GetEngineThreadsForLoad(2));
    }
}
