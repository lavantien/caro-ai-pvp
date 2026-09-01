using Caro.Domain;
using Caro.Engine;
using Xunit;

namespace Caro.Engine.Tests;

public class PonderTests
{
    private static Board PonderTestBoard() =>
        Board.NewBoard()
            .PlaceStone(7, 7, Player.Red)
            .PlaceStone(8, 8, Player.Blue);

    private static bool Eventually(Func<bool> condition, int timeoutMs, int pollMs)
    {
        DateTime deadline = DateTime.UtcNow.AddMilliseconds(timeoutMs);
        while (DateTime.UtcNow < deadline)
        {
            if (condition())
            {
                return true;
            }
            Thread.Sleep(pollMs);
        }
        return condition();
    }

    private static PonderConfig TestPonderConfig(int maxDepth, long timeCapMs) => new()
    {
        Threads = 1,
        MaxDepth = maxDepth,
        TimeCapMs = timeCapMs,
    };

    [Fact]
    public void PredictReplyAfterSearch()
    {
        using MinimaxAI ai = new(1, 64);

        Board b = Board.NewBoard()
            .PlaceStone(7, 7, Player.Red)
            .PlaceStone(8, 8, Player.Blue);

        (int x, int y, _) = ai.GetBestMove(b, Player.Red, new SearchOptions
        {
            TimeRemainingMs = 5000,
            ThreadCount = 1,
            TimeFraction = 1.0,
            MaxDepth = 4,
        }, CancellationToken.None);
        Assert.True(x >= 0);
        Assert.True(y >= 0);

        Board child = b.PlaceStone(x, y, Player.Red);

        (Position reply, bool ok) = ai.PredictReply(child);
        Assert.True(ok, "child of the searched root should have a TT entry");
        Assert.True(reply.IsValid());
        Assert.True(child.IsEmptyAt(reply.X, reply.Y));
    }

    [Fact]
    public void PredictReplyEmptyTT()
    {
        using MinimaxAI ai = new(1, 64);

        Board b = Board.NewBoard().PlaceStone(7, 7, Player.Red);
        (_, bool ok) = ai.PredictReply(b);
        Assert.False(ok, "no search ran, no prediction should be made");

        (_, ok) = ai.PredictReply(Board.NewBoard());
        Assert.False(ok, "zero hash must not false-positive on a fresh table");
    }

    [Fact]
    public void PredictReplyRejectsOccupiedCell()
    {
        using MinimaxAI ai = new(1, 64);

        Board b = Board.NewBoard().PlaceStone(7, 7, Player.Red);
        ai.TT.Store(new TTEntry { Hash = b.Hash, Depth = 3, MoveX = 7, MoveY = 7 });

        (_, bool ok) = ai.PredictReply(b);
        Assert.False(ok, "an entry pointing at an occupied cell must be rejected");
    }

    [Fact]
    public void StartPonderStopLifecycle()
    {
        using MinimaxAI ai = new(1, 64);
        Board b = PonderTestBoard();

        bool ok = ai.StartPonder(b, Player.Red, new Position(9, 9), TestPonderConfig(50, 10_000));
        Assert.True(ok, "no ponder running, start should succeed");
        Assert.True(ai.PonderActive());
        Assert.True(Eventually(() => ai.TT.Lookup(b.Hash, out TTEntry e) && e.Depth >= 1, 2000, 5),
            "depth 1 must complete before the stop");

        (PonderOutcome outcome, bool stopped) = ai.StopPonder();
        Assert.True(stopped);
        Assert.True(outcome.Completed, "depth 1 finished before the stop");
        Assert.True(outcome.BestX >= 0 && outcome.BestY >= 0);
        Assert.Equal(Player.Red, outcome.Player);
        Assert.Equal(new Position(9, 9), outcome.PredictedReply);
        Assert.Equal(PonderTestBoard().Hash, outcome.BoardHash);

        (_, stopped) = ai.StopPonder();
        Assert.False(stopped, "outcome is consumed exactly once");
        Assert.False(ai.PonderActive());
    }

    [Fact]
    public void StartPonderRefusesWhileRunning()
    {
        using MinimaxAI ai = new(1, 64);

        Assert.True(ai.StartPonder(PonderTestBoard(), Player.Red, new Position(9, 9), TestPonderConfig(50, 10_000)));
        Assert.False(ai.StartPonder(PonderTestBoard(), Player.Red, new Position(9, 9), TestPonderConfig(50, 10_000)),
            "a second start while running must be refused");
        ai.StopPonder();
    }

    [Fact]
    public void PonderCancelledBeforeCompletion()
    {
        using MinimaxAI ai = new(1, 64);

        CancellationTokenSource cts = new();
        cts.Cancel();
        Assert.True(ai.StartPonderWithContext(PonderTestBoard(), Player.Red,
            new Position(9, 9), TestPonderConfig(8, 5_000), cts.Token));

        (PonderOutcome outcome, bool stopped) = ai.StopPonder();
        Assert.True(stopped);
        Assert.False(outcome.Completed, "a cancelled ponder never completed a depth");
    }

