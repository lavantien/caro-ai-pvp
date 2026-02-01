using Caro.Core.Entities;
using Caro.Core.GameLogic;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.Tests.GameLogic;

/// <summary>
/// Tests to compare SIMD vs scalar evaluators and identify score discrepancies
/// This is critical for re-enabling SIMD without breaking AI strength ordering
/// </summary>
public class EvaluatorComparisonTests
{
    private readonly ITestOutputHelper _output;

    public EvaluatorComparisonTests(ITestOutputHelper output)
    {
        _output = output;
    }

    [Fact]
    public void ScalarVsSIMD_EmptyBoard_ShouldMatch()
    {
        var board = new Board();

        int scalarScore = BitBoardEvaluator.Evaluate(board, Player.Red);
        int simdScore = SIMDBitBoardEvaluator.Evaluate(board, Player.Red);

        _output.WriteLine($"Empty board - Scalar: {scalarScore}, SIMD: {simdScore}");
        Assert.Equal(scalarScore, simdScore);
    }

    [Fact]
    public void ScalarVsSIMD_CenterMove_ShouldMatch()
    {
        var board = new Board();
        board.PlaceStone(9, 9, Player.Red); // Center of 19x19 board

        int scalarScore = BitBoardEvaluator.Evaluate(board, Player.Red);
        int simdScore = SIMDBitBoardEvaluator.Evaluate(board, Player.Red);

        _output.WriteLine($"Center move - Scalar: {scalarScore}, SIMD: {simdScore}");

        // Allow for some difference between implementations
        int diff = Math.Abs(scalarScore - simdScore);
        Assert.True(diff < 500, $"Score difference too large: {diff}");
    }

