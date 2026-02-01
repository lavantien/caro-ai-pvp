using Caro.Core.Application.DTOs;
using Caro.Core.Application.Interfaces;
using Caro.Core.Domain.Entities;
using Microsoft.Extensions.Logging;

namespace Caro.Core.Infrastructure.AI;

/// <summary>
/// AI service implementation with per-game state isolation
/// Each game has its own AI state (TT, heuristics, etc.)
/// The search engine itself is stateless
/// </summary>
public sealed class AIService : IAIService
{
    private readonly StatelessSearchEngine _searchEngine;
    private readonly ILogger<AIService> _logger;
    private readonly Dictionary<Guid, AIGameState> _gameStates;
    private readonly Dictionary<Guid, Task> _activeCalculations;

    public AIService(StatelessSearchEngine searchEngine, ILogger<AIService> logger)
    {
        _searchEngine = searchEngine;
        _logger = logger;
        _gameStates = new Dictionary<Guid, AIGameState>();
        _activeCalculations = new Dictionary<Guid, Task>();
    }

    /// <summary>
    /// Calculate the best move for the current game state
    /// Creates or retrieves per-game AI state
    /// </summary>
    public async Task<AIMoveResponse> CalculateBestMoveAsync(
        GameState state,
        string difficulty,
        CancellationToken cancellationToken = default)
    {
        var gameId = Guid.NewGuid(); // Internal ID for this calculation

        try
        {
            // Get or create AI state for this game
            // In real implementation, gameId would be passed in
            var aiState = GetOrCreateAIState(gameId, difficulty);

            var options = GetSearchOptions(difficulty);

            _logger.LogDebug("Starting AI calculation for difficulty {Difficulty}", difficulty);

            var (x, y, score, stats) = await Task.Run(() =>
                _searchEngine.FindBestMove(state, aiState, options, cancellationToken),
                cancellationToken);

            _logger.LogInformation("AI move calculated: ({X},{Y}) score {Score}, depth {Depth}, nodes {Nodes}",
                x, y, score, stats.DepthReached, stats.NodesSearched);

            return new AIMoveResponse
            {
                X = x,
                Y = y,
                DepthAchieved = stats.DepthReached,
                NodesSearched = stats.NodesSearched,
                NodesPerSecond = stats.NodesPerSecond,
                TimeTakenMs = stats.ElapsedMs,
                Score = score,
                PonderingActive = false
            };
        }
        catch (OperationCanceledException)
        {
            _logger.LogInformation("AI calculation cancelled");
            throw;
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error calculating AI move");
            throw;
        }
    }

    /// <summary>
    /// Start pondering (background search during opponent's turn)
    /// </summary>
    public Task StartPonderingAsync(
        Guid gameId,
        GameState state,
        string difficulty,
        CancellationToken cancellationToken = default)
    {
        // Store pondering state for later retrieval
        var aiState = GetOrCreateAIState(gameId, difficulty);
        // Pondering would run in background - simplified for now
        return Task.CompletedTask;
    }

    /// <summary>
    /// Stop pondering for a game
    /// </summary>
    public Task StopPonderingAsync(Guid gameId, CancellationToken cancellationToken = default)
    {
        // Cancel any active pondering for this game
        if (_activeCalculations.TryGetValue(gameId, out var task))
        {
            // In real implementation, would cancel the task
            _activeCalculations.Remove(gameId);
        }
        return Task.CompletedTask;
    }

    /// <summary>
    /// Check if AI is currently calculating for a game
    /// </summary>
    public bool IsCalculating(Guid gameId)
    {
        return _activeCalculations.ContainsKey(gameId);
    }

    /// <summary>
    /// Get or create AI state for a game
    /// </summary>
    private AIGameState GetOrCreateAIState(Guid gameId, string difficulty)
    {
        if (!_gameStates.TryGetValue(gameId, out var aiState))
        {
            var settings = GetSearchOptions(difficulty);
            aiState = new AIGameState(settings.MaxDepth, 128);
            _gameStates[gameId] = aiState;
            _logger.LogDebug("Created AI state for game {GameId}", gameId);
        }
        return aiState;
    }

    /// <summary>
    /// Get search options based on difficulty
    /// </summary>
    private static SearchOptions GetSearchOptions(string difficulty)
    {
        return difficulty.ToLowerInvariant() switch
        {
            "easy" or "beginner" => new SearchOptions { MaxDepth = 5, TimeLimitMs = 1000 },
            "medium" or "intermediate" => new SearchOptions { MaxDepth = 10, TimeLimitMs = 3000 },
            "hard" or "expert" => new SearchOptions { MaxDepth = 15, TimeLimitMs = 5000 },
            "grandmaster" => new SearchOptions { MaxDepth = 20, TimeLimitMs = 10000, UseParallelSearch = true },
            _ => new SearchOptions { MaxDepth = 10, TimeLimitMs = 3000 }
        };
    }

    /// <summary>
    /// Clean up AI state for a game
    /// </summary>
    public void CleanupGame(Guid gameId)
    {
        if (_gameStates.TryGetValue(gameId, out var aiState))
        {
            aiState.Dispose();
            _gameStates.Remove(gameId);
            _logger.LogDebug("Cleaned up AI state for game {GameId}", gameId);
        }

        StopPonderingAsync(gameId).GetAwaiter().GetResult();
    }

    /// <summary>
    /// Clean up all game states
    /// </summary>
    public void CleanupAll()
    {
        foreach (var gameId in _gameStates.Keys.ToList())
        {
            CleanupGame(gameId);
        }
    }
}
