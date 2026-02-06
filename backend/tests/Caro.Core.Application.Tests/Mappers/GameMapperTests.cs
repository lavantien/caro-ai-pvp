using Caro.Core.Application.DTOs;
using Caro.Core.Application.Mappers;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using FluentAssertions;

namespace Caro.Core.Application.Tests.Mappers;

public class GameMapperTests
{
    [Fact]
    public void ToDto_ConvertsGameStateCorrectly()
    {
        // Arrange
        var state = GameStateFactory.CreateInitial();
        var gameId = Guid.NewGuid();

        // Act
        var dto = GameMapper.ToDto(state, gameId);

        // Assert
        dto.GameId.Should().Be(gameId);
        dto.CurrentPlayer.Should().Be("Red");
        dto.MoveNumber.Should().Be(0);
        dto.IsGameOver.Should().BeFalse();
        dto.Winner.Should().BeNull();
        dto.RedTimeRemaining.Should().Be("07:00");
        dto.BlueTimeRemaining.Should().Be("07:00");
        dto.MoveHistory.Should().BeEmpty();
    }

    [Fact]
    public void ToDto_WithMove_ConvertsCorrectly()
    {
        // Arrange
        var state = GameStateFactory.CreateInitial();
        state.RecordMove(9, 9);
        var gameId = Guid.NewGuid();

        // Act
        var dto = GameMapper.ToDto(state, gameId);

        // Assert
        dto.MoveNumber.Should().Be(1);
        dto.CurrentPlayer.Should().Be("Blue");
        dto.MoveHistory.Should().HaveCount(1);
        dto.MoveHistory[0].X.Should().Be(9);
        dto.MoveHistory[0].Y.Should().Be(9);
        dto.MoveHistory[0].Player.Should().Be("Red");
    }

    [Fact]
    public void ToBoardDto_ConvertsBoardCorrectly()
    {
        // Arrange
        var board = new Board();
        board.PlaceStone(5, 5, Player.Red);

        // Act
        var dto = GameMapper.ToBoardDto(board);

        // Assert
        dto.Cells.Should().HaveCount(361); // 19x19
        dto.Cells[5 * 19 + 5].Should().Be("Red"); // Position (5,5) in linear array
    }

    [Fact]
    public void ToPositionDtos_ConvertsPositionsCorrectly()
    {
        // Arrange
        var positions = new Position[]
        {
            new(0, 0),
            new(5, 10),
            new(18, 18)
        };

        // Act
        var dtos = GameMapper.ToPositionDtos(positions);

        // Assert
        dtos.Should().HaveCount(3);
        dtos[0].X.Should().Be(0);
        dtos[0].Y.Should().Be(0);
        dtos[1].X.Should().Be(5);
        dtos[1].Y.Should().Be(10);
    }

    [Fact]
    public void ToPosition_ConvertsDtoToPosition()
    {
        // Arrange
        var dto = new PositionDto(10, 15);

        // Act
        var position = GameMapper.ToPosition(dto);

        // Assert
        position.X.Should().Be(10);
        position.Y.Should().Be(15);
    }

    [Fact]
    public void ToPlayer_ConvertsStringToPlayer()
    {
        // Act & Assert
        GameMapper.ToPlayer("Red").Should().Be(Player.Red);
        GameMapper.ToPlayer("Blue").Should().Be(Player.Blue);
        GameMapper.ToPlayer("red").Should().Be(Player.Red); // Case insensitive
        GameMapper.ToPlayer("Invalid").Should().Be(Player.None);
    }

    [Fact]
    public void ParseTimeControl_ReturnsCorrectTimeSpans()
    {
        // Arrange
        var request = new CreateGameRequest
        {
            TimeControl = "Bullet",
            InitialTimeMinutes = 1,
            IncrementSeconds = 2
        };

        // Act
        var (initialTime, increment) = GameMapper.ParseTimeControl(request);

        // Assert
        initialTime.Should().Be(TimeSpan.FromMinutes(1));
        increment.Should().Be(TimeSpan.FromSeconds(2));
    }

    [Fact]
    public void ToAIMoveResponse_CreatesCorrectResponse()
    {
        // Act
        var response = GameMapper.ToAIMoveResponse(
            10, 15,  // x, y
            5,       // depth
            100000,  // nodes
            10000.0, // nps
            1000,    // timeMs
            100,     // score
            false    // pondering
        );

        // Assert
        response.X.Should().Be(10);
        response.Y.Should().Be(15);
        response.DepthAchieved.Should().Be(5);
        response.NodesSearched.Should().Be(100000);
        response.NodesPerSecond.Should().Be(10000.0);
        response.TimeTakenMs.Should().Be(1000);
        response.Score.Should().Be(100);
        response.PonderingActive.Should().BeFalse();
    }
}
