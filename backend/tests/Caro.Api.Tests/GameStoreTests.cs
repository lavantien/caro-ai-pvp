using Caro.Api;
using Caro.Domain;
using Xunit;
using static Caro.Api.Tests.GameSessionTests;

namespace Caro.Api.Tests;

public class GameStoreTests
{
    [Fact]
    public void NewInMemoryStoreEmpty()
    {
        GameStore s = new();
        Assert.Equal(0, s.Count());
        Assert.Equal(0, s.ActiveGameCount());
    }

    [Fact]
    public void StoreSetGetDelete()
    {
        GameStore s = new();
        GameSession session = NewTestSession();
        s.Set("game1", session);

        Assert.True(s.TryGet("game1", out GameSession got));
        Assert.Same(session, got);

        Assert.False(s.TryGet("nonexistent", out _));

        s.Delete("game1");
        Assert.False(s.TryGet("game1", out _));
        Assert.Equal(0, s.Count());
    }

    [Fact]
    public void StoreActiveGameCount()
    {
        GameStore s = new();
        s.Set("g1", NewTestSession());
        s.Set("g2", NewTestSession());
        Assert.Equal(2, s.ActiveGameCount());
    }

    [Fact]
    public void StoreCleanupAll()
    {
        GameStore s = new();
        s.Set("g1", NewTestSession());
        s.Set("g2", NewTestSession());
        int removed = s.CleanupAll();
        Assert.Equal(2, removed);
        Assert.Equal(0, s.Count());
    }

    [Fact]
    public void StoreCleanupCompletedRemovesFinishedGame()
    {
        GameStore s = new();
        GameSession session = NewTestSession();
        s.Set("g1", session);

        // Play a quick winning game
        (int, int)[] moves =
        [
            (0, 0), (0, 2),
            (3, 0), (1, 2),
            (1, 0), (2, 2),
            (4, 0), (3, 2),
            (2, 0),
        ];
        foreach ((int x, int y) in moves)
        {
            session.ApplyMove(x, y);
        }
        Assert.True(session.IsGameOver());

        int removed = s.CleanupCompleted();
        Assert.Equal(1, removed);
        Assert.Equal(0, s.Count());
    }

    [Fact]
    public void StoreCleanupCompletedSkipsActiveGame()
    {
        GameStore s = new();
        GameSession session = NewTestSession();
        s.Set("g1", session);
        Assert.False(session.IsGameOver());

        int removed = s.CleanupCompleted();
        Assert.Equal(0, removed);
        Assert.Equal(1, s.Count());
    }

    [Fact]
    public void StoreConcurrentAccess()
    {
        GameStore s = new();
        Parallel.For(0, 100, n =>
        {
            string id = ((char)('a' + n % 26)).ToString();
            GameSession session = new("rapid", 300_000, 2, GameMode.PvP, null, null, () => 1);
            s.Set(id, session);
            s.TryGet(id, out _);
            if (n % 3 == 0)
            {
                s.Delete(id);
            }
        });
    }
}
