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
        var game = GameState.CreateInitial();

        // Make 3 moves - now GameState manages its own Board
        game = game.WithMove(7, 7); // Red
        game = game.WithMove(7, 8); // Blue
        game = game.WithMove(8, 8); // Red

        var initialMoveNumber = game.MoveNumber;
        var initialPlayer = game.CurrentPlayer;

        // Act
        var undoneGame = game.UndoMove();

        // Assert
        undoneGame.MoveNumber.Should().Be(initialMoveNumber - 1);
        undoneGame.CurrentPlayer.Should().Be(initialPlayer);

        // Last move should be removed
        undoneGame.Board.GetCell(8, 8).Player.Should().Be(Player.None);
        undoneGame.Board.GetCell(7, 8).Player.Should().Be(Player.Blue);
        undoneGame.Board.GetCell(7, 7).Player.Should().Be(Player.Red);
    }

    [Fact]
    public void UndoMove_WhenOnlyOneMove_RevertsToInitialState()
    {
        // Arrange
        var game = GameState.CreateInitial();

        game = game.WithMove(7, 7);

        // Act
        var undoneGame = game.UndoMove();

        // Assert
        undoneGame.MoveNumber.Should().Be(0);
        undoneGame.CurrentPlayer.Should().Be(Player.Red);
        undoneGame.Board.GetCell(7, 7).Player.Should().Be(Player.None);
    }

    [Fact]
    public void UndoMove_WhenNoMoves_ThrowsInvalidOperationException()
    {
        // Arrange
        var game = GameState.CreateInitial();

        // Act & Assert
        FluentActions.Invoking(() => game.UndoMove())
            .Should().Throw<InvalidOperationException>()
            .WithMessage("No moves to undo");
    }

    [Fact]
    public void UndoMove_WhenGameOver_ThrowsInvalidOperationException()
    {
        // Arrange
        var game = GameState.CreateInitial();

        game = game.WithMove(7, 7);
        game = game.WithGameOver(Player.Red);

        // Act & Assert
        FluentActions.Invoking(() => game.UndoMove())
            .Should().Throw<InvalidOperationException>()
            .WithMessage("Cannot undo moves after game is over");
    }

    [Fact]
    public void CanUndo_WhenGameHasMoves_ReturnsTrue()
    {
        // Arrange
        var game = GameState.CreateInitial();

        game = game.WithMove(7, 7);

        // Act
        var canUndo = game.CanUndo();

        // Assert
        canUndo.Should().BeTrue();
    }

    [Fact]
    public void CanUndo_WhenNoMoves_ReturnsFalse()
    {
        // Arrange
        var game = GameState.CreateInitial();

        // Act
        var canUndo = game.CanUndo();

        // Assert
        canUndo.Should().BeFalse();
    }

    [Fact]
    public void CanUndo_WhenGameOver_ReturnsFalse()
    {
        // Arrange
        var game = GameState.CreateInitial();

        game = game.WithMove(7, 7);
        game = game.WithGameOver(Player.Red);

        // Act
        var canUndo = game.CanUndo();

        // Assert
        canUndo.Should().BeFalse();
    }

    [Fact]
    public void UndoMove_MultipleTimes_RevertsToInitialState()
    {
        // Arrange
        var game = GameState.CreateInitial();

        game = game.WithMove(7, 7); // Red
        game = game.WithMove(7, 8); // Blue
        game = game.WithMove(8, 8); // Red

        // Act - Undo twice
        var undoneOnce = game.UndoMove();
        var undoneTwice = undoneOnce.UndoMove();

        // Assert
        undoneTwice.MoveNumber.Should().Be(1);
        undoneTwice.CurrentPlayer.Should().Be(Player.Blue);
        undoneTwice.Board.GetCell(8, 8).Player.Should().Be(Player.None);
        undoneTwice.Board.GetCell(7, 8).Player.Should().Be(Player.None); // After 2 undos from 3 moves, only move 1 remains
        undoneTwice.Board.GetCell(7, 7).Player.Should().Be(Player.Red);
    }
}
