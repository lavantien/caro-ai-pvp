using System.Text.Json;
using Caro.Api;
using Caro.Domain;
using Caro.Engine;
using Caro.Persistence;
using Xunit;

namespace Caro.Api.Tests;

public class HandlerTests
{
    [Fact]
    public async Task CreateGameDefault()
    {
        await using TestApi api = TestHostFactory.Create();
        var (status, body) = await api.Client.PostJsonAsync("/api/game/new", "{}");
        Assert.Equal(200, status);
        Assert.NotEmpty(body.GameId());
        var state = body.State();
        Assert.Equal("red", state["currentPlayer"]!.ToString());
        Assert.Equal("7+5", state["timeControl"]!.ToString());
    }

    [Fact]
    public async Task CreateGameBlitz()
    {
        await using TestApi api = TestHostFactory.Create();
        var (_, body) = await api.Client.PostJsonAsync("/api/game/new",
            """{"timeControl":"3+2","gameMode":"aivai","difficulty":3}""");
        var state = body.State();
        Assert.Equal("3+2", state["timeControl"]!.ToString());
        Assert.Equal("aivai", state["gameMode"]!.ToString());
    }

    [Fact]
    public async Task CreateGameThreeZero()
    {
        await using TestApi api = TestHostFactory.Create();
        var (_, body) = await api.Client.PostJsonAsync("/api/game/new",
            """{"timeControl":"3+0","gameMode":"pvp"}""");
        var state = body.State();
        Assert.Equal("3+0", state["timeControl"]!.ToString());
        Assert.Equal(180, state.Num("initialTime"));
        Assert.Equal(0, state.Num("increment"));
    }

    [Fact]
    public async Task CreateGameInvalidDifficulty()
    {
        await using TestApi api = TestHostFactory.Create();
        var (status, _) = await api.Client.PostJsonAsync("/api/game/new", """{"difficulty":0}""");
        Assert.Equal(400, status);
    }

    [Fact]
    public async Task CreateGameTooMany()
    {
        await using TestApi api = TestHostFactory.Create();
        for (int i = 0; i < Constants.Limits.MaxConcurrentGames; i++)
        {
            var (status, _) = await api.Client.PostJsonAsync("/api/game/new", "{}");
            Assert.Equal(200, status);
        }

        var (finalStatus, _) = await api.Client.PostJsonAsync("/api/game/new", "{}");
        Assert.Equal(429, finalStatus);
    }

    [Fact]
    public async Task GetGameNotFound()
    {
        await using TestApi api = TestHostFactory.Create();
        var (status, _) = await api.Client.GetJsonAsync("/api/game/nonexistent");
        Assert.Equal(404, status);
    }

    [Fact]
    public async Task GetGameFound()
    {
        await using TestApi api = TestHostFactory.Create();
        var (_, created) = await api.Client.PostJsonAsync("/api/game/new", "{}");
        string gameID = created.GameId();

        var (status, _) = await api.Client.GetJsonAsync("/api/game/" + gameID);
        Assert.Equal(200, status);
    }

    [Fact]
    public async Task MakeMoveNotFound()
    {
        await using TestApi api = TestHostFactory.Create();
        var (status, _) = await api.Client.PostJsonAsync("/api/game/nonexistent/move", """{"x":7,"y":7}""");
        Assert.Equal(404, status);
    }

    [Fact]
    public async Task MakeMoveThenGet()
    {
        await using TestApi api = TestHostFactory.Create();
        var (_, created) = await api.Client.PostJsonAsync("/api/game/new", "{}");
        string gameID = created.GameId();

        var (status, body) = await api.Client.PostJsonAsync($"/api/game/{gameID}/move", """{"x":7,"y":7}""");
        Assert.Equal(200, status);
        var state = body.State();
        Assert.Equal(1, state.Num("moveNumber"));
    }

