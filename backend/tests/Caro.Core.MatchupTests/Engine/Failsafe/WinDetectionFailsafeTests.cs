using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Xunit;

namespace Caro.Core.MatchupTests.Engine.Failsafe;

[Trait("Category", "Failsafe")]
public class WinDetectionFailsafeTests
{
    private static Board PlaceStones(params (int x, int y, Player player)[] moves)
    {
        var board = new Board();
        foreach (var (x, y, player) in moves)
        {
            board = board.PlaceStone(x, y, player);
        }
        return board;
    }

    [Fact]
    public void Exactly5Horizontal_Wins()
    {
        var board = PlaceStones(
            (0, 0, Player.Red), (1, 0, Player.Red), (2, 0, Player.Red),
            (3, 0, Player.Red), (4, 0, Player.Red));

        var detector = new WinDetector();
        var result = detector.CheckWin(board);

        Assert.True(result.HasWinner);
        Assert.Equal(Player.Red, result.Winner);
        Assert.Equal(5, result.WinningLine.Count);
    }

    [Fact]
    public void Exactly5Vertical_Wins()
    {
        var board = PlaceStones(
            (0, 0, Player.Blue), (0, 1, Player.Blue), (0, 2, Player.Blue),
            (0, 3, Player.Blue), (0, 4, Player.Blue));

        var detector = new WinDetector();
        var result = detector.CheckWin(board);

        Assert.True(result.HasWinner);
        Assert.Equal(Player.Blue, result.Winner);
    }

    [Fact]
    public void Exactly5DiagonalDownRight_Wins()
    {
        var board = PlaceStones(
            (0, 0, Player.Red), (1, 1, Player.Red), (2, 2, Player.Red),
            (3, 3, Player.Red), (4, 4, Player.Red));

        var detector = new WinDetector();
        var result = detector.CheckWin(board);

        Assert.True(result.HasWinner);
        Assert.Equal(Player.Red, result.Winner);
    }

    [Fact]
    public void Exactly5DiagonalDownLeft_Wins()
    {
        var board = PlaceStones(
            (8, 4, Player.Red), (7, 5, Player.Red), (6, 6, Player.Red),
            (5, 7, Player.Red), (4, 8, Player.Red));

        var detector = new WinDetector();
        var result = detector.CheckWin(board);

        Assert.True(result.HasWinner);
        Assert.Equal(Player.Red, result.Winner);
    }

    [Fact]
    public void Overline6InRow_NoWin()
    {
        // 6 consecutive Red stones - overline, should NOT be a win in Caro
        var board = PlaceStones(
            (0, 0, Player.Red), (1, 0, Player.Red), (2, 0, Player.Red),
            (3, 0, Player.Red), (4, 0, Player.Red), (5, 0, Player.Red));

        var detector = new WinDetector();
        var result = detector.CheckWin(board);

        Assert.False(result.HasWinner);
    }

    [Fact]
    public void Overline7InRow_NoWin()
    {
        var board = PlaceStones(
            (0, 0, Player.Red), (1, 0, Player.Red), (2, 0, Player.Red),
            (3, 0, Player.Red), (4, 0, Player.Red), (5, 0, Player.Red),
            (6, 0, Player.Red));

        var detector = new WinDetector();
        var result = detector.CheckWin(board);

        Assert.False(result.HasWinner);
    }

    [Fact]
    public void Exactly5BothEndsBlocked_NoWin()
    {
        // 5 consecutive Red stones with Blue on both sides
        var board = PlaceStones(
            (0, 0, Player.Blue),
            (1, 0, Player.Red), (2, 0, Player.Red), (3, 0, Player.Red),
            (4, 0, Player.Red), (5, 0, Player.Red),
            (6, 0, Player.Blue));

        var detector = new WinDetector();
        var result = detector.CheckWin(board);

        Assert.False(result.HasWinner);
    }

    [Fact]
    public void EmptyBoard_NoWin()
    {
        var board = new Board();
        var detector = new WinDetector();
        var result = detector.CheckWin(board);

        Assert.False(result.HasWinner);
    }

    [Fact]
    public void SingleStone_NoWin()
    {
        var board = PlaceStones((5, 5, Player.Red));
        var detector = new WinDetector();
        var result = detector.CheckWin(board);

        Assert.False(result.HasWinner);
    }
}
