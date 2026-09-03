using Caro.Domain;
using Caro.Engine;
using Xunit;

namespace Caro.Engine.Tests;

/// <summary>
/// Solver observability counters: nodes tried and forced-chain length,
/// surfaced through VcfSearchResult and SearchStats for engine probing.
/// </summary>
public class VcfCountersTests
{
    [Fact]
    public void ImmediateWinCountsChainDepthOne()
    {
        Board b = Board.NewBoard();
        for (int x = 3; x < 7; x++)
        {
            b = b.PlaceStone(x, 5, Player.Red);
        }
        b = b.PlaceStone(10, 10, Player.Blue);

        VcfSearchResult r = Vcf.SolveVCFWithDepth(b, Player.Red, 4, 1000, CancellationToken.None);
        Assert.Equal(VCFResult.Win, r.Result);
        Assert.Equal(1, r.ChainDepth);
        Assert.True(r.NodesSearched >= 1);
    }

    [Fact]
    public void ChainedWinCountsChainDepthTwo()
    {
        Board b = VcfTests.M15VCFBoard();

        VcfSearchResult r = Vcf.SolveVCFWithDepth(b, Player.Red, 2, 5000, CancellationToken.None);
        Assert.Equal(VCFResult.Win, r.Result);
        Assert.Equal(2, r.ChainDepth);
        Assert.True(r.NodesSearched >= 2);
    }

    [Fact]
    public void NoWinStillCountsNodes()
    {
        Board b = Board.NewBoard()
            .PlaceStone(8, 8, Player.Red)
            .PlaceStone(9, 9, Player.Blue);

        VcfSearchResult r = Vcf.SolveVCFWithDepth(b, Player.Red, 4, 100, CancellationToken.None);
        Assert.Equal(VCFResult.NoWin, r.Result);
        Assert.Equal(0, r.ChainDepth);
        Assert.True(r.NodesSearched >= 1);
    }

    [Fact]
    public void TimeoutReportsZeroNodesAndChainDepth()
    {
        Board b = Board.NewBoard()
            .PlaceStone(8, 8, Player.Red)
            .PlaceStone(9, 9, Player.Red)
            .PlaceStone(0, 0, Player.Blue)
            .PlaceStone(1, 1, Player.Blue);
        CancellationTokenSource cts = new();
        cts.Cancel();

        VcfSearchResult r = Vcf.SolveVCFWithDepth(b, Player.Red, 4, 1000, cts.Token);
        Assert.Equal(VCFResult.Timeout, r.Result);
        Assert.Equal(0, r.ChainDepth);
        Assert.Equal(0, r.NodesSearched);
    }

    [Fact]
    public void SearchStatsCarryVCFCounters()
    {
        Board b = VcfTests.M15VCFBoard();

        SearchConfig full = new() { MaxDepth = 10, TimeLimitMs = 30_000, Threads = 1, UseVCF = true };
        (_, _, SearchStats stats) = SearchEngine.SearchPosition(b, Player.Red, full,
            new TranspositionTable(1), new SearchHeuristics(), CancellationToken.None);
        Assert.Equal("vcf", stats.MoveType);
        Assert.Equal(2, stats.VcfDepth);
        Assert.NotNull(stats.VcfNodes);
        Assert.True(stats.VcfNodes >= 2);
    }
}
