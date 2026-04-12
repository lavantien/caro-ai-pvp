using Xunit;
using FluentAssertions;
using Caro.Core.Domain.Configuration;
using Caro.Core.GameLogic.UCI;

namespace Caro.Core.Tests.GameLogic.UCI;

public sealed class UCIMoveNotationTests
{
    private const int BoardSize = GameConstants.BoardSize;
    private const int MaxBoardIndex = BoardSize - 1;

    // --- ToUCI ---

    [Theory]
    [InlineData(0, 0, "aa1")]
    [InlineData(15, 15, "dd16")]
    [InlineData(7, 8, "bd9")]
    [InlineData(8, 8, "ca9")]
    [InlineData(0, 15, "aa16")]
    [InlineData(15, 0, "dd1")]
    public void ToUCI_MapsCoordinatesCorrectly(int x, int y, string expected)
    {
        UCIMoveNotation.ToUCI(x, y).Should().Be(expected);
    }

    [Theory]
    [InlineData(-1, 0)]
    [InlineData(0, -1)]
    [InlineData(BoardSize, 0)]
    [InlineData(0, BoardSize)]
    [InlineData(-1, -1)]
    [InlineData(BoardSize, BoardSize)]
    public void ToUCI_OutOfBounds_ThrowsArgumentOutOfRangeException(int x, int y)
    {
        var act = () => UCIMoveNotation.ToUCI(x, y);
        act.Should().Throw<ArgumentOutOfRangeException>();
    }

    // --- FromUCI ---

    [Theory]
    [InlineData("aa1", 0, 0)]
    [InlineData("dd16", 15, 15)]
    [InlineData("bd9", 7, 8)]
    [InlineData("ca9", 8, 8)]
    [InlineData("aa16", 0, 15)]
    [InlineData("dd1", 15, 0)]
    [InlineData("BD9", 7, 8)] // Case-insensitive
    [InlineData("Dd1", 15, 0)]
    public void FromUCI_ParsesCorrectly(string move, int expectedX, int expectedY)
    {
        var pos = UCIMoveNotation.FromUCI(move);
        pos.X.Should().Be(expectedX);
        pos.Y.Should().Be(expectedY);
    }

    [Theory]
    [InlineData("")]
    [InlineData("  ")]
    [InlineData("a")]
    [InlineData("a1")]
    public void FromUCI_InvalidFormat_ThrowsArgumentException(string move)
    {
        var act = () => UCIMoveNotation.FromUCI(move);
        act.Should().Throw<ArgumentException>();
    }

    [Theory]
    [InlineData("ea1")]  // first letter out of a-d
    [InlineData("ae1")]  // second letter out of a-d
    [InlineData("zz1")]
    public void FromUCI_InvalidColumnLetters_ThrowsArgumentException(string move)
    {
        var act = () => UCIMoveNotation.FromUCI(move);
        act.Should().Throw<ArgumentException>();
    }

    [Fact]
    public void FromUCI_InvalidRow_ThrowsArgumentException()
    {
        var act = () => UCIMoveNotation.FromUCI("aaxyz");
        act.Should().Throw<ArgumentException>();
    }

    [Theory]
    [InlineData("aa0")]   // Row 0 doesn't exist (1-based)
    [InlineData("aa17")]  // Row 17 out of bounds
    [InlineData("aa-1")]  // Negative row
    public void FromUCI_OutOfBoundsRow_ThrowsArgumentException(string move)
    {
        var act = () => UCIMoveNotation.FromUCI(move);
        act.Should().Throw<ArgumentException>();
    }

    // --- Round-trip ---

    [Fact]
    public void ToUCI_FromUCI_RoundTripsAllPositions()
    {
        for (int x = 0; x < BoardSize; x++)
        {
            for (int y = 0; y < BoardSize; y++)
            {
                var uci = UCIMoveNotation.ToUCI(x, y);
                var pos = UCIMoveNotation.FromUCI(uci);
                pos.X.Should().Be(x, $"column {x} should round-trip via '{uci}'");
                pos.Y.Should().Be(y, $"row {y} should round-trip via '{uci}'");
            }
        }
    }

    // --- ColumnFromDoubleChar / ColumnToDoubleChar ---

    [Theory]
    [InlineData('a', 'a', 0)]
    [InlineData('a', 'b', 1)]
    [InlineData('a', 'c', 2)]
    [InlineData('a', 'd', 3)]
    [InlineData('b', 'a', 4)]
    [InlineData('b', 'b', 5)]
    [InlineData('b', 'c', 6)]
    [InlineData('b', 'd', 7)]
    [InlineData('c', 'a', 8)]
    [InlineData('c', 'b', 9)]
    [InlineData('c', 'c', 10)]
    [InlineData('c', 'd', 11)]
    [InlineData('d', 'a', 12)]
    [InlineData('d', 'b', 13)]
    [InlineData('d', 'c', 14)]
    [InlineData('d', 'd', 15)]
    public void ColumnFromDoubleChar_AllColumns_ReturnsCorrectIndex(char first, char second, int expected)
    {
        UCIMoveNotation.ColumnFromDoubleChar(first, second).Should().Be(expected);
    }

    [Theory]
    [InlineData(0, "aa")]
    [InlineData(1, "ab")]
    [InlineData(3, "ad")]
    [InlineData(4, "ba")]
    [InlineData(7, "bd")]
    [InlineData(8, "ca")]
    [InlineData(11, "cd")]
    [InlineData(12, "da")]
    [InlineData(15, "dd")]
    public void ColumnToDoubleChar_AllColumns_ReturnsCorrectString(int column, string expected)
    {
        UCIMoveNotation.ColumnToDoubleChar(column).Should().Be(expected);
    }

    [Fact]
    public void ColumnDoubleChar_RoundTripsAll16Columns()
    {
        for (int i = 0; i < BoardSize; i++)
        {
            var str = UCIMoveNotation.ColumnToDoubleChar(i);
            var recovered = UCIMoveNotation.ColumnFromDoubleChar(str[0], str[1]);
            recovered.Should().Be(i, $"column {i} should round-trip via '{str}'");
        }
    }

    // --- IsValidCoordinate ---

    [Theory]
    [InlineData(0, 0, true)]
    [InlineData(15, 15, true)]
    [InlineData(7, 8, true)]
    [InlineData(-1, 0, false)]
    [InlineData(0, -1, false)]
    [InlineData(BoardSize, 0, false)]
    [InlineData(0, BoardSize, false)]
    public void IsValidCoordinate_BoundaryChecks(int x, int y, bool expected)
    {
        UCIMoveNotation.IsValidCoordinate(x, y).Should().Be(expected);
    }

    // --- IsValidMove ---

    [Theory]
    [InlineData("aa1", true)]
    [InlineData("dd16", true)]
    [InlineData("bd9", true)]
    [InlineData("", false)]
    [InlineData("a", false)]
    [InlineData("ea1", false)]
    [InlineData("aa0", false)]
    [InlineData("aa17", false)]
    public void IsValidMove_ValidatesCorrectly(string move, bool expected)
    {
        UCIMoveNotation.IsValidMove(move).Should().Be(expected);
    }
}
