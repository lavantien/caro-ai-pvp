using Caro.Core.Domain.ValueObjects;
using FluentAssertions;

namespace Caro.Core.Domain.Tests.ValueObjects;

public class BitBoardTests
{
    [Fact]
    public void Default_Constructor_ReturnsEmptyBitBoard()
    {
        // Act
        var bitBoard = new BitBoard();

        // Assert
        bitBoard.IsEmpty.Should().BeTrue();
        bitBoard.CountBits().Should().Be(0);
    }

    [Fact]
    public void Empty_ReturnsEmptyBitBoard()
    {
        // Act & Assert
        BitBoard.Empty.IsEmpty.Should().BeTrue();
    }

    [Fact]
    public void SetBit_SetsBitAtPosition()
    {
        // Arrange
        var bitBoard = new BitBoard();

        // Act
        var result = bitBoard.SetBit(5, 5);

        // Assert
        bitBoard.GetBit(5, 5).Should().BeFalse("Original should be unchanged");
        result.GetBit(5, 5).Should().BeTrue("Result should have bit set");
    }

    [Fact]
    public void ClearBit_ClearsBitAtPosition()
    {
        // Arrange
        var bitBoard = new BitBoard().SetBit(5, 5);

        // Act
        var result = bitBoard.ClearBit(5, 5);

        // Assert
        bitBoard.GetBit(5, 5).Should().BeTrue("Original should still have bit set");
        result.GetBit(5, 5).Should().BeFalse("Result should have bit cleared");
    }

    [Fact]
    public void WithBit_SetsOrClearsBit()
    {
        // Arrange
        var bitBoard = new BitBoard();

        // Act
        var withBit = bitBoard.WithBit(5, 5, true);
        var clearedBit = withBit.WithBit(5, 5, false);

        // Assert
        withBit.GetBit(5, 5).Should().BeTrue();
        clearedBit.GetBit(5, 5).Should().BeFalse();
    }

    [Fact]
    public void CountBits_ReturnsNumberOfSetBits()
    {
        // Arrange
        var bitBoard = new BitBoard()
            .SetBit(0, 0)
            .SetBit(1, 0)
            .SetBit(2, 0);

        // Act
        var count = bitBoard.CountBits();

        // Assert
        count.Should().Be(3);
    }

    [Fact]
    public void BitwiseOr_CombinesTwoBitBoards()
    {
        // Arrange
        var bb1 = new BitBoard().SetBit(0, 0);
        var bb2 = new BitBoard().SetBit(1, 0);

        // Act
        var result = bb1 | bb2;

        // Assert
        result.GetBit(0, 0).Should().BeTrue();
        result.GetBit(1, 0).Should().BeTrue();
    }

    [Fact]
    public void BitwiseAnd_ReturnsIntersection()
    {
        // Arrange
        var bb1 = new BitBoard().SetBit(0, 0).SetBit(1, 0);
        var bb2 = new BitBoard().SetBit(1, 0).SetBit(2, 0);

        // Act
        var result = bb1 & bb2;

        // Assert
        result.GetBit(0, 0).Should().BeFalse();
        result.GetBit(1, 0).Should().BeTrue();
        result.GetBit(2, 0).Should().BeFalse();
    }

    [Fact]
    public void BitwiseXor_ReturnsSymmetricDifference()
    {
        // Arrange
        var bb1 = new BitBoard().SetBit(0, 0).SetBit(1, 0);
        var bb2 = new BitBoard().SetBit(1, 0).SetBit(2, 0);

        // Act
        var result = bb1 ^ bb2;

        // Assert
        result.GetBit(0, 0).Should().BeTrue();
        result.GetBit(1, 0).Should().BeFalse();
        result.GetBit(2, 0).Should().BeTrue();
    }

    [Fact]
    public void BitwiseComplement_InvertsAllBits()
    {
        // Arrange
        var bb = new BitBoard().SetBit(0, 0);

        // Act
        var result = ~bb;

        // Assert
        result.GetBit(0, 0).Should().BeFalse();
        // Note: many other bits are set due to complement
    }

    [Fact]
    public void ShiftLeft_MovesAllBitsLeft()
    {
        // Arrange
        var bb = new BitBoard().SetBit(5, 5);

        // Act
        var result = bb.ShiftLeft();

        // Assert
        bb.GetBit(5, 5).Should().BeTrue();
        result.GetBit(5, 5).Should().BeFalse();
        result.GetBit(4, 5).Should().BeTrue();
    }

