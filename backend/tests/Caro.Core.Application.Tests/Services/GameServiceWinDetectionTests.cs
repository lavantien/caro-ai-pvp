using Caro.Core.Application.DTOs;
using Caro.Core.Application.Interfaces;
using Caro.Core.Application.Services;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

namespace Caro.Core.Application.Tests.Services;

public class GameServiceWinDetectionTests
{
    private readonly Mock<IGameRepository> _mockRepository;
    private readonly Mock<IAIService> _mockAiService;
    private readonly Mock<ITimeManagementService> _mockTimeService;
    private readonly Mock<ILogger<GameService>> _mockLogger;
    private readonly GameService _service;

    public GameServiceWinDetectionTests()
    {
        _mockRepository = new Mock<IGameRepository>();
        _mockAiService = new Mock<IAIService>();
        _mockTimeService = new Mock<ITimeManagementService>();
        _mockLogger = new Mock<ILogger<GameService>>();
        _service = new GameService(
            _mockRepository.Object,
            _mockAiService.Object,
            _mockTimeService.Object,
            _mockLogger.Object);

        // Setup time service mocks
        _mockTimeService.Setup(x => x.StopTimerAsync(It.IsAny<Guid>(), It.IsAny<string>(), It.IsAny<CancellationToken>()))
            .ReturnsAsync(TimeSpan.FromMinutes(3));
        _mockTimeService.Setup(x => x.StartTimerAsync(It.IsAny<Guid>(), It.IsAny<string>(), It.IsAny<CancellationToken>()))
            .Returns(Task.CompletedTask);
    }

    [Fact]
    public async Task MakeMoveAsync_Exactly5InRow_ReturnsWin()
    {
        // Arrange - Create a game state with alternating moves:
        // Red: 9,9 | 5,7 | 6,7 | 7,7 | 8,7
        // Blue: 0,0 | 0,1 | 0,2 | 0,3
        // Red's turn to play (9,7) for 5 in a row
        var gameState = GameStateFactory.CreateInitial();
        gameState = gameState.WithMove(9, 9);  // Red
        gameState = gameState.WithMove(0, 0);  // Blue
        gameState = gameState.WithMove(5, 7);  // Red
        gameState = gameState.WithMove(0, 1);  // Blue
        gameState = gameState.WithMove(6, 7);  // Red
        gameState = gameState.WithMove(0, 2);  // Blue
        gameState = gameState.WithMove(7, 7);  // Red
        gameState = gameState.WithMove(0, 3);  // Blue
        gameState = gameState.WithMove(8, 7);  // Red
        gameState = gameState.WithMove(0, 4);  // Blue

        var gameId = Guid.NewGuid();
        _mockRepository.Setup(x => x.LoadAsync(gameId, It.IsAny<CancellationToken>()))
            .ReturnsAsync(gameState);
        _mockRepository.Setup(x => x.SaveAsync(gameId, It.IsAny<GameState>(), It.IsAny<CancellationToken>()))
            .Returns(Task.CompletedTask);

        // Act - Place the 5th Red stone at position 9,7
        var result = await _service.MakeMoveAsync(gameId, new MakeMoveRequest { X = 9, Y = 7 });

        // Assert
        result.Success.Should().BeTrue();
        result.State!.IsGameOver.Should().BeTrue();
        result.State.Winner.Should().Be("Red");
    }

