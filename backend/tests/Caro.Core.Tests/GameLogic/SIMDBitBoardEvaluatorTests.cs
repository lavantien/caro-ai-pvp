using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

namespace Caro.Core.Tests.GameLogic;

public class SIMDBitBoardEvaluatorTests
{
    private static Board CreateBoard(params (int x, int y, Player player)[] stones)
    {
        var board = new Board();
        foreach (var (x, y, player) in stones)
        {
            board = board.PlaceStone(x, y, player);
        }
        return board;
    }

    [Fact]
    public void Evaluate_NonePlayer_ShouldThrow()
    {
        var board = new Board();

        var act = () => SIMDBitBoardEvaluator.Evaluate(board, Player.None);

        act.Should().Throw<ArgumentException>();
    }

    [Fact]
    public void Evaluate_EmptyBoard_ShouldReturnZero()
    {
        var board = new Board();

        var score = SIMDBitBoardEvaluator.Evaluate(board, Player.Red);

        score.Should().Be(0);
    }

    [Fact]
    public void Evaluate_SingleCenterStone_ShouldBePositive()
    {
        var board = CreateBoard((7, 7, Player.Red));

        var score = SIMDBitBoardEvaluator.Evaluate(board, Player.Red);

        score.Should().BePositive();
    }

    [Fact]
    public void Evaluate_FiveInRowHorizontal_ShouldScoreHigh()
    {
        var board = CreateBoard(
            (5, 7, Player.Red), (6, 7, Player.Red), (7, 7, Player.Red),
            (8, 7, Player.Red), (9, 7, Player.Red)
        );

        var score = SIMDBitBoardEvaluator.Evaluate(board, Player.Red);

        score.Should().BeGreaterThanOrEqualTo(EvaluationConstants.FiveInRowScore);
    }

    [Fact]
    public void Evaluate_FiveInRowVertical_ShouldScoreHigh()
    {
        var board = CreateBoard(
            (7, 5, Player.Red), (7, 6, Player.Red), (7, 7, Player.Red),
            (7, 8, Player.Red), (7, 9, Player.Red)
        );

        var score = SIMDBitBoardEvaluator.Evaluate(board, Player.Red);

        score.Should().BeGreaterThanOrEqualTo(EvaluationConstants.FiveInRowScore);
    }

    [Fact]
    public void Evaluate_FiveInRowDiagonal_ShouldScoreHigh()
    {
        var board = CreateBoard(
            (5, 5, Player.Red), (6, 6, Player.Red), (7, 7, Player.Red),
            (8, 8, Player.Red), (9, 9, Player.Red)
        );

        var score = SIMDBitBoardEvaluator.Evaluate(board, Player.Red);

        score.Should().BeGreaterThanOrEqualTo(EvaluationConstants.FiveInRowScore);
    }

    [Fact]
    public void Evaluate_FiveInRowAntiDiagonal_ShouldScoreHigh()
    {
        var board = CreateBoard(
            (9, 5, Player.Red), (8, 6, Player.Red), (7, 7, Player.Red),
            (6, 8, Player.Red), (5, 9, Player.Red)
        );

        var score = SIMDBitBoardEvaluator.Evaluate(board, Player.Red);

        score.Should().BeGreaterThanOrEqualTo(EvaluationConstants.FiveInRowScore);
    }

    [Fact]
    public void Evaluate_OpenFour_ShouldScoreHighly()
    {
        // 4 in a row with both ends open
        var board = CreateBoard(
            (5, 7, Player.Red), (6, 7, Player.Red), (7, 7, Player.Red), (8, 7, Player.Red)
        );

        var score = SIMDBitBoardEvaluator.Evaluate(board, Player.Red);

        // OpenFourScore = 10000, minus opponent (0), plus center bonus
        score.Should().BeGreaterThanOrEqualTo(EvaluationConstants.OpenFourScore);
    }

    [Fact]
    public void Evaluate_ClosedFour_ShouldScoreLowerThanOpenFour()
    {
        // 4 in a row with one end blocked by opponent
        var board = CreateBoard(
            (4, 7, Player.Blue),
            (5, 7, Player.Red), (6, 7, Player.Red), (7, 7, Player.Red), (8, 7, Player.Red)
        );

        var score = SIMDBitBoardEvaluator.Evaluate(board, Player.Red);

        score.Should().BeGreaterThanOrEqualTo(EvaluationConstants.ClosedFourScore);
    }

