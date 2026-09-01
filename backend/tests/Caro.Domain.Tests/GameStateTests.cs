using Caro.Domain;
using Xunit;

namespace Caro.Domain.Tests;

public class GameStateTests
{
    [Fact]
    public void NewGameState()
    {
        GameState g = GameState.NewGameState(GameMode.PvP, "7+5", 420_000, 5);
        Assert.Equal(Player.Red, g.CurrentPlayer);
        Assert.Equal(0, g.MoveNumber);
        Assert.False(g.IsGameOver);
        Assert.Equal(Player.None, g.Winner);
        Assert.Equal(GameMode.PvP, g.GameMode);
        Assert.True(g.Board.IsEmpty());
    }

    [Fact]
    public void GameStateWithMove()
    {
        GameState g = GameState.NewGameState(GameMode.PvP, "7+5", 420_000, 5);
        GameState g2 = g.WithMove(8, 8);

        Assert.Equal(Player.Blue, g2.CurrentPlayer);
        Assert.Equal(1, g2.MoveNumber);
        Assert.Equal(Player.Red, g2.Board.GetPlayerAt(8, 8));

        Assert.Equal(0, g.MoveNumber);
        Assert.True(g.Board.IsEmpty());
    }

    [Fact]
    public void GameStateWithMoveGameOver()
    {
        GameState g = GameState.NewGameState(GameMode.PvP, "7+5", 420_000, 5);
        GameState g2 = g.WithGameOver(Player.Red, line: null);
        Assert.Throws<GameOverException>(() => g2.WithMove(5, 5));
    }

    [Fact]
    public void GameStateUndoMove()
    {
        GameState g = GameState.NewGameState(GameMode.PvP, "7+5", 420_000, 5);
        GameState g2 = g.WithMove(8, 8);
        GameState g3 = g2.UndoMove();

        Assert.Equal(0, g3.MoveNumber);
        Assert.Equal(Player.Red, g3.CurrentPlayer);
        Assert.True(g3.Board.IsEmpty());
    }

    [Fact]
    public void GameStateUndoNoMoves()
    {
        GameState g = GameState.NewGameState(GameMode.PvP, "7+5", 420_000, 5);
        Assert.Throws<NoMovesException>(() => g.UndoMove());
    }

    [Fact]
    public void GameStateWithGameOver()
    {
        GameState g = GameState.NewGameState(GameMode.PvP, "7+5", 420_000, 5);
        Position[] line =
        [
            new(3, 5), new(4, 5), new(5, 5), new(6, 5), new(7, 5),
        ];
        GameState g2 = g.WithGameOver(Player.Red, line);

        Assert.True(g2.IsGameOver);
        Assert.Equal(Player.Red, g2.Winner);
        Assert.Equal(5, g2.WinningLine!.Length);
        Assert.Equal(Player.None, g2.CurrentPlayer);
    }

    [Fact]
    public void GameStateCanUndo()
    {
        GameState g = GameState.NewGameState(GameMode.PvP, "7+5", 420_000, 5);
        Assert.False(g.CanUndo());

        GameState g2 = g.WithMove(8, 8);
        Assert.True(g2.CanUndo());

        GameState g3 = g2.WithGameOver(Player.Red, line: null);
        Assert.False(g3.CanUndo());
    }

    [Fact]
    public void GameStateOpenRuleViolation()
    {
        GameState g = GameState.NewGameState(GameMode.PvP, "7+5", 420_000, 5);
        GameState g2 = g.WithMove(8, 8);
        GameState g3 = g2.WithMove(0, 0);
        // Red's second move inside 5x5 zone (Chebyshev distance 2)
        Assert.Throws<OpenRuleException>(() => g3.WithMove(10, 9));
    }

    [Fact]
    public void GameStateOpenRuleValid()
    {
        GameState g = GameState.NewGameState(GameMode.PvP, "7+5", 420_000, 5);
        GameState g2 = g.WithMove(8, 8);
        GameState g3 = g2.WithMove(0, 0);
        // Red's second move outside 5x5 zone (Chebyshev distance 3)
        GameState g4 = g3.WithMove(11, 8);
        Assert.Equal(Player.Blue, g4.CurrentPlayer);
    }

    [Fact]
    public void GameStateOpenRuleNotAppliedAfterMoreMoves()
    {
        GameState g = GameState.NewGameState(GameMode.PvP, "7+5", 420_000, 5);
        GameState g2 = g.WithMove(8, 8);
        GameState g3 = g2.WithMove(0, 0);
        GameState g4 = g3.WithMove(11, 8);
        // Blue's second move close to red's second move is fine
        GameState g5 = g4.WithMove(11, 9);
        Assert.Equal(4, g5.MoveNumber);
    }
}
