using Xunit;

namespace Caro.Api.Tests;

public class IntegrationTests
{
    [Fact]
    public async Task IntegrationFullGameFlow()
    {
        await using TestApi api = TestHostFactory.Create();

        // 1. Create game
        var (status, body) = await api.Client.PostJsonAsync("/api/game/new",
            """{"timeControl":"3+2","gameMode":"pvp"}""");
        Assert.Equal(200, status);
        string gameID = body.GameId();

        // 2. Make moves to create a winning line: Red plays (3,0)-(7,0),
        // Blue plays (0,0)-(6,1). Open Rule: Red's second move (6,0) must be
        // outside the 5x5 zone around first move (3,0).
        (int, int)[] moves =
        [
            (3, 0), (0, 0),
            (6, 0), (6, 1),
            (4, 0), (4, 1),
            (5, 0), (5, 1),
            (7, 0),
        ];
        foreach ((int x, int y) in moves)
        {
            var (moveStatus, moveBody) = await api.Client.PostJsonAsync(
                $"/api/game/{gameID}/move", $$"""{"x":{{x}},"y":{{y}}}""");
            Assert.Equal(200, moveStatus);
            body = moveBody;
        }

        // 3. Verify game over via GET
        var (getStatus, getBody) = await api.Client.GetJsonAsync("/api/game/" + gameID);
        Assert.Equal(200, getStatus);
        Assert.True(getBody.State().Bool("isGameOver"));

        // 4. Delete game
        var (delStatus, _) = await api.Client.DeleteJsonAsync("/api/game/" + gameID);
        Assert.Equal(200, delStatus);

        // 5. Verify deleted
        var (afterDelete, _) = await api.Client.GetJsonAsync("/api/game/" + gameID);
        Assert.Equal(404, afterDelete);
    }

    [Fact]
    public async Task IntegrationCreateUndoRedo()
    {
        await using TestApi api = TestHostFactory.Create();

        var (_, created) = await api.Client.PostJsonAsync("/api/game/new", "{}");
        string gameID = created.GameId();

        var (_, moved) = await api.Client.PostJsonAsync($"/api/game/{gameID}/move", """{"x":7,"y":7}""");
        Assert.Equal(1, ((System.Text.Json.JsonElement)moved.State()["moveNumber"]!).GetDouble());

        var (_, undone) = await api.Client.PostJsonAsync($"/api/game/{gameID}/undo", "{}");
        var state = undone.State();
        Assert.Equal(0, state.Num("moveNumber"));
        Assert.Equal("red", state["currentPlayer"]!.ToString());

        var (_, redone) = await api.Client.PostJsonAsync($"/api/game/{gameID}/move", """{"x":7,"y":7}""");
        Assert.Equal(1, ((System.Text.Json.JsonElement)redone.State()["moveNumber"]!).GetDouble());
    }

    [Fact]
    public async Task IntegrationConcurrentGames()
    {
        await using TestApi api = TestHostFactory.Create();

        List<string> gameIds = [];
        await Task.WhenAll(Enumerable.Range(0, 4).Select(async n =>
        {
            var (status, body) = await api.Client.PostJsonAsync("/api/game/new", """{"timeControl":"1+0"}""");
            if (status == 200)
            {
                lock (gameIds)
                {
                    gameIds.Add(body.GameId());
                }
            }
        }));
        foreach (string id in gameIds)
        {
            Assert.NotEmpty(id);
        }
    }
}