    [Fact]
    public async Task MakeMoveOccupied()
    {
        await using TestApi api = TestHostFactory.Create();
        var (_, created) = await api.Client.PostJsonAsync("/api/game/new", "{}");
        string gameID = created.GameId();

        var (first, _) = await api.Client.PostJsonAsync($"/api/game/{gameID}/move", """{"x":7,"y":7}""");
        Assert.Equal(200, first);

        var (again, _) = await api.Client.PostJsonAsync($"/api/game/{gameID}/move", """{"x":7,"y":7}""");
        Assert.Equal(400, again);
    }

    [Fact]
    public async Task MakeMoveInvalidJSON()
    {
        await using TestApi api = TestHostFactory.Create();
        var (_, created) = await api.Client.PostJsonAsync("/api/game/new", "{}");
        string gameID = created.GameId();

        var (status, _) = await api.Client.PostJsonAsync($"/api/game/{gameID}/move", "not json");
        Assert.Equal(400, status);
    }

    [Fact]
    public async Task DeleteGame()
    {
        await using TestApi api = TestHostFactory.Create();
        var (_, created) = await api.Client.PostJsonAsync("/api/game/new", "{}");
        string gameID = created.GameId();

        var (delStatus, _) = await api.Client.DeleteJsonAsync("/api/game/" + gameID);
        Assert.Equal(200, delStatus);

        var (getAfter, _) = await api.Client.GetJsonAsync("/api/game/" + gameID);
        Assert.Equal(404, getAfter);
    }

    [Fact]
    public async Task DeleteGameNotFound()
    {
        await using TestApi api = TestHostFactory.Create();
        var (status, _) = await api.Client.DeleteJsonAsync("/api/game/nonexistent");
        Assert.Equal(404, status);
    }

    [Fact]
    public async Task UndoMove()
    {
        await using TestApi api = TestHostFactory.Create();
        var (_, created) = await api.Client.PostJsonAsync("/api/game/new", "{}");
        string gameID = created.GameId();

        var (moveStatus, _) = await api.Client.PostJsonAsync($"/api/game/{gameID}/move", """{"x":7,"y":7}""");
        Assert.Equal(200, moveStatus);

        var (undoStatus, body) = await api.Client.PostJsonAsync($"/api/game/{gameID}/undo", "{}");
        Assert.Equal(200, undoStatus);
        var state = body.State();
        Assert.Equal(0, state.Num("moveNumber"));
    }

    [Fact]
    public async Task UndoMoveNotFound()
    {
        await using TestApi api = TestHostFactory.Create();
        var (status, _) = await api.Client.PostJsonAsync("/api/game/nonexistent/undo", "{}");
        Assert.Equal(404, status);
    }

    [Fact]
    public async Task UndoMoveNoHistory()
    {
        await using TestApi api = TestHostFactory.Create();
        var (_, created) = await api.Client.PostJsonAsync("/api/game/new", "{}");
        string gameID = created.GameId();

        // Undo with no moves: NoMoves is not a mapped domain error, so 500.
        var (status, _) = await api.Client.PostJsonAsync($"/api/game/{gameID}/undo", "{}");
        Assert.Equal(500, status);
    }

    [Fact]
    public async Task MakeAIMove()
    {
        await using TestApi api = TestHostFactory.Create();
        var (_, created) = await api.Client.PostJsonAsync("/api/game/new",
            """{"gameMode":"pvai","blueDifficulty":1}""");
        string gameID = created.GameId();

        var (moveStatus, _) = await api.Client.PostJsonAsync($"/api/game/{gameID}/move", """{"x":7,"y":7}""");
        Assert.Equal(200, moveStatus);

        var (aiStatus, body) = await api.Client.PostJsonAsync($"/api/game/{gameID}/ai-move", "{}");
        Assert.Equal(200, aiStatus);
        var state = body.State();
        Assert.Equal(2, state.Num("moveNumber"));
    }

    [Fact]
    public async Task MakeAIMoveNotFound()
    {
        await using TestApi api = TestHostFactory.Create();
        var (status, _) = await api.Client.PostJsonAsync("/api/game/nonexistent/ai-move", "{}");
        Assert.Equal(404, status);
    }

