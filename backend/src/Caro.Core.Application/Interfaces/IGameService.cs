using Caro.Core.Application.DTOs;

namespace Caro.Core.Application.Interfaces;

/// <summary>
/// Interface for game management operations (Use Case/Port)
/// </summary>
public interface IGameService
{
    /// <summary>
    /// Create a new game
    /// </summary>
    Task<GameResponse> CreateGameAsync(CreateGameRequest request, CancellationToken cancellationToken = default);

    /// <summary>
    /// Get a game by ID
    /// </summary>
    Task<GameResponse?> GetGameAsync(Guid gameId, CancellationToken cancellationToken = default);

    /// <summary>
    /// Make a move in a game
    /// </summary>
    Task<GameResponse> MakeMoveAsync(Guid gameId, MakeMoveRequest request, CancellationToken cancellationToken = default);

    /// <summary>
    /// Undo the last move
    /// </summary>
    Task<GameResponse> UndoMoveAsync(Guid gameId, CancellationToken cancellationToken = default);

    /// <summary>
    /// Resign from a game
    /// </summary>
    Task<GameResponse> ResignAsync(Guid gameId, string player, CancellationToken cancellationToken = default);

    /// <summary>
    /// Request AI move calculation
    /// </summary>
    Task<AIMoveResponse> GetAIMoveAsync(Guid gameId, CancellationToken cancellationToken = default);

    /// <summary>
    /// Get list of active games
    /// </summary>
    Task<GameListDto> GetGamesAsync(CancellationToken cancellationToken = default);

    /// <summary>
    /// Delete a game
    /// </summary>
    Task<bool> DeleteGameAsync(Guid gameId, CancellationToken cancellationToken = default);
}

/// <summary>
/// Interface for game repository (Port for Infrastructure)
/// </summary>
public interface IGameRepository
{
    /// <summary>
    /// Save a game state
    /// </summary>
    Task SaveAsync(Guid gameId, Domain.Entities.GameState state, CancellationToken cancellationToken = default);

    /// <summary>
    /// Load a game state
    /// </summary>
    Task<Domain.Entities.GameState?> LoadAsync(Guid gameId, CancellationToken cancellationToken = default);

    /// <summary>
    /// Check if a game exists
    /// </summary>
    Task<bool> ExistsAsync(Guid gameId, CancellationToken cancellationToken = default);

    /// <summary>
    /// Delete a game
    /// </summary>
    Task<bool> DeleteAsync(Guid gameId, CancellationToken cancellationToken = default);

    /// <summary>
    /// Get all game IDs
    /// </summary>
    Task<Guid[]> GetAllIdsAsync(CancellationToken cancellationToken = default);
}

/// <summary>
/// Interface for AI service (Port for Infrastructure)
/// </summary>
public interface IAIService
{
    /// <summary>
    /// Calculate the best move for the current player
    /// </summary>
    Task<AIMoveResponse> CalculateBestMoveAsync(
        Domain.Entities.GameState state,
        string difficulty,
        CancellationToken cancellationToken = default);

    /// <summary>
    /// Start pondering (background search during opponent's turn)
    /// </summary>
    Task StartPonderingAsync(
        Guid gameId,
        Domain.Entities.GameState state,
        string difficulty,
        CancellationToken cancellationToken = default);

    /// <summary>
    /// Stop pondering
    /// </summary>
    Task StopPonderingAsync(Guid gameId, CancellationToken cancellationToken = default);

    /// <summary>
    /// Check if AI is currently calculating
    /// </summary>
    bool IsCalculating(Guid gameId);
}

/// <summary>
/// Interface for time management
/// </summary>
public interface ITimeManagementService
{
    /// <summary>
    /// Start the timer for a player
    /// </summary>
    Task StartTimerAsync(Guid gameId, string player, CancellationToken cancellationToken = default);

    /// <summary>
    /// Stop the timer for a player
    /// </summary>
    Task<TimeSpan> StopTimerAsync(Guid gameId, string player, CancellationToken cancellationToken = default);

    /// <summary>
    /// Get remaining time for a player
    /// </summary>
    Task<TimeSpan> GetRemainingTimeAsync(Guid gameId, string player, CancellationToken cancellationToken = default);

    /// <summary>
    /// Check if a player has run out of time
    /// </summary>
    Task<bool> IsTimeoutAsync(Guid gameId, string player, CancellationToken cancellationToken = default);
}
