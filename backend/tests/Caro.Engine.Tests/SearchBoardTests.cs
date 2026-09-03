using Caro.Domain;
using Caro.Engine;
using Xunit;

namespace Caro.Engine.Tests;

public class SearchBoardTests
{
    [Fact]
    public void SearchBoardMakeUnmake()
    {
        SearchBoard sb = new(Board.NewBoard());

        ulong hashBefore = sb.Hash();
        sb.MakeMove(8, 8, Player.Red);
        Assert.Equal(Player.Red, sb.PlayerAt(8, 8));
        Assert.NotEqual(hashBefore, sb.Hash());

        sb.UnmakeMove();
        Assert.Equal(Player.None, sb.PlayerAt(8, 8));
        Assert.Equal(hashBefore, sb.Hash());
    }

    [Fact]
    public void SearchBoardMultipleMoves()
    {
        SearchBoard sb = new(Board.NewBoard());

        sb.MakeMove(8, 8, Player.Red);
        sb.MakeMove(7, 7, Player.Blue);
        sb.MakeMove(9, 9, Player.Red);

        Assert.Equal(Player.Red, sb.PlayerAt(8, 8));
        Assert.Equal(Player.Blue, sb.PlayerAt(7, 7));
        Assert.Equal(Player.Red, sb.PlayerAt(9, 9));

        sb.UnmakeMove();
        Assert.Equal(Player.None, sb.PlayerAt(9, 9));
        Assert.Equal(Player.Blue, sb.PlayerAt(7, 7));
    }

    [Fact]
    public void SearchBoardFromDomain()
    {
        Board b = Board.NewBoard()
            .PlaceStone(5, 5, Player.Red)
            .PlaceStone(6, 6, Player.Blue);

        SearchBoard sb = new(b);
        Assert.Equal(Player.Red, sb.PlayerAt(5, 5));
        Assert.Equal(Player.Blue, sb.PlayerAt(6, 6));
        Assert.Equal(b.Hash, sb.Hash());
    }

    [Fact]
    public void SearchBoardPlayerAtBounds()
    {
        SearchBoard sb = new(Board.NewBoard());
        Assert.Equal(Player.None, sb.PlayerAt(-1, 0));
        Assert.Equal(Player.None, sb.PlayerAt(0, -1));
        Assert.Equal(Player.None, sb.PlayerAt(Constants.Board.Size, 0));
    }

    [Fact]
    public void SearchBoardIsEmptyBounds()
    {
        SearchBoard sb = new(Board.NewBoard());
        Assert.False(sb.IsEmpty(-1, 0));
        Assert.False(sb.IsEmpty(0, -1));
        Assert.False(sb.IsEmpty(Constants.Board.Size, 0));
        Assert.True(sb.IsEmpty(7, 7));
    }

    [Fact]
    public void SearchBoardNullMove()
    {
        SearchBoard sb = new(Board.NewBoard());
        ulong hashBefore = sb.Hash();
        sb.MakeNullMove();
        Assert.NotEqual(hashBefore, sb.Hash());
        sb.UnmakeNullMove();
        Assert.Equal(hashBefore, sb.Hash());
    }

    [Fact]
    public void SearchBoardBitBoardFor()
    {
        SearchBoard sb = new(Board.NewBoard().PlaceStone(5, 5, Player.Red));
        Assert.False(sb.BitBoardFor(Player.Red).IsZero());
        Assert.True(sb.BitBoardFor(Player.Blue).IsZero());
    }

    [Fact]
    public void SearchBoardOccupied()
    {
        SearchBoard sb = new(Board.NewBoard());
        Assert.True(sb.Occupied().IsZero());
        sb.MakeMove(5, 5, Player.Red);
        Assert.False(sb.Occupied().IsZero());
    }

    [Fact]
    public void SearchBoardNullMoveHashUnique()
    {
        SearchBoard sb = new(Board.NewBoard());
        ulong hashBefore = sb.Hash();
        sb.MakeNullMove();
        ulong nullHash = sb.Hash();
        Assert.NotEqual(hashBefore, nullHash);
        Assert.NotEqual(Zobrist.ZobristNullMove(), hashBefore);
        for (int x = 0; x < Constants.Board.Size; x++)
        {
            for (int y = 0; y < Constants.Board.Size; y++)
            {
                Assert.NotEqual(nullHash, Zobrist.ZobristKey(x, y, Player.Red));
                Assert.NotEqual(nullHash, Zobrist.ZobristKey(x, y, Player.Blue));
            }
        }
    }

    [Fact]
    public void SearchBoardNullMoveRoundTrip()
    {
        SearchBoard sb = new(Board.NewBoard().PlaceStone(5, 5, Player.Red));
        ulong hashBefore = sb.Hash();
        sb.MakeMove(7, 7, Player.Blue);
        ulong hashAfterMove = sb.Hash();
        sb.MakeNullMove();
        ulong hashAfterNull = sb.Hash();
        Assert.NotEqual(hashAfterMove, hashAfterNull);
        sb.UnmakeNullMove();
        Assert.Equal(hashAfterMove, sb.Hash());
        sb.UnmakeMove();
        Assert.Equal(hashBefore, sb.Hash());
    }
}
