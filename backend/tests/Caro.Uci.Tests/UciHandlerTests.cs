using Caro.Domain;
using Caro.Engine;
using Caro.Uci;
using Xunit;

namespace Caro.Uci.Tests;

/// <summary>
/// Collects reply lines from any thread.
/// </summary>
internal sealed class CollectingLineWriter : ILineWriter
{
    private readonly object _gate = new();
    private readonly List<string> _lines = [];

    public void WriteLine(string line)
    {
        lock (_gate)
        {
            _lines.Add(line);
        }
    }

    public string Output()
    {
        lock (_gate)
        {
            return string.Join('\n', _lines);
        }
    }

    public bool Contains(string needle) => Output().Contains(needle);
}

public class NotationTests
{
    [Fact]
    public void MoveToString()
    {
        Assert.Equal("aa", Notation.MoveToString(0, 0));
        Assert.Equal("bd", Notation.MoveToString(3, 1));
        Assert.Equal("pp", Notation.MoveToString(15, 15));
    }

    [Fact]
    public void ParseMove()
    {
        Assert.True(Notation.TryParseMove("aa", out int x, out int y));
        Assert.Equal(0, x);
        Assert.Equal(0, y);

        Assert.True(Notation.TryParseMove("bd", out x, out y));
        Assert.Equal(3, x);
        Assert.Equal(1, y);

        Assert.False(Notation.TryParseMove("z", out _, out _));
    }

    [Fact]
    public void NotationRoundTripAllCells()
    {
        for (int x = 0; x < Constants.BoardSize; x++)
        {
            for (int y = 0; y < Constants.BoardSize; y++)
            {
                string s = Notation.MoveToString(x, y);
                Assert.True(Notation.TryParseMove(s, out int px, out int py), $"ParseMove({s}) failed");
                Assert.Equal(x, px);
                Assert.Equal(y, py);
            }
        }
    }
}

public class UciHandlerTests
{
    private static string WaitForBestmove(CollectingLineWriter writer)
    {
        DateTime deadline = DateTime.UtcNow.AddSeconds(10);
        while (DateTime.UtcNow < deadline)
        {
            if (writer.Contains("bestmove "))
            {
                return writer.Output();
            }
            Thread.Sleep(10);
        }
        return writer.Output();
    }

    [Fact]
    public void UCIHandlerUCI()
    {
        CollectingLineWriter buf = new();
        using UciHandler h = new(buf);
        h.HandleCommand("uci");
        string output = buf.Output();
        Assert.Contains("id name Caro AI", output);
        Assert.Contains("uciok", output);
    }

    [Fact]
    public void UCIHandlerIsReady()
    {
        CollectingLineWriter buf = new();
        using UciHandler h = new(buf);
        h.HandleCommand("isready");
        Assert.Contains("readyok", buf.Output());
    }

    [Fact]
    public void UCIHandlerPosition()
    {
        CollectingLineWriter buf = new();
        using UciHandler h = new(buf);
        h.HandleCommand("position startpos moves aa");
        Assert.Equal("red", h.CurrentBoard().GetPlayerAt(0, 0).ToName());
    }

    [Fact]
    public void UCIHandlerNewGame()
    {
        CollectingLineWriter buf = new();
        using UciHandler h = new(buf);
        h.HandleCommand("position startpos moves aa");
        h.HandleCommand("ucinewgame");
        Assert.Equal("none", h.CurrentBoard().GetPlayerAt(0, 0).ToName());
    }

    [Fact]
    public void UCIHandlerGoMovetime()
    {
        CollectingLineWriter buf = new();
        using UciHandler h = new(buf);
        h.HandleCommand("go movetime 2000");
        string output = WaitForBestmove(buf);
        Assert.Contains("bestmove ", output);
        Assert.Contains("info ", output);
    }

    [Fact]
    public void UCIHandlerGoWtime()
    {
        CollectingLineWriter buf = new();
        using UciHandler h = new(buf);
        h.HandleCommand("go wtime 20000 btime 20000");
        string output = WaitForBestmove(buf);
        Assert.Contains("bestmove ", output);
    }

    [Fact]
    public void UCIHandlerStop()
    {
        CollectingLineWriter buf = new();
        using UciHandler h = new(buf);
        h.HandleCommand("stop");
        Assert.Equal(string.Empty, buf.Output());
    }

