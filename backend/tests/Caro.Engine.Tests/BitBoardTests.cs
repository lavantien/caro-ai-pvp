using Caro.Domain;
using Caro.Engine;
using Xunit;

namespace Caro.Engine.Tests;

public class BitBoardTests
{
    [Fact]
    public void BitBoardSetAndGet()
    {
        BitBoard bb = default;
        bb.Set(0, 0);
        Assert.True(bb.Get(0, 0));
        Assert.False(bb.Get(1, 0));

        bb.Set(15, 15);
        Assert.True(bb.Get(15, 15));
    }

    [Fact]
    public void BitBoardClear()
    {
        BitBoard bb = default;
        bb.Set(5, 5);
        bb.Clear(5, 5);
        Assert.False(bb.Get(5, 5));
    }

    [Fact]
    public void BitBoardOr()
    {
        BitBoard a = default;
        BitBoard b = default;
        a.Set(0, 0);
        b.Set(1, 0);
        BitBoard c = a.Or(b);
        Assert.True(c.Get(0, 0));
        Assert.True(c.Get(1, 0));
    }

    [Fact]
    public void BitBoardCount()
    {
        BitBoard bb = default;
        bb.Set(0, 0);
        bb.Set(1, 0);
        bb.Set(2, 0);
        Assert.Equal(3, bb.Count());
    }

    [Fact]
    public void BitBoardDilate()
    {
        BitBoard bb = default;
        bb.Set(8, 8);
        BitBoard dilated = bb.Dilate();
        Assert.True(dilated.Get(7, 7), "diagonal up-left");
        Assert.True(dilated.Get(8, 8), "center preserved");
        Assert.True(dilated.Get(9, 9), "diagonal down-right");
        Assert.True(dilated.Get(7, 8), "left");
        Assert.True(dilated.Get(9, 8), "right");
        Assert.True(dilated.Get(8, 7), "up");
        Assert.True(dilated.Get(8, 9), "down");
    }

    [Fact]
    public void BitBoardFromDomain()
    {
        Board b = Board.NewBoard().PlaceStone(3, 4, Player.Red);
        (BitBoard red, BitBoard blue) = BitBoard.BitBoardsFromDomain(b);
        Assert.True(red.Get(3, 4));
        Assert.False(blue.Get(3, 4));
    }

    [Fact]
    public void BitBoardAnd()
    {
        BitBoard a = default;
        BitBoard b = default;
        a.Set(0, 0);
        a.Set(1, 0);
        b.Set(1, 0);
        b.Set(2, 0);
        BitBoard c = a.And(b);
        Assert.False(c.Get(0, 0));
        Assert.True(c.Get(1, 0));
        Assert.False(c.Get(2, 0));
    }

    [Fact]
    public void BitBoardXor()
    {
        BitBoard a = default;
        BitBoard b = default;
        a.Set(0, 0);
        a.Set(1, 0);
        b.Set(1, 0);
        b.Set(2, 0);
        BitBoard c = a.Xor(b);
        Assert.True(c.Get(0, 0));
        Assert.False(c.Get(1, 0));
        Assert.True(c.Get(2, 0));
    }

    [Fact]
    public void BitBoardNot()
    {
        BitBoard bb = default;
        bb.Set(0, 0);
        BitBoard n = bb.Not();
        Assert.False(n.Get(0, 0));
        Assert.True(n.Get(1, 0));
    }

    [Fact]
    public void BitBoardIsZero()
    {
        BitBoard bb = default;
        Assert.True(bb.IsZero());
        bb.Set(5, 5);
        Assert.False(bb.IsZero());
    }
}