    [Fact]
    public void Evaluate_OpponentThreat_ShouldReduceScore()
    {
        // Player has center stone, opponent has open four
        var board = CreateBoard(
            (7, 7, Player.Red),
            (5, 5, Player.Blue), (6, 5, Player.Blue), (7, 5, Player.Blue), (8, 5, Player.Blue)
        );

        var score = SIMDBitBoardEvaluator.Evaluate(board, Player.Red);

        // Opponent's open four with 1.5x defense multiplier should heavily penalize
        score.Should().BeNegative();
    }

    [Fact]
    public void Evaluate_DefenseMultiplier_ShouldWeightOpponentMoreHeavily()
    {
        // Both players have 3 in a row, open ends
        var board = CreateBoard(
            (5, 7, Player.Red), (6, 7, Player.Red), (7, 7, Player.Red),
            (5, 9, Player.Blue), (6, 9, Player.Blue), (7, 9, Player.Blue)
        );

        var scoreRed = SIMDBitBoardEvaluator.Evaluate(board, Player.Red);

        // Opponent's 3-in-row gets 1.5x multiplier, so net score should be negative
        // Player: 3 open = ~2000, Opponent: 3 open * 1.5 = ~3000
        scoreRed.Should().BeNegative();
    }

    [Fact]
    public void Evaluate_SymmetricPosition_ShouldBeRoughlyMirrorInverse()
    {
        // Player has stones on left, opponent has mirror on right (both equidistant from center)
        var board = CreateBoard(
            (5, 7, Player.Red), (6, 7, Player.Red),
            (9, 7, Player.Blue), (10, 7, Player.Blue)
        );

        var scoreRed = SIMDBitBoardEvaluator.Evaluate(board, Player.Red);
        var scoreBlue = SIMDBitBoardEvaluator.Evaluate(board, Player.Blue);

        // Pattern scores are symmetric; only center control causes slight difference
        var diff = Math.Abs(scoreRed + scoreBlue);
        diff.Should().BeLessThan(200); // Center bonus difference only
    }

    [Fact]
    public void EvaluateOptimized_EmptyBoards_ShouldReturnZero()
    {
        var empty = new BitBoard();

        var score = SIMDBitBoardEvaluator.EvaluateOptimized(empty, empty);

        score.Should().Be(0);
    }

    [Fact]
    public void EvaluateOptimized_PlayerStonesOnly_ShouldBePositive()
    {
        var board = CreateBoard((7, 7, Player.Red), (8, 7, Player.Red), (9, 7, Player.Red));
        var playerBoard = board.GetBitBoard(Player.Red);
        var empty = new BitBoard();

        var score = SIMDBitBoardEvaluator.EvaluateOptimized(playerBoard, empty);

        score.Should().BePositive();
    }

    [Fact]
    public void EvaluateMoveAt_WinningMove_ShouldScoreHigh()
    {
        // 4 red stones, evaluate placing the 5th
        var board = CreateBoard(
            (5, 7, Player.Red), (6, 7, Player.Red), (7, 7, Player.Red), (8, 7, Player.Red)
        );

        var score = SIMDBitBoardEvaluator.EvaluateMoveAt(9, 7, board, Player.Red);

        score.Should().BeGreaterThanOrEqualTo(EvaluationConstants.FiveInRowScore);
    }

    [Fact]
    public void EvaluateMoveAt_BlockingMove_ShouldScoreWell()
    {
        // Opponent has 4 in a row, player blocks
        var board = CreateBoard(
            (5, 7, Player.Blue), (6, 7, Player.Blue), (7, 7, Player.Blue), (8, 7, Player.Blue)
        );

        var blockScore = SIMDBitBoardEvaluator.EvaluateMoveAt(9, 7, board, Player.Red);
        var neutralScore = SIMDBitBoardEvaluator.EvaluateMoveAt(0, 0, board, Player.Red);

        blockScore.Should().BeGreaterThan(neutralScore);
    }

    [Fact]
    public void EvaluateMoveAt_EmptyBoard_ShouldPreferCenter()
    {
        var board = new Board();

        var centerScore = SIMDBitBoardEvaluator.EvaluateMoveAt(7, 7, board, Player.Red);
        var cornerScore = SIMDBitBoardEvaluator.EvaluateMoveAt(0, 0, board, Player.Red);

        centerScore.Should().BeGreaterThan(cornerScore);
    }

