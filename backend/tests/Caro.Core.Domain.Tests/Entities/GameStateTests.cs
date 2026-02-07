using Caro.Core.Domain.Entities;
using FluentAssertions;
using System.Collections.Immutable;

namespace Caro.Core.Domain.Tests.Entities;

public class GameStateTests
{
    [Fact]
    public void CreateInitial_ReturnsInitialState()
    {
        // Act
        var state = GameStateFactory.CreateInitial();

        // Assert
        state.CurrentPlayer.Should().Be(Player.Red);
        state.MoveNumber.Should().Be(0);
        state.IsGameOver.Should().BeFalse();
        state.Winner.Should().Be(Player.None);
        state.Board.IsEmpty().Should().BeTrue();
    }

    [Fact]
    public void CreateInitial_WithCustomParameters()
    {
        // Arrange
        const string timeControl = "10+3";
        const long initialTimeMs = 600_000;
        const int incrementSeconds = 3;
        const string gameMode = "pvai";
        const string redAIDifficulty = "medium";

        // Act
        var state = GameState.CreateInitial(
            timeControl,
            initialTimeMs,
            incrementSeconds,
            gameMode,
            redAIDifficulty,
            null);

        // Assert
        state.TimeControl.Should().Be(timeControl);
        state.InitialTimeMs.Should().Be(initialTimeMs);
        state.IncrementSeconds.Should().Be(incrementSeconds);
        state.GameMode.Should().Be(gameMode);
        state.RedAIDifficulty.Should().Be(redAIDifficulty);
        state.BlueAIDifficulty.Should().BeNull();
    }

    [Fact]
    public void WithMove_ReturnsNewState_WithoutMutatingOriginal()
    {
        // Arrange
        var originalState = GameStateFactory.CreateInitial();

        // Act
        var newState = originalState.WithMove(9, 9);

        // Assert
        originalState.MoveNumber.Should().Be(0);
        originalState.Board.IsEmpty().Should().BeTrue();

        newState.MoveNumber.Should().Be(1);
        newState.Board.GetCell(9, 9).Player.Should().Be(Player.Red);
        newState.CurrentPlayer.Should().Be(Player.Blue);
    }

    [Fact]
    public void WithMove_SwitchesPlayer()
    {
        // Arrange
        var state = GameStateFactory.CreateInitial();

        // Act
        var newState = state.WithMove(9, 9);

        // Assert
        state.CurrentPlayer.Should().Be(Player.Red);
        newState.CurrentPlayer.Should().Be(Player.Blue);
    }

    [Fact]
    public void WithMove_AddsToMoveHistory()
    {
        // Arrange
        var state = GameStateFactory.CreateInitial();

        // Act
        var newState = state.WithMove(9, 9);

        // Assert
        state.MoveHistory.Length.Should().Be(0);
        newState.MoveHistory.Length.Should().Be(1);
        newState.MoveHistory[0].X.Should().Be(9);
        newState.MoveHistory[0].Y.Should().Be(9);
    }

    [Fact]
    public void WithMove_AfterGameOver_ThrowsInvalidOperationException()
    {
        // Arrange
        var state = GameStateFactory.CreateInitial().WithGameOver(Player.Red);

        // Act & Assert
        Action act = () => state.WithMove(9, 9);
        act.Should().Throw<InvalidOperationException>()
            .WithMessage("Cannot make moves after game is over*");
    }

    [Fact]
    public void WithMove_AtOccupiedCell_ThrowsInvalidOperationException()
    {
        // Arrange
        var state = GameStateFactory.CreateInitial().WithMove(9, 9);

        // Act & Assert
        Action act = () => state.WithMove(9, 9);
        act.Should().Throw<InvalidOperationException>()
            .WithMessage("*already occupied*");
    }

    [Fact]
    public void WithGameOver_SetsGameOverAndWinner()
    {
        // Arrange
        var state = GameStateFactory.CreateInitial();
        var winningLine = new[]
        {
            new Position(5, 5),
            new Position(6, 5),
            new Position(7, 5),
            new Position(8, 5),
            new Position(9, 5)
        }.ToImmutableArray();

        // Act
        var newState = state.WithGameOver(Player.Red, winningLine);

        // Assert
        newState.IsGameOver.Should().BeTrue();
        newState.Winner.Should().Be(Player.Red);
        newState.WinningLine.Length.Should().Be(5);
        newState.CurrentPlayer.Should().Be(Player.None);
    }