    [Fact]
    public void UCIHandlerSetOption()
    {
        CollectingLineWriter buf = new();
        using UciHandler h = new(buf);
        h.HandleCommand("setoption name Threads value 8");
        Assert.Equal(string.Empty, buf.Output());
    }

    [Fact]
    public void UCIHandlerQuit()
    {
        CollectingLineWriter buf = new();
        using UciHandler h = new(buf);
        h.HandleCommand("quit");
    }

    [Fact]
    public void UCIHandlerEmpty()
    {
        CollectingLineWriter buf = new();
        using UciHandler h = new(buf);
        h.HandleCommand("");
        Assert.Equal(string.Empty, buf.Output());
    }

    [Fact]
    public void RunUCILoop()
    {
        using StringReader reader = new("uci\nisready\nquit\n");
        CollectingLineWriter buf = new();
        using UciHandler h = new(buf);
        UciHandler.RunUciLoop(h, reader);
        string output = buf.Output();
        Assert.Contains("uciok", output);
        Assert.Contains("readyok", output);
    }

    [Fact]
    public void RunUCILoopSkipsEmpty()
    {
        using StringReader reader = new("uci\n\n\nisready\nquit\n");
        CollectingLineWriter buf = new();
        using UciHandler h = new(buf);
        UciHandler.RunUciLoop(h, reader);
        Assert.Contains("uciok", buf.Output());
    }

    /// <summary>
    /// The search must not block the command loop: stop arrives while the
    /// engine is thinking and bestmove comes back promptly. movetime is
    /// deliberately huge because the time manager only spends a fraction of
    /// the remaining clock.
    /// </summary>
    [Fact]
    public void StopInterruptsActiveSearch()
    {
        CollectingLineWriter buf = new();
        using UciHandler h = new(buf);
        h.HandleCommand("go movetime 600000");

        Thread.Sleep(300);
        Assert.False(buf.Contains("bestmove"), "precondition: search should still be running before stop");

        DateTime start = DateTime.UtcNow;
        h.HandleCommand("stop");

        while (DateTime.UtcNow < start.AddSeconds(2) && !buf.Contains("bestmove"))
        {
            Thread.Sleep(10);
        }
        Assert.True(buf.Contains("bestmove"), "stop must produce bestmove");
        Assert.True(DateTime.UtcNow - start < TimeSpan.FromSeconds(2), "stop must interrupt promptly");
    }

    [Fact]
    public void PositionRejectsBadMove()
    {
        CollectingLineWriter buf = new();
        using UciHandler h = new(buf);
        h.HandleCommand("position startpos moves aa zz bb");

        Assert.Contains("info string error", buf.Output());
        Assert.Equal("none", h.CurrentBoard().GetPlayerAt(0, 0).ToName());
    }

    [Fact]
    public void ParseGoOptionsClocksBySide()
    {
        string[] args = ["wtime", "1000", "btime", "200000", "winc", "3000", "binc", "0"];
        SearchOptions opts = UciHandler.ParseGoOptions(args, Player.Red, new SearchOptions());
        Assert.Equal(1000L, opts.TimeRemainingMs);
        Assert.Equal(3000L, opts.IncrementMs);

        opts = UciHandler.ParseGoOptions(args, Player.Blue, new SearchOptions());
        Assert.Equal(200_000L, opts.TimeRemainingMs);
        Assert.Equal(0L, opts.IncrementMs);
    }

    [Fact]
    public void ParseGoOptionsMovetimeAndDepth()
    {
        SearchOptions opts = UciHandler.ParseGoOptions(["movetime", "500"], Player.Red, new SearchOptions());
        Assert.Equal(500L, opts.TimeRemainingMs);

        opts = UciHandler.ParseGoOptions(["depth", "6"], Player.Red, new SearchOptions());
        Assert.Equal(6, opts.MaxDepth);
    }

    [Fact]
    public void SkillLevelChangesStrengthProfile()
    {
        CollectingLineWriter buf = new();
        using UciHandler h = new(buf);
        h.HandleCommand("setoption name Skill Level value 2");
        Assert.Equal(2, h.SkillLevel());

        DifficultyProfile profile = Difficulty.GetDifficultyProfile(2);
        Assert.Equal(profile.MaxDepth, h.SkillSearchOptions().MaxDepth);
    }
}
