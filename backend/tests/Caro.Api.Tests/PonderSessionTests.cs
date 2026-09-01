using Caro.Api;
using Caro.Domain;
using Caro.Engine;
using Xunit;
using static Caro.Api.Tests.GameSessionTests;

namespace Caro.Api.Tests;

public class PonderSessionTests
{
    private static GameSession NewPonderSession(GameMode mode, int? red, int? blue, long capMs = 300)
    {
        GameSession s = new("1+0", 60_000, 0, mode, red, blue, () => 1);
        s.SetPonderTimeCapForTest(capMs);
        return s;
    }

    // PlaySearchedAIMove computes a real searched move for player (so the TT
    // carries a prediction) and applies it, mirroring the handler flow.
    private static void PlaySearchedAIMove(GameSession s, Player player)
    {
        MinimaxAI ai = s.GetOrCreateAI(player);
        (Board board, _, bool over, long timeMs, int inc, int moveNum, int? diff) = s.ExtractForAI();
        Assert.False(over);
        Assert.NotNull(diff);

        (int x, int y, _) = ai.GetBestMove(board, player, new SearchOptions
        {
            // Generous absolute budget: the property under test is the ponder
            // lifecycle, and the root search must always complete at least
            // one depth (storing a TT prediction) even under coverage
            // instrumentation.
            TimeRemainingMs = 600_000,
            IncrementMs = inc * 1000L,
            MoveNumber = moveNum,
            ThreadCount = 1,
            TimeFraction = 0.1,
            MaxDepth = 4,
        }, CancellationToken.None);
        Assert.True(x >= 0);

        s.ApplyAIMove(x, y, player);
    }

    // LegalAlternativeReply returns an empty cell that satisfies the open
    // rule and differs from the predicted reply.
    private static Position LegalAlternativeReply(Board b, Position predicted)
    {
        for (int y = 0; y < Constants.BoardSize; y++)
        {
            for (int x = 0; x < Constants.BoardSize; x++)
            {
                Position p = new(x, y);
                if (p == predicted || !b.IsEmptyAt(x, y))
                {
                    continue;
                }
                if (OpenRule.IsValidSecondMove(b, x, y))
                {
                    return p;
                }
            }
        }
        Assert.Fail("no alternative legal reply found");
        return default;
    }

    private static bool Eventually(Func<bool> condition, int timeoutMs = 2000, int pollMs = 10)
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

    [Fact]
    public void PonderCapDerivedFromOpponentClock()
    {
        GameSession s = NewPonderSession(GameMode.AivAI, 5, null, capMs: 0);
        PlaySearchedAIMove(s, Player.Red);
        Assert.NotNull(s.ActivePonderForTest);

        Assert.True(s.ActivePonderForTest!.TimeCapMs > 55_000);
        Assert.True(s.ActivePonderForTest.TimeCapMs <= 60_000);

        GameSession s2 = NewPonderSession(GameMode.AivAI, 5, null, capMs: 0);
        s2.SetClockForTest(Player.Blue, 5_000);
        PlaySearchedAIMove(s2, Player.Red);
        Assert.NotNull(s2.ActivePonderForTest);
        Assert.True(s2.ActivePonderForTest!.TimeCapMs > 4_000);
        Assert.True(s2.ActivePonderForTest.TimeCapMs <= 5_000);
    }

    [Fact]
    public void PonderStartsAfterAIMoveAivAI()
    {
        GameSession s = NewPonderSession(GameMode.AivAI, 5, null);
        PlaySearchedAIMove(s, Player.Red);
        Assert.NotNull(s.ActivePonderForTest);
        Assert.Equal(Player.Red, s.ActivePonderForTest!.Player);
    }

    [Fact]
    public void PonderStartsAfterAIMovePvAI()
    {
        GameSession s = NewPonderSession(GameMode.PvAI, null, 5);
        s.ApplyHumanMove(7, 7);

        PlaySearchedAIMove(s, Player.Blue);
        Assert.NotNull(s.ActivePonderForTest);
        Assert.Equal(Player.Blue, s.ActivePonderForTest!.Player);

        Position pred = s.ActivePonderForTest.PredictedReply;
        Position alt = LegalAlternativeReply(s.GameForTest.Board, pred);
        s.ApplyHumanMove(alt.X, alt.Y);
        Assert.Null(s.ActivePonderForTest);
        Assert.NotNull(s.PendingPonderForTest);
        Assert.False(s.PendingPonderForTest!.Hit);
    }