    [Fact]
    public void UndoMove_RevertsToPreviousState()
    {
        // Arrange
        var state = GameStateFactory.CreateInitial()
            .WithMove(9, 9)
            .WithMove(10, 10);

        // Act
        var undone = state.UndoMove();

        // Assert
        state.MoveNumber.Should().Be(2);
        undone.MoveNumber.Should().Be(1);
        undone.Board.GetCell(9, 9).Player.Should().Be(Player.Red);
        undone.Board.GetCell(10, 10).Player.Should().Be(Player.None);
        // After undoing move 2 (Blue's move), CurrentPlayer stays Red (the player whose turn it was)
        undone.CurrentPlayer.Should().Be(Player.Red);
    }

    [Fact]
    public void UndoMove_RemovesStoneFromBoard()
    {
        // Arrange
        var state = GameStateFactory.CreateInitial().WithMove(9, 9);

        // Act
        var undone = state.UndoMove();

        // Assert
        undone.Board.IsEmpty().Should().BeTrue();
        undone.CurrentPlayer.Should().Be(Player.Red);
    }

    [Fact]
    public void UndoMove_WithNoMoves_ThrowsInvalidOperationException()
    {
        // Arrange
        var state = GameStateFactory.CreateInitial();

        // Act & Assert
        Action act = () => state.UndoMove();
        act.Should().Throw<InvalidOperationException>()
            .WithMessage("No moves to undo*");
    }

    [Fact]
    public void UndoMove_AfterGameOver_ThrowsInvalidOperationException()
    {
        // Arrange
        var state = GameStateFactory.CreateInitial().WithMove(9, 9).WithGameOver(Player.Red);

        // Act & Assert
        Action act = () => state.UndoMove();
        act.Should().Throw<InvalidOperationException>()
            .WithMessage("Cannot undo moves after game is over*");
    }

    [Fact]
    public void CanUndo_ReturnsTrueWhenMovesExist()
    {
        // Arrange
        var state = GameStateFactory.CreateInitial().WithMove(9, 9);

        // Act & Assert
        state.CanUndo().Should().BeTrue();
    }

    [Fact]
    public void CanUndo_ReturnsFalseWhenNoMoves()
    {
        // Arrange
        var state = GameStateFactory.CreateInitial();

        // Act & Assert
        state.CanUndo().Should().BeFalse();
    }

    [Fact]
    public void CanUndo_ReturnsFalseAfterGameOver()
    {
        // Arrange
        var state = GameStateFactory.CreateInitial().WithMove(9, 9).WithGameOver(Player.Red);

        // Act & Assert
        state.CanUndo().Should().BeFalse();
    }

    [Fact]
    public void Equality_StatesWithSameBoardHaveEqualProperties()
    {
        // Arrange
        var state1 = GameStateFactory.CreateInitial().WithMove(9, 9);
        var state2 = GameStateFactory.CreateInitial().WithMove(9, 9);

        // Act & Assert
        // Note: Board is a class, so record equality doesn't work as expected
        // We verify the properties are equal instead
        state1.MoveNumber.Should().Be(state2.MoveNumber);
        state1.CurrentPlayer.Should().Be(state2.CurrentPlayer);
        state1.IsGameOver.Should().Be(state2.IsGameOver);
        state1.Winner.Should().Be(state2.Winner);
        state1.MoveHistory.Should().BeEquivalentTo(state2.MoveHistory);
    }

    [Fact]
    public void Equality_StatesWithDifferentMovesHaveDifferentMoveHistory()
    {
        // Arrange
        var state1 = GameStateFactory.CreateInitial().WithMove(9, 9);
        var state2 = GameStateFactory.CreateInitial().WithMove(10, 10);

        // Act & Assert
        state1.MoveHistory.Should().NotBeEquivalentTo(state2.MoveHistory);
        state1.MoveHistory[0].Should().NotBe(state2.MoveHistory[0]);
    }
}
