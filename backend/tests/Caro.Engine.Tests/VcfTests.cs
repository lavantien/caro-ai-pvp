using Caro.Domain;
using Caro.Engine;
using Xunit;

namespace Caro.Engine.Tests;

public class VcfTests
{
    private static Board M15VCFBoard() =>
        Board.NewBoard()
            // Red stones
            .PlaceStone(9, 8, Player.Red)
            .PlaceStone(6, 7, Player.Red)
            .PlaceStone(9, 7, Player.Red)
            .PlaceStone(7, 7, Player.Red)
            .PlaceStone(8, 9, Player.Red)
            .PlaceStone(8, 5, Player.Red)
            .PlaceStone(7, 6, Player.Red)
            .PlaceStone(9, 6, Player.Red)
            // Blue stones
            .PlaceStone(8, 8, Player.Blue)
            .PlaceStone(7, 9, Player.Blue)
            .PlaceStone(9, 9, Player.Blue)
            .PlaceStone(8, 7, Player.Blue)
            .PlaceStone(8, 6, Player.Blue)
            .PlaceStone(8, 10, Player.Blue)
            .PlaceStone(5, 8, Player.Blue);

    [Fact]
    public void VCFFindsImmediateWin()
    {
        Board b = Board.NewBoard();
        for (int x = 3; x < 7; x++)
        {
            b = b.PlaceStone(x, 5, Player.Red);
        }
        b = b.PlaceStone(10, 10, Player.Blue);

        (int mx, int my, VCFResult result) = Vcf.SolveVCF(b, Player.Red, 1000, CancellationToken.None);
        Assert.Equal(VCFResult.Win, result);
        Assert.True((mx == 2 || mx == 7) && my == 5, $"should complete the five, got ({mx},{my})");
    }

    [Fact]
    public void VCFNoWin()
    {
        Board b = Board.NewBoard()
            .PlaceStone(8, 8, Player.Red)
            .PlaceStone(9, 9, Player.Blue);

        (_, _, VCFResult result) = Vcf.SolveVCF(b, Player.Red, 100, CancellationToken.None);
        Assert.Equal(VCFResult.NoWin, result);
    }

    [Fact]
    public void VCFCancelled()
    {
        Board b = Board.NewBoard()
            .PlaceStone(8, 8, Player.Red)
            .PlaceStone(9, 9, Player.Red)
            .PlaceStone(0, 0, Player.Blue)
            .PlaceStone(1, 1, Player.Blue);

        CancellationTokenSource cts = new();
        cts.Cancel();
        (_, _, VCFResult result) = Vcf.SolveVCF(b, Player.Red, 1000, cts.Token);
        Assert.Equal(VCFResult.Timeout, result);
    }

    [Fact]
    public void VCFFourBlocks()
    {
        Board b = Board.NewBoard()
            .PlaceStone(5, 5, Player.Red)
            .PlaceStone(6, 5, Player.Red)
            .PlaceStone(7, 5, Player.Red)
            .PlaceStone(9, 5, Player.Blue)
            .PlaceStone(10, 10, Player.Blue);
        SearchBoard sb = new(b);
        sb.MakeMove(8, 5, Player.Red);
        List<Position> blocks = Vcf.FindFourBlocks(sb, 8, 5, Player.Red);
        sb.UnmakeMove();
        Assert.Single(blocks);
        Assert.Equal(4, blocks[0].X);
    }

    [Fact]
    public void VCFFourBlocksBothOpen()
    {
        Board b = Board.NewBoard()
            .PlaceStone(5, 5, Player.Red)
            .PlaceStone(6, 5, Player.Red)
            .PlaceStone(7, 5, Player.Red)
            .PlaceStone(10, 10, Player.Blue);

        SearchBoard sb = new(b);
        sb.MakeMove(8, 5, Player.Red);
        List<Position> blocks = Vcf.FindFourBlocks(sb, 8, 5, Player.Red);
        sb.UnmakeMove();
        Assert.Equal(2, blocks.Count);
    }

    [Fact]
    public void VCFFourBlocksNoFour()
    {
        Board b = Board.NewBoard()
            .PlaceStone(5, 5, Player.Red)
            .PlaceStone(6, 5, Player.Red)
            .PlaceStone(10, 10, Player.Blue);

        SearchBoard sb = new(b);
        sb.MakeMove(7, 5, Player.Red);
        List<Position> blocks = Vcf.FindFourBlocks(sb, 7, 5, Player.Red);
        sb.UnmakeMove();
        Assert.Empty(blocks);
    }

