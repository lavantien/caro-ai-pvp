using Caro.Domain;
using Xunit;

namespace Caro.Domain.Tests;

public class PositionTests
{
    public static TheoryData<int, int, bool> ValidCases => new()
    {
        { 0, 0, true },
        { 8, 8, true },
        { 15, 15, true },
        { -1, 0, false },
        { 0, 16, false },
        { 16, 16, false },
    };

    [Theory]
    [MemberData(nameof(ValidCases))]
    public void PositionIsValid(int x, int y, bool expected)
    {
        Assert.Equal(expected, new Position(x, y).IsValid());
    }

    [Fact]
    public void PositionOffset()
    {
        Position p = new(5, 5);
        Assert.Equal(new Position(6, 7), p.Offset(1, 2));
    }
}
