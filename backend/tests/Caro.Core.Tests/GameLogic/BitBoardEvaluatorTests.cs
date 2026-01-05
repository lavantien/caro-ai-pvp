using Xunit;
using FluentAssertions;
using Caro.Core.Entities;
using Caro.Core.GameLogic;

namespace Caro.Core.Tests.GameLogic;

public class BitBoardEvaluatorTests
{
    [Fact]
    public void EvaluateLine_HorizontalFiveInRow_ReturnsMaxScore()
    {
        // Arrange
        var board = new Board();
        board.PlaceStone(5, 7, Player.Red);
        board.PlaceStone(6, 7, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(9, 7, Player.Red);

        // Act
        var score = BitBoardEvaluator.Evaluate(board, Player.Red);

        // Assert
        score.Should().BeGreaterThan(50000, "5-in-row should have maximum score");
    }

    [Fact]
    public void EvaluateLine_VerticalFiveInRow_ReturnsMaxScore()
    {
        // Arrange
        var board = new Board();
        board.PlaceStone(7, 5, Player.Red);
        board.PlaceStone(7, 6, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Red);
        board.PlaceStone(7, 9, Player.Red);

        // Act
        var score = BitBoardEvaluator.Evaluate(board, Player.Red);

        // Assert
        score.Should().BeGreaterThan(50000);
    }

    [Fact]
    public void EvaluateLine_DiagonalFiveInRow_ReturnsMaxScore()
    {
        // Arrange
        var board = new Board();
        board.PlaceStone(5, 5, Player.Red);
        board.PlaceStone(6, 6, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(8, 8, Player.Red);
        board.PlaceStone(9, 9, Player.Red);

        // Act
        var score = BitBoardEvaluator.Evaluate(board, Player.Red);

        // Assert
        score.Should().BeGreaterThan(50000);
    }

    [Fact]
    public void EvaluateLine_AntiDiagonalFiveInRow_ReturnsMaxScore()
    {
        // Arrange
        var board = new Board();
        board.PlaceStone(9, 5, Player.Red);
        board.PlaceStone(8, 6, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(6, 8, Player.Red);
        board.PlaceStone(5, 9, Player.Red);

        // Act
        var score = BitBoardEvaluator.Evaluate(board, Player.Red);

        // Assert
        score.Should().BeGreaterThan(50000);
    }

    [Fact]
    public void EvaluateLine_OpenFour_HighScore()
    {
        // Arrange
        var board = new Board();
        // _XXXX_ pattern
        board.PlaceStone(6, 7, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(9, 7, Player.Red);

        // Act
        var score = BitBoardEvaluator.Evaluate(board, Player.Red);

        // Assert
        score.Should().BeGreaterThan(10000, "Open 4 should have very high score");
    }

    [Fact]
    public void EvaluateLine_OpenThree_ModerateScore()
    {
        // Arrange
        var board = new Board();
        // _XXX_ pattern
        board.PlaceStone(6, 7, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(8, 7, Player.Red);

        // Act
        var score = BitBoardEvaluator.Evaluate(board, Player.Red);

        // Assert
        score.Should().BeGreaterThan(1000, "Open 3 should have good score");
        score.Should().BeLessThan(10000, "Open 3 should score less than open 4");
    }

    [Fact]
    public void EvaluateLine_BlockedFour_LowerScore()
    {
        // Arrange
        var board = new Board();
        // OXXXXO pattern (blocked on BOTH sides - truly dead)
        board.PlaceStone(5, 7, Player.Blue);
        board.PlaceStone(6, 7, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(9, 7, Player.Red);
        board.PlaceStone(10, 7, Player.Blue);

        // Act
        var score = BitBoardEvaluator.Evaluate(board, Player.Red);

        // Assert
        score.Should().BeLessThan(10000, "Completely blocked 4 should score less than open 4");
        score.Should().BeGreaterThanOrEqualTo(1000, "Should still score some points");
    }

    [Fact]
    public void EvaluateLine_CenterControl_GetsBonus()
    {
        // Arrange
        var board1 = new Board();
        board1.PlaceStone(7, 7, Player.Red);  // Center

        var board2 = new Board();
        board2.PlaceStone(0, 0, Player.Red);  // Corner

        // Act
        var centerScore = BitBoardEvaluator.Evaluate(board1, Player.Red);
        var cornerScore = BitBoardEvaluator.Evaluate(board2, Player.Red);

        // Assert
        centerScore.Should().BeGreaterThan(cornerScore, "Center position should score higher");
    }

    [Fact]
    public void EvaluateLine_BothPlayers_ReturnsRelativeScore()
    {
        // Arrange
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Red);
        board.PlaceStone(7, 9, Player.Red);

        board.PlaceStone(8, 7, Player.Blue);
        board.PlaceStone(8, 8, Player.Blue);
        board.PlaceStone(8, 9, Player.Blue);

        // Act
        var redScore = BitBoardEvaluator.Evaluate(board, Player.Red);
        var blueScore = BitBoardEvaluator.Evaluate(board, Player.Blue);

        // Assert
        // With asymmetric scoring (2.2x defense multiplier), both scores are negative
        // because opponent's threat (3-in-row) is weighted higher than own threat
        redScore.Should().BeNegative("Red sees Blue's 3-in-row as 2.2x threat vs own 1x, so net negative");
        blueScore.Should().BeNegative("Blue sees Red's 3-in-row as 2.2x threat vs own 1x, so net negative");

        // Red score and Blue score should be roughly equal (symmetric position)
        var diff = Math.Abs(redScore - blueScore);
        diff.Should().BeLessThan(100, "Symmetric positions should have similar scores");
    }

    [Fact]
    public void CountConsecutive_EmptyBoard_ReturnsZero()
    {
        // Arrange
        var board = new Board();
        var bitBoard = board.GetBitBoard(Player.Red);

        // Act
        var count = BitBoardEvaluator.CountConsecutive(bitBoard, 7, 7, 1, 0);

        // Assert
        count.Should().Be(0);
    }

    [Fact]
    public void CountConsecutive_HorizontalThree_ReturnsThree()
    {
        // Arrange
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(9, 7, Player.Red);
        var bitBoard = board.GetBitBoard(Player.Red);

        // Act
        var count = BitBoardEvaluator.CountConsecutive(bitBoard, 7, 7, 1, 0);

        // Assert
        count.Should().Be(3);
    }

    [Fact]
    public void CountConsecutive_VerticalTwo_ReturnsTwo()
    {
        // Arrange
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Red);
        var bitBoard = board.GetBitBoard(Player.Red);

        // Act
        var count = BitBoardEvaluator.CountConsecutive(bitBoard, 7, 7, 0, 1);

        // Assert
        count.Should().Be(2);
    }

    [Fact]
    public void CountConsecutive_DiagonalFour_ReturnsFour()
    {
        // Arrange
        var board = new Board();
        board.PlaceStone(5, 5, Player.Red);
        board.PlaceStone(6, 6, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(8, 8, Player.Red);
        var bitBoard = board.GetBitBoard(Player.Red);

        // Act
        var count = BitBoardEvaluator.CountConsecutive(bitBoard, 5, 5, 1, 1);

        // Assert
        count.Should().Be(4);
    }

    [Fact]
    public void CountConsecutive_StoppedByOpponent_ReturnsOnlyConsecutive()
    {
        // Arrange
        var board = new Board();
        board.PlaceStone(5, 7, Player.Red);
        board.PlaceStone(6, 7, Player.Red);
        board.PlaceStone(7, 7, Player.Blue);  // Blocks
        board.PlaceStone(8, 7, Player.Red);
        var bitBoard = board.GetBitBoard(Player.Red);

        // Act
        var count = BitBoardEvaluator.CountConsecutive(bitBoard, 5, 7, 1, 0);

        // Assert
        count.Should().Be(2, "Should count only consecutive Red stones, not past Blue");
    }

    [Fact]
    public void DetectPattern_StraightFour_ReturnsTrue()
    {
        // Arrange
        var board = new Board();
        board.PlaceStone(6, 7, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(9, 7, Player.Red);
        var bitBoard = board.GetBitBoard(Player.Red);
        var occupied = board.GetBitBoard(Player.Red) | board.GetBitBoard(Player.Blue);

        // Act
        var found = BitBoardEvaluator.DetectPattern(bitBoard, occupied, ThreatType.StraightFour, out var positions);

        // Assert
        found.Should().BeTrue();
        positions.Should().NotBeEmpty();
    }

    [Fact]
    public void DetectPattern_StraightThree_ReturnsTrue()
    {
        // Arrange
        var board = new Board();
        board.PlaceStone(6, 7, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(8, 7, Player.Red);
        var bitBoard = board.GetBitBoard(Player.Red);
        var occupied = board.GetBitBoard(Player.Red) | board.GetBitBoard(Player.Blue);

        // Act
        var found = BitBoardEvaluator.DetectPattern(bitBoard, occupied, ThreatType.StraightThree, out var positions);

        // Assert
        found.Should().BeTrue();
    }

    [Fact]
    public void EvaluateBitBoard_PatternBasedEvaluation_Works()
    {
        // Arrange
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(9, 7, Player.Red);

        var redBoard = board.GetBitBoard(Player.Red);
        var blueBoard = board.GetBitBoard(Player.Blue);

        // Act
        var score = BitBoardEvaluator.EvaluateBitBoard(redBoard, blueBoard);

        // Assert
        score.Should().BeGreaterThan(0, "3-in-row should have positive score");
    }

    [Fact]
    public void ShiftOperations_PatternDetection_WorksCorrectly()
    {
        // Arrange
        var board = new Board();
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(9, 7, Player.Red);

        var bitBoard = board.GetBitBoard(Player.Red);

        // Act - Shift right should detect potential 4-in-row
        var shifted = bitBoard.ShiftRight();
        var combined = bitBoard | shifted;

        // Assert
        combined.CountBits().Should().BeGreaterThan(bitBoard.CountBits(),
            "Combined board should have more bits set");
    }

    [Fact]
    public void DetectThreats_ComplexPosition_FindsAllThreats()
    {
        // Arrange
        var board = new Board();
        // Create multiple threats
        board.PlaceStone(5, 7, Player.Red);
        board.PlaceStone(6, 7, Player.Red);
        board.PlaceStone(7, 7, Player.Red);

        board.PlaceStone(7, 5, Player.Red);
        board.PlaceStone(7, 6, Player.Red);
        board.PlaceStone(7, 8, Player.Red);

        var redBoard = board.GetBitBoard(Player.Red);
        var occupied = board.GetBitBoard(Player.Red) | board.GetBitBoard(Player.Blue);

        // Act
        var threats = BitBoardEvaluator.DetectAllThreats(redBoard, occupied);

        // Assert
        threats.Should().NotBeEmpty();
    }

    [Fact]
    public void IsSandwiched_SandwichedFive_ReturnsTrue()
    {
        // Arrange
        var board = new Board();
        // OXXXXXO pattern - sandwiched
        board.PlaceStone(5, 7, Player.Blue);
        board.PlaceStone(6, 7, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(9, 7, Player.Red);
        board.PlaceStone(10, 7, Player.Red);
        board.PlaceStone(11, 7, Player.Blue);

        var redBoard = board.GetBitBoard(Player.Red);
        var occupied = board.GetBitBoard(Player.Red) | board.GetBitBoard(Player.Blue);

        // Act
        var isSandwiched = BitBoardEvaluator.IsSandwichedFive(redBoard, occupied, 6, 7, 1, 0);

        // Assert
        isSandwiched.Should().BeTrue("OXXXXXO is sandwiched and should not count as a win");
    }

    [Fact]
    public void IsOverline_SixInRow_ReturnsTrue()
    {
        // Arrange
        var board = new Board();
        // XXXXXX pattern - overline
        board.PlaceStone(6, 7, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(9, 7, Player.Red);
        board.PlaceStone(10, 7, Player.Red);
        board.PlaceStone(11, 7, Player.Red);

        var redBoard = board.GetBitBoard(Player.Red);

        // Act
        var count = BitBoardEvaluator.CountConsecutiveBoth(redBoard, 6, 7, 1, 0);

        // Assert
        count.Should().Be(6, "Should detect 6 consecutive stones");
    }
}
