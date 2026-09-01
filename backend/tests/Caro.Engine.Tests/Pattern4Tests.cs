using Caro.Domain;
using Caro.Engine;
using Xunit;

namespace Caro.Engine.Tests;

public class Pattern4Tests
{
    [Fact]
    public void ClassifyDirectionFlex3()
    {
        Board b = Board.NewBoard()
            .PlaceStone(5, 8, Player.Red)
            .PlaceStone(6, 8, Player.Red);
        SearchBoard sb = new(b);
        Pattern4 result = Pattern4Classifier.ClassifyDirection(sb, 7, 8, 1, 0, Player.Red);
        Assert.Equal(Pattern4.Flex3, result);
    }

    [Fact]
    public void ClassifyDirectionBlock3()
    {
        Board b = Board.NewBoard()
            .PlaceStone(5, 8, Player.Red)
            .PlaceStone(6, 8, Player.Red)
            .PlaceStone(4, 8, Player.Blue);
        SearchBoard sb = new(b);
        Pattern4 result = Pattern4Classifier.ClassifyDirection(sb, 7, 8, 1, 0, Player.Red);
        Assert.Equal(Pattern4.Block3, result);
    }

    [Fact]
    public void ClassifyDirectionFlex4()
    {
        Board b = Board.NewBoard()
            .PlaceStone(5, 8, Player.Red)
            .PlaceStone(6, 8, Player.Red)
            .PlaceStone(7, 8, Player.Red);
        SearchBoard sb = new(b);
        Pattern4 result = Pattern4Classifier.ClassifyDirection(sb, 8, 8, 1, 0, Player.Red);
        Assert.Equal(Pattern4.Flex4, result);
    }

    [Fact]
    public void ClassifyDirectionBlock4()
    {
        Board b = Board.NewBoard()
            .PlaceStone(5, 8, Player.Red)
            .PlaceStone(6, 8, Player.Red)
            .PlaceStone(7, 8, Player.Red)
            .PlaceStone(9, 8, Player.Blue);
        SearchBoard sb = new(b);
        Pattern4 result = Pattern4Classifier.ClassifyDirection(sb, 8, 8, 1, 0, Player.Red);
        Assert.Equal(Pattern4.Block4, result);
    }

    [Fact]
    public void ClassifyDirectionExactly5()
    {
        Board b = Board.NewBoard()
            .PlaceStone(5, 8, Player.Red)
            .PlaceStone(6, 8, Player.Red)
            .PlaceStone(7, 8, Player.Red)
            .PlaceStone(8, 8, Player.Red);
        SearchBoard sb = new(b);
        Pattern4 result = Pattern4Classifier.ClassifyDirection(sb, 9, 8, 1, 0, Player.Red);
        Assert.Equal(Pattern4.Exactly5, result);
    }

    [Fact]
    public void ClassifyDirectionBlocked5()
    {
        Board b = Board.NewBoard()
            .PlaceStone(5, 8, Player.Red)
            .PlaceStone(6, 8, Player.Red)
            .PlaceStone(7, 8, Player.Red)
            .PlaceStone(8, 8, Player.Red)
            .PlaceStone(4, 8, Player.Blue)
            .PlaceStone(10, 8, Player.Blue);
        SearchBoard sb = new(b);
        Pattern4 result = Pattern4Classifier.ClassifyDirection(sb, 9, 8, 1, 0, Player.Red);
        Assert.Equal(Pattern4.None, result);
    }

    [Fact]
    public void ClassifyDirectionOverline()
    {
        Board b = Board.NewBoard()
            .PlaceStone(5, 8, Player.Red)
            .PlaceStone(6, 8, Player.Red)
            .PlaceStone(7, 8, Player.Red)
            .PlaceStone(8, 8, Player.Red)
            .PlaceStone(9, 8, Player.Red);
        SearchBoard sb = new(b);
        Pattern4 result = Pattern4Classifier.ClassifyDirection(sb, 10, 8, 1, 0, Player.Red);
        Assert.Equal(Pattern4.Overline, result);
    }

    [Fact]
    public void ClassifyDirectionFlex2()
    {
        Board b = Board.NewBoard()
            .PlaceStone(5, 8, Player.Red);
        SearchBoard sb = new(b);
        Pattern4 result = Pattern4Classifier.ClassifyDirection(sb, 6, 8, 1, 0, Player.Red);
        Assert.Equal(Pattern4.Flex2, result);
    }

    [Fact]
    public void ClassifyDirectionBlock2()
    {
        Board b = Board.NewBoard()
            .PlaceStone(5, 8, Player.Red)
            .PlaceStone(4, 8, Player.Blue);
        SearchBoard sb = new(b);
        Pattern4 result = Pattern4Classifier.ClassifyDirection(sb, 6, 8, 1, 0, Player.Red);
        Assert.Equal(Pattern4.Block2, result);
    }

    [Fact]
    public void ClassifyDirectionFlex1()
    {
        SearchBoard sb = new(Board.NewBoard());
        Pattern4 result = Pattern4Classifier.ClassifyDirection(sb, 8, 8, 1, 0, Player.Red);
        Assert.Equal(Pattern4.Flex1, result);
    }

    [Fact]
    public void ClassifyDirectionNone()
    {
        Board b = Board.NewBoard()
            .PlaceStone(6, 8, Player.Red)
            .PlaceStone(7, 8, Player.Red)
            .PlaceStone(5, 8, Player.Blue)
            .PlaceStone(8, 8, Player.Blue);
        SearchBoard sb = new(b);
        Pattern4 result = Pattern4Classifier.ClassifyDirection(sb, 6, 8, 1, 0, Player.Red);
        Assert.Equal(Pattern4.None, result);
    }

    [Fact]
    public void HasDoubleFlex3()
    {
        // Move at (6,5): horizontal (5,5)+(7,5)=Flex3, vertical (6,4)+(6,6)=Flex3
        Board b = Board.NewBoard()
            .PlaceStone(5, 5, Player.Red)
            .PlaceStone(7, 5, Player.Red)
            .PlaceStone(6, 4, Player.Red)
            .PlaceStone(6, 6, Player.Red)
            .PlaceStone(0, 0, Player.Blue);
        SearchBoard sb = new(b);
        Assert.True(Pattern4Classifier.HasDoubleFlex3(sb, 6, 5, Player.Red));
    }

    [Fact]
    public void HasDoubleFlex3False()
    {
        SearchBoard sb = new(Board.NewBoard());
        Assert.False(Pattern4Classifier.HasDoubleFlex3(sb, 8, 8, Player.Red));
    }

    [Fact]
    public void HasFlex4PlusFlex3()
    {
        // Move at (6,5): horizontal (5,5)+(7,5)+(8,5)=Flex4, vertical (6,4)+(6,6)=Flex3
        Board b = Board.NewBoard()
            .PlaceStone(5, 5, Player.Red)
            .PlaceStone(7, 5, Player.Red)
            .PlaceStone(8, 5, Player.Red)
            .PlaceStone(6, 4, Player.Red)
            .PlaceStone(6, 6, Player.Red)
            .PlaceStone(0, 0, Player.Blue);
        SearchBoard sb = new(b);
        Assert.True(Pattern4Classifier.HasFlex4PlusFlex3(sb, 6, 5, Player.Red));
    }

    [Fact]
    public void HasFlex4PlusFlex3False()
    {
        SearchBoard sb = new(Board.NewBoard());
        Assert.False(Pattern4Classifier.HasFlex4PlusFlex3(sb, 8, 8, Player.Red));
    }
}
