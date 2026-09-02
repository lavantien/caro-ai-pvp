using Caro.Api;
using Caro.Persistence;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;

int httpPort = ServerConfig.HttpPort;
WebApplicationBuilder builder = WebApplication.CreateBuilder(args);
builder.WebHost.ConfigureKestrel(o => o.ListenAnyIP(httpPort));
builder.WebHost.UseShutdownTimeout(TimeSpan.FromSeconds(ServerConfig.ShutdownTimeoutSeconds));

string dbPath = Environment.GetEnvironmentVariable("MATCH_DB_PATH") is { Length: > 0 } env
    ? env
    : ServerConfig.DefaultDbPath;
MatchStore matches = new(dbPath);
builder.Services.AddCaroApi(matches);
builder.Services.AddHostedService<CleanupService>();

WebApplication app = builder.Build();
app.UseCaroPipeline();

string listenAddr = $"http://+:{httpPort}";
app.Logger.ServerStarting(listenAddr);

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
        using PeriodicTimer timer = new(TimeSpan.FromMinutes(ServerConfig.CleanupSweepMinutes));
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

/// <summary>
/// Host knobs for the local server: named defaults with a CARO_HTTP_PORT
/// override, matching the MATCH_DB_PATH pattern (no appsettings.json).
/// </summary>
internal static class ServerConfig
{
    public const int DefaultPort = 5207;
    public const int ShutdownTimeoutSeconds = 10;
    public const int CleanupSweepMinutes = 5;
    public const string DefaultDbPath = "data/matches.db";

    public static int HttpPort =>
        int.TryParse(Environment.GetEnvironmentVariable("CARO_HTTP_PORT"), out int port) && port is > 0 and < 65536
            ? port
            : DefaultPort;
}

internal static partial class ServerLog
{
    [LoggerMessage(Level = LogLevel.Information, Message = "server starting {Addr}")]
    public static partial void ServerStarting(this ILogger logger, string addr);

    [LoggerMessage(Level = LogLevel.Information, Message = "cleanup removed {Removed} games")]
    public static partial void CleanupRemoved(this ILogger logger, int removed);
}
