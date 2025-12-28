using Xunit;
using FluentAssertions;
using Caro.Core.Entities;

namespace Caro.Core.Tests.Entities;

public class GameStateTests
{
    [Fact]
    public void NewGame_InitialState_HasCorrectDefaults()
    {
        // Act
        var game = new GameState();

        // Assert
        game.CurrentPlayer.Should().Be(Player.Red);
        game.MoveNumber.Should().Be(0);
        game.IsGameOver.Should().BeFalse();
        game.RedTimeRemaining.Should().BeCloseTo(TimeSpan.FromMinutes(3), precision: TimeSpan.FromSeconds(1));
        game.BlueTimeRemaining.Should().BeCloseTo(TimeSpan.FromMinutes(3), precision: TimeSpan.FromSeconds(1));
    }

    [Fact]
    public void RecordMove_UpdatesMoveNumberAndSwitchesPlayer()
    {
        // Arrange
        var game = new GameState();
        var board = game.Board;

        // Act
        game.RecordMove(board, 7, 7);

        // Assert
        game.MoveNumber.Should().Be(1);
        game.CurrentPlayer.Should().Be(Player.Blue);
        board.GetCell(7, 7).Player.Should().Be(Player.Red);
    }

    [Fact]
    public void RecordMove_AlternatesPlayersCorrectly()
    {
        // Arrange
        var game = new GameState();
        var board = game.Board;

        // Act
        game.RecordMove(board, 7, 7);  // Red
        game.RecordMove(board, 8, 7);  // Blue

        // Assert
        game.MoveNumber.Should().Be(2);
        game.CurrentPlayer.Should().Be(Player.Red);
    }

    [Fact]
    public void RecordMove_Adds2SecondsToCurrentPlayer()
    {
        // Arrange
        var game = new GameState();
        var board = game.Board;
        var initialRedTime = game.RedTimeRemaining;

        // Act
        game.RecordMove(board, 7, 7);

        // Assert
        game.RedTimeRemaining.Should().Be(initialRedTime + TimeSpan.FromSeconds(2));
    }

    [Fact]
    public void RecordMove_SwitchesPlayer()
    {
        // Arrange
        var game = new GameState();
        var board = game.Board;

        // Act
        game.RecordMove(board, 7, 7);

        // Assert
        game.CurrentPlayer.Should().Be(Player.Blue);
    }

    [Fact]
    public void EndGame_SetsGameOverAndWinner()
    {
        // Arrange
        var game = new GameState();

        // Act
        game.EndGame(Player.Red);

        // Assert
        game.IsGameOver.Should().BeTrue();
        game.CurrentPlayer.Should().Be(Player.None);
    }

    [Fact]
    public void EndGame_StoresWinner()
    {
        // Arrange
        var game = new GameState();

        // Act
        game.EndGame(Player.Blue);

        // Assert
        game.Winner.Should().Be(Player.Blue);
    }

    [Fact]
    public void EndGame_StoresWinningLine()
    {
        // Arrange
        var game = new GameState();
        var winningLine = new List<Caro.Core.GameLogic.Position>
        {
            new(5, 7),
            new(6, 7),
            new(7, 7),
            new(8, 7),
            new(9, 7)
        };

        // Act
        game.EndGame(Player.Red, winningLine);

        // Assert
        game.WinningLine.Should().HaveCount(5);
        game.WinningLine[0].X.Should().Be(5);
        game.WinningLine[0].Y.Should().Be(7);
        game.WinningLine[4].X.Should().Be(9);
        game.WinningLine[4].Y.Should().Be(7);
    }

    [Fact]
    public void NewGame_HasNoWinnerInitially()
    {
        // Arrange & Act
        var game = new GameState();

        // Assert
        game.Winner.Should().Be(Player.None);
        game.WinningLine.Should().BeEmpty();
    }
}
