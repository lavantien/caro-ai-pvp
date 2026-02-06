namespace Caro.Core.Domain.Entities;

/// <summary>
/// Factory for creating game states.
/// Separated from GameState to keep the entity pure.
/// </summary>
public static class GameStateFactory
{
    /// <summary>
    /// Create an initial game state.
    /// </summary>
    public static GameState CreateInitial() => new();
}
