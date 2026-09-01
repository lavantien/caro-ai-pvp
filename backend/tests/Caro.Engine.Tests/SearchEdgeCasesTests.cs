using Caro.Domain;
using Caro.Engine;
using Xunit;

namespace Caro.Engine.Tests;

/// <summary>
/// Search-driver edge cases: degenerate candidate sets, deep tactical
/// positions, and the solver's zero-budget behavior.
/// </summary>
public class SearchEdgeCasesTests
{
    /// <summary>Fills every cell in a checkerboard pattern, leaving holes in row 0.</summary>
    private static Board NearFullBoard(int emptyCells)
    {
        Board b = Board.NewBoard();
        for (int x = 0; x < Constants.BoardSize; x++)
        {
            for (int y = 0; y < Constants.BoardSize; y++)
            {
                if (x < emptyCells && y == 0)
                {
                    continue;
                }
                b = b.PlaceStone(x, y, (x + y) % 2 == 0 ? Player.Red : Player.Blue);
            }
        }
        return b;
    }

    [Fact]
    public void SearchPositionNoCandidatesOnFullBoard()
    {
        Board full = NearFullBoard(0);
        using TranspositionTable tt = new(1);
        (int x, int y, _) = SearchEngine.SearchPosition(full, Player.Red,
            new SearchConfig { MaxDepth = 2, TimeLimitMs = 1000, Threads = 1 },
            tt, new SearchHeuristics(), CancellationToken.None);
        Assert.Equal(-1, x);
        Assert.Equal(-1, y);
    }

    [Fact]
    public void SearchPositionSingleCandidate()
    {
        Board nearFull = NearFullBoard(1);
        using TranspositionTable tt = new(1);
        (int x, int y, _) = SearchEngine.SearchPosition(nearFull, Player.Red,
            new SearchConfig { MaxDepth = 2, TimeLimitMs = 1000, Threads = 1 },
            tt, new SearchHeuristics(), CancellationToken.None);
        Assert.Equal(0, x);
        Assert.Equal(0, y);
    }

    [Fact]
    public void ParallelSearchDegenerateCandidates()
    {
        using TranspositionTable tt = new(1);

        (int x0, int y0, SearchStats none) = ParallelSearch.Run(NearFullBoard(0), Player.Red,
            new SearchConfig { MaxDepth = 2, TimeLimitMs = 1000, Threads = 2 },
            tt, new SearchHeuristics(), CancellationToken.None);
        Assert.Equal(-1, x0);
        Assert.Equal(-1, y0);
        Assert.Equal(2, none.ThreadCount);

        (int x1, int y1, SearchStats single) = ParallelSearch.Run(NearFullBoard(1), Player.Red,
            new SearchConfig { MaxDepth = 2, TimeLimitMs = 1000, Threads = 2 },
            tt, new SearchHeuristics(), CancellationToken.None);
        Assert.Equal(0, x1);
        Assert.Equal(0, y1);
        Assert.Equal(2, single.ThreadCount);
    }

    [Fact]
    public void SearchWinsOpenFour()
    {
        // Red to move with an open four: any search must complete the five.
        // The position's lopsided eval also drives the null-move and
        // aspiration-window paths hard.
        Board b = Board.NewBoard()
            .PlaceStone(5, 5, Player.Red)
            .PlaceStone(6, 5, Player.Red)
            .PlaceStone(7, 5, Player.Red)
            .PlaceStone(8, 5, Player.Red)
            .PlaceStone(10, 10, Player.Blue)
            .PlaceStone(11, 11, Player.Blue);

        using TranspositionTable tt = new(1);
        (int x, int y, _) = SearchEngine.SearchPosition(b, Player.Red,
            new SearchConfig { MaxDepth = 6, TimeLimitMs = 10_000, Threads = 1 },
            tt, new SearchHeuristics(), CancellationToken.None);
        Assert.Equal(5, y);
        Assert.True(x == 4 || x == 9, $"should complete the five, got ({x},{y})");
    }

