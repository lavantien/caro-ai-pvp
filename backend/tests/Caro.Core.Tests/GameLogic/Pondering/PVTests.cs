using Xunit;
using FluentAssertions;
using Caro.Core.GameLogic;

namespace Caro.Core.Tests.GameLogic.Pondering;

public class PVTests
{
    [Fact]
    public void Empty_HasNoMoves_ReturnsEmptyArray()
    {
        // Act
        var pv = PV.Empty;

        // Assert
        pv.Moves.Should().BeEmpty();
        pv.Depth.Should().Be(0);
        pv.Score.Should().Be(0);
        pv.IsEmpty.Should().BeTrue();
    }

    [Fact]
    public void FromSingleMove_CreatesPVWithOneMove()
    {
        // Act
        var pv = PV.FromSingleMove(7, 7, 5, 100);

        // Assert
        pv.Moves.Should().HaveCount(1);
        pv.Moves[0].Should().Be((7, 7));
        pv.Depth.Should().Be(5);
        pv.Score.Should().Be(100);
        pv.IsEmpty.Should().BeFalse();
    }

    [Fact]
    public void FromMoves_CreatesPVWithMultipleMoves()
    {
        // Arrange
        var moves = new[] { (7, 7), (8, 8), (9, 9) };

        // Act
        var pv = PV.FromMoves(moves, 10, 500);

        // Assert
        pv.Moves.Should().HaveCount(3);
        pv.Moves[0].Should().Be((7, 7));
        pv.Moves[1].Should().Be((8, 8));
        pv.Moves[2].Should().Be((9, 9));
        pv.Depth.Should().Be(10);
        pv.Score.Should().Be(500);
    }

    [Fact]
    public void GetBestMove_WithSingleMove_ReturnsTheMove()
    {
        // Arrange
        var pv = PV.FromSingleMove(5, 5, 3, 50);

        // Act
        var bestMove = pv.GetBestMove();

        // Assert
        bestMove.Should().NotBeNull();
        bestMove.Value.Item1.Should().Be(5);
        bestMove.Value.Item2.Should().Be(5);
    }

    [Fact]
    public void GetBestMove_WithMultipleMoves_ReturnsFirstMove()
    {
        // Arrange
        var moves = new[] { (3, 3), (7, 7), (10, 10) };
        var pv = PV.FromMoves(moves, 6, 200);

        // Act
        var bestMove = pv.GetBestMove();

        // Assert
        bestMove.Should().NotBeNull();
        bestMove.Value.Item1.Should().Be(3);
        bestMove.Value.Item2.Should().Be(3);
    }

    [Fact]
    public void GetBestMove_EmptyPV_ReturnsNull()
    {
        // Arrange
        var pv = PV.Empty;

        // Act
        var bestMove = pv.GetBestMove();

        // Assert
        bestMove.Should().BeNull();
    }

    [Fact]
    public void GetPredictedOpponentMove_WithSingleMove_ReturnsNull()
    {
        // Arrange - PV with only our move, no opponent response
        var pv = PV.FromSingleMove(7, 7, 3, 100);

        // Act
        var predictedMove = pv.GetPredictedOpponentMove();

        // Assert
        predictedMove.Should().BeNull();
    }

    [Fact]
    public void GetPredictedOpponentMove_WithTwoMoves_ReturnsSecondMove()
    {
        // Arrange - PV with our move and opponent response
        var moves = new[] { (7, 7), (8, 8) };
        var pv = PV.FromMoves(moves, 5, 100);

        // Act
        var predictedMove = pv.GetPredictedOpponentMove();

        // Assert
        predictedMove.Should().NotBeNull();
        predictedMove.Value.Item1.Should().Be(8);
        predictedMove.Value.Item2.Should().Be(8);
    }

    [Fact]
    public void GetPredictedOpponentMove_WithMultipleMoves_ReturnsSecondMove()
    {
        // Arrange - PV with full sequence
        var moves = new[] { (7, 7), (8, 8), (9, 9), (10, 10) };
        var pv = PV.FromMoves(moves, 8, 300);

        // Act
        var predictedMove = pv.GetPredictedOpponentMove();

        // Assert
        predictedMove.Should().NotBeNull();
        predictedMove.Value.Item1.Should().Be(8);
        predictedMove.Value.Item2.Should().Be(8);
    }

    [Fact]
    public void GetPredictedOpponentMove_EmptyPV_ReturnsNull()
    {
        // Arrange
        var pv = PV.Empty;

        // Act
        var predictedMove = pv.GetPredictedOpponentMove();

        // Assert
        predictedMove.Should().BeNull();
    }

    [Fact]
    public void IsEmpty_EmptyPV_ReturnsTrue()
    {
        // Arrange
        var pv = PV.Empty;

        // Act
        var isEmpty = pv.IsEmpty;

        // Assert
        isEmpty.Should().BeTrue();
    }

    [Fact]
    public void IsEmpty_NonEmptyPV_ReturnsFalse()
    {
        // Arrange
        var pv = PV.FromSingleMove(5, 5, 1, 10);

        // Act
        var isEmpty = pv.IsEmpty;

        // Assert
        isEmpty.Should().BeFalse();
    }

    [Fact]
    public void Depth_StoresCorrectValue()
    {
        // Arrange & Act
        var pv = PV.FromSingleMove(1, 1, 15, 1000);

        // Assert
        pv.Depth.Should().Be(15);
    }

    [Fact]
    public void Score_StoresCorrectValue()
    {
        // Arrange & Act
        var pv = PV.FromSingleMove(1, 1, 5, -500);

        // Assert
        pv.Score.Should().Be(-500);
    }

    [Fact]
    public void PV_ValuesAreAccessible()
    {
        // Arrange & Act
        var pv = PV.FromSingleMove(7, 7, 5, 100);

        // Assert
        pv.Depth.Should().Be(5);
        pv.Score.Should().Be(100);
    }
}
