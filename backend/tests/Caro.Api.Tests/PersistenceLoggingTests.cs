using Caro.Api;
using Caro.Domain;
using Caro.Persistence;
using Xunit;

namespace Caro.Api.Tests;

/// <summary>
/// Match persistence through the handlers: completions on the happy path,
/// and store failures that must be logged and swallowed, never surfaced to
/// the game flow.
/// </summary>
public class PersistenceLoggingTests
{
    private static string TempDbPath()
    {
        string dir = Path.Combine(Path.GetTempPath(), Guid.NewGuid().ToString("N"));
        Directory.CreateDirectory(dir);
        return Path.Combine(dir, "test.db");
    }

    private static async Task<string> NewGameWithMoveAsync(TestApi api, string request)
    {
        var (_, created) = await api.Client.PostJsonAsync("/api/game/new", request);
        string gameID = created.GameId();
        var (moveStatus, _) = await api.Client.PostJsonAsync($"/api/game/{gameID}/move",
            """{"x":7,"y":7}""");
        Assert.Equal(200, moveStatus);
        return gameID;
    }

    [Fact]
    public async Task HumanWinCompletesRecordedGame()
    {
        using MatchStore ms = new(TempDbPath());
        await using TestApi api = TestHostFactory.Create(matches: ms);
        var (_, created) = await api.Client.PostJsonAsync("/api/game/new",
            """{"timeControl":"1+0","gameMode":"pvp"}""");
        string gameID = created.GameId();

        (int, int)[] win =
        [
            (3, 0), (0, 0),
            (6, 0), (6, 1),
            (4, 0), (4, 1),
            (5, 0), (5, 1),
            (7, 0),
        ];
        foreach ((int x, int y) in win)
        {
            var (moveStatus, _) = await api.Client.PostJsonAsync(
                $"/api/game/{gameID}/move", $$"""{"x":{{x}},"y":{{y}}}""");
            Assert.Equal(200, moveStatus);
        }

        GameRecord? record = ms.GetGame(gameID);
        Assert.NotNull(record);
        Assert.Equal("red", record!.Winner);
        Assert.Equal(9, record.MoveCount);
        Assert.NotNull(record.CompletedAt);
        Assert.Equal(9, ms.GetMoves(gameID).Count);
    }

    [Fact]
    public async Task ClosedStoreFailuresAreSwallowed()
    {
        MatchStore ms = new(TempDbPath());
        ms.Close();
        using (ms)
        {
            await using TestApi api = TestHostFactory.Create(matches: ms);

            // Create: the game still starts even though the row cannot insert.
            var (createStatus, created) = await api.Client.PostJsonAsync("/api/game/new",
                """{"timeControl":"1+0","gameMode":"pvp"}""");
            Assert.Equal(200, createStatus);
            string gameID = created.GameId();

            // Human move: RecordMove fails, the move still lands.
            var (moveStatus, _) = await api.Client.PostJsonAsync($"/api/game/{gameID}/move",
                """{"x":7,"y":7}""");
            Assert.Equal(200, moveStatus);

            // Game-ending human move: both RecordMove and CompleteGame fail.
            foreach ((int x, int y) in new[] { (3, 0), (0, 0), (6, 0), (6, 1), (4, 0), (4, 1), (5, 0), (5, 1) })
            {
                await api.Client.PostJsonAsync($"/api/game/{gameID}/move",
                    $$"""{"x":{{x}},"y":{{y}}}""");
            }
            var (finalStatus, finalBody) = await api.Client.PostJsonAsync($"/api/game/{gameID}/move",
                """{"x":7,"y":0}""");
            Assert.Equal(200, finalStatus);
            Assert.True(finalBody.State().Bool("isGameOver"));

            // Delete: CompleteGame fails, the game is still removed.
            var (deleteStatus, _) = await api.Client.DeleteJsonAsync("/api/game/" + gameID);
            Assert.Equal(200, deleteStatus);
            var (afterDelete, _) = await api.Client.GetJsonAsync("/api/game/" + gameID);
            Assert.Equal(404, afterDelete);
        }
    }

    [Fact]
    public async Task ClosedStoreAIMoveStillLands()
    {
        MatchStore ms = new(TempDbPath());
        ms.Close();
        using (ms)
        {
            await using TestApi api = TestHostFactory.Create(matches: ms);
            string gameID = await NewGameWithMoveAsync(api,
                """{"timeControl":"1+0","gameMode":"pvai","blueDifficulty":1}""");

            var (aiStatus, body) = await api.Client.PostJsonAsync($"/api/game/{gameID}/ai-move", "{}");
            Assert.Equal(200, aiStatus);
            Assert.Equal(2, body.State().Num("moveNumber"));
        }
    }

