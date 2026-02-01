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
    public void Index_ReturnsLinearIndex()
    {
        // Arrange
        var position = new Position(5, 10);

        // Act
        var index = position.Index;

        // Assert
        index.Should().Be(10 * 19 + 5); // y * Size + x
    }

    [Fact]
    public void FromIndex_ReturnsCorrectPosition()
    {
        // Arrange & Act
        var position = Position.FromIndex(100);

        // Assert
        position.X.Should().Be(100 % 19);
        position.Y.Should().Be(100 / 19);
    }

    [Fact]
    public void FromIndex_OutOfRange_ThrowsArgumentOutOfRangeException()
    {
        // Act & Assert
        Action act = () => Position.FromIndex(-1);
        act.Should().Throw<ArgumentOutOfRangeException>();

        act = () => Position.FromIndex(361);
        act.Should().Throw<ArgumentOutOfRangeException>();
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
    public void GetAdjacentPositions_ReturnsFourNeighbors()
    {
        // Arrange
        var position = new Position(10, 10);

        // Act
        var neighbors = position.GetAdjacentPositions();

        // Assert
        neighbors.Length.Should().Be(4);
        neighbors.ToArray().Should().Contain(p => p.X == 10 && p.Y == 9);  // Up
        neighbors.ToArray().Should().Contain(p => p.X == 10 && p.Y == 11); // Down
        neighbors.ToArray().Should().Contain(p => p.X == 9 && p.Y == 10);  // Left
        neighbors.ToArray().Should().Contain(p => p.X == 11 && p.Y == 10); // Right
    }

    [Fact]
    public void GetAdjacentPositions_AtCorner_ReturnsOnlyValidNeighbors()
    {
        // Arrange
        var position = new Position(0, 0);

        // Act
        var neighbors = position.GetAdjacentPositions();

        // Assert
        neighbors.Length.Should().Be(2);
        neighbors.ToArray().Should().Contain(p => p.X == 0 && p.Y == 1);  // Down
        neighbors.ToArray().Should().Contain(p => p.X == 1 && p.Y == 0);  // Right
    }

    [Fact]
    public void GetNeighbors_ReturnsEightNeighbors()
    {
        // Arrange
        var position = new Position(10, 10);

        // Act
        var neighbors = position.GetNeighbors();

        // Assert
        neighbors.Length.Should().Be(8);
    }

    [Fact]
    public void GetNeighbors_AtCorner_ReturnsOnlyValidNeighbors()
    {
        // Arrange
        var position = new Position(0, 0);

        // Act
        var neighbors = position.GetNeighbors();

        // Assert
        neighbors.Length.Should().Be(3);
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
