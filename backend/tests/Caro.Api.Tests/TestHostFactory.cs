using System.Text.Json;
using Caro.Api;
using Caro.Domain;
using Caro.Persistence;
using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.TestHost;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Logging;
using Xunit;

namespace Caro.Api.Tests;

internal static class TestJson
{
    public static readonly System.Text.Json.JsonSerializerOptions Web =
        new(System.Text.Json.JsonSerializerDefaults.Web);
}

/// <summary>
/// Builds the same application the real host runs, served in-memory.
/// </summary>
internal sealed class TestApi : IAsyncDisposable
{
    public required WebApplication App { get; init; }
    public required HttpClient Client { get; init; }
    public required GameStore Store { get; init; }

    public async ValueTask DisposeAsync()
    {
        Client.Dispose();
        await App.DisposeAsync();
    }
}

internal static class TestHostFactory
{
    public static TestApi Create(MatchStore? matches = null, GameStore? store = null, CaroConfig? config = null)
    {
        WebApplicationBuilder builder = WebApplication.CreateBuilder();
        builder.Logging.ClearProviders();
        builder.WebHost.UseTestServer();
        store ??= new GameStore(config);
        builder.Services.AddCaroApi(matches, store, config);

        WebApplication app = builder.Build();
        app.UseCaroPipeline();
        app.StartAsync().GetAwaiter().GetResult();

        return new TestApi { App = app, Client = app.GetTestClient(), Store = store };
    }
}

internal static class HttpTestExtensions
{
    public static async Task<(int Status, Dictionary<string, object?> Body)> PostJsonAsync(
        this HttpClient client, string url, string json)
    {
        using StringContent content = new(json, System.Text.Encoding.UTF8, "application/json");
        HttpResponseMessage resp = await client.PostAsync(url, content);
        return await ReadAsync(resp);
    }

    public static async Task<(int Status, Dictionary<string, object?> Body)> GetJsonAsync(
        this HttpClient client, string url)
    {
        HttpResponseMessage resp = await client.GetAsync(url);
        return await ReadAsync(resp);
    }

    public static async Task<(int Status, Dictionary<string, object?> Body)> DeleteJsonAsync(
        this HttpClient client, string url)
    {
        HttpResponseMessage resp = await client.DeleteAsync(url);
        return await ReadAsync(resp);
    }

    private static async Task<(int, Dictionary<string, object?>)> ReadAsync(HttpResponseMessage resp)
    {
        string text = await resp.Content.ReadAsStringAsync();
        Dictionary<string, object?> body = [];
        if (text.Length > 0)
        {
            body = System.Text.Json.JsonSerializer.Deserialize<Dictionary<string, object?>>(text, TestJson.Web)
                ?? [];
        }
        return ((int)resp.StatusCode, body);
    }

    public static Dictionary<string, object?> State(this Dictionary<string, object?> body)
    {
        Assert.True(body.TryGetValue("state", out object? state), "response should have state");
        return JsonSerializer.Deserialize<Dictionary<string, object?>>(
            JsonSerializer.Serialize(state, TestJson.Web), TestJson.Web)!;
    }

    public static string GameId(this Dictionary<string, object?> body)
    {
        Assert.True(body.TryGetValue("gameId", out object? id), "response should have gameId");
        return id!.ToString()!;
    }

    public static double Num(this Dictionary<string, object?> body, string key)
    {
        object? v = body[key];
        return v is System.Text.Json.JsonElement e
            ? e.GetDouble()
            : Convert.ToDouble(v, System.Globalization.CultureInfo.InvariantCulture);
    }

    public static bool Bool(this Dictionary<string, object?> body, string key)
    {
        object? v = body[key];
        return v is System.Text.Json.JsonElement e
            ? e.GetBoolean()
            : Convert.ToBoolean(v, System.Globalization.CultureInfo.InvariantCulture);
    }
}