    [Fact]
    public async Task AIMoveWinCompletesRecordedGame()
    {
        using MatchStore ms = new(TempDbPath());
        await using TestApi api = TestHostFactory.Create(matches: ms);
        var (_, created) = await api.Client.PostJsonAsync("/api/game/new",
            """{"timeControl":"1+0","gameMode":"pvai","blueDifficulty":1}""");
        string gameID = created.GameId();

        // Stage a board where the engine's win is one move away; the AI move
        // then finishes the game and the completion is recorded.
        Assert.True(api.Store.TryGet(gameID, out GameSession session));
        Board staged = Board.NewBoard()
            .PlaceStone(0, 0, Player.Blue)
            .PlaceStone(1, 0, Player.Blue)
            .PlaceStone(2, 0, Player.Blue)
            .PlaceStone(3, 0, Player.Blue)
            .PlaceStone(10, 10, Player.Red)
            .PlaceStone(11, 11, Player.Red);
        session.InstallBoardForTest(staged, 6, Player.Blue);

        var (aiStatus, body) = await api.Client.PostJsonAsync($"/api/game/{gameID}/ai-move", "{}");
        Assert.Equal(200, aiStatus);
        Assert.True(body.State().Bool("isGameOver"));
        Assert.Equal("blue", body.State()["winner"]!.ToString());

        GameRecord? record = ms.GetGame(gameID);
        Assert.NotNull(record);
        Assert.Equal("blue", record!.Winner);
        Assert.Equal(7, record.MoveCount);

        // Blue made the winning move; the persisted row must say so. The
        // derivation from the post-move CurrentPlayer inverted blue's
        // game-ending moves in every archived blue-winner game
        // (docs/artifacts/tournaments/ANOMALIES.md, finding 6).
        List<MoveRecord> moves = ms.GetMoves(gameID);
        Assert.Equal("blue", moves[^1].Player);
    }

    [Fact]
    public async Task FinishedDrawDeleteKeepsNoneWinner()
    {
        using MatchStore ms = new(TempDbPath());
        await using TestApi api = TestHostFactory.Create(matches: ms);
        var (_, created) = await api.Client.PostJsonAsync("/api/game/new",
            """{"timeControl":"1+0","gameMode":"pvp"}""");
        string gameID = created.GameId();
        Assert.True(api.Store.TryGet(gameID, out GameSession session));

        // 2x2-block checkerboard: same-color runs stay far below five in
        // every direction, so filling the last cell is a board-full draw.
        Board staged = Board.NewBoard();
        for (int x = 0; x < Constants.Board.Size; x++)
        {
            for (int y = 0; y < Constants.Board.Size; y++)
            {
                if (x == Constants.Board.Size - 1 && y == Constants.Board.Size - 1)
                {
                    continue;
                }
                Player color = (x / 2 + y / 2) % 2 == 0 ? Player.Red : Player.Blue;
                staged = staged.PlaceStone(x, y, color);
            }
        }
        Assert.False(WinDetector.CheckWin(staged).HasWinner);
        Player lastCellSafeFor = WinDetector.CheckWin(
            staged.PlaceStone(Constants.Board.Size - 1, Constants.Board.Size - 1, Player.Red)).HasWinner
            ? Player.Blue
            : Player.Red;
        Assert.False(WinDetector.CheckWin(
            staged.PlaceStone(Constants.Board.Size - 1, Constants.Board.Size - 1, lastCellSafeFor)).HasWinner);
        session.InstallBoardForTest(staged, 255, lastCellSafeFor);

        var (moveStatus, body) = await api.Client.PostJsonAsync($"/api/game/{gameID}/move",
            $$"""{"x":{{Constants.Board.Size - 1}},"y":{{Constants.Board.Size - 1}}}""");
        Assert.Equal(200, moveStatus);
        Assert.True(body.State().Bool("isGameOver"));
        Assert.Equal("none", body.State()["winner"]!.ToString());

        GameRecord? drawn = ms.GetGame(gameID);
        Assert.NotNull(drawn);
        Assert.Equal("none", drawn!.Winner);

        // Deleting a finished game must not relabel the draw as abandoned.
        var (deleteStatus, _) = await api.Client.DeleteJsonAsync("/api/game/" + gameID);
        Assert.Equal(200, deleteStatus);
        Assert.Equal("none", ms.GetGame(gameID)!.Winner);
    }

    [Fact]
    public async Task LiveGameDeleteRecordsAbandoned()
    {
        using MatchStore ms = new(TempDbPath());
        await using TestApi api = TestHostFactory.Create(matches: ms);
        var (_, created) = await api.Client.PostJsonAsync("/api/game/new",
            """{"timeControl":"1+0","gameMode":"pvp"}""");
        string gameID = created.GameId();
        var (moveStatus, _) = await api.Client.PostJsonAsync($"/api/game/{gameID}/move",
            """{"x":7,"y":7}""");
        Assert.Equal(200, moveStatus);

        var (deleteStatus, _) = await api.Client.DeleteJsonAsync("/api/game/" + gameID);
        Assert.Equal(200, deleteStatus);

        GameRecord? record = ms.GetGame(gameID);
        Assert.NotNull(record);
        Assert.Equal("abandoned", record!.Winner);
    }
}
