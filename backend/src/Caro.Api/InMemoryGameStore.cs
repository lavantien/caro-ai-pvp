using System.Collections.Concurrent;

namespace Caro.Api;

/// <summary>
/// In-memory game session store using ConcurrentDictionary.
/// </summary>
public sealed class InMemoryGameStore : IGameStore
{
    private readonly ConcurrentDictionary<string, GameSession> _games = new();

    public GameSession? Get(string gameId)
        => _games.TryGetValue(gameId, out var session) ? session : null;

    public void Set(string gameId, GameSession session)
        => _games[gameId] = session;

    public bool TryGetValue(string gameId, out GameSession session)
        => _games.TryGetValue(gameId, out session!);

    public bool Remove(string gameId)
        => _games.TryRemove(gameId, out _);
}