    [Fact]
    public void ShiftRight_MovesAllBitsRight()
    {
        // Arrange
        var bb = new BitBoard().SetBit(5, 5);

        // Act
        var result = bb.ShiftRight();

        // Assert
        bb.GetBit(5, 5).Should().BeTrue();
        result.GetBit(5, 5).Should().BeFalse();
        result.GetBit(6, 5).Should().BeTrue();
    }

    [Fact]
    public void ShiftUp_MovesAllBitsUp()
    {
        // Arrange
        var bb = new BitBoard().SetBit(5, 5);

        // Act
        var result = bb.ShiftUp();

        // Assert
        bb.GetBit(5, 5).Should().BeTrue();
        result.GetBit(5, 5).Should().BeFalse();
        result.GetBit(5, 4).Should().BeTrue();
    }

    [Fact]
    public void ShiftDown_MovesAllBitsDown()
    {
        // Arrange
        var bb = new BitBoard().SetBit(5, 5);

        // Act
        var result = bb.ShiftDown();

        // Assert
        bb.GetBit(5, 5).Should().BeTrue();
        result.GetBit(5, 5).Should().BeFalse();
        result.GetBit(5, 6).Should().BeTrue();
    }

    [Fact]
    public void ShiftDiagonal_CombinesTwoShifts()
    {
        // Arrange
        var bb = new BitBoard().SetBit(5, 5);

        // Act
        var upLeft = bb.ShiftUpLeft();
        var upRight = bb.ShiftUpRight();
        var downLeft = bb.ShiftDownLeft();
        var downRight = bb.ShiftDownRight();

        // Assert
        upLeft.GetBit(4, 4).Should().BeTrue();
        upRight.GetBit(6, 4).Should().BeTrue();
        downLeft.GetBit(4, 6).Should().BeTrue();
        downRight.GetBit(6, 6).Should().BeTrue();
    }

    [Fact]
    public void GetRawValues_ReturnsAllUlongs()
    {
        // Arrange
        var bb = new BitBoard(
            1UL, 2UL, 3UL, 4UL, 5UL, 6UL
        );

        // Act
        var (b0, b1, b2, b3, b4, b5) = bb.GetRawValues();

        // Assert
        b0.Should().Be(1UL);
        b1.Should().Be(2UL);
        b2.Should().Be(3UL);
        b3.Should().Be(4UL);
        b4.Should().Be(5UL);
        b5.Should().Be(6UL);
    }

    [Fact]
    public void FromRawValues_CreatesBitBoard()
    {
        // Act
        var bb = BitBoard.FromRawValues(1UL, 2UL, 3UL, 4UL, 5UL, 6UL);

        // Assert
        var (b0, b1, b2, b3, b4, b5) = bb.GetRawValues();
        b0.Should().Be(1UL);
        b5.Should().Be(6UL);
    }

    [Fact]
    public void GetSetPositions_ReturnsAllSetBits()
    {
        // Arrange
        var bb = new BitBoard()
            .SetBit(0, 0)
            .SetBit(5, 5)
            .SetBit(18, 18);

        // Act
        var positions = bb.GetSetPositions();

        // Assert
        positions.Should().HaveCount(3);
        positions.Should().Contain((5, 5));
        positions.Should().Contain((18, 18));
    }

    [Fact]
    public void Equality_SameBitBoardsAreEqual()
    {
        // Arrange
        var bb1 = new BitBoard().SetBit(5, 5);
        var bb2 = new BitBoard().SetBit(5, 5);

        // Act & Assert
        bb1.Should().Be(bb2);
    }

    [Fact]
    public void Equality_DifferentBitBoardsAreNotEqual()
    {
        // Arrange
        var bb1 = new BitBoard().SetBit(5, 5);
        var bb2 = new BitBoard().SetBit(6, 6);

        // Act & Assert
        bb1.Should().NotBe(bb2);
    }

    [Fact]
    public void EqualityOperator_WorksCorrectly()
    {
        // Arrange
        var bb1 = new BitBoard().SetBit(5, 5);
        var bb2 = new BitBoard().SetBit(5, 5);
        var bb3 = new BitBoard().SetBit(6, 6);

        // Act & Assert
        (bb1 == bb2).Should().BeTrue();
        (bb1 == bb3).Should().BeFalse();
        (bb1 != bb3).Should().BeTrue();
    }
}
