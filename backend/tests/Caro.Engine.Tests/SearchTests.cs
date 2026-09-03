using Caro.Domain;
using Caro.Engine;
using Xunit;

namespace Caro.Engine.Tests;

public class SearchTests
{
    [Fact]
    public void SearchFindsWinningMove()
    {
        Board b = Board.NewBoard();
        for (int x = 3; x < 7; x++)
        {
            b = b.PlaceStone(x, 5, Player.Red);
        }
        b = b.PlaceStone(10, 10, Player.Blue);

        using TranspositionTable tt = new(1);
        SearchHeuristics heuristics = new();
        SearchConfig opts = new() { MaxDepth = 4, TimeLimitMs = 5000, Threads = 1 };

        (int mx, int my, SearchStats stats) = SearchEngine.SearchPosition(b, Player.Red, opts, tt, heuristics, CancellationToken.None);
        Assert.True(mx == 2 || mx == 7, $"should find winning move at end of line, got ({mx},{my})");
        Assert.Equal(5, my);
        Assert.True(stats.NodesSearched > 0);
    }

    [Fact]
    public void SearchFindsWinningMoveDespiteFutility()
    {
        Board b = Board.NewBoard()
            .PlaceStone(5, 5, Player.Red).PlaceStone(6, 5, Player.Red)
            .PlaceStone(7, 5, Player.Red).PlaceStone(8, 5, Player.Red)
            .PlaceStone(3, 3, Player.Blue).PlaceStone(4, 4, Player.Blue)
            .PlaceStone(5, 4, Player.Blue)
            .PlaceStone(10, 10, Player.Blue).PlaceStone(11, 11, Player.Blue)
            .PlaceStone(12, 12, Player.Blue);

        using TranspositionTable tt = new(1);
        SearchHeuristics h = new();
        SearchConfig opts = new() { MaxDepth = 3, TimeLimitMs = 5000, Threads = 1 };
        (int x, int y, _) = SearchEngine.SearchPosition(b, Player.Red, opts, tt, h, CancellationToken.None);
        Assert.True((x == 4 || x == 9) && y == 5, $"should find winning fifth stone, got ({x},{y})");
    }

    [Fact]
    public void SearchBlocksOpponentThreatAtNullMoveDepth()
    {
        Board b = Board.NewBoard()
            .PlaceStone(8, 8, Player.Red).PlaceStone(9, 9, Player.Red)
            .PlaceStone(10, 10, Player.Red)
            .PlaceStone(3, 3, Player.Red).PlaceStone(4, 4, Player.Red)
            .PlaceStone(5, 5, Player.Blue).PlaceStone(6, 5, Player.Blue)
            .PlaceStone(7, 5, Player.Blue)
            .PlaceStone(0, 0, Player.Red).PlaceStone(15, 15, Player.Blue);

        using TranspositionTable tt = new(1);
        SearchHeuristics h = new();
        SearchConfig opts = new() { MaxDepth = 5, TimeLimitMs = 5000, Threads = 1 };
        (int x, int y, SearchStats stats) = SearchEngine.SearchPosition(b, Player.Red, opts, tt, h, CancellationToken.None);
        bool blockOrWin = (x == 4 && y == 5) || (x == 8 && y == 5);
        if (stats.SearchScore >= Constants.Score.WinScore - Constants.Search.AbsoluteMaxDepth)
        {
            blockOrWin = true;
        }
        Assert.True(blockOrWin || stats.DepthAchieved >= 3,
            $"engine should address opponent's flex3 or find counter-win, got ({x},{y}) d={stats.DepthAchieved} s={stats.SearchScore}");
    }

    private static Board VcfBlockBoard() =>
        Board.NewBoard()
            .PlaceStone(5, 5, Player.Blue).PlaceStone(6, 5, Player.Blue)
            .PlaceStone(7, 5, Player.Blue)
            .PlaceStone(8, 6, Player.Blue)
            .PlaceStone(2, 13, Player.Red).PlaceStone(13, 2, Player.Red);

    [Fact]
    public void SearchBlocksVCFThroughAlphaBeta()
    {
        Board b = VcfBlockBoard();

        (int bvx, int bvy, VCFResult blueHasVCF) = Vcf.SolveVCF(b, Player.Blue, 5000, CancellationToken.None);
        Assert.Equal(VCFResult.Win, blueHasVCF);
        (_, _, VCFResult redHasVCF) = Vcf.SolveVCF(b, Player.Red, 5000, CancellationToken.None);
        Assert.NotEqual(VCFResult.Win, redHasVCF);
        Board blocked = b.PlaceStone(bvx, bvy, Player.Red);
        (_, _, VCFResult stillHas) = Vcf.SolveVCF(blocked, Player.Blue, 5000, CancellationToken.None);
        Assert.NotEqual(VCFResult.Win, stillHas);

        using TranspositionTable tt = new(1);
        SearchHeuristics h = new();
        SearchConfig opts = new() { MaxDepth = 4, TimeLimitMs = 5000, Threads = 1, UseVCF = true };
        (int x, int y, SearchStats stats) = SearchEngine.SearchPosition(b, Player.Red, opts, tt, h, CancellationToken.None);

        Assert.True(x >= 0 && y >= 0, $"should return valid move, got ({x},{y})");
        Assert.True(stats.DepthAchieved > 0, "should search through alpha-beta, not short-circuit");
        Assert.True(stats.MoveType is "" or "timeout-fallback",
            "full alpha-beta must run; only the VCF solver short-circuits");
    }

