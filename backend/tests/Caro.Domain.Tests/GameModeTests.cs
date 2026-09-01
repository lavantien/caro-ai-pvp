using Caro.Domain;
using Xunit;

namespace Caro.Domain.Tests;

public class GameModeTests
{
    [Fact]
    public void GameModeName()
    {
        Assert.Equal("pvp", GameMode.PvP.ToName());
        Assert.Equal("pvai", GameMode.PvAI.ToName());
        Assert.Equal("aivai", GameMode.AivAI.ToName());
        Assert.Equal("pvp", ((GameMode)99).ToName());
    }

    [Fact]
    public void ParseGameMode()
    {
        Assert.Equal(GameMode.PvP, GameModes.Parse("pvp"));
        Assert.Equal(GameMode.PvAI, GameModes.Parse("pvai"));
        Assert.Equal(GameMode.AivAI, GameModes.Parse("aivai"));
        Assert.Equal(GameMode.PvP, GameModes.Parse("unknown"));
        Assert.Equal(GameMode.PvP, GameModes.Parse(""));
    }

    [Fact]
    public void CellIsEmpty()
    {
        Assert.True(new Cell(0, 0, Player.None).IsEmpty());
        Assert.False(new Cell(0, 0, Player.Red).IsEmpty());
        Assert.False(new Cell(0, 0, Player.Blue).IsEmpty());
    }

    [Fact]
    public void BoardIsNotEmpty()
    {
        Board b = Board.NewBoard().PlaceStone(8, 8, Player.Red);
        Assert.False(b.IsEmpty());
    }
}