    [Fact]
    public async Task CreateGameAIvAI()
    {
        await using TestApi api = TestHostFactory.Create();
        var (_, body) = await api.Client.PostJsonAsync("/api/game/new",
            """{"gameMode":"aivai","redDifficulty":3,"blueDifficulty":3}""");
        var state = body.State();
        Assert.Equal("aivai", state["gameMode"]!.ToString());
    }

    [Fact]
    public async Task ServerCORS()
    {
        await using TestApi api = TestHostFactory.Create();
        using HttpRequestMessage req = new(HttpMethod.Options, "/api/game/new");
        req.Headers.Add("Origin", "http://localhost:5173");
        HttpResponseMessage resp = await api.Client.SendAsync(req);
        Assert.Equal(System.Net.HttpStatusCode.NoContent, resp.StatusCode);
        Assert.Equal("http://localhost:5173",
            resp.Headers.GetValues("Access-Control-Allow-Origin").First());
    }

    [Fact]
    public async Task CleanupSurfaces()
    {
        await using TestApi api = TestHostFactory.Create();
        await api.Client.PostJsonAsync("/api/game/new", "{}");
        Assert.Equal(1, api.Store.Count());
        Assert.Equal(1, api.Store.ActiveGameCount());
        Assert.Equal(0, api.Store.CleanupCompleted());
        Assert.Equal(1, api.Store.CleanupAll());
        Assert.Equal(0, api.Store.Count());
    }

    [Fact]
    public async Task LogHumanAndAIMovesWithMatches()
    {
        string dir = Path.Combine(Path.GetTempPath(), Guid.NewGuid().ToString("N"));
        Directory.CreateDirectory(dir);
        using MatchStore ms = new(Path.Combine(dir, "test.db"));
        await using TestApi api = TestHostFactory.Create(matches: ms);
        var (_, created) = await api.Client.PostJsonAsync("/api/game/new",
            """{"gameMode":"pvai","blueDifficulty":1}""");
        string gameID = created.GameId();

        var (moveStatus, _) = await api.Client.PostJsonAsync($"/api/game/{gameID}/move", """{"x":7,"y":7}""");
        Assert.Equal(200, moveStatus);

        var (aiStatus, _) = await api.Client.PostJsonAsync($"/api/game/{gameID}/ai-move", "{}");
        Assert.Equal(200, aiStatus);
    }

    [Fact]
    public async Task DeleteGameWithMatchesRecordsAbandoned()
    {
        string dir = Path.Combine(Path.GetTempPath(), Guid.NewGuid().ToString("N"));
        Directory.CreateDirectory(dir);
        using MatchStore ms = new(Path.Combine(dir, "test.db"));
        await using TestApi api = TestHostFactory.Create(matches: ms);
        var (_, created) = await api.Client.PostJsonAsync("/api/game/new", "{}");
        string gameID = created.GameId();

        var (delStatus, _) = await api.Client.DeleteJsonAsync("/api/game/" + gameID);
        Assert.Equal(200, delStatus);

        GameRecord? record = ms.GetGame(gameID);
        Assert.NotNull(record);
        Assert.Equal("abandoned", record!.Winner);
    }

    [Fact]
    public void FormatStatlineNodes()
    {
        Assert.Equal("0", Statline.FormatStatlineNodes(0));
        Assert.Equal("42", Statline.FormatStatlineNodes(42));
        Assert.Equal("999", Statline.FormatStatlineNodes(999));
        Assert.Equal("1.5K", Statline.FormatStatlineNodes(1500));
        Assert.Equal("1.2M", Statline.FormatStatlineNodes(1_200_000));
        Assert.Equal("2.0M", Statline.FormatStatlineNodes(2_000_000));
    }

