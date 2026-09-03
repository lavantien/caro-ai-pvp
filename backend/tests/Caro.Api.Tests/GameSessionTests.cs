using Caro.Api;
using Caro.Domain;
using Caro.Engine;
using Xunit;
using static Caro.Api.GameSession;

namespace Caro.Api.Tests;

public class GameSessionTests
{
    internal static GameSession NewTestSession() =>
        new("rapid", 300_000, 2, GameMode.AivAI, redDiff: 3, blueDiff: null, activeGameCount: () => 1);

    [Fact]
    public void NewGameSessionInitialState()
    {
        GameSession s = NewTestSession();
        GameResponse resp = s.GetResponse();

        Assert.Equal("none", resp.Winner);
        Assert.False(resp.IsGameOver);
        Assert.Equal(0, resp.MoveNumber);
        Assert.Equal("red", resp.CurrentPlayer);
        Assert.Equal("rapid", resp.TimeControl);
        Assert.Equal(300, resp.InitialTime);
        Assert.Equal(2, resp.Increment);
        Assert.Equal("aivai", resp.GameMode);
        Assert.InRange(resp.RedTimeRemaining, 299.99, 300.01);
        Assert.InRange(resp.BlueTimeRemaining, 299.99, 300.01);
        Assert.Equal(3, resp.RedDifficulty);
        Assert.Null(resp.BlueDifficulty);
    }

    [Fact]
    public void SessionApplyMove()
    {
        GameSession s = NewTestSession();
        GameResponse resp = s.ApplyMove(7, 7);
        Assert.Equal(1, resp.MoveNumber);
        Assert.Equal("blue", resp.CurrentPlayer);
    }

    [Fact]
    public void SessionApplyMoveOutOfBounds()
    {
        GameSession s = NewTestSession();
        Assert.Throws<PositionBoundsException>(() => s.ApplyMove(99, 99));
    }