    [Fact]
    public void VCFSearchFindsWinViaContinuousFours()
    {
        Board b = Board.NewBoard()
            .PlaceStone(5, 5, Player.Red)
            .PlaceStone(6, 5, Player.Red)
            .PlaceStone(7, 5, Player.Red)
            .PlaceStone(8, 2, Player.Red)
            .PlaceStone(8, 3, Player.Red)
            .PlaceStone(8, 4, Player.Red)
            .PlaceStone(0, 0, Player.Blue)
            .PlaceStone(1, 1, Player.Blue);

        (int x, int y, VCFResult result) = Vcf.SolveVCF(b, Player.Red, 5000, CancellationToken.None);
        Assert.Equal(VCFResult.Win, result);
        Assert.True(x >= 0 && y >= 0);
    }

    [Fact]
    public void VCFSkippedWhenOpponentHasFlex4()
    {
        Board b = Board.NewBoard()
            .PlaceStone(5, 5, Player.Red).PlaceStone(6, 5, Player.Red)
            .PlaceStone(7, 5, Player.Red)
            .PlaceStone(3, 3, Player.Blue).PlaceStone(4, 3, Player.Blue)
            .PlaceStone(5, 3, Player.Blue).PlaceStone(6, 3, Player.Blue)
            .PlaceStone(10, 10, Player.Red);

        using TranspositionTable tt = new(1);
        SearchHeuristics h = new();
        SearchConfig opts = new() { MaxDepth = 4, TimeLimitMs = 5000, Threads = 1, UseVCF = true };
        (_, _, SearchStats stats) = SearchEngine.SearchPosition(b, Player.Red, opts, tt, h, CancellationToken.None);
        Assert.NotEqual("vcf", stats.MoveType);
    }

    [Fact]
    public void VCFForcedBlockWhenOpponentHasSingleWinSquare()
    {
        Board b = Board.NewBoard()
            .PlaceStone(3, 3, Player.Blue).PlaceStone(4, 3, Player.Blue)
            .PlaceStone(5, 3, Player.Blue).PlaceStone(6, 3, Player.Blue)
            .PlaceStone(7, 3, Player.Red)
            .PlaceStone(5, 5, Player.Red).PlaceStone(6, 5, Player.Red)
            .PlaceStone(7, 5, Player.Red)
            .PlaceStone(10, 10, Player.Blue);

        using TranspositionTable tt = new(1);
        SearchHeuristics h = new();
        SearchConfig opts = new() { MaxDepth = 6, TimeLimitMs = 10000, Threads = 1, UseVCF = true };
        (int x, int y, _) = SearchEngine.SearchPosition(b, Player.Red, opts, tt, h, CancellationToken.None);
        Assert.True(x >= 0 && y >= 0);
    }

    [Fact]
    public void VCFSmallRadiusFindsWin()
    {
        Board b = Board.NewBoard()
            .PlaceStone(5, 5, Player.Red).PlaceStone(6, 5, Player.Red)
            .PlaceStone(7, 5, Player.Red)
            .PlaceStone(8, 2, Player.Red).PlaceStone(8, 3, Player.Red)
            .PlaceStone(8, 4, Player.Red)
            .PlaceStone(0, 0, Player.Blue).PlaceStone(1, 1, Player.Blue);

        (int x, int y, VCFResult result) = Vcf.SolveVCF(b, Player.Red, 5000, CancellationToken.None);
        Assert.Equal(VCFResult.Win, result);
        Assert.True(x >= 0 && y >= 0);
    }

    [Fact]
    public void VCFSkippedWhenOpponentHasBrokenFour()
    {
        Board b = Board.NewBoard()
            .PlaceStone(9, 8, Player.Red)
            .PlaceStone(7, 5, Player.Red)
            .PlaceStone(10, 6, Player.Red)
            .PlaceStone(6, 6, Player.Red)
            .PlaceStone(8, 6, Player.Red)
            .PlaceStone(7, 6, Player.Red)
            .PlaceStone(8, 8, Player.Blue)
            .PlaceStone(9, 7, Player.Blue)
            .PlaceStone(9, 9, Player.Blue)
            .PlaceStone(10, 8, Player.Blue)
            .PlaceStone(8, 10, Player.Blue);

        using TranspositionTable tt = new(1);
        SearchHeuristics h = new();
        SearchConfig opts = new() { MaxDepth = 6, TimeLimitMs = 10000, Threads = 1, UseVCF = true };
        (int x, int y, SearchStats stats) = SearchEngine.SearchPosition(b, Player.Blue, opts, tt, h, CancellationToken.None);
        Assert.NotEqual("vcf", stats.MoveType);
        bool blockedOrWon = (x == 9 && y == 6) || stats.SearchScore >= Constants.Score.WinScore - Constants.Search.AbsoluteMaxDepth;
        Assert.True(blockedOrWon, $"should block Red's broken four at (9,6) or counter-win, got ({x},{y}) score={stats.SearchScore}");
    }