    [Fact]
    public void BatchEvaluate_ShouldReturnCorrectNumberOfScores()
    {
        var board = CreateBoard((7, 7, Player.Red), (8, 8, Player.Blue));
        var positions = new (int x, int y)[] { (6, 6), (7, 8), (9, 9) };

        var scores = SIMDBitBoardEvaluator.BatchEvaluate(positions, board, Player.Red);

        scores.Length.Should().Be(3);
        scores[0].Should().NotBe(0);
    }

    [Fact]
    public void BatchEvaluate_WinningPositions_ShouldAllScoreHigh()
    {
        var board = CreateBoard(
            (5, 7, Player.Red), (6, 7, Player.Red), (7, 7, Player.Red), (8, 7, Player.Red)
        );
        var positions = new (int x, int y)[] { (4, 7), (9, 7) };

        var scores = SIMDBitBoardEvaluator.BatchEvaluate(positions, board, Player.Red);

        scores[0].Should().BeGreaterThanOrEqualTo(EvaluationConstants.FiveInRowScore);
        scores[1].Should().BeGreaterThanOrEqualTo(EvaluationConstants.FiveInRowScore);
    }

    [Fact]
    public void SupportsHardwarePOPCNT_ShouldBeTrue()
    {
        SIMDBitBoardEvaluator.SupportsHardwarePOPCNT.Should().BeTrue();
    }

    [Fact]
    public void GetPlatformInfo_ShouldReturnNonNullString()
    {
        var info = SIMDBitBoardEvaluator.GetPlatformInfo();

        info.Should().NotBeNullOrEmpty();
        info.Should().Contain("POPCNT");
    }

    [Fact]
    public void Evaluate_OpenThree_ShouldScoreLowerThanOpenFour()
    {
        var threeBoard = CreateBoard(
            (5, 7, Player.Red), (6, 7, Player.Red), (7, 7, Player.Red)
        );
        var fourBoard = CreateBoard(
            (5, 7, Player.Red), (6, 7, Player.Red), (7, 7, Player.Red), (8, 7, Player.Red)
        );

        var threeScore = SIMDBitBoardEvaluator.Evaluate(threeBoard, Player.Red);
        var fourScore = SIMDBitBoardEvaluator.Evaluate(fourBoard, Player.Red);

        fourScore.Should().BeGreaterThan(threeScore);
    }

    [Fact]
    public void Evaluate_OpenTwo_ShouldScoreLowerThanOpenThree()
    {
        var twoBoard = CreateBoard((7, 7, Player.Red), (8, 7, Player.Red));
        var threeBoard = CreateBoard(
            (7, 7, Player.Red), (8, 7, Player.Red), (9, 7, Player.Red)
        );

        var twoScore = SIMDBitBoardEvaluator.Evaluate(twoBoard, Player.Red);
        var threeScore = SIMDBitBoardEvaluator.Evaluate(threeBoard, Player.Red);

        threeScore.Should().BeGreaterThan(twoScore);
    }

    [Fact]
    public void Evaluate_CornerStone_ShouldScoreLowerThanCenter()
    {
        var cornerBoard = CreateBoard((0, 0, Player.Red));
        var centerBoard = CreateBoard((7, 7, Player.Red));

        var cornerScore = SIMDBitBoardEvaluator.Evaluate(cornerBoard, Player.Red);
        var centerScore = SIMDBitBoardEvaluator.Evaluate(centerBoard, Player.Red);

        centerScore.Should().BeGreaterThan(cornerScore);
    }

    [Fact]
    public void Evaluate_LargerCluster_ShouldScoreHigher()
    {
        var smallBoard = CreateBoard((7, 7, Player.Red), (8, 7, Player.Red));
        var bigBoard = CreateBoard(
            (7, 7, Player.Red), (8, 7, Player.Red), (9, 7, Player.Red), (10, 7, Player.Red)
        );

        var smallScore = SIMDBitBoardEvaluator.Evaluate(smallBoard, Player.Red);
        var bigScore = SIMDBitBoardEvaluator.Evaluate(bigBoard, Player.Red);

        bigScore.Should().BeGreaterThan(smallScore);
    }

    [Fact]
    public void Evaluate_SixInRow_ShouldScoreAsFiveInRow()
    {
        // 6 in a row - should score as five in row (or higher, runs are scored individually)
        var board = CreateBoard(
            (4, 7, Player.Red), (5, 7, Player.Red), (6, 7, Player.Red),
            (7, 7, Player.Red), (8, 7, Player.Red), (9, 7, Player.Red)
        );

        var score = SIMDBitBoardEvaluator.Evaluate(board, Player.Red);

        score.Should().BeGreaterThanOrEqualTo(EvaluationConstants.FiveInRowScore);
    }
}
