using Caro.Domain;
using Caro.Engine;
using Xunit;

namespace Caro.Engine.Tests;

public class ParallelSearchTests
{
    [Fact]
    public void ParallelSearchFindsWinningMove()
    {
        Board b = Board.NewBoard();
        for (int x = 3; x < 7; x++)
        {
            b = b.PlaceStone(x, 5, Player.Red);
        }
        b = b.PlaceStone(10, 10, Player.Blue);

        using TranspositionTable tt = new(1);
        SearchHeuristics heuristics = new();
        SearchConfig opts = new()
        {
            MaxDepth = 4,
            TimeLimitMs = 5000,
            Threads = 2,
        };

        (int mx, int my, SearchStats stats) = ParallelSearch.Run(b, Player.Red, opts, tt, heuristics, CancellationToken.None);
        Assert.True(mx == 2 || mx == 7, $"should find winning move, got ({mx},{my})");
        Assert.Equal(5, my);
        Assert.True(stats.NodesSearched > 0);
        Assert.Equal(2, stats.ThreadCount);
    }

    [Fact]
    public void ParallelSearchFallsBackToSingleThread()
    {
        Board b = Board.NewBoard().PlaceStone(8, 8, Player.Red);

        using TranspositionTable tt = new(1);
        SearchHeuristics heuristics = new();
        SearchConfig opts = new()
        {
            MaxDepth = 2,
            TimeLimitMs = 1000,
            Threads = 1,
        };

        (int x, int y, _) = ParallelSearch.Run(b, Player.Blue, opts, tt, heuristics, CancellationToken.None);
        Assert.True(x >= 0 && x < Constants.BoardSize);
        Assert.True(y >= 0 && y < Constants.BoardSize);
    }

    [Fact]
    public void ParallelSearchSharesTT()
    {
        Board b = Board.NewBoard()
            .PlaceStone(8, 8, Player.Red)
            .PlaceStone(9, 9, Player.Blue);

        using TranspositionTable tt = new(4);
        SearchHeuristics heuristics = new();
        SearchConfig opts = new()
        {
            MaxDepth = 3,
            TimeLimitMs = 3000,
            Threads = 3,
        };

        (_, _, SearchStats stats) = ParallelSearch.Run(b, Player.Red, opts, tt, heuristics, CancellationToken.None);
        (long probes, _) = tt.Stats();
        Assert.True(probes > 0);
        Assert.Equal(3, stats.ThreadCount);
    }

    [Fact]
    public void ParallelSearchVCFFlag()
    {
        Board b = Board.NewBoard();
        for (int x = 3; x < 7; x++)
        {
            b = b.PlaceStone(x, 5, Player.Red);
        }
        b = b.PlaceStone(10, 10, Player.Blue);

        using TranspositionTable tt = new(1);
        SearchHeuristics heuristics = new();
        SearchConfig opts = new()
        {
            MaxDepth = 4,
            TimeLimitMs = 5000,
            Threads = 2,
            UseVCF = true,
        };

        (_, _, SearchStats stats) = ParallelSearch.Run(b, Player.Red, opts, tt, heuristics, CancellationToken.None);
        Assert.Equal("vcf", stats.MoveType);
    }
}

public class MinimaxAITests
{
    [Fact]
    public void MinimaxAIFindsWinningMove()
    {
        using MinimaxAI ai = new(1, 64);
        Board b = Board.NewBoard();
        for (int x = 3; x < 7; x++)
        {
            b = b.PlaceStone(x, 5, Player.Red);
        }
        b = b.PlaceStone(10, 10, Player.Blue);

        SearchOptions opts = new()
        {
            TimeRemainingMs = 5000,
            IncrementMs = 0,
            MoveNumber = 6,
            ThreadCount = 1,
            TimeFraction = 1.0,
        };

        (int mx, int my, SearchStats stats) = ai.GetBestMove(b, Player.Red, opts, CancellationToken.None);
        Assert.True(mx == 2 || mx == 7, $"should find winning move, got ({mx},{my})");
        Assert.Equal(5, my);
        Assert.True(stats.NodesSearched > 0);

        SearchStats gotStats = ai.GetStats();
        Assert.Equal(stats.NodesSearched, gotStats.NodesSearched);
    }

    [Fact]
    public void MinimaxAIDispose()
    {
        MinimaxAI ai = new(2, 64);
        ai.Dispose();
    }

    [Fact]
    public void NewMinimaxAIMinThreads()
    {
        MinimaxAI ai = new(0, 64);
        ai.Dispose();

        ai = new(-1, 64);
        ai.Dispose();
    }

    [Fact]
    public void MinimaxAIWithContextCancel()
    {
        using MinimaxAI ai = new(1, 64);

        Board b = Board.NewBoard()
            .PlaceStone(7, 7, Player.Red)
            .PlaceStone(8, 8, Player.Blue);

        using CancellationTokenSource cts = new(TimeSpan.FromMilliseconds(50));

        (int x, int y, _) = ai.GetBestMove(b, Player.Red, new SearchOptions
        {
            TimeRemainingMs = 5000,
            ThreadCount = 1,
            TimeFraction = 1.0,
        }, cts.Token);
        Assert.True(x >= 0 && y >= 0);
    }
}
