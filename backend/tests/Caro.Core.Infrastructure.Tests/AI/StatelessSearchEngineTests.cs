using Caro.Core.Application.DTOs;
using Caro.Core.Application.Extensions;
using Caro.Core.Domain.Entities;
using Caro.Core.Domain.Interfaces;
using Caro.Core.Domain.ValueObjects;
using Caro.Core.GameLogic;
using Caro.Core.Infrastructure.AI;
using FluentAssertions;
using Microsoft.Extensions.Logging;
using Xunit;
using SearchOptions = Caro.Core.Infrastructure.AI.SearchOptions;

namespace Caro.Core.Infrastructure.Tests.AI;

public sealed class StatelessSearchEngineTests
{
    private readonly StatelessSearchEngine _engine;
    private readonly MockLogger<StatelessSearchEngine> _logger;

    public StatelessSearchEngineTests()
    {
        _logger = new MockLogger<StatelessSearchEngine>();
        _engine = new StatelessSearchEngine(_logger);
    }

    [Fact]
    public void FindBestMove_EmptyBoard_ReturnsValidMove()
    {
        // Arrange
        var state = GameStateFactory.CreateInitial();
        var aiState = new AIGameState(maxDepth: 5, tableSizeMB: 16);
        var options = new SearchOptions { MaxDepth = 3, TimeLimitMs = 5000 };

        // Act
        var (x, y, score, stats) = _engine.FindBestMove(state, aiState, options);

        // Assert
        // Should return a valid move near the center (AI prefers center positions)
        (x, y).Should().NotBe((-1, -1));
        x.Should().BeGreaterOrEqualTo(7);
        x.Should().BeLessOrEqualTo(11);
        y.Should().BeGreaterOrEqualTo(7);
        y.Should().BeLessOrEqualTo(11);
        score.Should().NotBe(int.MinValue);
        stats.NodesSearched.Should().BeGreaterThan(0);
        stats.DepthReached.Should().BeGreaterThan(0);
    }

    [Fact]
    public void FindBestMove_NonEmptyBoard_ReturnsValidMove()
    {
        // Arrange - Create a position with both players having stones
        // Red has stones at (9,5), (9,6), (9,7), (9,8)
        // Blue has stones at (0,0), (0,1), (0,2), (1,1)
        var state = GameStateFactory.CreateInitial()
            .MakeMove(9, 5).MakeMove(0, 0)  // Red at (9,5), Blue at (0,0)
            .MakeMove(9, 6).MakeMove(0, 1)  // Red at (9,6), Blue at (0,1)
            .MakeMove(9, 7).MakeMove(0, 2)  // Red at (9,7), Blue at (0,2)
            .MakeMove(9, 8).MakeMove(1, 1); // Red at (9,8), Blue at (1,1) - Blue's turn
        var aiState = new AIGameState(maxDepth: 5, tableSizeMB: 16);
        var options = new SearchOptions { MaxDepth = 3, TimeLimitMs = 5000 };

        // Act
        var (x, y, score, stats) = _engine.FindBestMove(state, aiState, options);

        // Assert - Blue should make a valid move
        (x, y).Should().NotBe((-1, -1));
        x.Should().BeGreaterOrEqualTo(0);
        y.Should().BeGreaterOrEqualTo(0);
    }

    [Fact]
    public void FindBestMove_RespectsMaxDepth()
    {
        // Arrange - Each test should use a fresh AIGameState
        var state = GameStateFactory.CreateInitial();
        var aiState = new AIGameState(maxDepth: 5, tableSizeMB: 16);
        var options = new SearchOptions { MaxDepth = 1, TimeLimitMs = 5000 };

        // Act
        var (_, _, _, stats) = _engine.FindBestMove(state, aiState, options);

        // Assert - With MaxDepth=1, should not exceed depth 1
        stats.DepthReached.Should().BeLessOrEqualTo(1);
    }

    [Fact]
    public void FindBestMove_PopulatesStatistics()
    {
        // Arrange
        var state = GameStateFactory.CreateInitial();
        var aiState = new AIGameState(maxDepth: 5, tableSizeMB: 16);
        var options = new SearchOptions { MaxDepth = 2, TimeLimitMs = 5000 };

        // Act
        var (_, _, _, stats) = _engine.FindBestMove(state, aiState, options);

        // Assert
        stats.NodesSearched.Should().BeGreaterThan(0);
        stats.DepthReached.Should().BeGreaterThan(0);
        stats.ElapsedMs.Should().BeGreaterOrEqualTo(0);
        stats.NodesPerSecond.Should().BeGreaterThan(0);
        stats.TableLookups.Should().BeGreaterOrEqualTo(0);
        stats.TableHits.Should().BeGreaterOrEqualTo(0);
        stats.HitRate.Should().BeGreaterOrEqualTo(0);
        stats.HitRate.Should().BeLessOrEqualTo(1);
    }

    [Fact]
    public void FindBestMove_UsesTranspositionTable()
    {
        // Arrange
        var state = GameStateFactory.CreateInitial();
        var aiState = new AIGameState(maxDepth: 5, tableSizeMB: 16);
        var options = new SearchOptions { MaxDepth = 2, TimeLimitMs = 5000 };

        // Act - First search
        _engine.FindBestMove(state, aiState, options);
        var firstLookups = aiState.TableLookups;

        // Second search (should have some TT hits)
        aiState.ResetStatistics();
        _engine.FindBestMove(state, aiState, options);

        // Assert
        firstLookups.Should().BeGreaterThan(0);
        aiState.TableLookups.Should().BeGreaterThan(0);
    }

    [Fact]
    public void FindBestMove_UpdatesHistoryHeuristic()
    {
        // Arrange
        var state = GameStateFactory.CreateInitial();
        var aiState = new AIGameState(maxDepth: 5, tableSizeMB: 16);
        var options = new SearchOptions { MaxDepth = 2, TimeLimitMs = 5000 };

        // Act
        _engine.FindBestMove(state, aiState, options);

        // Assert - Center position should have history score
        var centerScore = aiState.GetHistoryScore(new Position(9, 9));
        centerScore.Should().BeGreaterOrEqualTo(0);
    }

    [Fact]
    public void FindBestMove_Cancelled_ThrowsOperationCanceledException()
    {
        // Arrange
        var state = GameStateFactory.CreateInitial();
        var aiState = new AIGameState(maxDepth: 5, tableSizeMB: 16);
        var options = new SearchOptions { MaxDepth = 10, TimeLimitMs = 5000 };
        var cts = new CancellationTokenSource();

        // Act
        cts.Cancel();
        var act = () => _engine.FindBestMove(state, aiState, options, cts.Token);

        // Assert
        act.Should().Throw<OperationCanceledException>();
    }

    [Fact]
    public async Task FindBestMove_Cancelled_ReturnsBestMoveFound()
    {
        // Arrange
        var state = GameStateFactory.CreateInitial();
        var aiState = new AIGameState(maxDepth: 5, tableSizeMB: 16);
        var options = new SearchOptions { MaxDepth = 10, TimeLimitMs = 5000 };
        var cts = new CancellationTokenSource();

        // Act - Cancel after a short delay
        var task = Task.Run(() => _engine.FindBestMove(state, aiState, options, cts.Token));
        await Task.Delay(50);
        cts.Cancel();
        var (x, y, score, stats) = await task;

        // Assert - Should return some move, not throw
        (x, y).Should().NotBe((-1, -1));
        stats.NodesSearched.Should().BeGreaterOrEqualTo(0);
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
