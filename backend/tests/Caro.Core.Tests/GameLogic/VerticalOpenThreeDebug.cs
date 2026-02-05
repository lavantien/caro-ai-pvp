using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.Tests.GameLogic;

/// <summary>
/// Debug test for vertical open three at column 14 (edge)
/// </summary>
public class VerticalOpenThreeDebug
{
    private readonly ITestOutputHelper _output;

    public VerticalOpenThreeDebug(ITestOutputHelper output)
    {
        _output = output;
    }

    [Fact]
    public void Debug_VerticalOpenThree_AtColumn14()
    {
        var board = new Board();

        // Blue has three in a row vertically at column 14, rows 10-12
        // Both ends are open (row 9 and row 13 are empty)
        board.PlaceStone(14, 10, Player.Blue);
        board.PlaceStone(14, 11, Player.Blue);
        board.PlaceStone(14, 12, Player.Blue);

        // Verify the position is correct
        _output.WriteLine("Board state:");
        _output.WriteLine(board.ToString());

        int scalarScore = BitBoardEvaluator.Evaluate(board, Player.Red);
        int simdScore = SIMDBitBoardEvaluator.Evaluate(board, Player.Red);

        _output.WriteLine($"\nFrom Red's perspective:");
        _output.WriteLine($"  Scalar: {scalarScore}");
        _output.WriteLine($"  SIMD: {simdScore}");
        _output.WriteLine($"  Diff: {Math.Abs(scalarScore - simdScore)}");

        // Expected: Blue's open three should be penalized as -2200 (1000 * 2.2)
        _output.WriteLine($"\nExpected penalty for Blue's open three: -2200");

        // Test from Blue's perspective too
        int scalarBlue = BitBoardEvaluator.Evaluate(board, Player.Blue);
        int simdBlue = SIMDBitBoardEvaluator.Evaluate(board, Player.Blue);

        _output.WriteLine($"\nFrom Blue's perspective:");
        _output.WriteLine($"  Scalar: {scalarBlue}");
        _output.WriteLine($"  SIMD: {simdBlue}");
        _output.WriteLine($"  Diff: {Math.Abs(scalarBlue - simdBlue)}");

        _output.WriteLine($"\nExpected bonus for Blue's open three: +1000");
    }
}
