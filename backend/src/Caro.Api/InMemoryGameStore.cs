using System.Collections.Concurrent;
using Caro.Core.Domain.Configuration;

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

    public int Count => _games.Count;

    public bool Remove(string gameId)
        => _games.TryRemove(gameId, out _);

    /// <summary>
    /// Number of active (non-completed) games.
    /// Used to scale per-game thread count dynamically.
    /// </summary>
    public int ActiveGameCount => _games.Count(kvp => !kvp.Value.IsGameOver);

    public int CleanupCompleted()
    {
        int removed = 0;
        foreach (var kvp in _games.ToList())
        {
            var isAbandoned = (DateTime.UtcNow - kvp.Value.LastActivityAt).TotalMinutes > TimeConstants.AbandonedTimeoutMinutes;
            if (kvp.Value.IsGameOver || isAbandoned)
            {
                if (_games.TryRemove(kvp.Key, out var session))
                {
                    session.DisposeAI();
                    removed++;
                }
            }
        }
        if (removed > 0)
            GC.Collect(2, GCCollectionMode.Aggressive, blocking: false, compacting: true);
        return removed;
    }

    /// <summary>
    /// Dispose and remove ALL game sessions (used during graceful shutdown).
    /// </summary>
    public int CleanupAll()
    {
        int removed = 0;
        foreach (var kvp in _games.ToList())
        {
            if (_games.TryRemove(kvp.Key, out var session))
            {
                session.DisposeAI();
                removed++;
            }
        }
        if (removed > 0)
            GC.Collect(2, GCCollectionMode.Aggressive, blocking: true, compacting: true);
        return removed;
    }
}
