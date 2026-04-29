using Caro.Core.Application.Interfaces;
using Microsoft.Extensions.Logging;
using System.Collections.Concurrent;

namespace Caro.Core.Infrastructure.Time;

/// <summary>
/// In-memory implementation of ITimeManagementService
/// Tracks time per game and per player
/// State is isolated per game
/// </summary>
public sealed class TimeManagementService : ITimeManagementService
{
    private readonly ConcurrentDictionary<string, GameTimer> _timers = new();
    private readonly ILogger<TimeManagementService> _logger;

    public TimeManagementService(ILogger<TimeManagementService> logger)
    {
        _logger = logger;
    }

    public Task StartTimerAsync(Guid gameId, string player, CancellationToken cancellationToken = default)
    {
        var key = GetKey(gameId, player);
        _timers[key] = new GameTimer
        {
            GameId = gameId,
            Player = player,
            StartTime = DateTimeOffset.UtcNow
        };
        _logger.LogDebug("Started timer for {Player} in game {GameId}", player, gameId);
        return Task.CompletedTask;
    }

    public Task<TimeSpan> StopTimerAsync(Guid gameId, string player, CancellationToken cancellationToken = default)
    {
        var key = GetKey(gameId, player);
        if (_timers.TryRemove(key, out var timer))
        {
            var elapsed = DateTimeOffset.UtcNow - timer.StartTime;
            _logger.LogDebug("Stopped timer for {Player} in game {GameId}, elapsed: {Elapsed}", player, gameId, elapsed);
            return Task.FromResult(elapsed);
        }

        _logger.LogWarning("Timer not found for {Player} in game {GameId}", player, gameId);
        return Task.FromResult(TimeSpan.Zero);
    }

    public Task<TimeSpan> GetRemainingTimeAsync(Guid gameId, string player, CancellationToken cancellationToken = default)
    {
        // This would be tracked in the game state itself
        // This implementation returns a placeholder
        return Task.FromResult(TimeSpan.FromMinutes(7));
    }

    public Task<bool> IsTimeoutAsync(Guid gameId, string player, CancellationToken cancellationToken = default)
    {
        // Check if the current running timer has exceeded a limit
        var key = GetKey(gameId, player);
        if (_timers.TryGetValue(key, out var timer))
        {
            var elapsed = DateTimeOffset.UtcNow - timer.StartTime;
            return Task.FromResult(elapsed > TimeSpan.FromMinutes(10)); // 10 min default
        }
        return Task.FromResult(false);
    }

    /// <summary>
    /// Get a unique key for game+player combination
    /// </summary>
    private static string GetKey(Guid gameId, string player) => $"{gameId}:{player}";

    /// <summary>
    /// Internal timer tracking
    /// </summary>
    private sealed class GameTimer
    {
        public Guid GameId { get; set; }
        public string Player { get; set; } = null!;
        public DateTimeOffset StartTime { get; set; }
    }

    /// <summary>
    /// Clear all timers (useful for testing)
    /// </summary>
    public void Clear()
    {
        _timers.Clear();
    }
}
