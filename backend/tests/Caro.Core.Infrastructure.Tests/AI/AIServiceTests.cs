using Caro.Core.Application.DTOs;
using Caro.Core.Domain.Entities;
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
        var engine = new StatelessSearchEngine(new MockLogger<StatelessSearchEngine>());
        _service = new AIService(engine, _logger);
    }

    public void Dispose()
    {
        _service.CleanupAll();
    }

    [Fact]
    public async Task CalculateBestMoveAsync_EmptyBoard_ReturnsValidMove()
    {
        // Arrange
        var state = GameState.CreateInitial();

        // Act
        var response = await _service.CalculateBestMoveAsync(state, "medium");

        // Assert
        response.X.Should().BeGreaterOrEqualTo(0);
        response.Y.Should().BeGreaterOrEqualTo(0);
        response.X.Should().BeLessThan(19);
        response.Y.Should().BeLessThan(19);
        response.DepthAchieved.Should().BeGreaterThan(0);
        response.NodesSearched.Should().BeGreaterThan(0);
        response.NodesPerSecond.Should().BeGreaterThan(0);
        response.TimeTakenMs.Should().BeGreaterOrEqualTo(0);
    }

    [Fact]
    public async Task CalculateBestMoveAsync_MultipleDifficulties_AllWork()
    {
        // Arrange
        var difficulties = new[] { "easy", "medium", "hard" };

        // Act & Assert
        foreach (var difficulty in difficulties)
        {
            var state = GameState.CreateInitial();
            var response = await _service.CalculateBestMoveAsync(state, difficulty);
            response.X.Should().BeGreaterOrEqualTo(0);
            response.Y.Should().BeGreaterOrEqualTo(0);
            response.X.Should().BeLessThan(19);
            response.Y.Should().BeLessThan(19);
        }
    }

    [Fact]
    public async Task CalculateBestMoveAsync_Grandmaster_Works()
    {
        // Arrange
        var state = GameState.CreateInitial();

        // Act
        var response = await _service.CalculateBestMoveAsync(state, "grandmaster");

        // Assert
        response.X.Should().BeGreaterOrEqualTo(0);
        response.Y.Should().BeGreaterOrEqualTo(0);
        response.X.Should().BeLessThan(19);
        response.Y.Should().BeLessThan(19);
        response.DepthAchieved.Should().BeGreaterThan(0);
    }

    [Fact]
    public async Task CalculateBestMoveAsync_WithCancellation_Throws()
    {
        // Arrange
        var state = GameState.CreateInitial();
        var cts = new CancellationTokenSource();
        cts.Cancel();

        // Act
        var act = async () => await _service.CalculateBestMoveAsync(state, "medium", cts.Token);

        // Assert
        await act.Should().ThrowAsync<OperationCanceledException>();
    }

    [Fact]
    public void IsCalculating_InitiallyReturnsFalse()
    {
        // Act
        var isCalculating = _service.IsCalculating(Guid.NewGuid());

        // Assert
        isCalculating.Should().BeFalse();
    }

    [Fact]
    public async Task StartPonderingAsync_DoesNotThrow()
    {
        // Arrange
        var gameId = Guid.NewGuid();
        var state = GameState.CreateInitial();

        // Act
        var act = async () => await _service.StartPonderingAsync(gameId, state, "medium");

        // Assert
        await act.Should().NotThrowAsync();
    }

    [Fact]
    public async Task StopPonderingAsync_DoesNotThrow()
    {
        // Arrange
        var gameId = Guid.NewGuid();

        // Act
        var act = async () => await _service.StopPonderingAsync(gameId);

        // Assert
        await act.Should().NotThrowAsync();
    }

    [Fact]
    public void CleanupGame_RemovesAIState()
    {
        // Arrange
        var gameId = Guid.NewGuid();

        // Act
        _service.CleanupGame(gameId);

        // Assert - Should not throw, state is cleaned up
        _service.IsCalculating(gameId).Should().BeFalse();
    }

    [Fact]
    public void CleanupAll_RemovesAllStates()
    {
        // Act
        _service.CleanupAll();

        // Assert - Should not throw
        _service.IsCalculating(Guid.NewGuid()).Should().BeFalse();
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
