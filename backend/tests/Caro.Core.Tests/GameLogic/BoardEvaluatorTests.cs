using Xunit;
using FluentAssertions;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

namespace Caro.Core.Tests.GameLogic;

public class BoardEvaluatorTests
{
    [Fact]
    public void Evaluate_EmptyBoard_ReturnsZero()
    {
        // Arrange
        var evaluator = new BoardEvaluator();
        var board = new Board();

        // Act
        var score = evaluator.Evaluate(board, Player.Red);

        // Assert
        score.Should().Be(0);
    }

    [Fact]
    public void Evaluate_RedHas4InRow_ReturnsHighPositiveScore()
    {
        // Arrange
        var evaluator = new BoardEvaluator();
        var board = new Board();

        // Place 4 red stones in a row (not blocked)
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(9, 7, Player.Red);
        board.PlaceStone(10, 7, Player.Red);

        // Act
        var score = evaluator.Evaluate(board, Player.Red);

        // Assert
        score.Should().BeGreaterThan(1000); // Very high score for almost winning
    }

    [Fact]
    public void Evaluate_BlueHas4InRow_ReturnsHighNegativeScore_ForRedPlayer()
    {
        // Arrange
        var evaluator = new BoardEvaluator();
        var board = new Board();

        // Place 4 blue stones in a row
        board.PlaceStone(7, 7, Player.Blue);
        board.PlaceStone(8, 7, Player.Blue);
        board.PlaceStone(9, 7, Player.Blue);
        board.PlaceStone(10, 7, Player.Blue);

        // Act
        var score = evaluator.Evaluate(board, Player.Red);

        // Assert
        score.Should().BeLessThan(-1000); // Very low score (opponent almost winning)
    }

    [Fact]
    public void Evaluate_RedHas3InRowOpen_ReturnsPositiveScore()
    {
        // Arrange
        var evaluator = new BoardEvaluator();
        var board = new Board();

        // Place 3 red stones in a row, ends open
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(9, 7, Player.Red);

        // Act
        var score = evaluator.Evaluate(board, Player.Red);

        // Assert
        score.Should().BeGreaterThan(1000); // Positive for 3 in a row
        score.Should().BeLessThan(10000); // But less than 4 in a row
    }

    [Fact]
    public void Evaluate_CenterCellHigherValueThanCorner()
    {
        // Arrange
        var evaluator = new BoardEvaluator();
        var board1 = new Board();
        var board2 = new Board();

        // Place red at center (7,7)
        board1.PlaceStone(7, 7, Player.Red);

        // Place red at corner
        board2.PlaceStone(0, 7, Player.Red);

        // Act
        var centerScore = evaluator.Evaluate(board1, Player.Red);
        var cornerScore = evaluator.Evaluate(board2, Player.Red);

        // Assert
        centerScore.Should().BeGreaterThan(cornerScore);
    }

    [Fact]
    public void Evaluate_BlockedSequence_ReturnsLowerScore()
    {
        // Arrange
        var evaluator = new BoardEvaluator();
        var board = new Board();

        // 3 in a row with both ends blocked
        board.PlaceStone(6, 7, Player.Blue);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(9, 7, Player.Red);
        board.PlaceStone(10, 7, Player.Blue);

        // Act
        var score = evaluator.Evaluate(board, Player.Red);

        // Assert
        score.Should().BeLessThan(500); // Much lower than open-ended 3-in-row (~1000+)
    }

    [Fact]
    public void Evaluate_OpenEnded3InRow_HigherThanBlocked()
    {
        // Arrange
        var evaluator = new BoardEvaluator();
        var board = new Board();

        // Open-ended 3 in a row
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(8, 7, Player.Red);
        board.PlaceStone(9, 7, Player.Red);

        var openScore = evaluator.Evaluate(board, Player.Red);

        // Block one end
        board.PlaceStone(6, 7, Player.Blue);

        var blockedScore = evaluator.Evaluate(board, Player.Red);

        // Assert
        openScore.Should().BeGreaterThan(blockedScore);
    }
}