    [Fact]
    public async Task MakeMoveAsync_6InRow_ReturnsNoWin_OverlineRule()
    {
        // Arrange - Create a game state where Red has 5 in a row (positions 5-9)
        // and tries to extend to 6
        var gameState = GameStateFactory.CreateInitial();
        gameState = gameState.WithMove(9, 9);  // Red
        gameState = gameState.WithMove(0, 0);  // Blue
        gameState = gameState.WithMove(5, 7);  // Red
        gameState = gameState.WithMove(0, 1);  // Blue
        gameState = gameState.WithMove(6, 7);  // Red
        gameState = gameState.WithMove(0, 2);  // Blue
        gameState = gameState.WithMove(7, 7);  // Red
        gameState = gameState.WithMove(0, 3);  // Blue
        gameState = gameState.WithMove(8, 7);  // Red
        gameState = gameState.WithMove(0, 4);  // Blue
        gameState = gameState.WithMove(9, 7);  // Red (5 in a row, but open end at 10,7)
        gameState = gameState.WithMove(0, 5);  // Blue

        var gameId = Guid.NewGuid();
        _mockRepository.Setup(x => x.LoadAsync(gameId, It.IsAny<CancellationToken>()))
            .ReturnsAsync(gameState);
        _mockRepository.Setup(x => x.SaveAsync(gameId, It.IsAny<GameState>(), It.IsAny<CancellationToken>()))
            .Returns(Task.CompletedTask);

        // Act - Place the 6th Red stone at position 10,7
        var result = await _service.MakeMoveAsync(gameId, new MakeMoveRequest { X = 10, Y = 7 });

        // Assert - Caro rules: 6 in a row (overline) is NOT a win
        result.Success.Should().BeTrue();
        result.State!.IsGameOver.Should().BeFalse();
    }

    [Fact]
    public async Task MakeMoveAsync_5InRowWithBothEndsBlocked_ReturnsNoWin_BlockedRule()
    {
        // Arrange - Create a board with Blue blocking both ends
        // Blue at (4,7), Red at (5-9,7), Blue at (10,7)
        var gameState = GameStateFactory.CreateInitial();
        // Place Blue blockers first on empty board - we need to orchestrate turns
        // Red starts, so Red places first
        gameState = gameState.WithMove(9, 9);  // Red (somewhere else)
        gameState = gameState.WithMove(4, 7);  // Blue - left blocker
        gameState = gameState.WithMove(5, 7);  // Red
        gameState = gameState.WithMove(10, 7); // Blue - right blocker
        gameState = gameState.WithMove(6, 7);  // Red
        gameState = gameState.WithMove(0, 0);  // Blue
        gameState = gameState.WithMove(7, 7);  // Red
        gameState = gameState.WithMove(0, 1);  // Blue
        gameState = gameState.WithMove(8, 7);  // Red
        gameState = gameState.WithMove(0, 2);  // Blue
        // Now Red plays (9,7) to complete 5 in a row - but both ends are blocked

        var gameId = Guid.NewGuid();
        _mockRepository.Setup(x => x.LoadAsync(gameId, It.IsAny<CancellationToken>()))
            .ReturnsAsync(gameState);
        _mockRepository.Setup(x => x.SaveAsync(gameId, It.IsAny<GameState>(), It.IsAny<CancellationToken>()))
            .Returns(Task.CompletedTask);

        // Act - Place the 5th Red stone at position 9,7
        var result = await _service.MakeMoveAsync(gameId, new MakeMoveRequest { X = 9, Y = 7 });

        // Assert - Caro rules: both ends blocked is NOT a win
        result.Success.Should().BeTrue();
        result.State!.IsGameOver.Should().BeFalse();
    }

    [Fact]
    public async Task MakeMoveAsync_5InRowWithOneEndBlocked_ReturnsWin()
    {
        // Arrange - Create a board with only one end blocked
        // Blue at (4,7), Red at (5-9,7), empty at (10,7)
        var gameState = GameStateFactory.CreateInitial();
        gameState = gameState.WithMove(9, 9);  // Red (somewhere else)
        gameState = gameState.WithMove(4, 7);  // Blue - left blocker only
        gameState = gameState.WithMove(5, 7);  // Red
        gameState = gameState.WithMove(0, 0);  // Blue (somewhere else, not blocking right)
        gameState = gameState.WithMove(6, 7);  // Red
        gameState = gameState.WithMove(0, 1);  // Blue
        gameState = gameState.WithMove(7, 7);  // Red
        gameState = gameState.WithMove(0, 2);  // Blue
        gameState = gameState.WithMove(8, 7);  // Red
        gameState = gameState.WithMove(0, 3);  // Blue
        // Now Red plays (9,7) to complete 5 in a row - only left end is blocked

        var gameId = Guid.NewGuid();
        _mockRepository.Setup(x => x.LoadAsync(gameId, It.IsAny<CancellationToken>()))
            .ReturnsAsync(gameState);
        _mockRepository.Setup(x => x.SaveAsync(gameId, It.IsAny<GameState>(), It.IsAny<CancellationToken>()))
            .Returns(Task.CompletedTask);

        // Act - Place the 5th Red stone at position 9,7
        var result = await _service.MakeMoveAsync(gameId, new MakeMoveRequest { X = 9, Y = 7 });

        // Assert - Caro rules: one end blocked IS a win
        result.Success.Should().BeTrue();
        result.State!.IsGameOver.Should().BeTrue();
        result.State.Winner.Should().Be("Red");
    }

