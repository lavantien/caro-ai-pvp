using Caro.Domain;
using Caro.Engine;
using Xunit;

namespace Caro.Engine.Tests;

/// <summary>
/// Unit-level clamps and guards: facade bounds, ponder config sanitizing,
/// undo-stack growth, monitor disposal, eval clamping, picker guards.
/// </summary>
public class EngineDetailsTests
{
    [Fact]
    public void MinimaxAITTSizeClampedToDefault()
    {
        using MinimaxAI ai = new(1, 0);
        Assert.NotNull(ai.TT);
        Assert.NotNull(ai.Heuristics);
    }

    [Fact]
    public void MinimaxAIClampsNegativeTimeFraction()
    {
        using MinimaxAI ai = new(2, 32);
        Board b = Board.NewBoard()
            .PlaceStone(8, 8, Player.Red)
            .PlaceStone(9, 9, Player.Blue);

        // Negative fraction clamps the budget to zero: the search aborts
        // immediately and falls back to move ordering, still returning a
        // legal square. ThreadCount 0 clamps to one worker.
        (int x, int y, SearchStats stats) = ai.GetBestMove(b, Player.Red, new SearchOptions
        {
            TimeRemainingMs = 60_000,
            ThreadCount = 0,
            ParallelEnabled = false,
            TimeFraction = -0.5,
        }, CancellationToken.None);
        Assert.True(x >= 0 && x < Constants.BoardSize);
        Assert.True(y >= 0 && y < Constants.BoardSize);
        Assert.True(stats.NodesSearched >= 0);
    }

    [Fact]
    public void PonderConfigClampedToValidRanges()
    {
        using MinimaxAI ai = new(2, 32);
        Board pondered = Board.NewBoard()
            .PlaceStone(8, 8, Player.Red)
            .PlaceStone(9, 9, Player.Blue);

        Assert.True(ai.StartPonder(pondered, Player.Red, new Position(5, 5), new PonderConfig
        {
            Threads = 0,
            MaxDepth = 0,
            TimeCapMs = 300,
        }));

        DateTime deadline = DateTime.UtcNow.AddSeconds(10);
        while (ai.PonderActive() && DateTime.UtcNow < deadline)
        {
            Thread.Sleep(10);
        }
        Assert.False(ai.PonderActive());

        (PonderOutcome outcome, bool ok) = ai.StopPonder();
        Assert.True(ok);
        Assert.Equal(Player.Red, outcome.Player);
        Assert.Equal(pondered.Hash, outcome.BoardHash);
    }

    [Fact]
    public void SearchBoardUndoStackGrows()
    {
        SearchBoard sb = new(Board.NewBoard());
        for (int i = 0; i < 100; i++)
        {
            sb.MakeMove(i % Constants.BoardSize, i / Constants.BoardSize, i % 2 == 0 ? Player.Red : Player.Blue);
        }
        Assert.Equal(100, sb.StoneCount());
        for (int i = 0; i < 100; i++)
        {
            sb.UnmakeMove();
        }
        Assert.Equal(0, sb.StoneCount());

        for (int i = 0; i < 100; i++)
        {
            sb.MakeNullMove();
        }
        for (int i = 0; i < 100; i++)
        {
            sb.UnmakeNullMove();
        }
        Assert.Equal(0, sb.StoneCount());
    }

    [Fact]
    public void TimeMonitorTokenAndDispose()
    {
        CancellationTokenSource cts = new();
        cts.Cancel();
        using TimeMonitor monitor = new(5000, cts.Token);
        Assert.True(monitor.Token.CanBeCanceled);
        Assert.True(monitor.ShouldStop());
    }

    [Fact]
    public void TimeManagerLatePhaseDividesSlower()
    {
        TimeAllocation early = TimeManager.AllocateTime(120_000, 1000, 5);
        TimeAllocation late = TimeManager.AllocateTime(120_000, 1000, 30);
        Assert.True(early.OptimalMs > 0);
        Assert.True(late.OptimalMs > 0);
        Assert.True(late.OptimalMs < early.OptimalMs);
    }

    [Fact]
    public void EvaluationClampsToMaxEval()
    {
        Board b = Board.NewBoard()
            .PlaceStone(0, 5, Player.Red)
            .PlaceStone(1, 5, Player.Red)
            .PlaceStone(2, 5, Player.Red)
            .PlaceStone(3, 5, Player.Red)
            .PlaceStone(4, 5, Player.Red)
            .PlaceStone(10, 10, Player.Blue)
            .PlaceStone(11, 11, Player.Blue);
        SearchBoard sb = new(b);

        Assert.Equal(Evaluation.MaxCorrectedEval, Evaluation.Evaluate(sb, Player.Red));
        Assert.Equal(-Evaluation.MaxCorrectedEval, Evaluation.Evaluate(sb, Player.Blue));
    }

    [Fact]
    public void OrderMovesReturnsSingleCandidateUnchanged()
    {
        SearchBoard sb = new(Board.NewBoard().PlaceStone(8, 8, Player.Red));
        List<Position> candidates = [new(7, 7)];
        List<Position> ordered = MoveOrdering.OrderMoves(candidates, sb, Player.Blue, 2, null, new SearchHeuristics());
        Assert.Single(ordered);
        Assert.Equal(new Position(7, 7), ordered[0]);
    }

    [Fact]
    public void OrderMovesSkipsInvalidKillers()
    {
        Board b = Board.NewBoard()
            .PlaceStone(8, 8, Player.Red)
            .PlaceStone(7, 7, Player.Blue);
        SearchBoard sb = new(b);
        SearchHeuristics h = new();
        h.RecordKiller(2, new Position(-1, -1));

        List<Position> candidates = Candidates.GetCandidates(sb, Constants.MaxSearchRadius);
        Assert.True(candidates.Count > 1);

        // The killer stage may append out-of-candidate squares (a fresh
        // heuristics table holds default (0,0) killers), so the output is a
        // superset of the candidates.
        List<Position> ordered = MoveOrdering.OrderMoves(candidates, sb, Player.Red, 2, null, h);
        Assert.True(ordered.Count >= candidates.Count);
        Assert.All(candidates, c => Assert.Contains(c, ordered));

        // Depth -1 exercises the killer-depth guard without dropping moves.
        List<Position> shallow = MoveOrdering.OrderMoves(candidates, sb, Player.Red, -1, null, h);
        Assert.True(shallow.Count >= candidates.Count);
    }

    [Fact]
    public void OrderMovesCapsSaturatedHistory()
    {
        Board b = Board.NewBoard()
            .PlaceStone(8, 8, Player.Red)
            .PlaceStone(7, 7, Player.Blue);
        SearchBoard sb = new(b);
        SearchHeuristics h = new();
        for (int i = 0; i < 8000; i++)
        {
            h.RecordHistory(Player.Red, 7, 8, 12);
        }
        Assert.True(h.HistoryScore(Player.Red, 7, 8) * 2 > 300_000, "history must saturate the picker cap");

        List<Position> candidates = Candidates.GetCandidates(sb, Constants.MaxSearchRadius);
        List<Position> ordered = MoveOrdering.OrderMoves(candidates, sb, Player.Red, 1, null, h);
        Assert.True(ordered.Count >= candidates.Count);
    }
}
