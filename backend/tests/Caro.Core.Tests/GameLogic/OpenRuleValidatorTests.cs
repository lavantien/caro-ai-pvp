using Xunit;
using FluentAssertions;
using Caro.Core.Entities;
using Caro.Core.GameLogic;

namespace Caro.Core.Tests.GameLogic;

public class OpenRuleValidatorTests
{
    [Theory]
    [InlineData(6, 6, false)]  // Inside 3x3 zone - invalid
    [InlineData(7, 6, false)]
    [InlineData(8, 6, false)]
    [InlineData(6, 7, false)]
    [InlineData(7, 7, false)]  // Center - invalid
    [InlineData(8, 7, false)]
    [InlineData(6, 8, false)]
    [InlineData(7, 8, false)]
    [InlineData(8, 8, false)]
    [InlineData(5, 7, true)]   // Outside zone - valid
    [InlineData(7, 5, true)]
    [InlineData(9, 7, true)]
    [InlineData(7, 9, true)]
    [InlineData(0, 0, true)]   // Far corner - valid
    public void IsValidSecondMove_ForMove3_Checks3x3Zone(int x, int y, bool expectedValid)
    {
        // Arrange
        var validator = new OpenRuleValidator();
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);   // Move #1 (Red at center)
        board.PlaceStone(5, 7, Player.Blue);  // Move #2 (Blue somewhere else)

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
        board.PlaceStone(7, 7, Player.Red);   // Move #1
        board.PlaceStone(5, 7, Player.Blue);  // Move #2 (Blue can place anywhere)
        board.PlaceStone(9, 7, Player.Red);   // Move #3 (Open Rule applied)

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
