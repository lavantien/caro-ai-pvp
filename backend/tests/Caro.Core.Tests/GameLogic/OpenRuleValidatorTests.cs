using Xunit;
using FluentAssertions;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

namespace Caro.Core.Tests.GameLogic;

public class OpenRuleValidatorTests
{
    [Theory]
    [InlineData(5, 5, false)]  // Inside 5x5 zone - invalid
    [InlineData(6, 5, false)]
    [InlineData(7, 5, false)]
    [InlineData(8, 5, false)]
    [InlineData(9, 5, false)]
    [InlineData(5, 6, false)]
    [InlineData(6, 6, false)]
    [InlineData(7, 6, false)]
    [InlineData(8, 6, false)]
    [InlineData(9, 6, false)]
    [InlineData(5, 7, false)]
    [InlineData(6, 7, false)]
    [InlineData(7, 7, false)]  // Center - invalid
    [InlineData(8, 7, false)]
    [InlineData(9, 7, false)]
    [InlineData(5, 8, false)]
    [InlineData(6, 8, false)]
    [InlineData(7, 8, false)]
    [InlineData(8, 8, false)]
    [InlineData(9, 8, false)]
    [InlineData(5, 9, false)]
    [InlineData(6, 9, false)]
    [InlineData(7, 9, false)]
    [InlineData(8, 9, false)]
    [InlineData(9, 9, false)]
    [InlineData(4, 7, true)]   // Outside zone - valid
    [InlineData(7, 4, true)]
    [InlineData(10, 7, true)]
    [InlineData(7, 10, true)]
    [InlineData(0, 0, true)]   // Far corner - valid
    public void IsValidSecondMove_ForMove3_Checks5x5Zone(int x, int y, bool expectedValid)
    {
        // Arrange
        var validator = new OpenRuleValidator();
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);   // Move #1 (Red at center)
        board = board.PlaceStone(4, 7, Player.Blue);  // Move #2 (Blue somewhere else)

        // Act - Move #3: Red's second move, Open Rule applies
        var isValid = validator.IsValidSecondMove(board, x, y);

        // Assert
        isValid.Should().Be(expectedValid);
    }

    [Fact]
    public void IsValidSecondMove_AfterMove3_ReturnsTrue()
    {
        // Arrange
        var validator = new OpenRuleValidator();
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);   // Move #1
        board = board.PlaceStone(4, 7, Player.Blue);  // Move #2 (Blue can place anywhere)
        board = board.PlaceStone(10, 7, Player.Red);  // Move #3 (Open Rule applied)

        // Act - Move #4 onwards, no restriction
        var isValid = validator.IsValidSecondMove(board, 7, 8);

        // Assert
        isValid.Should().BeTrue();
    }

    [Fact]
    public void IsValidSecondMove_Move1_NoRestriction()
    {
        // Arrange
        var validator = new OpenRuleValidator();
        var board = new Board();

        // Act - Move #1, no restriction
        var isValid = validator.IsValidSecondMove(board, 7, 7);

        // Assert
        isValid.Should().BeTrue();
    }
}
