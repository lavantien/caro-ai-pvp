using Xunit;
using FluentAssertions;
using Caro.Core.Application.Extensions;
using Caro.Core.Domain.Entities;

namespace Caro.Core.Application.Tests.Extensions;

public sealed class GameStateExtensionsTests
{
    // --- CreateInitial ---

    [Fact]
    public void CreateInitial_ReturnsInitialState()
    {
        var state = GameStateExtensions.CreateInitial();

        state.Should().NotBeNull();
        state.CurrentPlayer.Should().Be(Player.Red);
        state.MoveNumber.Should().Be(0);
        state.IsGameOver.Should().BeFalse();
        state.Winner.Should().Be(Player.None);
    }

    [Fact]
    public void CreateInitial_WithTimeControl_ReturnsInitialState()
    {
        var state = GameStateExtensions.CreateInitial(TimeSpan.FromMinutes(3), TimeSpan.FromSeconds(2));

        state.Should().NotBeNull();
        state.CurrentPlayer.Should().Be(Player.Red);
        state.MoveNumber.Should().Be(0);
        state.IsGameOver.Should().BeFalse();
    }

    // --- MakeMove ---

    [Fact]
    public void MakeMove_PlacesStoneAndSwitchesPlayer()
    {
        var state = GameStateExtensions.CreateInitial();

        var next = state.MakeMove(7, 7);

        next.CurrentPlayer.Should().Be(Player.Blue);
        next.MoveNumber.Should().Be(1);
        next.Board.GetCell(7, 7).Player.Should().Be(Player.Red);
    }

    [Fact]
    public void MakeMove_SecondMoveSwitchesBack()
    {
        var state = GameStateExtensions.CreateInitial()
            .MakeMove(7, 7)
            .MakeMove(8, 8);

        state.CurrentPlayer.Should().Be(Player.Red);
        state.MoveNumber.Should().Be(2);
        state.Board.GetCell(8, 8).Player.Should().Be(Player.Blue);
    }

    // --- WithTimeRemaining ---

    [Fact]
    public void WithTimeRemaining_ReturnsSameState()
    {
        var state = GameStateExtensions.CreateInitial();
        var elapsed = TimeSpan.FromSeconds(5);

        var result = state.WithTimeRemaining(elapsed);

        result.Should().BeSameAs(state);
    }

    // --- WithEndGame (winner + winning line) ---

    [Fact]
    public void WithEndGame_WithWinnerAndWinningLine_SetsGameOver()
    {
        var state = GameStateExtensions.CreateInitial();
        var winningLine = new List<Position>
        {
            new(5, 7), new(6, 7), new(7, 7), new(8, 7), new(9, 7)
        };

        var ended = state.WithEndGame(Player.Red, winningLine);

        ended.IsGameOver.Should().BeTrue();
        ended.Winner.Should().Be(Player.Red);
        ended.WinningLine.Should().HaveCount(5);
    }

    [Fact]
    public void WithEndGame_WithWinnerOnly_SetsGameOver()
    {
        var state = GameStateExtensions.CreateInitial();

        var ended = state.WithEndGame(Player.Blue);

        ended.IsGameOver.Should().BeTrue();
        ended.Winner.Should().Be(Player.Blue);
        ended.WinningLine.Should().BeEmpty();
    }

    [Fact]
    public void WithEndGame_WithNonePlayer_SetsDraw()
    {
        var state = GameStateExtensions.CreateInitial();

        var ended = state.WithEndGame(Player.None);

        ended.IsGameOver.Should().BeTrue();
        ended.Winner.Should().Be(Player.None);
    }

    [Fact]
    public void WithEndGame_WithReadOnlyMemory_SetsGameOver()
    {
        var state = GameStateExtensions.CreateInitial();
        var line = new Position[]
        {
            new(5, 7), new(6, 7), new(7, 7), new(8, 7), new(9, 7)
        };

        var ended = state.WithEndGame(Player.Red, new ReadOnlyMemory<Position>(line));

        ended.IsGameOver.Should().BeTrue();
        ended.Winner.Should().Be(Player.Red);
        ended.WinningLine.Should().HaveCount(5);
    }

    [Fact]
    public void WithEndGame_EmptyReadOnlyMemory_NoWinningLine()
    {
        var state = GameStateExtensions.CreateInitial();

        var ended = state.WithEndGame(Player.Red, ReadOnlyMemory<Position>.Empty);

        ended.IsGameOver.Should().BeTrue();
        ended.Winner.Should().Be(Player.Red);
        ended.WinningLine.Should().BeEmpty();
    }

    // --- UndoMoveAndReturn ---

    [Fact]
    public void UndoMoveAndReturn_RevertsLastMove()
    {
        var state = GameStateExtensions.CreateInitial()
            .MakeMove(7, 7);

        var undone = state.UndoMoveAndReturn();

        undone.MoveNumber.Should().Be(0);
        undone.CurrentPlayer.Should().Be(Player.Red);
        undone.Board.GetCell(7, 7).IsEmpty.Should().BeTrue();
    }

    [Fact]
    public void UndoMoveAndReturn_MultipleMoves_UndoesOneAtATime()
    {
        var state = GameStateExtensions.CreateInitial()
            .MakeMove(7, 7)
            .MakeMove(8, 8);

        var undone = state.UndoMoveAndReturn();

        undone.MoveNumber.Should().Be(1);
        // After 2 moves (Red, Blue), CurrentPlayer is Red. Undo keeps same player since MoveNumber-1 != 0.
        undone.CurrentPlayer.Should().Be(Player.Red);
        undone.Board.GetCell(7, 7).Player.Should().Be(Player.Red);
        undone.Board.GetCell(8, 8).IsEmpty.Should().BeTrue();
    }
}
