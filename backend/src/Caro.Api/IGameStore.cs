namespace Caro.Api;

/// <summary>
/// Abstraction for game session storage.
/// Enables swapping between in-memory, Redis, or database-backed stores.
/// </summary>
public interface IGameStore
{
    GameSession? Get(string gameId);
    void Set(string gameId, GameSession session);
    bool TryGetValue(string gameId, out GameSession session);
    bool Remove(string gameId);
}