    [Fact]
    public void PonderTimeCapEndsSearch()
    {
        using MinimaxAI ai = new(1, 64);

        Assert.True(ai.StartPonder(PonderTestBoard(), Player.Red, new Position(9, 9), TestPonderConfig(50, 50)));
        Assert.True(Eventually(() => !ai.PonderActive(), 2000, 10), "the cap must stop an idle ponder");
    }

    [Fact]
    public void PonderSharesTTNotHeuristics()
    {
        using MinimaxAI ai = new(1, 64);
        Board b = PonderTestBoard();

        Assert.True(ai.StartPonder(b, Player.Red, new Position(9, 9), TestPonderConfig(6, 3_000)));
        Assert.True(Eventually(() => ai.TT.Lookup(b.Hash, out TTEntry e) && e.Depth >= 1, 2000, 5));
        (PonderOutcome outcome, bool stopped) = ai.StopPonder();
        Assert.True(stopped);
        Assert.True(outcome.Completed);

        Assert.True(ai.TT.Lookup(b.Hash, out TTEntry entry), "ponder must warm the shared TT at the pondered root");
        Assert.True(entry.Depth >= 1);

        (int x, int y, _) = ai.GetBestMove(b, Player.Red, new SearchOptions
        {
            TimeRemainingMs = 5000,
            ThreadCount = 1,
            TimeFraction = 1.0,
        }, CancellationToken.None);
        Assert.True(x >= 0 && y >= 0, "normal search must still work after pondering");
    }

    [Fact]
    public void GetBestMoveStopsPonder()
    {
        using MinimaxAI ai = new(1, 64);
        Board b = PonderTestBoard();

        Assert.True(ai.StartPonder(b, Player.Red, new Position(9, 9), TestPonderConfig(50, 10_000)));

        (int x, int y, _) = ai.GetBestMove(b, Player.Red, new SearchOptions
        {
            TimeRemainingMs = 5000,
            ThreadCount = 1,
            TimeFraction = 1.0,
        }, CancellationToken.None);
        Assert.True(x >= 0 && y >= 0);
        Assert.False(ai.PonderActive(), "GetBestMove must drain any running ponder first");
    }

    [Fact]
    public void PonderDisposeDuringPonderNoRace()
    {
        MinimaxAI ai = new(1, 64);
        Assert.True(ai.StartPonder(PonderTestBoard(), Player.Red, new Position(9, 9), TestPonderConfig(50, 10_000)));
        ai.Dispose();
        Assert.False(ai.PonderActive());
    }

    [Fact]
    public void PonderCompletedVCF()
    {
        Assert.True(MinimaxAI.PonderCompleted(new SearchStats { MoveType = "vcf" }));
        Assert.True(MinimaxAI.PonderCompleted(new SearchStats { DepthAchieved = 3 }));
        Assert.False(MinimaxAI.PonderCompleted(new SearchStats()));
        Assert.False(MinimaxAI.PonderCompleted(new SearchStats { MoveType = "timeout-fallback" }));
    }

    [Fact]
    public void TTIsolationBetweenAIInstances()
    {
        using MinimaxAI red = new(1, 64);
        using MinimaxAI blue = new(1, 64);

        Board b = PonderTestBoard();
        (int x, int y, _) = red.GetBestMove(b, Player.Red, new SearchOptions
        {
            TimeRemainingMs = 5000,
            ThreadCount = 1,
            TimeFraction = 1.0,
            MaxDepth = 4,
        }, CancellationToken.None);
        Assert.True(x >= 0);
        Board searchRoot = b.PlaceStone(x, y, Player.Red);

        Assert.True(red.StartPonder(searchRoot, Player.Red, new Position(9, 9), TestPonderConfig(4, 500)));
        (PonderOutcome outcome, bool stopped) = red.StopPonder();
        Assert.True(stopped);

        // Red searched and pondered; every position it touched must be absent
        // from blue's table.
        foreach (ulong hash in new[] { b.Hash, searchRoot.Hash, outcome.BoardHash })
        {
            Assert.False(blue.TT.Lookup(hash, out _), $"red's work leaked into blue's table (hash {hash})");
        }

        // Symmetric: blue searches a position red never saw.
        Board b2 = b.PlaceStone(0, 0, Player.Red).PlaceStone(0, 1, Player.Blue);
        (int bx, int by, _) = blue.GetBestMove(b2, Player.Blue, new SearchOptions
        {
            TimeRemainingMs = 5000,
            ThreadCount = 1,
            TimeFraction = 1.0,
            MaxDepth = 4,
        }, CancellationToken.None);
        Assert.True(bx >= 0);
        Assert.True(by >= 0);
        Assert.True(blue.TT.Lookup(b2.Hash, out _), "blue's own search populated its table");
        Assert.False(red.TT.Lookup(b2.Hash, out _), "blue's search leaked into red's table");
    }
}
