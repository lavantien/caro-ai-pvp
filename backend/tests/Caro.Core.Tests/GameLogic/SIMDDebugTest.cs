using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.Tests.GameLogic;

/// <summary>
/// Debug test to identify SIMD evaluator scoring issues
/// </summary>
public class SIMDDebugTest
{
    private readonly ITestOutputHelper _output;

    public SIMDDebugTest(ITestOutputHelper output)
    {
        _output = output;
    }

    [Fact]
    public void Debug_BlueOpenFour_RedThree()
    {
        var board = new Board();

        // Red has three in a row horizontally at y=7
        board.PlaceStone(5, 7, Player.Red);
        board.PlaceStone(6, 7, Player.Red);
        board.PlaceStone(7, 7, Player.Red);

        // Blue has four in a row horizontally at y=8 (OPEN FOUR - should be heavily penalized)
        board.PlaceStone(5, 8, Player.Blue);
        board.PlaceStone(6, 8, Player.Blue);
        board.PlaceStone(7, 8, Player.Blue);
        board.PlaceStone(8, 8, Player.Blue);

        int scalarScore = BitBoardEvaluator.Evaluate(board, Player.Red);
        int simdScore = SIMDBitBoardEvaluator.Evaluate(board, Player.Red);

        string scalarSign = scalarScore < 0 ? "NEGATIVE" : "POSITIVE";
        string simdSign = simdScore < 0 ? "NEGATIVE" : "POSITIVE";
        int diff = Math.Abs(scalarScore - simdScore);

        _output.WriteLine($"Scalar total: {scalarScore}, sign: {scalarSign}");
        _output.WriteLine($"SIMD total: {simdScore}, sign: {simdSign}");
        _output.WriteLine($"Absolute difference: {diff}");

        // Expected: Scalar should be negative because Blue's open four (10000 * 2.2 = 22000) outweighs Red's three
        // They should match within 100 points
        Assert.True(diff < 100, $"Score difference {diff} is too large. Scalar: {scalarScore}, SIMD: {simdScore}");
    }

    [Fact]
    public void Debug_HorizontalFourOnly()
    {
        var board = new Board();

        // Only Red has four in a row
        board.PlaceStone(5, 7, Player.Red);
        board.PlaceStone(6, 7, Player.Red);
        board.PlaceStone(7, 7, Player.Red);
        board.PlaceStone(8, 7, Player.Red);

        int scalarScore = BitBoardEvaluator.Evaluate(board, Player.Red);
        int simdScore = SIMDBitBoardEvaluator.Evaluate(board, Player.Red);

        _output.WriteLine($"Horizontal four only - Scalar: {scalarScore}, SIMD: {simdScore}, Diff: {simdScore - scalarScore}");

        // Both should be positive and similar
        Assert.True(scalarScore > 0, $"Scalar should be positive: {scalarScore}");
        Assert.True(simdScore > 0, $"SIMD should be positive: {simdScore}");

        int diff = Math.Abs(scalarScore - simdScore);
        Assert.True(diff < 100, $"Score difference {diff} is too large");
    }
}
