using Caro.Domain;
using Caro.Engine;
using Xunit;

namespace Caro.Engine.Tests;

public class EvaluationTests
{
    [Fact]
    public void EvaluateEmptyBoard()
    {
        SearchBoard sb = new(Board.NewBoard());
        int score = Evaluation.Evaluate(sb, Player.Red);
        Assert.Equal(0, score);
    }

    [Fact]
    public void EvaluateFavorsFourInRow()
    {
        Board b = Board.NewBoard();
        for (int x = 3; x < 7; x++)
        {
            b = b.PlaceStone(x, 5, Player.Red);
        }
        SearchBoard sb = new(b);
        int scoreRed = Evaluation.Evaluate(sb, Player.Red);
        Assert.True(scoreRed > 0, "red with 4 in a row should be positive for red");
    }

    [Fact]
    public void EvaluateZeroSumProperty()
    {
        Board[] boards =
        [
            Board.NewBoard(),
            Board.NewBoard().PlaceStone(8, 8, Player.Red).PlaceStone(7, 7, Player.Blue),
            Board.NewBoard()
                .PlaceStone(5, 5, Player.Red).PlaceStone(6, 5, Player.Red)
                .PlaceStone(7, 5, Player.Red)
                .PlaceStone(0, 0, Player.Blue).PlaceStone(1, 1, Player.Blue),
            Board.NewBoard()
                .PlaceStone(3, 3, Player.Red).PlaceStone(4, 3, Player.Red)
                .PlaceStone(5, 3, Player.Red).PlaceStone(6, 3, Player.Red)
                .PlaceStone(10, 10, Player.Blue),
        ];
        for (int i = 0; i < boards.Length; i++)
        {
            SearchBoard sb = new(boards[i]);
            int scoreRed = Evaluation.Evaluate(sb, Player.Red);
            int scoreBlue = Evaluation.Evaluate(sb, Player.Blue);
            Assert.Equal(-scoreBlue, scoreRed);
        }
    }

    [Fact]
    public void EvaluateOpponentThreatsPenalized()
    {
        Board b = Board.NewBoard();
        for (int x = 3; x < 6; x++)
        {
            b = b.PlaceStone(x, 5, Player.Red);
        }
        SearchBoard sb = new(b);
        int scoreRed = Evaluation.Evaluate(sb, Player.Red);
        int scoreBlue = Evaluation.Evaluate(sb, Player.Blue);
        Assert.True(scoreRed > 0, "player with 3-in-a-row should have positive score");
        Assert.Equal(-scoreRed, scoreBlue);
    }
}