    [Fact]
    public void SearchSurvivesLostPosition()
    {
        // Blue to move against an overwhelming attack: the score collapses
        // across depths, forcing the aspiration windows to re-widen.
        Board b = Board.NewBoard()
            .PlaceStone(5, 5, Player.Red)
            .PlaceStone(6, 5, Player.Red)
            .PlaceStone(7, 5, Player.Red)
            .PlaceStone(8, 5, Player.Red)
            .PlaceStone(5, 6, Player.Red)
            .PlaceStone(6, 6, Player.Red)
            .PlaceStone(7, 6, Player.Red)
            .PlaceStone(0, 0, Player.Blue)
            .PlaceStone(15, 15, Player.Blue);

        using TranspositionTable tt = new(1);
        (int x, int y, _) = SearchEngine.SearchPosition(b, Player.Blue,
            new SearchConfig { MaxDepth = 6, TimeLimitMs = 10_000, Threads = 1, UseVCF = true },
            tt, new SearchHeuristics(), CancellationToken.None);
        Assert.True(x >= 0 && x < Constants.BoardSize);
        Assert.True(y >= 0 && y < Constants.BoardSize);
    }

    [Fact]
    public void ParallelSearchBlocksOpponentVCF()
    {
        // Blue to move against red's overwhelming threats: the parallel path
        // must run the opponent-VCF probe and prefer the blocking square.
        Board b = Board.NewBoard()
            .PlaceStone(5, 5, Player.Red)
            .PlaceStone(6, 5, Player.Red)
            .PlaceStone(7, 5, Player.Red)
            .PlaceStone(8, 5, Player.Red)
            .PlaceStone(5, 6, Player.Red)
            .PlaceStone(6, 6, Player.Red)
            .PlaceStone(7, 6, Player.Red)
            .PlaceStone(0, 0, Player.Blue)
            .PlaceStone(15, 15, Player.Blue);

        using TranspositionTable tt = new(1);
        (int x, int y, SearchStats stats) = ParallelSearch.Run(b, Player.Blue,
            new SearchConfig { MaxDepth = 4, TimeLimitMs = 10_000, Threads = 2, UseVCF = true },
            tt, new SearchHeuristics(), CancellationToken.None);
        Assert.True(x >= 0 && x < Constants.BoardSize);
        Assert.True(y >= 0 && y < Constants.BoardSize);
        Assert.Equal(2, stats.ThreadCount);
    }

    [Fact]
    public void DeepSearchOnStaticAdvantage()
    {
        // Red holds two blocked fours and a flex three: a large static edge
        // with no forced win, so the full-depth search (and the null-move
        // pruning it enables) actually runs.
        Board b = Board.NewBoard()
            .PlaceStone(0, 5, Player.Red)
            .PlaceStone(1, 5, Player.Red)
            .PlaceStone(2, 5, Player.Red)
            .PlaceStone(4, 5, Player.Red)
            .PlaceStone(0, 6, Player.Red)
            .PlaceStone(1, 6, Player.Red)
            .PlaceStone(2, 6, Player.Red)
            .PlaceStone(4, 6, Player.Red)
            .PlaceStone(6, 8, Player.Red)
            .PlaceStone(7, 8, Player.Red)
            .PlaceStone(8, 8, Player.Red)
            .PlaceStone(0, 0, Player.Blue)
            .PlaceStone(15, 15, Player.Blue);

        using TranspositionTable tt = new(1);
        (int x, int y, _) = SearchEngine.SearchPosition(b, Player.Red,
            new SearchConfig { MaxDepth = 6, TimeLimitMs = 20_000, Threads = 1 },
            tt, new SearchHeuristics(), CancellationToken.None);
        Assert.True(x >= 0 && x < Constants.BoardSize);
        Assert.True(y >= 0 && y < Constants.BoardSize);
    }

    [Fact]
    public void VCFZeroBudgetTimesOut()
    {
        Board b = Board.NewBoard()
            .PlaceStone(8, 8, Player.Red)
            .PlaceStone(9, 9, Player.Red)
            .PlaceStone(0, 0, Player.Blue)
            .PlaceStone(1, 1, Player.Blue);

        (_, _, VCFResult result) = Vcf.SolveVCFWithDepth(b, Player.Red, 9, 0, CancellationToken.None);
        Assert.Equal(VCFResult.Timeout, result);
    }
}
