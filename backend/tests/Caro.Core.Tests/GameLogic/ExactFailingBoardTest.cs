using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.Tests.GameLogic;

/// <summary>
/// Exact board from failing test case
/// </summary>
public class ExactFailingBoardTest
{
    private readonly ITestOutputHelper _output;

    public ExactFailingBoardTest(ITestOutputHelper output)
    {
        _output = output;
    }

    [Fact]
    public void Debug_ExactFailingBoard()
    {
        var board = new Board();

        // Recreate the exact board from iteration 1522
        // Row 0: ..........B....  (col 10)
        board.PlaceStone(10, 0, Player.Blue);
        // Row 1: .......B.......  (col 7)
        board.PlaceStone(7, 1, Player.Blue);
        // Row 2: ............B..  (col 12)
        board.PlaceStone(12, 2, Player.Blue);
        // Row 3: .......B...R...  (col 7 Blue, col 11 Red)
        board.PlaceStone(7, 3, Player.Blue);
        board.PlaceStone(11, 3, Player.Red);
        // Row 4: ..R............  (col 2 Red)
        board.PlaceStone(2, 4, Player.Red);
        // Row 5: .BR............  (col 1 Blue, col 2 Red)
        board.PlaceStone(1, 5, Player.Blue);
        board.PlaceStone(2, 5, Player.Red);
        // Row 6: .......R..BB...  (col 7 Red, col 10 Blue, col 11 Blue)
        board.PlaceStone(7, 6, Player.Red);
        board.PlaceStone(10, 6, Player.Blue);
        board.PlaceStone(11, 6, Player.Blue);
        // Row 7: .B.............  (col 1 Blue)
        board.PlaceStone(1, 7, Player.Blue);
        // Row 8: ..............R  (col 14 Red)
        board.PlaceStone(14, 8, Player.Red);
        // Row 9: ...R..B.R......  (col 3 Red, col 6 Blue, col 8 Red)
        board.PlaceStone(3, 9, Player.Red);
        board.PlaceStone(6, 9, Player.Blue);
        board.PlaceStone(8, 9, Player.Red);
        // Row 10: ..............B  (col 14 Blue)
        board.PlaceStone(14, 10, Player.Blue);
        // Row 11: ..............B  (col 14 Blue)
        board.PlaceStone(14, 11, Player.Blue);
        // Row 12: ..............B  (col 14 Blue)
        board.PlaceStone(14, 12, Player.Blue);

        _output.WriteLine(board.ToString());

        int scalarScore = BitBoardEvaluator.Evaluate(board, Player.Red);
        int simdScore = SIMDBitBoardEvaluator.Evaluate(board, Player.Red);

        _output.WriteLine($"\nFrom Red's perspective:");
        _output.WriteLine($"  Scalar: {scalarScore}");
        _output.WriteLine($"  SIMD: {simdScore}");
        _output.WriteLine($"  Diff: {Math.Abs(scalarScore - simdScore)}");

        // Break down by direction
        // Also check from Blue's perspective for completeness
        int scalarBlue = BitBoardEvaluator.Evaluate(board, Player.Blue);
        int simdBlue = SIMDBitBoardEvaluator.Evaluate(board, Player.Blue);

        _output.WriteLine($"\nFrom Blue's perspective:");
        _output.WriteLine($"  Scalar: {scalarBlue}");
        _output.WriteLine($"  SIMD: {simdBlue}");
        _output.WriteLine($"  Diff: {Math.Abs(scalarBlue - simdBlue)}");

        // Expected: Diff should be 2200
        _output.WriteLine($"\nExpected diff: 2200 (Blue vertical open three * 2.2)");
    }
}
