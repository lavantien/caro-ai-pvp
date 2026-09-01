using Caro.Domain;
using Caro.Engine;
using Xunit;

namespace Caro.Engine.Tests;

public class PatternWindowTests
{
    // splitFourBoard: red has 5,6 gap 8 on row 5. Playing (7,5) fills the gap
    // into a five; playing (9,5) or (4,5) extends into a split four with the
    // gap as its only completion.
    private static Board SplitThreeBoard() =>
        Board.NewBoard()
            .PlaceStone(5, 5, Player.Red)
            .PlaceStone(6, 5, Player.Red)
            .PlaceStone(8, 5, Player.Red)
            .PlaceStone(0, 0, Player.Blue)
            .PlaceStone(0, 1, Player.Blue);

    [Fact]
    public void SplitFourPlacementIsFour()
    {
        SearchBoard sb = new(SplitThreeBoard());

        Assert.True(PlacementAnalysis.CreatesFourType(sb, 9, 5, Player.Red),
            "playing (9,5) leaves a split four whose gap fill wins: a four");
        Assert.True(PlacementAnalysis.CreatesFourType(sb, 4, 5, Player.Red),
            "playing (4,5) leaves a split four on the other side: a four");
    }

    [Fact]
    public void BrokenThreePlacementIsFlex3()
    {
        // Only 5,6 on the row: playing (8,5) makes a broken three that can
        // become a flex four by filling (7,5).
        Board b = Board.NewBoard()
            .PlaceStone(5, 5, Player.Red)
            .PlaceStone(6, 5, Player.Red)
            .PlaceStone(0, 0, Player.Blue);
        SearchBoard sb = new(b);

        Assert.True(PlacementAnalysis.CreatesOpenThree(sb, 8, 5, Player.Red),
            "a broken three (can reach a two-completion four) must classify as flex three");
    }

    [Fact]
    public void EvalValuesSplitFour()
    {
        // Red already holds a split four on the board (5,6 gap 8,9 on row 5).
        Board b = Board.NewBoard()
            .PlaceStone(5, 5, Player.Red)
            .PlaceStone(6, 5, Player.Red)
            .PlaceStone(8, 5, Player.Red)
            .PlaceStone(9, 5, Player.Red)
            .PlaceStone(0, 0, Player.Blue);
        SearchBoard sb = new(b);

        PlayerPattern4 pp = Pattern4Classifier.ClassifyBoard(sb, Player.Red);
        Assert.True(pp.Block4Count + pp.Flex4Count >= 1,
            "a split four on the board must count as a four-class pattern");

        int score = Evaluation.Evaluate(sb, Player.Red);
        Assert.True(score >= 4000, "static eval must value a split four like a simple four");
    }

    [Fact]
    public void PlacementPatternHelpersAgreeWithClasses()
    {
        Board b = Board.NewBoard();
        // Straight open four placement: 5,6,7 on row 5, playing 8 with both
        // ends open gives two completions.
        for (int x = 5; x <= 7; x++)
        {
            b = b.PlaceStone(x, 5, Player.Red);
        }
        b = b.PlaceStone(0, 0, Player.Blue);
        SearchBoard sb = new(b);

        Assert.True(PlacementAnalysis.CreatesOpenFour(sb, 8, 5, Player.Red),
            "straight open four must still classify as flex four");
        Assert.False(PlacementAnalysis.CreatesOpenFour(sb, 8, 5, Player.Blue));
    }
}
