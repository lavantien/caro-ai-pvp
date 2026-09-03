using Caro.Domain;
using Caro.Engine;
using Caro.Uci;
using Xunit;

namespace Caro.Uci.Tests;

public class UciConfigTests
{
    private static CaroConfig NarrowConfig()
    {
        CaroConfig config = new()
        {
            Uci = new UciOptions(),
        };
        config.Uci.Threads.Default = 2;
        config.Uci.Threads.Min = 1;
        config.Uci.Threads.Max = 8;
        config.Uci.HashMB.Min = 16;
        config.Validate();
        return config;
    }

    [Fact]
    public void HandshakeAdvertisesConfiguredBounds()
    {
        CollectingLineWriter buf = new();
        using UciHandler h = new(buf, NarrowConfig());
        h.HandleCommand("uci");

        Assert.Contains("option name Threads type spin default 2 min 1 max 8", buf.Output());
        Assert.Contains("option name Hash type spin default 256 min 16 max 4096", buf.Output());
        Assert.Contains("option name Skill Level type spin default 5 min 1 max 5", buf.Output());
    }

    [Fact]
    public void SetoptionRejectsValueOutsideConfiguredMax()
    {
        CollectingLineWriter buf = new();
        using UciHandler h = new(buf, NarrowConfig());
        h.HandleCommand("setoption name Threads value 9");

        Assert.Equal(2, h.CurrentThreads());
    }

    [Fact]
    public void SetoptionAcceptsValueInsideConfiguredRange()
    {
        CollectingLineWriter buf = new();
        using UciHandler h = new(buf, NarrowConfig());
        h.HandleCommand("setoption name Threads value 8");

        Assert.Equal(8, h.CurrentThreads());
    }

    [Fact]
    public void SkillSearchOptionsFollowConfiguredProfile()
    {
        CaroConfig config = NarrowConfig();
        config.DifficultyProfiles[3].MaxDepth = 3;
        config.DifficultyProfiles[3].ThreadsMode = "Two";

        CollectingLineWriter buf = new();
        using UciHandler h = new(buf, config);
        h.HandleCommand("setoption name Skill Level value 3");

        SearchOptions opts = h.SkillSearchOptions();
        Assert.Equal(3, opts.MaxDepth);
        Assert.Equal(2, opts.ThreadCount);
    }
}
