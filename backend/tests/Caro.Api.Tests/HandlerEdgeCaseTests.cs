using Caro.Api;
using Caro.Domain;
using Caro.Persistence;
using Microsoft.Extensions.DependencyInjection;
using Xunit;

namespace Caro.Api.Tests;

/// <summary>
/// Create-game variants and the turn/game-over guards the happy-path
/// handler tests do not reach.
/// </summary>
public class HandlerEdgeCaseTests
{
    [Fact]
    public async Task CreateGameMalformedJson()
    {
        await using TestApi api = TestHostFactory.Create();
        var (status, body) = await api.Client.PostJsonAsync("/api/game/new", "{not json");
        Assert.Equal(400, status);
        Assert.Equal("bad_request", body["error"]!.ToString());
    }

    [Fact]
    public async Task CreateGameTenZero()
    {
        await using TestApi api = TestHostFactory.Create();
        var (status, body) = await api.Client.PostJsonAsync("/api/game/new",
            """{"timeControl":"10+0"}""");
        Assert.Equal(200, status);
        var state = body.State();
        Assert.Equal("10+0", state["timeControl"]!.ToString());
        Assert.Equal(600, state.Num("initialTime"));
    }

    [Fact]
    public async Task CreateGameClassicalAliases()
    {
        await using TestApi api = TestHostFactory.Create();
        var (_, body) = await api.Client.PostJsonAsync("/api/game/new",
            """{"timeControl":"classical"}""");
        Assert.Equal("15+10", body.State()["timeControl"]!.ToString());

        var (_, body2) = await api.Client.PostJsonAsync("/api/game/new",
            """{"timeControl":"15+10"}""");
        Assert.Equal("15+10", body2.State()["timeControl"]!.ToString());
    }

    [Fact]
    public async Task CreateGameRandomOpeningAppliesSeededMoves()
    {
        await using TestApi api = TestHostFactory.Create();
        var (status, body) = await api.Client.PostJsonAsync("/api/game/new",
            """{"gameMode":"aivai","difficulty":3,"randomOpening":true,"seed":42}""");
        Assert.Equal(200, status);
        var state = body.State();
        Assert.Equal(2, state.Num("moveNumber"));
    }

    [Fact]
    public async Task CreateGameAIvAIRecordsBotTypes()
    {
        string dir = Path.Combine(Path.GetTempPath(), Guid.NewGuid().ToString("N"));
        Directory.CreateDirectory(dir);
        using MatchStore ms = new(Path.Combine(dir, "test.db"));
        await using TestApi api = TestHostFactory.Create(matches: ms);

        var (_, created) = await api.Client.PostJsonAsync("/api/game/new",
            """{"gameMode":"aivai","redDifficulty":1,"blueDifficulty":1}""");
        string gameID = created.GameId();

        GameRecord? record = ms.GetGame(gameID);
        Assert.NotNull(record);
        Assert.Equal("bot", record!.RedType);
        Assert.Equal("bot", record.BlueType);
    }

    [Fact]
    public async Task CreateGamePvAIRedBotRecordsRedBot()
    {
        string dir = Path.Combine(Path.GetTempPath(), Guid.NewGuid().ToString("N"));
        Directory.CreateDirectory(dir);
        using MatchStore ms = new(Path.Combine(dir, "test.db"));
        await using TestApi api = TestHostFactory.Create(matches: ms);

        var (_, created) = await api.Client.PostJsonAsync("/api/game/new",
            """{"gameMode":"pvai","redDifficulty":2}""");
        string gameID = created.GameId();

        GameRecord? record = ms.GetGame(gameID);
        Assert.NotNull(record);
        Assert.Equal("bot", record!.RedType);
        Assert.Equal("human", record.BlueType);
    }

    [Fact]
    public async Task MakeMoveOnAIvAIIsNotYourTurn()
    {
        await using TestApi api = TestHostFactory.Create();
        var (_, created) = await api.Client.PostJsonAsync("/api/game/new",
            """{"gameMode":"aivai","difficulty":1}""");
        string gameID = created.GameId();

        var (status, body) = await api.Client.PostJsonAsync($"/api/game/{gameID}/move",
            """{"x":7,"y":7}""");
        Assert.Equal(409, status);
        Assert.Equal("not_your_turn", body["error"]!.ToString());
    }

    [Fact]
    public async Task MakeMoveOnEngineTurnIsNotYourTurn()
    {
        await using TestApi api = TestHostFactory.Create();
        var (_, created) = await api.Client.PostJsonAsync("/api/game/new",
            """{"gameMode":"pvai","redDifficulty":1}""");
        string gameID = created.GameId();

        // Red is the engine in this configuration; a human move for red is 409.
        var (status, _) = await api.Client.PostJsonAsync($"/api/game/{gameID}/move",
            """{"x":7,"y":7}""");
        Assert.Equal(409, status);
    }

    [Fact]
    public async Task MovesAndAIMoveAfterGameOverRejected()
    {
        await using TestApi api = TestHostFactory.Create();
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

        var (humanStatus, humanBody) = await api.Client.PostJsonAsync(
            $"/api/game/{gameID}/move", """{"x":10,"y":10}""");
        Assert.Equal(400, humanStatus);
        Assert.Equal("bad_request", humanBody["error"]!.ToString());

        var (aiStatus, aiBody) = await api.Client.PostJsonAsync(
            $"/api/game/{gameID}/ai-move", "{}");
        Assert.Equal(400, aiStatus);
        Assert.Equal("bad_request", aiBody["error"]!.ToString());
    }

    [Fact]
    public async Task MakeAIMoveOnHumanGameUsesDefaultOptions()
    {
        // A PvP game has no difficulty; ai-move falls back to the unbounded
        // default profile and still answers with a legal move.
        await using TestApi api = TestHostFactory.Create();
        var (_, created) = await api.Client.PostJsonAsync("/api/game/new",
            """{"timeControl":"1+0","gameMode":"pvp"}""");
        string gameID = created.GameId();

        await api.Client.PostJsonAsync($"/api/game/{gameID}/move", """{"x":7,"y":7}""");

        var (status, body) = await api.Client.PostJsonAsync($"/api/game/{gameID}/ai-move", "{}");
        Assert.Equal(200, status);
        Assert.Equal(2, body.State().Num("moveNumber"));
    }

    [Fact]
    public void AddCaroApiCreatesDefaultStore()
    {
        ServiceCollection services = new();
        services.AddCaroApi();
        using ServiceProvider provider = services.BuildServiceProvider();
        Assert.NotNull(provider.GetRequiredService<GameStore>());
    }
}