    [Fact]
    public void SessionApplyMoveAfterGameOver()
    {
        GameSession s = NewTestSession();
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
            s.ApplyMove(x, y);
        }
        Assert.True(s.IsGameOver());
        Assert.Throws<GameOverException>(() => s.ApplyMove(5, 5));
    }

    [Fact]
    public void SessionExtractForAI()
    {
        GameSession s = NewTestSession();
        (Board board, Player player, bool isOver, long timeMs, int inc, int moveNum, int? diff) = s.ExtractForAI();
        Assert.Equal(Player.Red, player);
        Assert.False(isOver);
        Assert.Equal(300_000L, timeMs);
        Assert.Equal(2, inc);
        Assert.Equal(0, moveNum);
        Assert.Equal(3, diff);
    }

    [Fact]
    public void SessionExtractForAIBlue()
    {
        GameSession s = NewTestSession();
        s.ApplyMove(7, 7);
        (_, Player player, _, long timeMs, _, _, int? diff) = s.ExtractForAI();
        Assert.Equal(Player.Blue, player);
        Assert.Equal(300_000L, timeMs);
        Assert.Null(diff);
    }

    [Fact]
    public void SessionGetOrCreateAI()
    {
        GameSession s = NewTestSession();
        MinimaxAI ai = s.GetOrCreateAI(Player.Red);
        Assert.NotNull(ai);
        MinimaxAI ai2 = s.GetOrCreateAI(Player.Red);
        Assert.Same(ai, ai2);
        MinimaxAI ai3 = s.GetOrCreateAI(Player.Blue);
        Assert.NotNull(ai3);
    }

    [Fact]
    public void SessionDisposeAI()
    {
        GameSession s = NewTestSession();
        s.GetOrCreateAI(Player.Red);
        s.GetOrCreateAI(Player.Blue);
        s.DisposeAI();
        MinimaxAI ai = s.GetOrCreateAI(Player.Red);
        Assert.NotNull(ai);
    }

    [Fact]
    public void SessionUndoMove()
    {
        GameSession s = NewTestSession();
        s.ApplyMove(7, 7);
        Assert.Equal(1, s.GetResponse().MoveNumber);

        GameResponse resp = s.UndoLastMove();
        Assert.Equal(0, resp.MoveNumber);
        Assert.Equal("red", resp.CurrentPlayer);
    }

    [Fact]
    public void ApplyAIMoveRejectsStalePlayer()
    {
        GameSession s = new("1+0", 60_000, 0, GameMode.PvAI, null, null, () => 1);
        Assert.Throws<NotPlayerTurnException>(() => s.ApplyAIMove(8, 8, Player.Blue));

        s.ApplyAIMove(8, 8, Player.Red);
    }

    [Fact]
    public void UndoInPvAITakesBackFullTurn()
    {
        GameSession s = new("1+0", 60_000, 0, GameMode.PvAI, null, 5, () => 1);
        s.ApplyHumanMove(8, 8);
        s.ApplyAIMove(8, 9, Player.Blue);
        Assert.Equal(2, s.GetResponse().MoveNumber);

        GameResponse resp = s.UndoLastMove();
        Assert.Equal(0, resp.MoveNumber);
        Assert.Equal("red", resp.CurrentPlayer);
    }

    [Fact]
    public void UndoInPvpStaysSinglePly()
    {
        GameSession s = new("1+0", 60_000, 0, GameMode.PvP, null, null, () => 1);
        s.ApplyHumanMove(8, 8);
        s.ApplyHumanMove(8, 9);

        GameResponse resp = s.UndoLastMove();
        Assert.Equal(1, resp.MoveNumber);
    }

    [Fact]
    public void BoardFullEndsInDraw()
    {
        GameSession s = new("15+10", 900_000, 10, GameMode.PvP, null, null, () => 1);

        // Build a full board minus (15,15) directly: rows come out
        // monochrome (16-runs are overlines, never a win) and columns and
        // diagonals alternate, so no exactly-five can exist for either side.
        Board board = Board.NewBoard();
        int k = 0;
        for (int x = 0; x < Constants.Board.Size; x++)
        {
            for (int y = 0; y < Constants.Board.Size; y++)
            {
                if (x == Constants.Board.Size - 1 && y == Constants.Board.Size - 1)
                {
                    continue;
                }
                Player player = k % 2 == 0 ? Player.Red : Player.Blue;
                board = board.PlaceStone(x, y, player);
                k++;
            }
        }
        s.InstallBoardForTest(board, Constants.Board.MaxMoves - 1, Player.Red);

        GameResponse resp = s.ApplyHumanMove(Constants.Board.Size - 1, Constants.Board.Size - 1);
        Assert.True(resp.IsGameOver, "a full board must end the game");
        Assert.Equal("draw", resp.EndReason);
        Assert.Equal("none", resp.Winner);
    }

    [Fact]
    public void ClocksCountDownBetweenMoves()
    {
        GameSession s = new("1+0", 60_000, 0, GameMode.PvP, null, null, () => 1);
        s.ApplyHumanMove(8, 8);

        double before = s.GetResponse().BlueTimeRemaining;
        s.BackdateLastMoveForTest(TimeSpan.FromSeconds(10));
        double after = s.GetResponse().BlueTimeRemaining;

        Assert.InRange(before - after, 8.5, 11.5);
    }

    [Fact]
    public void SessionTimesOutCurrentPlayer()
    {
        GameSession s = new("1+0", 60_000, 0, GameMode.PvAI, null, null, () => 1);
        s.ApplyMove(8, 8);

        s.BackdateLastMoveForTest(TimeSpan.FromHours(2));

        Assert.ThrowsAny<CaroException>(() => s.ApplyMove(8, 9));
        GameResponse resp = s.GetResponse();
        Assert.True(resp.IsGameOver);
        Assert.Equal("red", resp.Winner);
        Assert.Equal("timeout", resp.EndReason);
        Assert.Equal(0.0, resp.BlueTimeRemaining);
    }

    [Fact]
    public void SessionTimeoutOnRead()
    {
        GameSession s = new("1+0", 60_000, 0, GameMode.PvAI, null, null, () => 1);
        s.BackdateLastMoveForTest(TimeSpan.FromHours(3));

        GameResponse resp = s.GetResponse();
        Assert.True(resp.IsGameOver);
        Assert.Equal("blue", resp.Winner);
        Assert.Equal("timeout", resp.EndReason);
    }

    [Fact]
    public void SessionNoTimeoutWhileClockRuns()
    {
        GameSession s = new("1+0", 60_000, 0, GameMode.PvAI, null, null, () => 1);
        GameResponse resp = s.GetResponse();
        Assert.False(resp.IsGameOver);
        Assert.Equal(string.Empty, resp.EndReason);
    }

    [Fact]
    public void ApplyHumanMoveGuardsAivAI()
    {
        GameSession s = new("1+0", 60_000, 0, GameMode.AivAI, null, 5, () => 1);
        Assert.Throws<NotPlayerTurnException>(() => s.ApplyHumanMove(8, 8));
    }

    [Fact]
    public void ApplyHumanMoveGuardsPvAIEngineTurn()
    {
        GameSession s = new("1+0", 60_000, 0, GameMode.PvAI, null, 5, () => 1);
        s.ApplyHumanMove(8, 8); // red is human: allowed
        Assert.Throws<NotPlayerTurnException>(() => s.ApplyHumanMove(8, 9)); // blue is the engine
    }

    [Fact]
    public void ApplyHumanMoveBothSidesInPvp()
    {
        GameSession s = new("1+0", 60_000, 0, GameMode.PvP, null, null, () => 1);
        s.ApplyHumanMove(8, 8);
        s.ApplyHumanMove(8, 9);
    }

    [Fact]
    public void DifficultyDepthCapsMonotone()
    {
        int prev = 0;
        for (int level = 1; level <= 5; level++)
        {
            DifficultyProfile p = Difficulty.GetDifficultyProfile(level);
            Assert.True(p.MaxDepth >= prev);
            Assert.True(p.MaxDepth <= Constants.Search.AbsoluteMaxDepth);
            if (level == 3)
            {
                Assert.True(p.UseVCF && p.VCFDepth > 0);
            }
            prev = p.MaxDepth;
        }
    }

    [Fact]
    public void AllocateTimeNeverNegative()
    {
        foreach (long remaining in new long[] { 0, 1, 10, 50, 100 })
        {
            TimeAllocation alloc = TimeManager.AllocateTime(remaining, 0, 10);
            Assert.True(alloc.HardBoundMs >= 0);
        }
        TimeAllocation live = TimeManager.AllocateTime(1000, 0, 10);
        Assert.True(live.HardBoundMs > 0);
    }
}

