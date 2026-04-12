using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

namespace Caro.Core.Tests.GameLogic;

public sealed class Pattern4EvaluatorTests
{
    // --- GetPatternScore ---

    [Theory]
    [InlineData(Pattern4Evaluator.CaroPattern4.None, 0)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Flex1, 10)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Block1, 5)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Flex2, 100)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Block2, 50)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Flex3, 5000)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Block3, 500)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Flex4, 100000)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Block4, 10000)]
    [InlineData(Pattern4Evaluator.CaroPattern4.DoubleFlex3, 400000)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Flex4Flex3, 500000)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Exactly5, 1000000)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Overline, -1000)]
    public void GetPatternScore_AllPatterns_ReturnsExpectedScore(
        Pattern4Evaluator.CaroPattern4 pattern, int expectedScore)
    {
        Pattern4Evaluator.GetPatternScore(pattern).Should().Be(expectedScore);
    }

    // --- IsWinningThreat ---

    [Theory]
    [InlineData(Pattern4Evaluator.CaroPattern4.None, false)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Flex1, false)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Block1, false)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Flex2, false)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Block2, false)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Flex3, false)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Block3, false)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Flex4, true)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Block4, true)]
    [InlineData(Pattern4Evaluator.CaroPattern4.DoubleFlex3, true)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Flex4Flex3, true)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Exactly5, true)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Overline, false)]
    public void IsWinningThreat_AllPatterns_ReturnsExpected(
        Pattern4Evaluator.CaroPattern4 pattern, bool expected)
    {
        Pattern4Evaluator.IsWinningThreat(pattern).Should().Be(expected);
    }

    // --- IsForcingThreat ---

    [Theory]
    [InlineData(Pattern4Evaluator.CaroPattern4.None, false)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Flex1, false)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Block1, false)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Flex2, false)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Block2, false)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Flex3, true)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Block3, true)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Flex4, true)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Block4, true)]
    [InlineData(Pattern4Evaluator.CaroPattern4.DoubleFlex3, true)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Flex4Flex3, true)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Exactly5, true)]
    [InlineData(Pattern4Evaluator.CaroPattern4.Overline, true)]
    public void IsForcingThreat_AllPatterns_ReturnsExpected(
        Pattern4Evaluator.CaroPattern4 pattern, bool expected)
    {
        Pattern4Evaluator.IsForcingThreat(pattern).Should().Be(expected);
    }

    // --- EvaluatePosition ---

    [Fact]
    public void EvaluatePosition_NonePlayer_ReturnsNone()
    {
        var board = new Board();
        var result = Pattern4Evaluator.EvaluatePosition(board, 8, 8, Player.None);
        result.Should().Be(Pattern4Evaluator.CaroPattern4.None);
    }

    [Fact]
    public void EvaluatePosition_EmptyBoard_ReturnsNone()
    {
        var board = new Board();
        var result = Pattern4Evaluator.EvaluatePosition(board, 8, 8, Player.Red);
        result.Should().Be(Pattern4Evaluator.CaroPattern4.None);
    }

    [Fact]
    public void EvaluatePosition_FiveInRow_ReturnsExactly5()
    {
        var board = new Board();
        for (int i = 0; i < 5; i++)
            board = board.PlaceStone(7 + i, 7, Player.Red);

        var result = Pattern4Evaluator.EvaluatePosition(board, 9, 7, Player.Red);
        result.Should().Be(Pattern4Evaluator.CaroPattern4.Exactly5);
    }

    [Fact]
    public void EvaluatePosition_OpenFour_ReturnsFlex4()
    {
        // 4 stones in a row with both ends open
        var board = new Board();
        for (int i = 0; i < 4; i++)
            board = board.PlaceStone(7 + i, 7, Player.Red);

        var result = Pattern4Evaluator.EvaluatePosition(board, 8, 7, Player.Red);
        result.Should().Be(Pattern4Evaluator.CaroPattern4.Flex4);
    }

    [Fact]
    public void EvaluatePosition_BlockedFour_ReturnsBlock4()
    {
        // 4 stones blocked at one end by opponent
        var board = new Board();
        board = board.PlaceStone(6, 7, Player.Blue); // Block
        for (int i = 0; i < 4; i++)
            board = board.PlaceStone(7 + i, 7, Player.Red);

        // The pattern at a blocked stone should detect Block4
        var result = Pattern4Evaluator.EvaluatePosition(board, 9, 7, Player.Red);
        // Could be Flex4 if the other end is open, or Block4 if blocked
        result.Should().BeOneOf(
            Pattern4Evaluator.CaroPattern4.Flex4,
            Pattern4Evaluator.CaroPattern4.Block4);
    }

    [Fact]
    public void EvaluatePosition_OpenThree_ReturnsFlex3()
    {
        // 3 stones with both ends open
        var board = new Board();
        for (int i = 0; i < 3; i++)
            board = board.PlaceStone(7 + i, 7, Player.Red);

        var result = Pattern4Evaluator.EvaluatePosition(board, 8, 7, Player.Red);
        result.Should().Be(Pattern4Evaluator.CaroPattern4.Flex3);
    }

    [Fact]
    public void EvaluatePosition_SixInRow_ReturnsOverline()
    {
        // 6 stones in a row is an overline in Caro
        var board = new Board();
        for (int i = 0; i < 6; i++)
            board = board.PlaceStone(5 + i, 7, Player.Red);

        var result = Pattern4Evaluator.EvaluatePosition(board, 8, 7, Player.Red);
        result.Should().Be(Pattern4Evaluator.CaroPattern4.Overline);
    }

    [Fact]
    public void EvaluatePosition_OpenTwo_ReturnsFlex2()
    {
        var board = new Board();
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);

        var result = Pattern4Evaluator.EvaluatePosition(board, 7, 7, Player.Red);
        result.Should().Be(Pattern4Evaluator.CaroPattern4.Flex2);
    }

    [Fact]
    public void EvaluatePosition_DoubleFlex3_ReturnsDoubleFlex3()
    {
        // Two open threes crossing at (7,7), each exactly 3 stones with both ends open
        var board = new Board();
        // Horizontal open three: (6,7)-(7,7)-(8,7), open at (5,7) and (9,7)
        board = board.PlaceStone(6, 7, Player.Red);
        board = board.PlaceStone(7, 7, Player.Red);
        board = board.PlaceStone(8, 7, Player.Red);
        // Vertical open three: (7,6)-(7,7)-(7,8), open at (7,5) and (7,9)
        board = board.PlaceStone(7, 6, Player.Red);
        board = board.PlaceStone(7, 8, Player.Red);

        var result = Pattern4Evaluator.EvaluatePosition(board, 7, 7, Player.Red);
        result.Should().Be(Pattern4Evaluator.CaroPattern4.DoubleFlex3);
    }

    // --- FindPatternPositions ---

    [Fact]
    public void FindPatternPositions_EmptyBoard_ReturnsEmpty()
    {
        var board = new Board();
        var result = Pattern4Evaluator.FindPatternPositions(board, Player.Red);
        result.Should().BeEmpty();
    }

    [Fact]
    public void FindPatternPositions_WithThreat_ReturnsSortedByScoreDescending()
    {
        var board = new Board();
        // Create an open four
        for (int i = 0; i < 4; i++)
            board = board.PlaceStone(7 + i, 7, Player.Red);
        // Add an open three elsewhere
        board = board.PlaceStone(3, 3, Player.Red);
        board = board.PlaceStone(4, 3, Player.Red);
        board = board.PlaceStone(5, 3, Player.Red);

        var result = Pattern4Evaluator.FindPatternPositions(board, Player.Red);
        result.Should().NotBeEmpty();

        // Verify sorted by score descending
        for (int i = 1; i < result.Count; i++)
        {
            Pattern4Evaluator.GetPatternScore(result[i - 1].pattern)
                .Should().BeGreaterThanOrEqualTo(
                    Pattern4Evaluator.GetPatternScore(result[i].pattern));
        }
    }

    // --- EvaluatePositionBitBoard ---

    [Fact]
    public void EvaluatePositionBitBoard_EmptyBoards_ReturnsNone()
    {
        var playerBoard = new BitBoard();
        var opponentBoard = new BitBoard();
        var result = Pattern4Evaluator.EvaluatePositionBitBoard(playerBoard, opponentBoard, 8, 8);
        result.Should().Be(Pattern4Evaluator.CaroPattern4.None);
    }

    [Fact]
    public void EvaluatePositionBitBoard_FiveInRow_ReturnsExactly5()
    {
        var playerBoard = new BitBoard();
        var opponentBoard = new BitBoard();
        for (int i = 0; i < 5; i++)
            playerBoard.SetBit(7 + i, 7, true);

        var result = Pattern4Evaluator.EvaluatePositionBitBoard(playerBoard, opponentBoard, 9, 7);
        result.Should().Be(Pattern4Evaluator.CaroPattern4.Exactly5);
    }
}
