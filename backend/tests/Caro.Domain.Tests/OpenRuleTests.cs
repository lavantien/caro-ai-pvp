using Caro.Domain;
using Xunit;

namespace Caro.Domain.Tests;

public class OpenRuleTests
{
    [Fact]
    public void OpenRuleFirstMove()
    {
        Board b = Board.NewBoard();
        Assert.True(OpenRule.IsValidSecondMove(b, 5, 5), "first move is always valid");
    }

    [Fact]
    public void OpenRuleSecondRedMove()
    {
        Board b = Board.NewBoard().PlaceStone(8, 8, Player.Red);
        Assert.False(OpenRule.IsValidSecondMove(b, 9, 9), "inside 5x5 zone");
        Assert.False(OpenRule.IsValidSecondMove(b, 10, 9), "inside 5x5 zone");
        Assert.True(OpenRule.IsValidSecondMove(b, 11, 8), "outside 5x5 zone");
        Assert.True(OpenRule.IsValidSecondMove(b, 8, 11), "outside 5x5 zone");
        Assert.True(OpenRule.IsValidSecondMove(b, 0, 0), "far away, valid");
    }

    [Fact]
    public void OpenRuleAfterBlueMove()
    {
        Board b = Board.NewBoard()
            .PlaceStone(8, 8, Player.Red)
            .PlaceStone(0, 0, Player.Blue);
        Assert.False(OpenRule.IsValidSecondMove(b, 9, 9), "inside 5x5 zone even after blue has played");
        Assert.False(OpenRule.IsValidSecondMove(b, 10, 9), "inside 5x5 zone");
        Assert.True(OpenRule.IsValidSecondMove(b, 11, 8), "outside 5x5 zone after blue move is valid");
    }
}
