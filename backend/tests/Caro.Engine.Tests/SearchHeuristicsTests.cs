using Caro.Domain;
using Caro.Engine;
using Xunit;

namespace Caro.Engine.Tests;

public class SearchHeuristicsTests
{
    [Fact]
    public void KillerMoves()
    {
        SearchHeuristics h = new();
        Position pos = new(5, 5);
        h.RecordKiller(3, pos);
        Assert.True(h.IsKiller(3, pos));
        Assert.False(h.IsKiller(2, pos));
    }

    [Fact]
    public void KillerMovesDisplaces()
    {
        SearchHeuristics h = new();
        Position pos1 = new(3, 3);
        Position pos2 = new(7, 7);
        h.RecordKiller(0, pos1);
        h.RecordKiller(0, pos2);
        Assert.True(h.IsKiller(0, pos1), "old killer should be in slot 1");
        Assert.True(h.IsKiller(0, pos2), "new killer should be in slot 0");
    }

    [Fact]
    public void KillerScore()
    {
        SearchHeuristics h = new();
        Position pos = new(4, 4);
        Assert.Equal(0, h.KillerScore(0, pos));
        h.RecordKiller(0, pos);
        Assert.Equal(500_000, h.KillerScore(0, pos));

        Position other = new(3, 3);
        h.RecordKiller(0, other);
        Assert.Equal(400_000, h.KillerScore(0, pos));
    }

    [Fact]
    public void HistoryScore()
    {
        SearchHeuristics h = new();
        h.RecordHistory(Player.Red, 5, 5, 4);
        Assert.True(h.HistoryScore(Player.Red, 5, 5) > 0);
        Assert.Equal(0, h.HistoryScore(Player.Blue, 5, 5));
    }

    [Fact]
    public void HistoryClamp()
    {
        SearchHeuristics h = new();
        for (int i = 0; i < 2000; i++)
        {
            h.RecordHistory(Player.Red, 0, 0, 64);
        }
        Assert.True(h.HistoryScore(Player.Red, 0, 0) <= 1_000_000);
    }

    [Fact]
    public void HeuristicsClear()
    {
        SearchHeuristics h = new();
        h.RecordKiller(0, new Position(1, 1));
        h.RecordHistory(Player.Red, 5, 5, 10);
        h.Clear();
        Assert.False(h.IsKiller(0, new Position(1, 1)));
        Assert.Equal(0, h.HistoryScore(Player.Red, 5, 5));
    }

    [Fact]
    public void KillerMovesOutOfBounds()
    {
        SearchHeuristics h = new();
        h.RecordKiller(-1, new Position(1, 1));
        h.RecordKiller(64, new Position(1, 1));
        Assert.False(h.IsKiller(-1, new Position(1, 1)));
        Assert.False(h.IsKiller(64, new Position(1, 1)));
        Assert.Equal(0, h.KillerScore(-1, new Position(1, 1)));
        Assert.Equal(0, h.KillerScore(64, new Position(1, 1)));
    }

    [Fact]
    public void HistoryScoreOutOfBounds()
    {
        SearchHeuristics h = new();
        h.RecordHistory(Player.Red, -1, 5, 4);
        Assert.Equal(0, h.HistoryScore(Player.Red, -1, 5));
        Assert.Equal(0, h.HistoryScore(Player.Red, 0, -1));
    }

    [Fact]
    public void ContHistoryNegativeBounds()
    {
        SearchHeuristics h = new();
        h.RecordContHistory(Player.Red, -1, 0, 5, 5, 4);
        h.RecordContHistory(Player.Red, 0, -1, 5, 5, 4);
        h.RecordContHistory(Player.Red, 0, 0, -1, 5, 4);
        h.RecordContHistory(Player.Red, 0, 0, 5, -1, 4);
        Assert.Equal(0, h.ContHistoryScore(Player.Red, -1, 0, 5, 5));
        Assert.Equal(0, h.ContHistoryScore(Player.Red, 0, -1, 5, 5));
    }

    [Fact]
    public void ContHistoryClamp()
    {
        SearchHeuristics h = new();
        for (int i = 0; i < 200; i++)
        {
            h.RecordContHistory(Player.Red, 5, 5, 6, 6, 64);
        }
        Assert.True(h.ContHistoryScore(Player.Red, 5, 5, 6, 6) <= 30_000);
    }

    [Fact]
    public void CounterMove()
    {
        SearchHeuristics h = new();
        h.RecordCounterMove(Player.Red, 5, 5, 7, 7);
        Position pos = h.CounterMoveFor(Player.Red, 5, 5);
        Assert.Equal(new Position(7, 7), pos);
    }

    [Fact]
    public void CounterMoveNegativeBounds()
    {
        SearchHeuristics h = new();
        h.RecordCounterMove(Player.Red, -1, 0, 5, 5);
        Position pos = h.CounterMoveFor(Player.Red, -1, 0);
        Assert.Equal(new Position(-1, -1), pos);
    }
}
