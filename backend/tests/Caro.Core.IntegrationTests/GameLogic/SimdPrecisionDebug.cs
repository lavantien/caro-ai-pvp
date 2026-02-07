using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.Tests.GameLogic;

/// <summary>
/// Debug test to find the exact scenario causing 2200 point difference
/// </summary>
[Trait("Category", "Debug")]
public class SimdPrecisionDebug
{
    private readonly ITestOutputHelper _output;

    public SimdPrecisionDebug(ITestOutputHelper output)
    {
        _output = output;
    }

    [Fact]
    public void Find2200DifferenceScenario()
    {
        var random = new Random(42); // Fixed seed for reproducibility
        int totalTests = 10000;

        for (int i = 0; i < totalTests; i++)
        {
            var testBoard = new Board();
            int stoneCount = random.Next(5, 30);

            for (int j = 0; j < stoneCount; j++)
            {
                int x = random.Next(15);
                int y = random.Next(15);
                Player player = random.Next(2) == 0 ? Player.Red : Player.Blue;

                if (testBoard.GetCell(x, y).IsEmpty)
                {
                    testBoard = testBoard.PlaceStone(x, y, player);
                }
            }

            int scalarScore = BitBoardEvaluator.Evaluate(testBoard, Player.Red);
            int simdScore = SIMDBitBoardEvaluator.Evaluate(testBoard, Player.Red);

            int diff = Math.Abs(scalarScore - simdScore);

            if (diff >= 2000 && diff <= 2500)
            {
                _output.WriteLine($"Found 2200-range diff at iteration {i}:");
                _output.WriteLine($"  Scalar: {scalarScore}, SIMD: {simdScore}, Diff: {diff}");
                _output.WriteLine($"  Board ({stoneCount} stones):");

                // Print board state
                for (int y = 0; y < 15; y++)
                {
                    string row = "";
                    for (int x = 0; x < 15; x++)
                    {
                        var cell = testBoard.GetCell(x, y);
                        if (cell.Player == Player.Red) row += "R";
                        else if (cell.Player == Player.Blue) row += "B";
                        else row += ".";
                    }
                    if (!row.All(c => c == '.'))
                        _output.WriteLine($"    {row}");
                }

                // Analyze components
                var redBoard = testBoard.GetBitBoard(Player.Red);
                var blueBoard = testBoard.GetBitBoard(Player.Blue);

                // Check Red's score components
                var (p0, p1, p2, p3, p4, p5) = redBoard.GetRawValues();
                var (b0, b1, b2, b3, b4, b5) = blueBoard.GetRawValues();

                // Check open threes - 1000 * 2.2 = 2200
                _output.WriteLine($"\n  Checking for open threes (1000 * 2.2 = 2200):");

                return; // Found one, stop
            }
        }

        _output.WriteLine($"No 2200-range difference found in {totalTests} iterations");
    }
}
