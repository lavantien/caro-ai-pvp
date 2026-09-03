using Caro.Domain;
using Xunit;

namespace Caro.Api.Tests;

/// <summary>
/// End-to-end behavior follows startup overrides: a smaller concurrent-game
/// cap rejects earlier and an overridden time-control default changes what
/// unknown create-game requests resolve to.
/// </summary>
public class ConfigOverrideTests
{
    private static TestApi CreateConfiguredHost()
    {
        CaroConfig config = new()
        {
            MaxConcurrentGames = 2,
            TimeControl = new TimeControlOptions
            {
                Default = "10+0",
                DefaultInitialTimeMs = 600_000,
                DefaultIncrementSeconds = 0,
            },
        };
        config.Validate();
        return TestHostFactory.Create(config: config);
    }

    [Fact]
    public async Task UnknownTimeControlResolvesToConfiguredDefault()
    {
        await using TestApi api = CreateConfiguredHost();
        var (status, body) = await api.Client.PostJsonAsync("/api/game/new",
            """{"timeControl":"banana","gameMode":"pvp"}""");
        Assert.Equal(200, status);

        var state = body.State();
        Assert.Equal("10+0", state["timeControl"]!.ToString());
        Assert.Equal(600, state.Num("initialTime"));
        Assert.Equal(0, state.Num("increment"));
    }

    [Fact]
    public async Task ConcurrencyCapFollowsConfig()
    {
        await using TestApi api = CreateConfiguredHost();
        for (int i = 0; i < 2; i++)
        {
            var (status, _) = await api.Client.PostJsonAsync("/api/game/new", """{"gameMode":"pvp"}""");
            Assert.Equal(200, status);
        }

        var (finalStatus, _) = await api.Client.PostJsonAsync("/api/game/new", """{"gameMode":"pvp"}""");
        Assert.Equal(429, finalStatus);
    }
}