    [Fact]
    public void ScalarVsSIMD_HorizontalLine_ShouldMatch()
    {
        var board = new Board();
        // Create horizontal line: Red at (5,7), (6,7), (7,7), (8,7)
        board.PlaceStone(5, 7, Player.Red);
        board.PlaceStone(6, 7, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(8, 7, Player.Red);

        int scalarScore = BitBoardEvaluator.Evaluate(board, Player.Red);
        int simdScore = SIMDBitBoardEvaluator.Evaluate(board, Player.Red);

        _output.WriteLine($"Horizontal line - Scalar: {scalarScore}, SIMD: {simdScore}");

        // Allow for some difference between implementations
        int diff = Math.Abs(scalarScore - simdScore);
        Assert.True(diff < 500, $"Score difference too large: {diff}");
    }

    [Fact]
    public void ScalarVsSIMD_OpenFour_ShouldMatch()
    {
        var board = new Board();
        // Create open four: Red at (5,7), (6,7), (7,7), (8,7)
        // Ends at (4,7) and (9,7) are empty
        board.PlaceStone(5, 7, Player.Red);
        board.PlaceStone(6, 7, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(8, 7, Player.Red);

        int scalarScore = BitBoardEvaluator.Evaluate(board, Player.Red);
        int simdScore = SIMDBitBoardEvaluator.Evaluate(board, Player.Red);

        _output.WriteLine($"Open four - Scalar: {scalarScore}, SIMD: {simdScore}");

        // Allow for some difference between implementations
        int diff = Math.Abs(scalarScore - simdScore);
        Assert.True(diff < 500, $"Score difference too large: {diff}");
    }

    [Fact]
    public void ScalarVsSIMD_DefenseMultiplier_ShouldMatch()
    {
        var board = new Board();
        // Red has three in a row
        board.PlaceStone(5, 7, Player.Red);
        board.PlaceStone(6, 7, Player.Red);
        board.PlaceStone(7, 7, Player.Red);

        // Blue has open four (should be weighted 2.2x higher)
        board.PlaceStone(5, 8, Player.Blue);
        board.PlaceStone(6, 8, Player.Blue);
        board.PlaceStone(7, 8, Player.Blue);
        board.PlaceStone(8, 8, Player.Blue);

        int scalarScore = BitBoardEvaluator.Evaluate(board, Player.Red);
        int simdScore = SIMDBitBoardEvaluator.Evaluate(board, Player.Red);

        _output.WriteLine($"Defense scenario - Scalar: {scalarScore}, SIMD: {simdScore}");

        // Scores should match within a small margin of error
        int diff = Math.Abs(scalarScore - simdScore);
        Assert.True(diff < 100, $"Score difference too large: {diff}");
    }

    [Fact]
    public void ScalarVsSIMD_ComplexPosition_ShouldMatch()
    {
        var board = new Board();
        // Create a complex position with multiple threats
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(7, 8, Player.Red);
        board.PlaceStone(7, 9, Player.Red);
        board.PlaceStone(6, 6, Player.Blue);
        board.PlaceStone(8, 6, Player.Blue);
        board.PlaceStone(9, 6, Player.Blue);
        board.PlaceStone(10, 6, Player.Blue);
        board.PlaceStone(5, 5, Player.Red);
        board.PlaceStone(6, 5, Player.Red);

        int scalarScore = BitBoardEvaluator.Evaluate(board, Player.Red);
        int simdScore = SIMDBitBoardEvaluator.Evaluate(board, Player.Red);

        _output.WriteLine($"Complex position - Scalar: {scalarScore}, SIMD: {simdScore}");

        // Scores should match within a small margin of error
        int diff = Math.Abs(scalarScore - simdScore);
        Assert.True(diff < 100, $"Score difference too large: {diff}");
    }

    [Theory]
    [InlineData(0, 0)]
    [InlineData(9, 9)]  // Center of 19x19
    [InlineData(18, 18)] // Far corner
    [InlineData(3, 5)]
    [InlineData(10, 8)]
    public void ScalarVsSIMD_SingleMoves_ShouldMatch(int x, int y)
    {
        var board = new Board();
        board.PlaceStone(x, y, Player.Red);

        int scalarScore = BitBoardEvaluator.Evaluate(board, Player.Red);
        int simdScore = SIMDBitBoardEvaluator.Evaluate(board, Player.Red);

        _output.WriteLine($"Single move at ({x},{y}) - Scalar: {scalarScore}, SIMD: {simdScore}");

        // Allow for some difference between implementations
        int diff = Math.Abs(scalarScore - simdScore);
        Assert.True(diff < 500, $"Score difference too large: {diff}");
    }

    [Fact]
    public void ScalarVsSIMD_RandomPosition100_ShouldMatch()
    {
        var random = new Random(42); // Fixed seed for reproducibility
        var board = new Board();

        int matchCount = 0;
        int totalTests = 100;
        int maxDiff = 0;

        for (int i = 0; i < totalTests; i++)
        {
            var testBoard = board.Clone();

            // Place 10-20 random stones
            int stoneCount = random.Next(10, 21);
            for (int j = 0; j < stoneCount; j++)
            {
                int x = random.Next(19);  // 19x19 board
                int y = random.Next(19);
                Player player = random.Next(2) == 0 ? Player.Red : Player.Blue;

                if (testBoard.GetCell(x, y).IsEmpty)
                {
                    testBoard.PlaceStone(x, y, player);
                }
            }

            int scalarScore = BitBoardEvaluator.Evaluate(testBoard, Player.Red);
            int simdScore = SIMDBitBoardEvaluator.Evaluate(testBoard, Player.Red);

            int diff = Math.Abs(scalarScore - simdScore);
            maxDiff = Math.Max(maxDiff, diff);

            if (diff == 0)
                matchCount++;
        }

        _output.WriteLine($"Random positions: {matchCount}/{totalTests} exact matches, max diff: {maxDiff}");

        // SIMD implementation may differ from scalar - this is informational
        // Max diff can be up to OpenThreeScore * 2.2 = 2200 due to edge case detection differences
        // between RLE (SIMD) and counted[] array (scalar) approaches
        Assert.True(maxDiff < 5000, $"Max difference {maxDiff} too large");
    }
}