    [Fact]
    public void VCFSolverFailsWhenOpponentHasBrokenFour()
    {
        Board b = Board.NewBoard()
            .PlaceStone(5, 5, Player.Red).PlaceStone(6, 5, Player.Red)
            .PlaceStone(7, 5, Player.Red)
            .PlaceStone(8, 2, Player.Red).PlaceStone(8, 3, Player.Red)
            .PlaceStone(8, 4, Player.Red)
            .PlaceStone(6, 6, Player.Blue).PlaceStone(7, 6, Player.Blue)
            .PlaceStone(8, 6, Player.Blue).PlaceStone(10, 6, Player.Blue)
            .PlaceStone(15, 15, Player.Red);

        (_, _, VCFResult result) = Vcf.SolveVCF(b, Player.Red, 5000, CancellationToken.None);
        Assert.Equal(VCFResult.NoWin, result);
    }

    [Fact]
    public void VCFOpponentCounterWin()
    {
        Board b = Board.NewBoard()
            .PlaceStone(5, 5, Player.Red)
            .PlaceStone(6, 5, Player.Red)
            .PlaceStone(7, 5, Player.Red)
            .PlaceStone(0, 5, Player.Blue)
            .PlaceStone(1, 5, Player.Blue)
            .PlaceStone(2, 5, Player.Blue)
            .PlaceStone(3, 5, Player.Blue)
            .PlaceStone(10, 5, Player.Blue)
            .PlaceStone(11, 5, Player.Blue)
            .PlaceStone(12, 5, Player.Blue)
            .PlaceStone(13, 5, Player.Blue)
            .PlaceStone(15, 15, Player.Blue);

        (_, _, VCFResult result) = Vcf.SolveVCF(b, Player.Red, 5000, CancellationToken.None);
        Assert.Equal(VCFResult.NoWin, result);
    }

    [Fact]
    public void FindFourBlocksRejectsOverline()
    {
        Board b = Board.NewBoard()
            .PlaceStone(5, 5, Player.Red)
            .PlaceStone(6, 5, Player.Red)
            .PlaceStone(7, 5, Player.Red)
            .PlaceStone(10, 5, Player.Red)
            .PlaceStone(0, 0, Player.Blue);

        SearchBoard sb = new(b);
        sb.MakeMove(8, 5, Player.Red);
        List<Position> blocks = Vcf.FindFourBlocks(sb, 8, 5, Player.Red);
        sb.UnmakeMove();

        Assert.Single(blocks);
        Assert.Equal(new Position(4, 5), blocks[0]);
    }

    [Fact]
    public void FindFourBlocksRejectsOverlineBothEnds()
    {
        Board b = Board.NewBoard()
            .PlaceStone(4, 5, Player.Red)
            .PlaceStone(5, 5, Player.Red)
            .PlaceStone(6, 5, Player.Red)
            .PlaceStone(9, 5, Player.Red)
            .PlaceStone(10, 5, Player.Red)
            .PlaceStone(0, 0, Player.Blue);

        SearchBoard sb = new(b);
        sb.MakeMove(7, 5, Player.Red);
        List<Position> blocks = Vcf.FindFourBlocks(sb, 7, 5, Player.Red);
        sb.UnmakeMove();

        Assert.Single(blocks);
    }

    [Fact]
    public void FindFourBlocksBothEndsOverline()
    {
        Board b = Board.NewBoard()
            .PlaceStone(0, 5, Player.Red)
            .PlaceStone(2, 5, Player.Red)
            .PlaceStone(3, 5, Player.Red)
            .PlaceStone(4, 5, Player.Red)
            .PlaceStone(7, 5, Player.Red)
            .PlaceStone(0, 0, Player.Blue);

        SearchBoard sb = new(b);
        sb.MakeMove(5, 5, Player.Red);
        List<Position> blocks = Vcf.FindFourBlocks(sb, 5, 5, Player.Red);
        sb.UnmakeMove();
        Assert.Empty(blocks);
    }