public class SessionOpeningTests
{
    private static (int RedX, int RedY, int BlueX, int BlueY) OpeningStones(GameSession s)
    {
        int redX = -1, redY = -1, blueX = -1, blueY = -1;
        for (int x = 0; x < Constants.Board.Size; x++)
        {
            for (int y = 0; y < Constants.Board.Size; y++)
            {
                switch (s.GameForTest.Board.GetPlayerAt(x, y))
                {
                    case Player.Red:
                        redX = x;
                        redY = y;
                        break;
                    case Player.Blue:
                        blueX = x;
                        blueY = y;
                        break;
                }
            }
        }
        return (redX, redY, blueX, blueY);
    }

    [Fact]
    public void RandomOpeningDeterministicPerSeed()
    {
        GameSession s1 = new("3+0", 180_000, 0, GameMode.AivAI, null, null, () => 1);
        s1.ApplyRandomOpening(42);
        (int r1x, int r1y, int b1x, int b1y) = OpeningStones(s1);

        GameSession s2 = new("3+0", 180_000, 0, GameMode.AivAI, null, null, () => 1);
        s2.ApplyRandomOpening(42);
        (int r2x, int r2y, int b2x, int b2y) = OpeningStones(s2);

        Assert.Equal((r1x, r1y, b1x, b1y), (r2x, r2y, b2x, b2y));

        // Red starts near the center, blue responds nearby.
        Assert.InRange(r1x, 4, 11);
        Assert.InRange(r1y, 4, 11);
        int cheb = Math.Max(Math.Abs(b1x - r1x), Math.Abs(b1y - r1y));
        Assert.True(cheb <= 3, "blue's reply must stay local to red's first stone");
        Assert.Equal(2, s1.GameForTest.MoveNumber);
        Assert.Equal(Player.Red, s1.GameForTest.CurrentPlayer);
    }

    [Fact]
    public void RandomOpeningVariesAcrossSeeds()
    {
        HashSet<int> seen = [];
        for (long seed = 1; seed <= 40; seed++)
        {
            GameSession s = new("3+0", 180_000, 0, GameMode.AivAI, null, null, () => 1);
            s.ApplyRandomOpening(seed);
            (int rx, int ry, _, _) = OpeningStones(s);
            seen.Add(ry * Constants.Board.Size + rx);
        }
        Assert.True(seen.Count > 10, "40 seeds must produce a varied set of openings");
    }
}
