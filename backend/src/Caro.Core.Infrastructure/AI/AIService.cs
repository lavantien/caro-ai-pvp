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
    private readonly HashSet<Guid> _activeCalculations = new();

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
                _ai.GetBestMove(state.Board, state.CurrentPlayer, diff),
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

    public Task StartPonderingAsync(
        Guid gameId,
        GameState state,
        string difficulty,
        CancellationToken cancellationToken = default)
    {
        _activeCalculations.Add(gameId);
        return Task.CompletedTask;
    }

    public Task StopPonderingAsync(Guid gameId, CancellationToken cancellationToken = default)
    {
        _activeCalculations.Remove(gameId);
        return Task.CompletedTask;
    }

    public bool IsCalculating(Guid gameId) => _activeCalculations.Contains(gameId);

    public void CleanupGame(Guid gameId)
    {
        StopPonderingAsync(gameId).GetAwaiter().GetResult();
    }

    public void CleanupAll()
    {
        _activeCalculations.Clear();
    }
}