    [Fact]
    public void FormatStatlineNPS()
    {
        Assert.Equal("500", Statline.FormatStatlineNps(500));
        Assert.Equal("5K", Statline.FormatStatlineNps(5000));
        Assert.Equal("142K", Statline.FormatStatlineNps(142_000));
        Assert.Equal("1M", Statline.FormatStatlineNps(1_000_000));
    }

    [Fact]
    public void BuildMoveDetail()
    {
        GameResponse resp = new()
        {
            CurrentPlayer = "red",
            MoveNumber = 3,
            RedTimeRemaining = 415.5,
            BlueTimeRemaining = 300.2,
        };
        SearchStats stats = new()
        {
            DepthAchieved = 12,
            NodesSearched = 1_200_000,
            NodesPerSecond = 142_000,
            SearchScore = 340,
            TableHitRate = 0.87,
            AllocatedTimeMs = 12_000,
            ThreadCount = 4,
        };

        MoveDetailResponse detail = Statline.BuildMoveDetail(resp, "blue", 8, 8, stats, 10_800, ponderHit: false);

        Assert.Equal(2, detail.MoveNumber);
        Assert.Equal("blue", detail.Player);
        Assert.Equal("i9", detail.Pos);
        Assert.Equal(10_800L, detail.ThinkTimeMs);
        Assert.Equal(300_200L, detail.RemainingTimeMs);

        Assert.Contains("M 2", detail.Statline);
        Assert.Contains("blue", detail.Statline);
        Assert.Contains("i9", detail.Statline);
        Assert.Contains("d=12", detail.Statline);
        Assert.Contains("n=1.2M", detail.Statline);
        Assert.Contains("nps=142K", detail.Statline);
        Assert.Contains("tt= 87%", detail.Statline);
        Assert.Contains("s=+340", detail.Statline);
        Assert.Contains("t=10.8s", detail.Statline);

        Assert.Equal(12, detail.EngineStats.Depth);
        Assert.Equal(1_200_000L, detail.EngineStats.Nodes);
        Assert.Equal(142_000, detail.EngineStats.NPS);
        Assert.Equal(0.87, detail.EngineStats.TTHitRate);
        Assert.Equal(340, detail.EngineStats.Score);
        Assert.Equal(4, detail.EngineStats.Threads);
        Assert.Equal(12_000L, detail.EngineStats.AllocatedTimeMs);
        Assert.Equal("exact", detail.EngineStats.MoveType);
    }

    [Fact]
    public void OpponentOf()
    {
        Assert.Equal("blue", Statline.OpponentOf("red"));
        Assert.Equal("red", Statline.OpponentOf("blue"));
        Assert.Equal("red", Statline.OpponentOf("other"));
    }

    [Fact]
    public async Task MakeAIMoveReturnsLastMove()
    {
        await using TestApi api = TestHostFactory.Create();
        var (_, created) = await api.Client.PostJsonAsync("/api/game/new",
            """{"gameMode":"pvai","blueDifficulty":1}""");
        string gameID = created.GameId();

        await api.Client.PostJsonAsync($"/api/game/{gameID}/move", """{"x":7,"y":7}""");
        var (status, body) = await api.Client.PostJsonAsync($"/api/game/{gameID}/ai-move", "{}");
        Assert.Equal(200, status);

        Assert.True(body.TryGetValue("lastMove", out object? lastObj), "response should contain lastMove");
        Dictionary<string, object?> lastMove =
            JsonSerializer.Deserialize<Dictionary<string, object?>>(JsonSerializer.Serialize(lastObj))!;
        Assert.Equal(1, lastMove.Num("moveNumber"));
        Assert.Equal("blue", lastMove["player"]!.ToString());
        string statline = (lastMove["statline"]?.ToString()) ?? "";
        Assert.NotEmpty(statline);

        Assert.True(lastMove.TryGetValue("engineStats", out object? esObj));
        Dictionary<string, object?> es =
            JsonSerializer.Deserialize<Dictionary<string, object?>>(JsonSerializer.Serialize(esObj))!;
        Assert.NotNull(es["depth"]);
        Assert.NotNull(es["nodes"]);
    }
}
