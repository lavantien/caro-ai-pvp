using Caro.Core.Domain.Entities;
using FluentAssertions;

namespace Caro.Core.Domain.Tests.Entities;

public class PositionTests
{
    [Fact]
    public void Invalid_Position_IsInvalid()
    {
        // Arrange
        var position = new Position(-1, 0);

        // Act & Assert
        position.IsValid.Should().BeFalse();
    }

    [Fact]
    public void InRange_Position_IsValid()
    {
        // Arrange
        var positions = new[]
        {
            new Position(0, 0),
            new Position(9, 9),
            new Position(18, 18)
        };

        // Act & Assert
        foreach (var pos in positions)
        {
            pos.IsValid.Should().BeTrue();
        }
    }

    [Fact]
    public void OutOfRange_Position_IsInvalid()
    {
        // Arrange
        var positions = new[]
        {
            new Position(-1, 9),
            new Position(9, -1),
            new Position(19, 9),
            new Position(9, 19)
        };

        // Act & Assert
        foreach (var pos in positions)
        {
            pos.IsValid.Should().BeFalse();
        }
    }

    
    [Fact]
    public void Offset_ReturnsNewPosition()
    {
        // Arrange
        var position = new Position(10, 10);

        // Act
        var offset = position.Offset(1, 2);

        // Assert
        offset.X.Should().Be(11);
        offset.Y.Should().Be(12);
    }

    [Fact]
    public void ToTuple_ConvertsToTuple()
    {
        // Arrange
        var position = new Position(5, 10);

        // Act
        var tuple = position.ToTuple();

        // Assert
        tuple.x.Should().Be(5);
        tuple.y.Should().Be(10);
    }

    [Fact]
    public void FromTuple_CreatesPositionFromTuple()
    {
        // Arrange
        var tuple = (x: 5, y: 10);

        // Act
        var position = Position.FromTuple(tuple);

        // Assert
        position.X.Should().Be(5);
        position.Y.Should().Be(10);
    }

    [Fact]
    public void Deconstruct_ReturnsCoordinates()
    {
        // Arrange
        var position = new Position(5, 10);

        // Act
        var (x, y) = position;

        // Assert
        x.Should().Be(5);
        y.Should().Be(10);
    }

    [Fact]
    public void Equality_SamePositionsAreEqual()
    {
        // Arrange
        var pos1 = new Position(5, 10);
        var pos2 = new Position(5, 10);

        // Act & Assert
        pos1.Should().Be(pos2);
    }

    [Fact]
    public void Equality_DifferentPositionsAreNotEqual()
    {
        // Arrange
        var pos1 = new Position(5, 10);
        var pos2 = new Position(6, 10);

        // Act & Assert
        pos1.Should().NotBe(pos2);
    }
}
