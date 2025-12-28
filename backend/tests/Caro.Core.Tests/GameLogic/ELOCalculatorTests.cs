using Xunit;
using FluentAssertions;
using Caro.Core.GameLogic;

namespace Caro.Core.Tests.GameLogic;

public class ELOCalculatorTests
{
    [Fact]
    public void CalculateNewRating_WhenBothPlayersSameRating_Exchange16Points()
    {
        // Arrange
        var calculator = new ELOCalculator();
        var playerRating = 1500;
        var opponentRating = 1500;

        // Act
        var newRating = calculator.CalculateNewRating(playerRating, opponentRating, won: true);

        // Assert
        newRating.Should().Be(1516); // +16 for winning against equal opponent
    }

    [Fact]
    public void CalculateNewRating_WhenPlayerLoses_DecreasesBy16Points()
    {
        // Arrange
        var calculator = new ELOCalculator();
        var playerRating = 1500;
        var opponentRating = 1500;

        // Act
        var newRating = calculator.CalculateNewRating(playerRating, opponentRating, won: false);

        // Assert
        newRating.Should().Be(1484); // -16 for losing against equal opponent
    }

    [Fact]
    public void CalculateNewRating_WhenPlayerHigherRating_Wins_GainsLessPoints()
    {
        // Arrange
        var calculator = new ELOCalculator();
        var playerRating = 1700;
        var opponentRating = 1300;

        // Act
        var newRating = calculator.CalculateNewRating(playerRating, opponentRating, won: true);

        // Assert
        newRating.Should().Be(1703); // +3 for beating much weaker opponent (rounded)
    }

    [Fact]
    public void CalculateNewRating_WhenPlayerLowerRating_Wins_GainsMorePoints()
    {
        // Arrange
        var calculator = new ELOCalculator();
        var playerRating = 1300;
        var opponentRating = 1700;

        // Act
        var newRating = calculator.CalculateNewRating(playerRating, opponentRating, won: true);

        // Assert
        newRating.Should().Be(1329); // +29 for beating much stronger opponent (rounded)
    }

    [Fact]
    public void CalculateNewRating_WithDifficultyMultiplier_AppliesMultiplier()
    {
        // Arrange
        var calculator = new ELOCalculator();
        var playerRating = 1500;
        var opponentRating = 1500;
        double difficultyMultiplier = 2.0; // Expert difficulty

        // Act
        var newRating = calculator.CalculateNewRating(
            playerRating,
            opponentRating,
            won: true,
            difficultyMultiplier: difficultyMultiplier
        );

        // Assert
        newRating.Should().Be(1532); // 16 * 2 = +32
    }

    [Fact]
    public void CalculateNewRating_WhenLosingWithMultiplier_LosesMorePoints()
    {
        // Arrange
        var calculator = new ELOCalculator();
        var playerRating = 1500;
        var opponentRating = 1500;
        double difficultyMultiplier = 1.5; // Hard difficulty

        // Act
        var newRating = calculator.CalculateNewRating(
            playerRating,
            opponentRating,
            won: false,
            difficultyMultiplier: difficultyMultiplier
        );

        // Assert
        newRating.Should().Be(1476); // -16 * 1.5 = -24 (rounded)
    }

    [Fact]
    public void CalculateExpectedScore_HigherRatedPlayer_HasHigherExpectedScore()
    {
        // Arrange
        var calculator = new ELOCalculator();

        // Act
        var expectedScore = calculator.CalculateExpectedScore(1700, 1300);

        // Assert
        expectedScore.Should().BeGreaterThan(0.5);
        expectedScore.Should().BeLessThan(1.0);
    }

    [Fact]
    public void CalculateExpectedScore_EqualRatings_ReturnsPointFive()
    {
        // Arrange
        var calculator = new ELOCalculator();

        // Act
        var expectedScore = calculator.CalculateExpectedScore(1500, 1500);

        // Assert
        expectedScore.Should().BeApproximately(0.5, 0.001);
    }
}
