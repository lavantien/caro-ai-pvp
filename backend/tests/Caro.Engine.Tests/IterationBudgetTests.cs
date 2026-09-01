using Caro.Engine;
using Xunit;

namespace Caro.Engine.Tests;

public class IterationBudgetTests
{
    [Fact]
    public void IterationGrowthDefaultsWithoutHistory()
    {
        Assert.Equal(4.0, IterationBudget.IterationGrowth(120, 0));
        Assert.Equal(4.0, IterationBudget.IterationGrowth(0, 100));
    }

    [Fact]
    public void IterationGrowthClampsMeasuredRatio()
    {
        Assert.Equal(1.5, IterationBudget.IterationGrowth(10, 100));
        Assert.Equal(6.0, IterationBudget.IterationGrowth(1000, 10));
        Assert.Equal(3.0, IterationBudget.IterationGrowth(300, 100), 5);
    }

    [Fact]
    public void NextIterationFits()
    {
        Assert.True(IterationBudget.NextIterationFits(800, 100, 100, 1000));
        Assert.False(IterationBudget.NextIterationFits(800, 500, 100, 1000));
        Assert.True(IterationBudget.NextIterationFits(9900, 500, 100, 0));
        Assert.True(IterationBudget.NextIterationFits(800, 0, 0, 1000));
        Assert.True(IterationBudget.NextIterationFits(850, 100, 100, 1000));
    }
}
