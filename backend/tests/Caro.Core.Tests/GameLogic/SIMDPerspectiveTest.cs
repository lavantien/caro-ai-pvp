using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.Tests.GameLogic;

/// <summary>
/// Test to verify SIMD evaluator returns correct perspective
/// </summary>
public class SIMDPerspectiveTest
{
    private readonly ITestOutputHelper _output;

    public SIMDPerspectiveTest(ITestOutputHelper output)
    {
        _output = output;
    }

    [Fact]
    public void SIMD_EvaluatesFromCorrectPerspective()
    {
        var board = new Board();

        // Red has 3 in a row
        board = board.PlaceStone(5, 7, Player.Red);
        board = board.PlaceStone(6, 7, Player.Red);
        board = board.PlaceStone(7, 7, Player.Red);

        // Blue has 4 in a row (OPEN FOUR) - should be BAD for Red
        board = board.PlaceStone(5, 8, Player.Blue);
        board = board.PlaceStone(6, 8, Player.Blue);
        board = board.PlaceStone(7, 8, Player.Blue);
        board = board.PlaceStone(8, 8, Player.Blue);

        // Evaluate for RED (should be NEGATIVE because Blue has open four)
        int redScore_scalar = BitBoardEvaluator.Evaluate(board, Player.Red);
        int redScore_simd = SIMDBitBoardEvaluator.Evaluate(board, Player.Red);

        _output.WriteLine($"Red perspective - Scalar: {redScore_scalar}, SIMD: {redScore_simd}");
        _output.WriteLine($"Expected: Both should be NEGATIVE (penalizing Blue's threat)");

        // Both should be negative
        Assert.True(redScore_scalar < 0, $"Scalar should be negative for Red when Blue has open four: {redScore_scalar}");
        Assert.True(redScore_simd < 0, $"SIMD should be negative for Red when Blue has open four: {redScore_simd}");

        // Test from Blue's perspective (should be POSITIVE for Blue)
        int blueScore_scalar = BitBoardEvaluator.Evaluate(board, Player.Blue);
        int blueScore_simd = SIMDBitBoardEvaluator.Evaluate(board, Player.Blue);

        _output.WriteLine($"Blue perspective - Scalar: {blueScore_scalar}, SIMD: {blueScore_simd}");
        _output.WriteLine($"Expected: Both should be POSITIVE (Blue benefits from its open four)");

        // Both should be positive
        Assert.True(blueScore_scalar > 0, $"Scalar should be positive for Blue when Blue has open four: {blueScore_scalar}");
        Assert.True(blueScore_simd > 0, $"SIMD should be positive for Blue when Blue has open four: {blueScore_simd}");

        // The scores should be opposite (approximately)
        _output.WriteLine($"Red + Blue sum - Scalar: {redScore_scalar + blueScore_scalar}, SIMD: {redScore_simd + blueScore_simd}");
    }
}
