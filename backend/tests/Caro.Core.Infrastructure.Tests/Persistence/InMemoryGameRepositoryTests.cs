using Caro.Core.Application.Extensions;
using Caro.Core.Domain.Entities;
using Caro.Core.Infrastructure.Persistence;
using FluentAssertions;
using Microsoft.Extensions.Logging;
using Xunit;

namespace Caro.Core.Infrastructure.Tests.Persistence;

public sealed class InMemoryGameRepositoryTests
{
    private readonly InMemoryGameRepository _repository;
    private readonly MockLogger<InMemoryGameRepository> _logger;

    public InMemoryGameRepositoryTests()
    {
        _logger = new MockLogger<InMemoryGameRepository>();
        _repository = new InMemoryGameRepository(_logger);
    }

    [Fact]
    public async Task SaveAsync_ThenLoadAsync_ReturnsSavedState()
    {
        // Arrange
        var gameId = Guid.NewGuid();
        var state = GameStateFactory.CreateInitial();

        // Act
        await _repository.SaveAsync(gameId, state);
        var loaded = await _repository.LoadAsync(gameId);

        // Assert
        loaded.Should().NotBeNull();
        loaded!.MoveNumber.Should().Be(state.MoveNumber);
        loaded.CurrentPlayer.Should().Be(state.CurrentPlayer);
    }

    [Fact]
    public async Task ExistsAsync_ExistingGame_ReturnsTrue()
    {
        // Arrange
        var gameId = Guid.NewGuid();
        var state = GameStateFactory.CreateInitial();
        await _repository.SaveAsync(gameId, state);

        // Act
        var exists = await _repository.ExistsAsync(gameId);

        // Assert
        exists.Should().BeTrue();
    }

    [Fact]
    public async Task ExistsAsync_NonExistentGame_ReturnsFalse()
    {
        // Act
        var exists = await _repository.ExistsAsync(Guid.NewGuid());

        // Assert
        exists.Should().BeFalse();
    }

    [Fact]
    public async Task DeleteAsync_ExistingGame_ReturnsTrue()
    {
        // Arrange
        var gameId = Guid.NewGuid();
        var state = GameStateFactory.CreateInitial();
        await _repository.SaveAsync(gameId, state);

        // Act
        var deleted = await _repository.DeleteAsync(gameId);

        // Assert
        deleted.Should().BeTrue();
        (await _repository.ExistsAsync(gameId)).Should().BeFalse();
    }

    [Fact]
    public async Task DeleteAsync_NonExistentGame_ReturnsFalse()
    {
        // Act
        var deleted = await _repository.DeleteAsync(Guid.NewGuid());

        // Assert
        deleted.Should().BeFalse();
    }

    [Fact]
    public async Task GetAllIdsAsync_ReturnsAllGameIds()
    {
        // Arrange
        var id1 = Guid.NewGuid();
        var id2 = Guid.NewGuid();
        var id3 = Guid.NewGuid();
        await _repository.SaveAsync(id1, GameStateFactory.CreateInitial());
        await _repository.SaveAsync(id2, GameStateFactory.CreateInitial());
        await _repository.SaveAsync(id3, GameStateFactory.CreateInitial());

        // Act
        var ids = await _repository.GetAllIdsAsync();

        // Assert
        ids.Should().HaveCount(3);
        ids.Should().Contain(id1);
        ids.Should().Contain(id2);
        ids.Should().Contain(id3);
    }

    [Fact]
    public async Task SaveAsync_UpdatesExistingGame()
    {
        // Arrange
        var gameId = Guid.NewGuid();
        var state1 = GameStateFactory.CreateInitial();
        await _repository.SaveAsync(gameId, state1);

        // Act - MakeMove returns a new state with incremented move number
        var state2 = state1.MakeMove(9, 9);
        await _repository.SaveAsync(gameId, state2);
        var loaded = await _repository.LoadAsync(gameId);

        // Assert
        loaded.Should().NotBeNull();
        // After first move, move number should be 2 (0 is initial, then Red plays = 1, then Blue plays = 2)
        // But actually MakeMove changes CurrentPlayer to the other player, so:
        // Initial: MoveNumber=0, CurrentPlayer=Red
        // After MakeMove(9,9): MoveNumber=1, CurrentPlayer=Blue (Red just moved)
        loaded!.MoveNumber.Should().Be(1);
        (await _repository.GetAllIdsAsync()).Should().HaveCount(1);
    }

    [Fact]
    public async Task LoadAsync_NonExistentGame_ReturnsNull()
    {
        // Act
        var loaded = await _repository.LoadAsync(Guid.NewGuid());

        // Assert
        loaded.Should().BeNull();
    }

    [Fact]
    public async Task Clear_RemovesAllGames()
    {
        // Arrange
        await _repository.SaveAsync(Guid.NewGuid(), GameStateFactory.CreateInitial());
        await _repository.SaveAsync(Guid.NewGuid(), GameStateFactory.CreateInitial());

        // Act
        _repository.Clear();
        var ids = await _repository.GetAllIdsAsync();

        // Assert
        ids.Should().BeEmpty();
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
