using Caro.Domain;
using Xunit;

namespace Caro.Domain.Tests;

public class WinTests
{
    [Fact]
    public void WinDetectorEmpty()
    {
        WinResult result = WinDetector.CheckWin(Board.NewBoard());
        Assert.False(result.HasWinner);
    }

    [Fact]
    public void WinDetectorFiveInRowHorizontal()
    {
        Board b = Board.NewBoard();
        for (int x = 3; x < 8; x++)
        {
            b = b.PlaceStone(x, 5, Player.Red);
        }
        WinResult result = WinDetector.CheckWin(b);
        Assert.True(result.HasWinner);
        Assert.Equal(Player.Red, result.Winner);
        Assert.Equal(5, result.WinningLine!.Length);
    }

    [Fact]
    public void WinDetectorFiveInRowVertical()
    {
        Board b = Board.NewBoard();
        for (int y = 0; y < 5; y++)
        {
            b = b.PlaceStone(5, y, Player.Blue);
        }
        WinResult result = WinDetector.CheckWin(b);
        Assert.True(result.HasWinner);
        Assert.Equal(Player.Blue, result.Winner);
    }

    [Fact]
    public void WinDetectorFiveInRowDiagonal()
    {
        Board b = Board.NewBoard();
        for (int i = 0; i < 5; i++)
        {
            b = b.PlaceStone(3 + i, 3 + i, Player.Red);
        }
        WinResult result = WinDetector.CheckWin(b);
        Assert.True(result.HasWinner);
        Assert.Equal(Player.Red, result.Winner);
    }

    [Fact]
    public void WinDetectorSixNotWin()
    {
        Board b = Board.NewBoard();
        for (int x = 3; x < 9; x++)
        {
            b = b.PlaceStone(x, 5, Player.Red);
        }
        WinResult result = WinDetector.CheckWin(b);
        Assert.False(result.HasWinner, "6 in a row should not win in Caro (overline)");
    }

    [Fact]
    public void WinDetectorBlockedEnds()
    {
        Board b = Board.NewBoard();
        for (int x = 3; x < 8; x++)
        {
            b = b.PlaceStone(x, 5, Player.Red);
        }
        b = b.PlaceStone(2, 5, Player.Blue);
        b = b.PlaceStone(8, 5, Player.Blue);
        WinResult result = WinDetector.CheckWin(b);
        Assert.False(result.HasWinner, "blocked five should not win in Caro");
    }

    [Fact]
    public void WinDetectorOpenEnd()
    {
        Board b = Board.NewBoard();
        for (int x = 3; x < 8; x++)
        {
            b = b.PlaceStone(x, 5, Player.Red);
        }
        WinResult result = WinDetector.CheckWin(b);
        Assert.True(result.HasWinner, "open five should win");
    }

    [Fact]
    public void WinDetectorOneBlockedEnd()
    {
        Board b = Board.NewBoard();
        for (int x = 3; x < 8; x++)
        {
            b = b.PlaceStone(x, 5, Player.Red);
        }
        b = b.PlaceStone(2, 5, Player.Blue);
        WinResult result = WinDetector.CheckWin(b);
        Assert.True(result.HasWinner, "one blocked end still wins");
    }

    [Fact]
    public void WinDetectorFromMove()
    {
        Board b = Board.NewBoard();
        for (int x = 3; x < 7; x++)
        {
            b = b.PlaceStone(x, 5, Player.Red);
        }
        b = b.PlaceStone(7, 5, Player.Red);
        WinResult result = WinDetector.CheckWinFromMove(b, 7, 5);
        Assert.True(result.HasWinner);
        Assert.Equal(Player.Red, result.Winner);
    }

    [Fact]
    public void WinDetectorFromMoveEmpty()
    {
        WinResult result = WinDetector.CheckWinFromMove(Board.NewBoard(), 5, 5);
        Assert.False(result.HasWinner);
    }

    [Fact]
    public void WinDetectorFourNotWin()
    {
        Board b = Board.NewBoard();
        for (int x = 3; x < 7; x++)
        {
            b = b.PlaceStone(x, 5, Player.Red);
        }
        WinResult result = WinDetector.CheckWin(b);
        Assert.False(result.HasWinner, "4 in a row should not win");
    }

    [Fact]
    public void WinDetectorAntiDiagonal()
    {
        Board b = Board.NewBoard();
        for (int i = 0; i < 5; i++)
        {
            b = b.PlaceStone(3 + i, 7 - i, Player.Blue);
        }
        WinResult result = WinDetector.CheckWin(b);
        Assert.True(result.HasWinner);
        Assert.Equal(Player.Blue, result.Winner);
    }
}