    [Fact]
    public void PonderDisabledForLowerLevels()
    {
        GameSession s = NewPonderSession(GameMode.AivAI, 4, null);
        PlaySearchedAIMove(s, Player.Red);
        Assert.Null(s.ActivePonderForTest);
    }

    [Fact]
    public void PonderKillSwitch()
    {
        bool prev = GameSession.PonderEnvDisabled;
        GameSession.PonderEnvDisabled = true;
        try
        {
            GameSession s = NewPonderSession(GameMode.AivAI, 5, null);
            PlaySearchedAIMove(s, Player.Red);
            Assert.Null(s.ActivePonderForTest);
        }
        finally
        {
            GameSession.PonderEnvDisabled = prev;
        }
    }

    [Fact]
    public void PonderHitRecordedOnPredictedReply()
    {
        GameSession s = NewPonderSession(GameMode.AivAI, 5, null);
        PlaySearchedAIMove(s, Player.Red);
        Position pred = s.ActivePonderForTest!.PredictedReply;

        Assert.True(Eventually(() => s.RedAIFromTest?.PonderActive() == false),
            "the short cap lets the ponder finish");

        s.ApplyAIMove(pred.X, pred.Y, Player.Blue);
        Assert.NotNull(s.PendingPonderForTest);
        Assert.Equal(Player.Red, s.PendingPonderForTest!.Player);
        Assert.True(s.PendingPonderForTest.Hit);
    }

    [Fact]
    public void PonderMissRecordedAsNotHit()
    {
        GameSession s = NewPonderSession(GameMode.AivAI, 5, null);
        PlaySearchedAIMove(s, Player.Red);
        Position pred = s.ActivePonderForTest!.PredictedReply;
        Position alt = LegalAlternativeReply(s.GameForTest.Board, pred);

        s.ApplyAIMove(alt.X, alt.Y, Player.Blue);
        Assert.NotNull(s.PendingPonderForTest);
        Assert.False(s.PendingPonderForTest!.Hit);
        Assert.Null(s.ActivePonderForTest);
    }

    [Fact]
    public void PonderIncompleteIsNotHit()
    {
        // Forced zero budget: no depth can ever complete.
        GameSession s = NewPonderSession(GameMode.AivAI, 5, null, capMs: -1);
        PlaySearchedAIMove(s, Player.Red);
        Position pred = s.ActivePonderForTest!.PredictedReply;

        Assert.True(Eventually(() => s.RedAIFromTest?.PonderActive() == false, pollMs: 5));

        s.ApplyAIMove(pred.X, pred.Y, Player.Blue);
        Assert.NotNull(s.PendingPonderForTest);
        Assert.False(s.PendingPonderForTest!.Hit);
    }

    [Fact]
    public void PonderNotStartedWhenPredictionAbsent()
    {
        GameSession s = NewPonderSession(GameMode.AivAI, 5, null);
        s.GetOrCreateAI(Player.Red);
        s.ApplyAIMove(7, 7, Player.Red);
        Assert.Null(s.ActivePonderForTest);
    }

    [Fact]
    public void UndoInvalidatesPonder()
    {
        GameSession s = NewPonderSession(GameMode.AivAI, 5, null);
        PlaySearchedAIMove(s, Player.Red);
        Position pred = s.ActivePonderForTest!.PredictedReply;
        Assert.True(Eventually(() => s.RedAIFromTest?.PonderActive() == false));

        s.ApplyAIMove(pred.X, pred.Y, Player.Blue);
        Assert.NotNull(s.PendingPonderForTest);

        s.UndoLastMove();
        Assert.Null(s.PendingPonderForTest);
        Assert.Null(s.ActivePonderForTest);
        Assert.False(s.RedAIFromTest!.PonderActive());
    }

    [Fact]
    public void DisposeAIDuringPonderDrains()
    {
        GameSession s = NewPonderSession(GameMode.AivAI, 5, null);
        PlaySearchedAIMove(s, Player.Red);
        MinimaxAI ai = s.RedAIFromTest!;
        Assert.True(ai.PonderActive());

        s.DisposeAI();
        Assert.False(ai.PonderActive());
        Assert.Null(s.ActivePonderForTest);
        Assert.Null(s.PendingPonderForTest);
    }

    [Fact]
    public void FlagFallStopsPonder()
    {
        GameSession s = NewPonderSession(GameMode.AivAI, 5, null);
        PlaySearchedAIMove(s, Player.Red);
        MinimaxAI ai = s.RedAIFromTest!;
        Assert.True(ai.PonderActive());

        s.BackdateLastMoveForTest(TimeSpan.FromHours(2));

        GameResponse resp = s.GetResponse();
        Assert.True(resp.IsGameOver);
        Assert.Equal("timeout", resp.EndReason);
        Assert.False(ai.PonderActive());
    }