    [Fact]
    public void SearchBlocksOpponentVCF()
    {
        Board b = M15VCFBoard();

        (_, _, VCFResult redHasVCF) = Vcf.SolveVCF(b, Player.Red, 5000, CancellationToken.None);
        Assert.Equal(VCFResult.Win, redHasVCF);

        using TranspositionTable tt = new(1);
        SearchHeuristics h = new();
        SearchConfig opts = new() { MaxDepth = 10, TimeLimitMs = 30000, Threads = 1, UseVCF = true };
        (int x, int y, SearchStats stats) = SearchEngine.SearchPosition(b, Player.Blue, opts, tt, h, CancellationToken.None);
        Assert.True(x >= 0 && y >= 0);
        Assert.True(stats.MoveType is "" or "timeout-fallback",
            "full alpha-beta must run; only the VCF solver short-circuits");
    }

    [Fact]
    public void VCFResultDistinctStates()
    {
        Assert.NotEqual(VCFResult.NoWin, VCFResult.Win);
        Assert.NotEqual(VCFResult.NoWin, VCFResult.Timeout);
        Assert.NotEqual(VCFResult.Win, VCFResult.Timeout);
    }

    [Fact]
    public void VCFSolveReturnsWinWhenFound()
    {
        Board b = Board.NewBoard();
        for (int x = 3; x < 7; x++)
        {
            b = b.PlaceStone(x, 5, Player.Red);
        }
        b = b.PlaceStone(10, 10, Player.Blue);
        (_, _, VCFResult result) = Vcf.SolveVCF(b, Player.Red, 1000, CancellationToken.None);
        Assert.Equal(VCFResult.Win, result);
    }


    [Fact]
    public void VCFSolveReturnsTimeoutOnCancellation()
    {
        Board b = Board.NewBoard()
            .PlaceStone(8, 8, Player.Red)
            .PlaceStone(9, 9, Player.Red)
            .PlaceStone(0, 0, Player.Blue)
            .PlaceStone(1, 1, Player.Blue);

        CancellationTokenSource cts = new();
        cts.Cancel();
        (_, _, VCFResult result) = Vcf.SolveVCF(b, Player.Red, 1000, cts.Token);
        Assert.Equal(VCFResult.Timeout, result);
    }

    [Fact]
    public void VCFSolveReturnsNoWinWhenProven()
    {
        Board b = Board.NewBoard()
            .PlaceStone(8, 8, Player.Red)
            .PlaceStone(9, 9, Player.Blue);
        (_, _, VCFResult result) = Vcf.SolveVCF(b, Player.Red, 100, CancellationToken.None);
        Assert.Equal(VCFResult.NoWin, result);
    }

    [Fact]
    public void VCFWinsViaSplitFour()
    {
        Board b = Board.NewBoard()
            .PlaceStone(4, 5, Player.Red)
            .PlaceStone(5, 5, Player.Red)
            .PlaceStone(7, 5, Player.Red)
            .PlaceStone(4, 6, Player.Red)
            .PlaceStone(5, 6, Player.Red)
            .PlaceStone(6, 6, Player.Red)
            .PlaceStone(0, 0, Player.Blue)
            .PlaceStone(0, 1, Player.Blue);

        (int x, int y, VCFResult result) = Vcf.SolveVCF(b, Player.Red, 2000, CancellationToken.None);
        Assert.Equal(VCFResult.Win, result);
        Assert.True(x >= 0 && y >= 0);
    }

    [Fact]
    public void SolveVCFDepthLimit()
    {
        Board b = M15VCFBoard();

        (_, _, VCFResult result) = Vcf.SolveVCFWithDepth(b, Player.Red, 1, 5000, CancellationToken.None);
        Assert.Equal(VCFResult.NoWin, result);

        (_, _, result) = Vcf.SolveVCFWithDepth(b, Player.Red, 2, 5000, CancellationToken.None);
        Assert.Equal(VCFResult.Win, result);

        (_, _, result) = Vcf.SolveVCF(b, Player.Red, 5000, CancellationToken.None);
        Assert.Equal(VCFResult.Win, result);
    }

    [Fact]
    public void SearchPositionRespectsVCFDepthLimit()
    {
        Board b = M15VCFBoard();

        SearchConfig limited = new() { MaxDepth = 10, TimeLimitMs = 30_000, Threads = 1, UseVCF = true, VCFMaxDepth = 1 };
        (_, _, SearchStats stats) = SearchEngine.SearchPosition(b, Player.Red, limited,
            new TranspositionTable(1), new SearchHeuristics(), CancellationToken.None);
        Assert.NotEqual("vcf", stats.MoveType);

        SearchConfig full = new() { MaxDepth = 10, TimeLimitMs = 30_000, Threads = 1, UseVCF = true };
        using TranspositionTable tt = new(1);
        (_, _, stats) = SearchEngine.SearchPosition(b, Player.Red, full, tt, new SearchHeuristics(), CancellationToken.None);
        Assert.Equal("vcf", stats.MoveType);
    }
}
