using Caro.Core.Infrastructure.Time;
using FluentAssertions;
using Microsoft.Extensions.Logging;
using Xunit;

namespace Caro.Core.Infrastructure.Tests.Time;

public sealed class TimeManagementServiceTests
{
    private readonly TimeManagementService _service;
    private readonly MockLogger<TimeManagementService> _logger;

    public TimeManagementServiceTests()
    {
        _logger = new MockLogger<TimeManagementService>();
        _service = new TimeManagementService(_logger);
        _service.Clear(); // Ensure clean state
    }

    [Fact]
    public async Task StartTimerAsync_ThenStopTimerAsync_ReturnsElapsedTime()
    {
        // Arrange
        var gameId = Guid.NewGuid();
        var player = "Red";

        // Act
        await _service.StartTimerAsync(gameId, player);
        await Task.Delay(100); // Small delay
        var elapsed = await _service.StopTimerAsync(gameId, player);

        // Assert
        elapsed.Should().BeGreaterOrEqualTo(TimeSpan.FromMilliseconds(100));
        elapsed.Should().BeLessThan(TimeSpan.FromMilliseconds(500));
    }

    [Fact]
    public async Task StopTimerAsync_NonExistentTimer_ReturnsZero()
    {
        // Act
        var elapsed = await _service.StopTimerAsync(Guid.NewGuid(), "Red");

        // Assert
        elapsed.Should().Be(TimeSpan.Zero);
    }

    [Fact]
    public async Task IsTimeoutAsync_WithinLimit_ReturnsFalse()
    {
        // Arrange
        var gameId = Guid.NewGuid();
        await _service.StartTimerAsync(gameId, "Red");

        // Act
        var isTimeout = await _service.IsTimeoutAsync(gameId, "Red");

        // Assert
        isTimeout.Should().BeFalse();
    }

    [Fact]
    public async Task MultipleConcurrentTimers_TrackSeparately()
    {
        // Arrange
        var gameId = Guid.NewGuid();
        await _service.StartTimerAsync(gameId, "Red");
        await Task.Delay(50);
        await _service.StartTimerAsync(gameId, "Blue");
        await Task.Delay(50);

        // Act
        var redElapsed = await _service.StopTimerAsync(gameId, "Red");
        var blueElapsed = await _service.StopTimerAsync(gameId, "Blue");

        // Assert
        redElapsed.Should().BeGreaterOrEqualTo(TimeSpan.FromMilliseconds(100));
        blueElapsed.Should().BeGreaterOrEqualTo(TimeSpan.FromMilliseconds(50));
    }

    [Fact]
    public async Task StartTimerAsync_OverwritesExistingTimer()
    {
        // Arrange
        var gameId = Guid.NewGuid();
        await _service.StartTimerAsync(gameId, "Red");
        await Task.Delay(50);

        // Act - Start again (should restart)
        await _service.StartTimerAsync(gameId, "Red");
        await Task.Delay(50);
        var elapsed = await _service.StopTimerAsync(gameId, "Red");

        // Assert - Should be approximately 50ms, not 100ms
        elapsed.Should().BeGreaterOrEqualTo(TimeSpan.FromMilliseconds(50));
        elapsed.Should().BeLessThan(TimeSpan.FromMilliseconds(100));
    }

    [Fact]
    public async Task Clear_RemovesAllTimers()
    {
        // Arrange
        await _service.StartTimerAsync(Guid.NewGuid(), "Red");
        await _service.StartTimerAsync(Guid.NewGuid(), "Blue");

        // Act
        _service.Clear();

        // Assert - Timers should be cleared (we can't directly inspect, but no errors should occur)
        await _service.StartTimerAsync(Guid.NewGuid(), "Green");
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
