using Caro.Core.Domain.Entities;
using Caro.Core.Domain.ValueObjects;
using FluentAssertions;

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
    public void CreateInitial_WithCustomTimeControl()
    {
        // Arrange
        var initialTime = TimeSpan.FromMinutes(5);
        var increment = TimeSpan.FromSeconds(3);

        // Act
        var state = GameState.CreateInitial(initialTime, increment);

        // Assert
        state.RedTimeRemaining.Should().Be(initialTime);
        state.BlueTimeRemaining.Should().Be(initialTime);
    }

    [Fact]
    public void MakeMove_ReturnsNewState_WithoutMutatingOriginal()
    {
        // Arrange
        var originalState = GameStateFactory.CreateInitial();

        // Act
        var newState = originalState.MakeMove(9, 9);

        // Assert
        originalState.MoveNumber.Should().Be(0);
        originalState.Board.IsEmpty().Should().BeTrue();

        newState.MoveNumber.Should().Be(1);
        newState.Board.GetCell(9, 9).Should().Be(Player.Red);
        newState.CurrentPlayer.Should().Be(Player.Blue);
    }

    [Fact]
    public void MakeMove_SwitchesPlayer()
    {
        // Arrange
        var state = GameStateFactory.CreateInitial();

        // Act
        var newState = state.MakeMove(9, 9);

        // Assert
        state.CurrentPlayer.Should().Be(Player.Red);
        newState.CurrentPlayer.Should().Be(Player.Blue);
    }

    [Fact]
    public void MakeMove_IncrementsTimeForCurrentPlayer()
    {
        // Arrange
        var state = GameState.CreateInitial(TimeSpan.FromMinutes(7), TimeSpan.FromSeconds(5));

        // Act
        var newState = state.MakeMove(9, 9);

        // Assert
        newState.RedTimeRemaining.Should().Be(TimeSpan.FromMinutes(7) + TimeSpan.FromSeconds(5));
        newState.BlueTimeRemaining.Should().Be(TimeSpan.FromMinutes(7)); // Unchanged
    }

    [Fact]
    public void MakeMove_AddsToMoveHistory()
    {
        // Arrange
        var state = GameStateFactory.CreateInitial();

        // Act
        var newState = state.MakeMove(9, 9);

        // Assert
        state.MoveHistory.Length.Should().Be(0);
        newState.MoveHistory.Length.Should().Be(1);
        newState.MoveHistory.Span[0].Move.Should().Be(new Move(9, 9, Player.Red));
        newState.MoveHistory.Span[0].MoveNumber.Should().Be(1);
    }

    [Fact]
    public void MakeMove_AfterGameOver_ThrowsGameOverException()
    {
        // Arrange
        var state = GameStateFactory.CreateInitial().EndGame(Player.Red);

        // Act & Assert
        Action act = () => state.MakeMove(9, 9);
        act.Should().Throw<GameOverException>()
            .WithMessage("*Cannot make moves after game is over*");
    }

    [Fact]
    public void MakeMove_AtOccupiedCell_ThrowsInvalidMoveException()
    {
        // Arrange
        var state = GameStateFactory.CreateInitial().MakeMove(9, 9);

        // Act & Assert
        Action act = () => state.MakeMove(9, 9);
        act.Should().Throw<InvalidMoveException>()
            .WithMessage("*already occupied*");
    }

    [Fact]
    public void EndGame_SetsGameOverAndWinner()
    {
        // Arrange
        var state = GameStateFactory.CreateInitial();
        var winningLine = new Position[]
        {
            new(5, 5), new(6, 5), new(7, 5), new(8, 5), new(9, 5)
        };

        // Act
        var newState = state.EndGame(Player.Red, winningLine);

        // Assert
        newState.IsGameOver.Should().BeTrue();
        newState.Winner.Should().Be(Player.Red);
        newState.WinningLine.Length.Should().Be(5);
    }

    [Fact]
    public void UndoMove_RevertsToPreviousState()
    {
        // Arrange
        var state = GameStateFactory.CreateInitial()
            .MakeMove(9, 9)
            .MakeMove(10, 10);

        // Act
        var undone = state.UndoMove();

        // Assert
        state.MoveNumber.Should().Be(2);
        undone.MoveNumber.Should().Be(1);
        undone.Board.GetCell(9, 9).Should().Be(Player.Red);
        undone.Board.GetCell(10, 10).Should().Be(Player.None);
        undone.CurrentPlayer.Should().Be(Player.Blue);
    }

    [Fact]
    public void UndoMove_RemovesStoneFromBoard()
    {
        // Arrange
        var state = GameStateFactory.CreateInitial().MakeMove(9, 9);

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
            .WithMessage("*No moves to undo*");
    }

    [Fact]
    public void UndoMove_AfterGameOver_ThrowsGameOverException()
    {
        // Arrange
        var state = GameStateFactory.CreateInitial().WithEndGame();

        // Act & Assert
        Action act = () => state.UndoMove();
        act.Should().Throw<GameOverException>()
            .WithMessage("*Cannot undo moves after game is over*");
    }

    [Fact]
    public void CanUndo_ReturnsTrueWhenMovesExist()
    {
        // Arrange
        var state = GameStateFactory.CreateInitial().MakeMove(9, 9);

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
        var state = GameStateFactory.CreateInitial().MakeMove(9, 9).EndGame();

        // Act & Assert
        state.CanUndo().Should().BeFalse();
    }

    [Fact]
    public void GetCurrentPlayerTimeRemaining_ReturnsCurrentPlayerTime()
    {
        // Arrange
        var state = GameState.CreateInitial(TimeSpan.FromMinutes(5), TimeSpan.FromSeconds(0));

        // Act
        var redTime = state.GetCurrentPlayerTimeRemaining();
        state = state.MakeMove(9, 9);
        var blueTime = state.GetCurrentPlayerTimeRemaining();

        // Assert
        redTime.Should().Be(TimeSpan.FromMinutes(5));
        blueTime.Should().Be(TimeSpan.FromMinutes(5));
    }

    [Fact]
    public void WithTimeRemaining_UpdatesCurrentPlayerTime()
    {
        // Arrange
        var state = GameStateFactory.CreateInitial();

        // Act
        var newState = state.WithTimeRemaining(TimeSpan.FromMinutes(3));

        // Assert
        state.RedTimeRemaining.Should().Be(TimeSpan.FromMinutes(7));
        newState.RedTimeRemaining.Should().Be(TimeSpan.FromMinutes(3));
        newState.BlueTimeRemaining.Should().Be(TimeSpan.FromMinutes(7));
    }

    [Fact]
    public void Equality_StatesWithSameBoardAreEqual()
    {
        // Arrange
        var state1 = GameStateFactory.CreateInitial().MakeMove(9, 9);
        var state2 = GameStateFactory.CreateInitial().MakeMove(9, 9);

        // Act & Assert
        state1.Should().Be(state2);
    }

    [Fact]
    public void Equality_StatesWithDifferentBoardsAreNotEqual()
    {
        // Arrange
        var state1 = GameStateFactory.CreateInitial().MakeMove(9, 9);
        var state2 = GameStateFactory.CreateInitial().MakeMove(10, 10);

        // Act & Assert
        state1.Should().NotBe(state2);
    }
}
