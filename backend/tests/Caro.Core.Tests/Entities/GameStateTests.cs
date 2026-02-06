using Xunit;
using FluentAssertions;
using Caro.Core.Domain.Entities;

namespace Caro.Core.Tests.Entities;

public class GameStateUndoTests
{
    [Fact]
    public void UndoMove_WhenGameHasMoves_RevertsLastMove()
    {
        // Arrange
        var game = new GameState();

        // Make 3 moves - now GameState manages its own Board
        game.RecordMove(7, 7); // Red
        game.RecordMove(7, 8); // Blue
        game.RecordMove(8, 8); // Red

        var initialMoveNumber = game.MoveNumber;
        var initialPlayer = game.CurrentPlayer;

        // Act
        game.UndoMove();

        // Assert
        game.MoveNumber.Should().Be(initialMoveNumber - 1);
        game.CurrentPlayer.Should().Be(initialPlayer);

        // Last move should be removed
        game.Board.GetCell(8, 8).Player.Should().Be(Player.None);
        game.Board.GetCell(7, 8).Player.Should().Be(Player.Blue);
        game.Board.GetCell(7, 7).Player.Should().Be(Player.Red);
    }

    [Fact]
    public void UndoMove_WhenOnlyOneMove_RevertsToInitialState()
    {
        // Arrange
        var game = new GameState();

        game.RecordMove(7, 7);

        // Act
        game.UndoMove();

        // Assert
        game.MoveNumber.Should().Be(0);
        game.CurrentPlayer.Should().Be(Player.Red);
        game.Board.GetCell(7, 7).Player.Should().Be(Player.None);
    }

    [Fact]
    public void UndoMove_WhenNoMoves_ThrowsInvalidOperationException()
    {
        // Arrange
        var game = new GameState();

        // Act & Assert
        FluentActions.Invoking(() => game.UndoMove())
            .Should().Throw<InvalidOperationException>()
            .WithMessage("No moves to undo");
    }

    [Fact]
    public void UndoMove_WhenGameOver_ThrowsInvalidOperationException()
    {
        // Arrange
        var game = new GameState();

        game.RecordMove(7, 7);
        game.EndGame(Player.Red);

        // Act & Assert
        FluentActions.Invoking(() => game.UndoMove())
            .Should().Throw<InvalidOperationException>()
            .WithMessage("Cannot undo moves after game is over");
    }

    [Fact]
    public void CanUndo_WhenGameHasMoves_ReturnsTrue()
    {
        // Arrange
        var game = new GameState();

        game.RecordMove(7, 7);

        // Act
        var canUndo = game.CanUndo();

        // Assert
        canUndo.Should().BeTrue();
    }

    [Fact]
    public void CanUndo_WhenNoMoves_ReturnsFalse()
    {
        // Arrange
        var game = new GameState();

        // Act
        var canUndo = game.CanUndo();

        // Assert
        canUndo.Should().BeFalse();
    }

    [Fact]
    public void CanUndo_WhenGameOver_ReturnsFalse()
    {
        // Arrange
        var game = new GameState();

        game.RecordMove(7, 7);
        game.EndGame(Player.Red);

        // Act
        var canUndo = game.CanUndo();

        // Assert
        canUndo.Should().BeFalse();
    }

    [Fact]
    public void UndoMove_MultipleTimes_RevertsToInitialState()
    {
        // Arrange
        var game = new GameState();

        game.RecordMove(7, 7); // Red
        game.RecordMove(7, 8); // Blue
        game.RecordMove(8, 8); // Red

        // Act - Undo twice
        game.UndoMove();
        game.UndoMove();

        // Assert
        game.MoveNumber.Should().Be(1);
        game.CurrentPlayer.Should().Be(Player.Blue);
        game.Board.GetCell(8, 8).Player.Should().Be(Player.None);
        game.Board.GetCell(7, 8).Player.Should().Be(Player.None); // After 2 undos from 3 moves, only move 1 remains
        game.Board.GetCell(7, 7).Player.Should().Be(Player.Red);
    }
}
