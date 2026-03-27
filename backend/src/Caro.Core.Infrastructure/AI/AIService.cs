using System.Collections.Concurrent;
using Caro.Core.Application.DTOs;
using Caro.Core.Application.Interfaces;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Microsoft.Extensions.Logging;

namespace Caro.Core.Infrastructure.AI;

/// <summary>
/// AI service implementation using MinimaxAI engine.
/// </summary>
public sealed class AIService : IAIService
{
    private readonly MinimaxAI _ai;
    private readonly ILogger<AIService> _logger;
    private readonly ConcurrentDictionary<Guid, Player> _activePondering = new();

    public AIService(MinimaxAI ai, ILogger<AIService> logger)
    {
        _ai = ai ?? throw new ArgumentNullException(nameof(ai));
        _logger = logger ?? throw new ArgumentNullException(nameof(logger));
    }

    public async Task<AIMoveResponse> CalculateBestMoveAsync(
        GameState state,
        string difficulty,
        CancellationToken cancellationToken = default)
    {
        if (!Enum.TryParse<AIDifficulty>(difficulty, true, out var diff))
            diff = AIDifficulty.Medium;

        _logger.LogDebug("Starting AI calculation for difficulty {Difficulty}", difficulty);

        try
        {
            var stopwatch = System.Diagnostics.Stopwatch.StartNew();

            var (x, y) = await Task.Run(() =>
                _ai.GetBestMove(state.Board, state.CurrentPlayer, diff, ponderingEnabled: true),
                cancellationToken);

            stopwatch.Stop();

            var (depthAchieved, nodesSearched, nodesPerSecond, _, _, _, _, _, _, _, _, _, _, score, _, _) = _ai.GetSearchStatistics();

            return new AIMoveResponse
            {
                X = x,
                Y = y,
                DepthAchieved = depthAchieved,
                NodesSearched = nodesSearched,
                NodesPerSecond = nodesPerSecond,
                TimeTakenMs = stopwatch.ElapsedMilliseconds,
                Score = score,
                PonderingActive = !_activePondering.IsEmpty
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

    public Task StartPonderingAsync(
        Guid gameId,
        GameState state,
        string difficulty,
        CancellationToken cancellationToken = default)
    {
        if (!Enum.TryParse<AIDifficulty>(difficulty, true, out var diff))
            diff = AIDifficulty.Medium;

        var aiColor = state.CurrentPlayer.Opponent();

        _logger.LogDebug(
            "Starting pondering for game {GameId}, AI color {AIColor}, difficulty {Difficulty}",
            gameId, aiColor, difficulty);

        _ai.StartPonderingNow(state.Board, state.CurrentPlayer, diff, aiColor);
        _activePondering[gameId] = aiColor;

        return Task.CompletedTask;
    }

    public Task StopPonderingAsync(Guid gameId, CancellationToken cancellationToken = default)
    {
        if (_activePondering.TryRemove(gameId, out var aiColor))
        {
            _logger.LogDebug("Stopping pondering for game {GameId}, AI color {AIColor}", gameId, aiColor);
            _ai.StopPondering(aiColor);
        }

        return Task.CompletedTask;
    }

    public bool IsCalculating(Guid gameId) => _activePondering.ContainsKey(gameId);

    public void CleanupGame(Guid gameId)
    {
        if (_activePondering.TryRemove(gameId, out var aiColor))
        {
            _ai.StopPondering(aiColor);
        }
    }

    public void CleanupAll()
    {
        foreach (var kvp in _activePondering)
        {
            _ai.StopPondering(kvp.Value);
        }

        _activePondering.Clear();
    }
}