    [Fact]
    public void StoreDeleteDrainsPonder()
    {
        GameStore store = new();
        GameSession s = NewPonderSession(GameMode.AivAI, 5, null);
        PlaySearchedAIMove(s, Player.Red);
        MinimaxAI ai = s.RedAIFromTest!;
        Assert.True(ai.PonderActive());

        store.Set("g1", s);
        store.Delete("g1");
        Assert.False(ai.PonderActive());
    }

    [Fact]
    public void BlueAIAndRedClockSeamsReachable()
    {
        GameSession s = NewPonderSession(GameMode.PvAI, null, 5);
        Assert.NotNull(s.GetOrCreateAI(Player.Blue));
        Assert.NotNull(s.BlueAIFromTest);

        s.SetClockForTest(Player.Red, 12_345);
        s.SetClockForTest(Player.Blue, 999);
        (Board _, Player player, _, long timeMs, _, _, _) = s.ExtractForAI();
        Assert.Equal(Player.Red, player);
        Assert.Equal(12_345, timeMs);
    }

    [Fact]
    public void TakePonderInfoConsumesOnceAndFiltersPlayer()
    {
        GameSession s = NewPonderSession(GameMode.AivAI, 5, null);
        PlaySearchedAIMove(s, Player.Red);
        Position alt = LegalAlternativeReply(s.GameForTest.Board, s.ActivePonderForTest!.PredictedReply);
        s.ApplyAIMove(alt.X, alt.Y, Player.Blue);
        Assert.NotNull(s.PendingPonderForTest);

        // Right player: consumed exactly once.
        (SearchStats stats, bool hit, bool had) = s.TakePonderInfo(Player.Red);
        Assert.True(had);
        Assert.False(hit);
        Assert.True(stats.NodesSearched >= 0);
        (_, _, had) = s.TakePonderInfo(Player.Red);
        Assert.False(had);
    }

    [Fact]
    public void TakePonderInfoForWrongPlayerStillConsumes()
    {
        GameSession s = NewPonderSession(GameMode.AivAI, 5, null);
        PlaySearchedAIMove(s, Player.Red);
        Position alt = LegalAlternativeReply(s.GameForTest.Board, s.ActivePonderForTest!.PredictedReply);
        s.ApplyAIMove(alt.X, alt.Y, Player.Blue);
        Assert.NotNull(s.PendingPonderForTest);

        // Consume-once holds for any caller: a wrong-player take yields
        // nothing and drops the staged info.
        (SearchStats _, bool _, bool had) = s.TakePonderInfo(Player.Blue);
        Assert.False(had);
        Assert.Null(s.PendingPonderForTest);
    }

    [Fact]
    public async Task HandlerAnnotatesAIMoveWithPonderInfo()
    {
        await using TestApi api = TestHostFactory.Create();
        var (_, created) = await api.Client.PostJsonAsync("/api/game/new",
            """{"timeControl":"1+0","gameMode":"pvai","blueDifficulty":5}""");
        string gameID = created.GameId();
        Assert.True(api.Store.TryGet(gameID, out GameSession session));
        session.SetPonderTimeCapForTest(300);

        // Human opens; the engine answers and starts pondering on the reply.
        var (openStatus, _) = await api.Client.PostJsonAsync($"/api/game/{gameID}/move",
            """{"x":7,"y":7}""");
        Assert.Equal(200, openStatus);

        var (firstAI, _) = await api.Client.PostJsonAsync($"/api/game/{gameID}/ai-move", "{}");
        Assert.Equal(200, firstAI);
        Assert.True(Eventually(() => session.ActivePonderForTest != null), "ponder starts after the engine move");

        // Human moves again: the ponder stops and stages its info.
        var (replyStatus, _) = await api.Client.PostJsonAsync($"/api/game/{gameID}/move",
            """{"x":0,"y":0}""");
        Assert.Equal(200, replyStatus);
        Assert.NotNull(session.PendingPonderForTest);

        // The next ai-move consumes the ponder info for its statline row.
        var (secondAI, body) = await api.Client.PostJsonAsync($"/api/game/{gameID}/ai-move", "{}");
        Assert.Equal(200, secondAI);
        Assert.Equal(4, body.State().Num("moveNumber"));
        Assert.Null(session.PendingPonderForTest);
    }
}
