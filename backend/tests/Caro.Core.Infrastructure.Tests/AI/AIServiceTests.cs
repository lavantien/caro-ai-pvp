using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Infrastructure.AI;
using FluentAssertions;
using Microsoft.Extensions.Logging;
using Xunit;

namespace Caro.Core.Infrastructure.Tests.AI;

public sealed class AIServiceTests : IDisposable
{
    private readonly AIService _service;
    private readonly MockLogger<AIService> _logger;

    public AIServiceTests()
    {
        _logger = new MockLogger<AIService>();
        var ai = new MinimaxAI(logger: new MockLogger<MinimaxAI>());
        _service = new AIService(ai, _logger);
    }

    public void Dispose()
    {
        _service.CleanupAll();
    }

    [Fact]
    public async Task CalculateBestMoveAsync_EmptyBoard_ReturnsValidMove()
    {
        var state = GameStateFactory.CreateInitial();

        var response = await _service.CalculateBestMoveAsync(state);

        response.X.Should().BeGreaterOrEqualTo(0);
        response.Y.Should().BeGreaterOrEqualTo(0);
        response.X.Should().BeLessThan(GameConstants.BoardSize);
        response.Y.Should().BeLessThan(GameConstants.BoardSize);
        response.DepthAchieved.Should().BeGreaterThan(0);
        response.NodesSearched.Should().BeGreaterThan(0);
        response.TimeTakenMs.Should().BeGreaterOrEqualTo(0);
    }

    [Fact]
    public async Task CalculateBestMoveAsync_WithCancellation_Throws()
    {
        var state = GameStateFactory.CreateInitial();
        var cts = new CancellationTokenSource();
        cts.Cancel();

        var act = async () => await _service.CalculateBestMoveAsync(state, cts.Token);

        await act.Should().ThrowAsync<OperationCanceledException>();
    }

    [Fact]
    public void IsCalculating_InitiallyReturnsFalse()
    {
        var isCalculating = _service.IsCalculating(Guid.NewGuid());

        isCalculating.Should().BeFalse();
    }

    [Fact]
    public async Task StartPonderingAsync_StartsPondering()
    {
        var gameId = Guid.NewGuid();
        var state = GameStateFactory.CreateInitial();

        await _service.StartPonderingAsync(gameId, state);

        _service.IsCalculating(gameId).Should().BeTrue();
        _service.CleanupGame(gameId);
    }

    [Fact]
    public async Task StopPonderingAsync_StopsPondering()
    {
        var gameId = Guid.NewGuid();
        var state = GameStateFactory.CreateInitial();

        await _service.StartPonderingAsync(gameId, state);
        _service.IsCalculating(gameId).Should().BeTrue();

        await _service.StopPonderingAsync(gameId);

        _service.IsCalculating(gameId).Should().BeFalse();
    }

    [Fact]
    public async Task StopPonderingAsync_WithoutStart_DoesNotThrow()
    {
        var gameId = Guid.NewGuid();

        var act = async () => await _service.StopPonderingAsync(gameId);

        await act.Should().NotThrowAsync();
    }

    [Fact]
    public async Task CleanupGame_RemovesAIState()
    {
        var gameId = Guid.NewGuid();
        var state = GameStateFactory.CreateInitial();

        await _service.StartPonderingAsync(gameId, state);
        _service.IsCalculating(gameId).Should().BeTrue();

        _service.CleanupGame(gameId);

        _service.IsCalculating(gameId).Should().BeFalse();
    }

    [Fact]
    public async Task CleanupAll_RemovesAllStates()
    {
        var game1 = Guid.NewGuid();
        var game2 = Guid.NewGuid();
        var state = GameStateFactory.CreateInitial();

        await _service.StartPonderingAsync(game1, state);
        _service.CleanupGame(game1);
        await _service.StartPonderingAsync(game2, state);

        _service.IsCalculating(game2).Should().BeTrue();

        _service.CleanupAll();

        _service.IsCalculating(game1).Should().BeFalse();
        _service.IsCalculating(game2).Should().BeFalse();
    }

    private sealed class MockLogger<T> : ILogger<T>
    {
        public IDisposable? BeginScope<TState>(TState state) where TState : notnull => null;
        public bool IsEnabled(LogLevel logLevel) => true;
        public void Log<TState>(LogLevel logLevel, EventId eventId, TState state, Exception? exception, Func<TState, Exception?, string> formatter)
        {
        }
    }
}