    [Fact]
    public async Task MakeMoveAsync_5InVertical_ReturnsWin()
    {
        // Arrange - Create a vertical line of Red stones
        var gameState = GameStateFactory.CreateInitial();
        gameState = gameState.WithMove(9, 9);  // Red
        gameState = gameState.WithMove(0, 0);  // Blue
        gameState = gameState.WithMove(7, 5);  // Red
        gameState = gameState.WithMove(1, 1);  // Blue
        gameState = gameState.WithMove(7, 6);  // Red
        gameState = gameState.WithMove(2, 2);  // Blue
        gameState = gameState.WithMove(7, 7);  // Red
        gameState = gameState.WithMove(3, 3);  // Blue
        gameState = gameState.WithMove(7, 8);  // Red
        gameState = gameState.WithMove(4, 4);  // Blue

        var gameId = Guid.NewGuid();
        _mockRepository.Setup(x => x.LoadAsync(gameId, It.IsAny<CancellationToken>()))
            .ReturnsAsync(gameState);
        _mockRepository.Setup(x => x.SaveAsync(gameId, It.IsAny<GameState>(), It.IsAny<CancellationToken>()))
            .Returns(Task.CompletedTask);

        // Act - Place the 5th Red stone at (7,9)
        var result = await _service.MakeMoveAsync(gameId, new MakeMoveRequest { X = 7, Y = 9 });

        // Assert
        result.Success.Should().BeTrue();
        result.State!.IsGameOver.Should().BeTrue();
        result.State.Winner.Should().Be("Red");
    }

    [Fact]
    public async Task MakeMoveAsync_5InDiagonal_ReturnsWin()
    {
        // Arrange - Create a diagonal line of Red stones
        var gameState = GameStateFactory.CreateInitial();
        gameState = gameState.WithMove(9, 0);  // Red
        gameState = gameState.WithMove(0, 0);  // Blue
        gameState = gameState.WithMove(5, 5);  // Red
        gameState = gameState.WithMove(1, 1);  // Blue
        gameState = gameState.WithMove(6, 6);  // Red
        gameState = gameState.WithMove(2, 2);  // Blue
        gameState = gameState.WithMove(7, 7);  // Red
        gameState = gameState.WithMove(3, 3);  // Blue
        gameState = gameState.WithMove(8, 8);  // Red
        gameState = gameState.WithMove(4, 4);  // Blue

        var gameId = Guid.NewGuid();
        _mockRepository.Setup(x => x.LoadAsync(gameId, It.IsAny<CancellationToken>()))
            .ReturnsAsync(gameState);
        _mockRepository.Setup(x => x.SaveAsync(gameId, It.IsAny<GameState>(), It.IsAny<CancellationToken>()))
            .Returns(Task.CompletedTask);

        // Act - Place the 5th Red stone at (9,9)
        var result = await _service.MakeMoveAsync(gameId, new MakeMoveRequest { X = 9, Y = 9 });

        // Assert
        result.Success.Should().BeTrue();
        result.State!.IsGameOver.Should().BeTrue();
        result.State.Winner.Should().Be("Red");
    }
}
