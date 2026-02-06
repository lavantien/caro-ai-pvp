using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.Tests.GameLogic;

/// <summary>
/// Debug tests to trace SIMD evaluation step by step
/// </summary>
public class SimdDebugTest2
{
    private readonly ITestOutputHelper _output;

    public SimdDebugTest2(ITestOutputHelper output)
    {
        _output = output;
    }

    [Fact(Skip = "Outdated - written for 15x15 board, now using 19x19")]
    public void Debug_BlueOpenFour_Row8_Components()
    {
        var board = new Board();

        // Blue has open four at y=8: (5,8), (6,8), (7,8), (8,8)
        // Empty at (4,8) and (9,8) - should be OPEN
        board = board.PlaceStone(5, 8, Player.Blue);
        board = board.PlaceStone(6, 8, Player.Blue);
        board = board.PlaceStone(7, 8, Player.Blue);
        board = board.PlaceStone(8, 8, Player.Blue);

        // Get BitBoards
        var blueBoard = board.GetBitBoard(Player.Blue);
        var redBoard = board.GetBitBoard(Player.Red);
        var occupied = blueBoard | redBoard;

        _output.WriteLine($"Blue Board raw: {blueBoard}");
        _output.WriteLine($"Occupied raw: {occupied}");

        // Check what SIMD sees for row 8
        var (p0, p1, p2, p3, p4, p5) = blueBoard.GetRawValues();
        var (occ0, occ1, occ2, occ3, occ4, occ5) = occupied.GetRawValues();

        // Row 8 is in p2 (rows 6-8 for 19x19: 3 rows per ulong except last)
        int p2RowIdx = 8 - 6; // row 8 is the 3rd row in p2's group
        int bitOffset = p2RowIdx * 19;
        ulong rowMask = 0x7FFFFUL << bitOffset; // 19 bits for 19x19
        ulong row = (p2 & rowMask) >> bitOffset;
        ulong rowOcc = (occ2 & rowMask) >> bitOffset;

        _output.WriteLine($"\nRow 8 Blue bits: {Convert.ToString((long)row, 2).PadLeft(19, '0')}");
        _output.WriteLine($"Row 8 Occupied bits: {Convert.ToString((long)rowOcc, 2).PadLeft(19, '0')}");

        // Check each position 5-8
        for (int i = 5; i <= 8; i++)
        {
            bool hasStone = (row & (1UL << i)) != 0;
            _output.WriteLine($"  Position {i}: has stone = {hasStone}");
        }

        // Check positions 4 and 9 for openness
        bool pos4Empty = (rowOcc & (1UL << 4)) == 0;
        bool pos9Empty = (rowOcc & (1UL << 9)) == 0;
        _output.WriteLine($"  Position 4 empty: {pos4Empty}");
        _output.WriteLine($"  Position 9 empty: {pos9Empty}");

        // Now evaluate using SIMD
        int simdBlueScore = SIMDBitBoardEvaluator.Evaluate(board, Player.Blue);
        int scalarBlueScore = BitBoardEvaluator.Evaluate(board, Player.Blue);

        _output.WriteLine($"\nBlue's perspective - SIMD: {simdBlueScore}, Scalar: {scalarBlueScore}");

        // From Red's perspective, Blue's threat should be penalized
        int simdRedScore = SIMDBitBoardEvaluator.Evaluate(board, Player.Red);
        int scalarRedScore = BitBoardEvaluator.Evaluate(board, Player.Red);

        _output.WriteLine($"Red's perspective - SIMD: {simdRedScore}, Scalar: {scalarRedScore}");
    }
}
