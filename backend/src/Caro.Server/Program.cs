using Caro.Api;
using Caro.Persistence;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;

WebApplicationBuilder builder = WebApplication.CreateBuilder(args);
builder.WebHost.ConfigureKestrel(o => o.ListenAnyIP(5207));
builder.WebHost.UseShutdownTimeout(TimeSpan.FromSeconds(10));

string dbPath = Environment.GetEnvironmentVariable("MATCH_DB_PATH") is { Length: > 0 } env
    ? env
    : Path.Combine("data", "matches.db");
MatchStore matches = new(dbPath);
builder.Services.AddCaroApi(matches);
builder.Services.AddHostedService<CleanupService>();

WebApplication app = builder.Build();
app.UseCaroPipeline();

app.Logger.ServerStarting("http://+:5207");

app.Lifetime.ApplicationStopping.Register(() =>
{
    app.Services.GetRequiredService<GameStore>().CleanupAll();
    matches.Close();
});

await app.RunAsync();

/// <summary>
/// Sweeps finished and abandoned games every five minutes so stale sessions
/// never hold AI engines or the concurrent-game slots.
/// </summary>
internal sealed class CleanupService(GameStore store, ILogger<CleanupService> logger)
    : BackgroundService
{
    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        using PeriodicTimer timer = new(TimeSpan.FromMinutes(5));
        try
        {
            while (await timer.WaitForNextTickAsync(stoppingToken))
            {
                int removed = store.CleanupCompleted();
                if (removed > 0)
                {
                    logger.CleanupRemoved(removed);
                }
            }
        }
        catch (OperationCanceledException)
        {
            // Shutdown.
        }
    }
}

internal static partial class ServerLog
{
    [LoggerMessage(Level = LogLevel.Information, Message = "server starting {Addr}")]
    public static partial void ServerStarting(this ILogger logger, string addr);

    [LoggerMessage(Level = LogLevel.Information, Message = "cleanup removed {Removed} games")]
    public static partial void CleanupRemoved(this ILogger logger, int removed);
}