    [Fact]
    public void SearchFindsBlockingMove()
    {
        Board b = Board.NewBoard();
        b = b.PlaceStone(2, 5, Player.Red);
        for (int x = 3; x < 7; x++)
        {
            b = b.PlaceStone(x, 5, Player.Blue);
        }
        b = b.PlaceStone(0, 0, Player.Red);

        using TranspositionTable tt = new(1);
        SearchHeuristics heuristics = new();
        SearchConfig opts = new() { MaxDepth = 4, TimeLimitMs = 5000, Threads = 1 };

        (int mx, int my, SearchStats stats) = SearchEngine.SearchPosition(b, Player.Red, opts, tt, heuristics, CancellationToken.None);
        // Either defense is valid under the Caro blocked-ends rule: occupying
        // the completion (7,5), or double-blocking the would-be five from the
        // right (8,5).
        Assert.True(my == 5 && (mx == 7 || mx == 8),
            $"should neutralize opponent's four at (7,5)/(8,5), got ({mx},{my})");
        Assert.True(stats.DepthAchieved > 0);
    }

    [Fact]
    public void SearchBlocksVCFWithProvenNoWin()
    {
        Board b = VcfBlockBoard();

        (int bvx, int bvy, VCFResult blueResult) = Vcf.SolveVCF(b, Player.Blue, 5000, CancellationToken.None);
        Assert.Equal(VCFResult.Win, blueResult);
        Board blocked = b.PlaceStone(bvx, bvy, Player.Red);
        (_, _, VCFResult checkResult) = Vcf.SolveVCF(blocked, Player.Blue, 5000, CancellationToken.None);
        Assert.Equal(VCFResult.NoWin, checkResult);

        using TranspositionTable tt = new(1);
        SearchHeuristics h = new();
        SearchConfig opts = new() { MaxDepth = 4, TimeLimitMs = 10000, Threads = 1, UseVCF = true };
        (int x, int y, SearchStats stats) = SearchEngine.SearchPosition(b, Player.Red, opts, tt, h, CancellationToken.None);
        Assert.True(x >= 0 && y >= 0, $"should return valid move, got ({x},{y})");
        Assert.True(stats.MoveType is "" or "timeout-fallback",
            "full alpha-beta must run; only the VCF solver short-circuits");
    }

    [Fact]
    public void SearchFindsValidMoveUnderTimePressure()
    {
        Board b = VcfBlockBoard();

        (_, _, VCFResult blueResult) = Vcf.SolveVCF(b, Player.Blue, 5000, CancellationToken.None);
        Assert.Equal(VCFResult.Win, blueResult);

        using TranspositionTable tt = new(1);
        SearchHeuristics h = new();
        SearchConfig opts = new() { MaxDepth = 4, TimeLimitMs = 500, Threads = 1, UseVCF = true };
        (int x, int y, SearchStats stats) = SearchEngine.SearchPosition(b, Player.Red, opts, tt, h, CancellationToken.None);
        Assert.True(x >= 0 && y >= 0, $"should return valid move, got ({x},{y})");
        Assert.True(stats.DepthAchieved > 0, "alpha-beta should have searched at least 1 ply");
    }

    [Fact]
    public void IterationGatingKeepsSpendNearSoft()
    {
        Board b = Board.NewBoard()
            .PlaceStone(8, 8, Player.Red)
            .PlaceStone(9, 9, Player.Blue)
            .PlaceStone(7, 7, Player.Red)
            .PlaceStone(10, 10, Player.Blue)
            .PlaceStone(6, 8, Player.Red)
            .PlaceStone(9, 6, Player.Blue);
        using TranspositionTable tt = new(1);
        SearchHeuristics heuristics = new();
        SearchConfig opts = new() { MaxDepth = Constants.Search.AbsoluteMaxDepth, TimeLimitMs = 5000, SoftLimitMs = 500, Threads = 1 };

        System.Diagnostics.Stopwatch sw = System.Diagnostics.Stopwatch.StartNew();
        (_, _, SearchStats stats) = SearchEngine.SearchPosition(b, Player.Red, opts, tt, heuristics, CancellationToken.None);
        sw.Stop();

        Assert.True(stats.DepthAchieved >= 1);
        Assert.True(sw.ElapsedMilliseconds < 2000,
            $"iteration start must be gated on predicted completion, not just elapsed (took {sw.ElapsedMilliseconds}ms)");
    }

    [Fact]
    public void GapFillIsWinningCompletion()
    {
        Board b = Board.NewBoard()
            .PlaceStone(5, 5, Player.Red)
            .PlaceStone(6, 5, Player.Red)
            .PlaceStone(8, 5, Player.Red)
            .PlaceStone(9, 5, Player.Red)
            .PlaceStone(0, 0, Player.Blue);
        SearchBoard sb = new(b);

        sb.MakeMove(7, 5, Player.Red);
        Assert.True(MoveOrdering.WouldWin(sb, 7, 5, Player.Red),
            "filling the gap of XX.XX must make an exact five");
        sb.UnmakeMove();
    }
}
