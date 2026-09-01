using Caro.Domain;
using Xunit;

namespace Caro.Domain.Tests;

public class PlayerTests
{
    public static TheoryData<Player, Player> OpponentCases => new()
    {
        { Player.Red, Player.Blue },
        { Player.Blue, Player.Red },
        { Player.None, Player.None },
    };

    [Theory]
    [MemberData(nameof(OpponentCases))]
    public void PlayerOpponent(Player player, Player expected)
    {
        Assert.Equal(expected, player.Opponent());
    }

    [Fact]
    public void PlayerIsValid()
    {
        Assert.True(Player.Red.IsValid());
        Assert.True(Player.Blue.IsValid());
        Assert.False(Player.None.IsValid());
    }

    [Fact]
    public void PlayerName()
    {
        Assert.Equal("red", Player.Red.ToName());
        Assert.Equal("blue", Player.Blue.ToName());
        Assert.Equal("none", Player.None.ToName());
    }

    [Fact]
    public void ParsePlayer()
    {
        Assert.True(Players.TryParse("red", out Player p));
        Assert.Equal(Player.Red, p);

        Assert.True(Players.TryParse("blue", out p));
        Assert.Equal(Player.Blue, p);

        Assert.True(Players.TryParse("none", out p));
        Assert.Equal(Player.None, p);

        Assert.False(Players.TryParse("invalid", out _));
    }
}
