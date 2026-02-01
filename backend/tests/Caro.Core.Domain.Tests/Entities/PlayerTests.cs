using Caro.Core.Domain.Entities;
using FluentAssertions;

namespace Caro.Core.Domain.Tests.Entities;

public class PlayerTests
{
    [Fact]
    public void Opponent_RedReturnsBlue()
    {
        // Act
        var opponent = Player.Red.Opponent();

        // Assert
        opponent.Should().Be(Player.Blue);
    }

    [Fact]
    public void Opponent_BlueReturnsRed()
    {
        // Act
        var opponent = Player.Blue.Opponent();

        // Assert
        opponent.Should().Be(Player.Red);
    }

    [Fact]
    public void Opponent_NoneReturnsNone()
    {
        // Act
        var opponent = Player.None.Opponent();

        // Assert
        opponent.Should().Be(Player.None);
    }

    [Fact]
    public void IsValid_RedAndBlueAreValid()
    {
        // Act & Assert
        Player.Red.IsValid().Should().BeTrue();
        Player.Blue.IsValid().Should().BeTrue();
    }

    [Fact]
    public void IsValid_NoneIsInvalid()
    {
        // Act & Assert
        Player.None.IsValid().Should().BeFalse();
    }

    [Fact]
    public void IsRed_ReturnsTrueOnlyForRed()
    {
        // Act & Assert
        Player.Red.IsRed().Should().BeTrue();
        Player.Blue.IsRed().Should().BeFalse();
        Player.None.IsRed().Should().BeFalse();
    }

    [Fact]
    public void IsBlue_ReturnsTrueOnlyForBlue()
    {
        // Act & Assert
        Player.Blue.IsBlue().Should().BeTrue();
        Player.Red.IsBlue().Should().BeFalse();
        Player.None.IsBlue().Should().BeFalse();
    }
}
