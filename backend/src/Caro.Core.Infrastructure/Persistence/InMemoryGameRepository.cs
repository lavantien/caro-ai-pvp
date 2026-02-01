using Caro.Core.Application.Interfaces;
using Caro.Core.Domain.Entities;
using Microsoft.Extensions.Logging;

namespace Caro.Core.Infrastructure.Persistence;

/// <summary>
/// In-memory implementation of IGameRepository for development and testing
/// In production, this would be replaced with a database-backed implementation
/// State is isolated per game ID
/// </summary>
public sealed class InMemoryGameRepository : IGameRepository
{
    private readonly Dictionary<Guid, GameState> _games = new();
    private readonly ILogger<InMemoryGameRepository> _logger;

    public InMemoryGameRepository(ILogger<InMemoryGameRepository> logger)
    {
        _logger = logger;
    }

    public Task SaveAsync(Guid gameId, GameState state, CancellationToken cancellationToken = default)
    {
        _games[gameId] = state;
        _logger.LogDebug("Saved game {GameId} with move number {MoveNumber}", gameId, state.MoveNumber);
        return Task.CompletedTask;
    }

    public Task<GameState?> LoadAsync(Guid gameId, CancellationToken cancellationToken = default)
    {
        _games.TryGetValue(gameId, out var state);
        return Task.FromResult(state);
    }

    public Task<bool> ExistsAsync(Guid gameId, CancellationToken cancellationToken = default)
    {
        return Task.FromResult(_games.ContainsKey(gameId));
    }

    public Task<bool> DeleteAsync(Guid gameId, CancellationToken cancellationToken = default)
    {
        var deleted = _games.Remove(gameId);
        if (deleted)
        {
            _logger.LogDebug("Deleted game {GameId}", gameId);
        }
        return Task.FromResult(deleted);
    }

    public Task<Guid[]> GetAllIdsAsync(CancellationToken cancellationToken = default)
    {
        return Task.FromResult(_games.Keys.ToArray());
    }

    /// <summary>
    /// Clear all games (useful for testing)
    /// </summary>
    public void Clear()
    {
        _games.Clear();
    }
}
