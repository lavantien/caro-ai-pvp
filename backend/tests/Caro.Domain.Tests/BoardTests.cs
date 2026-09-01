using Caro.Domain;
using Xunit;

namespace Caro.Domain.Tests;

public class BoardTests
{
    [Fact]
    public void NewBoardIsEmpty()
    {
        Board b = Board.NewBoard();
        Assert.True(b.IsEmpty());
        for (int x = 0; x < Constants.BoardSize; x++)
        {
            for (int y = 0; y < Constants.BoardSize; y++)
            {
                Assert.Equal(Player.None, b.GetCell(x, y).Player);
            }
        }
        Assert.Equal(0UL, b.Hash);
    }

    [Fact]
    public void BoardPlaceStoneImmutable()
    {
        Board original = Board.NewBoard();
        Board placed = original.PlaceStone(8, 8, Player.Red);

        Assert.Equal(Player.None, original.GetCell(8, 8).Player);
        Assert.Equal(Player.Red, placed.GetCell(8, 8).Player);
        Assert.NotEqual(original.Hash, placed.Hash);
    }

    [Fact]
    public void BoardPlaceStoneMultiple()
    {
        Board b = Board.NewBoard()
            .PlaceStone(8, 8, Player.Red)
            .PlaceStone(7, 7, Player.Blue)
            .PlaceStone(9, 9, Player.Red);

        Assert.Equal(Player.Red, b.GetCell(8, 8).Player);
        Assert.Equal(Player.Blue, b.GetCell(7, 7).Player);
        Assert.Equal(Player.Red, b.GetCell(9, 9).Player);
    }

    [Fact]
    public void BoardPlaceStoneOccupied()
    {
        Board b = Board.NewBoard().PlaceStone(8, 8, Player.Red);
        Assert.Throws<CellOccupiedException>(() => b.PlaceStone(8, 8, Player.Blue));
    }

    [Fact]
    public void BoardPlaceStoneOutOfBounds()
    {
        Board b = Board.NewBoard();
        Assert.Throws<PositionBoundsException>(() => b.PlaceStone(-1, 0, Player.Red));
        Assert.Throws<PositionBoundsException>(() => b.PlaceStone(16, 0, Player.Red));
    }

    [Fact]
    public void BoardBitBoardBits()
    {
        Board b = Board.NewBoard().PlaceStone(0, 0, Player.Red);
        ulong[] redBits = b.BitBoardBits(Player.Red);
        Assert.NotEqual(0UL, redBits[0]);

        ulong[] blueBits = b.BitBoardBits(Player.Blue);
        Assert.Equal(0UL, blueBits[0]);
    }

    [Fact]
    public void BoardHashIncremental()
    {
        Board b1 = Board.NewBoard().PlaceStone(5, 5, Player.Red);
        ulong expectedHash = 0UL ^ Zobrist.ZobristKey(5, 5, Player.Red);
        Assert.Equal(expectedHash, b1.Hash);

        Board b2 = b1.PlaceStone(6, 6, Player.Blue);
        ulong expectedHash2 = expectedHash ^ Zobrist.ZobristKey(6, 6, Player.Blue);
        Assert.Equal(expectedHash2, b2.Hash);
    }

    [Fact]
    public void BoardIsEmptyAt()
    {
        Board b = Board.NewBoard();
        Assert.True(b.IsEmptyAt(8, 8));
        Assert.False(b.IsEmptyAt(-1, 0));

        Board placed = b.PlaceStone(8, 8, Player.Red);
        Assert.False(placed.IsEmptyAt(8, 8));
    }

    [Fact]
    public void BoardGetPlayerAt()
    {
        Board b = Board.NewBoard().PlaceStone(3, 4, Player.Blue);
        Assert.Equal(Player.Blue, b.GetPlayerAt(3, 4));
        Assert.Equal(Player.None, b.GetPlayerAt(5, 5));
        Assert.Equal(Player.None, b.GetPlayerAt(-1, 0));
    }

    [Fact]
    public void BoardBitBoardOps()
    {
        Board b = Board.NewBoard();
        for (int x = 0; x < 4; x++)
        {
            b = b.PlaceStone(x, 0, Player.Red);
        }
        ulong[] redBits = b.BitBoardBits(Player.Red);
        Assert.Equal(0x0FUL, redBits[0]);
    }

    [Fact]
    public void BoardPlaceStoneRequiresValid()
    {
        Board b = Board.NewBoard().PlaceStone(5, 5, Player.Red);

        Assert.Throws<CellOccupiedException>(() => b.PlaceStone(5, 5, Player.Blue));
        Assert.Throws<PositionBoundsException>(() => b.PlaceStone(-1, 5, Player.Red));
    }

    [Fact]
    public void BoardGetCellOutOfBounds()
    {
        Board b = Board.NewBoard();
        Assert.Equal(Player.None, b.GetCell(-1, 0).Player);
        Assert.Equal(Player.None, b.GetCell(0, -1).Player);
        Assert.Equal(Player.None, b.GetCell(Constants.BoardSize, 0).Player);
    }

    [Fact]
    public void BoardPlaceStoneThrowsOnOccupied()
    {
        Board b = Board.NewBoard().PlaceStone(5, 5, Player.Red);
        Assert.Throws<CellOccupiedException>(() => b.PlaceStone(5, 5, Player.Blue));
    }

    [Fact]
    public void BoardPlaceStoneThrowsOutOfBounds()
    {
        Board b = Board.NewBoard();
        Assert.Throws<PositionBoundsException>(() => b.PlaceStone(-1, 0, Player.Red));
    }

    [Fact]
    public void BoardIsEmptyWithStones()
    {
        Board b = Board.NewBoard().PlaceStone(0, 0, Player.Red);
        Assert.False(b.IsEmpty());
    }
}
