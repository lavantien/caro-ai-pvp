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

    // Combination-bonus cascade pins (ENGINE_FEATURES 5.3): the highest
    // matching category wins, in strict descending order flex4 15k,
    // double block4 14k, block4+flex3 13k, double flex3 12k.

    private static Board BlockedFourRow8() =>
        Board.NewBoard()
            .PlaceStone(2, 8, Player.Blue)
            .PlaceStone(3, 8, Player.Red).PlaceStone(4, 8, Player.Red)
            .PlaceStone(5, 8, Player.Red).PlaceStone(6, 8, Player.Red);

    private static Board TwoOpenThrees() =>
        Board.NewBoard()
            .PlaceStone(3, 3, Player.Red).PlaceStone(4, 3, Player.Red)
            .PlaceStone(5, 3, Player.Red)
            .PlaceStone(12, 4, Player.Red).PlaceStone(12, 5, Player.Red)
            .PlaceStone(12, 6, Player.Red);

    [Fact]
    public void BlockedFourWithDoubleFlex3UsesB4F3Bonus()
    {
        Board b = BlockedFourRow8()
            .PlaceStone(3, 3, Player.Red).PlaceStone(4, 3, Player.Red)
            .PlaceStone(5, 3, Player.Red)
            .PlaceStone(12, 4, Player.Red).PlaceStone(12, 5, Player.Red)
            .PlaceStone(12, 6, Player.Red)
            .PlaceStone(14, 2, Player.Blue);
        SearchBoard sb = new(b);

        PlayerPattern4 red = Pattern4Classifier.ClassifyBoard(sb, Player.Red);
        Assert.Equal(0, red.Exactly5Count);
        Assert.Equal(0, red.Flex4Count);
        Assert.Equal(1, red.Block4Count);
        Assert.Equal(2, red.Flex3Count);

        // 13,000 B4F3 bonus + 1x5,000 block4 + 2x1,000 flex3 = 20,000 red,
        // 20 blue flex1, red center 196, blue center 28.
        Assert.Equal(20_148, Evaluation.Evaluate(sb, Player.Red));
    }

    [Fact]
    public void DoubleFlex3WithoutBlock4KeepsDoubleF3Bonus()
    {
        Board b = TwoOpenThrees().PlaceStone(14, 2, Player.Blue).PlaceStone(0, 0, Player.Blue);
        SearchBoard sb = new(b);

        PlayerPattern4 red = Pattern4Classifier.ClassifyBoard(sb, Player.Red);
        Assert.Equal(0, red.Block4Count);
        Assert.Equal(2, red.Flex3Count);

        // 12,000 double-F3 bonus + 2x1,000 flex3 = 14,000 red, 20 blue
        // flex1, red center 96, blue center 8.
        Assert.Equal(14_068, Evaluation.Evaluate(sb, Player.Red));
    }

    [Fact]
    public void DoubleBlock4StillOutranksCombinationBonuses()
    {
        Board b = BlockedFourRow8()
            .PlaceStone(1, 1, Player.Blue)
            .PlaceStone(1, 2, Player.Red).PlaceStone(1, 3, Player.Red)
            .PlaceStone(1, 4, Player.Red).PlaceStone(1, 5, Player.Red);
        SearchBoard sb = new(b);

        PlayerPattern4 red = Pattern4Classifier.ClassifyBoard(sb, Player.Red);
        Assert.Equal(0, red.Exactly5Count);
        Assert.Equal(0, red.Flex4Count);
        Assert.Equal(2, red.Block4Count);
        Assert.Equal(0, red.Flex3Count);

        // 14,000 double-B4 bonus + 2x5,000 block4 = 24,000 red, 20 blue
        // flex1, red center 136, blue center 24.
        Assert.Equal(24_092, Evaluation.Evaluate(sb, Player.Red));
    }
}
