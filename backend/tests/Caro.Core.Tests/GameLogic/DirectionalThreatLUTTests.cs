using Xunit;
using Caro.Core.GameLogic;

namespace Caro.Core.Tests.GameLogic;

/// <summary>
/// Tests for DirectionalThreatLUT - systematic threat detection using precomputed patterns
/// Tests verify correct handling of Caro rules (exactly 5 wins, 6+ is overline, sandwiched wins are invalid)
///
/// IMPORTANT: All encodings have position 4 (center) as 00 (Empty) because we're evaluating
/// "what threat exists if I play at center?" The board state has center empty.
/// </summary>
public class DirectionalThreatLUTTests
{
    [Fact]
    public void LUT_TableSize_Is262144()
    {
        var lut = new DirectionalThreatLUT();
        Assert.Equal(262144, lut.TableSize);
    }

    [Fact]
    public void LUT_EmptyBoard_NoThreat()
    {
        var lut = new DirectionalThreatLUT();

        // Pattern: All empty (________)
        int key = EncodePattern("00 00 00 00 00 00 00 00 00");
        Assert.Equal(DirectionalThreatLUT.NoThreat, lut.GetThreatValue(key));
    }

    [Fact]
    public void LUT_SingleStone_NoThreat()
    {
        var lut = new DirectionalThreatLUT();

        // Pattern: ___X____ (single stone, center is empty)
        // Position 2 has a stone, center (4) is empty
        int key = EncodePattern("00 00 00 00 00 01 00 00 00");
        Assert.Equal(DirectionalThreatLUT.NoThreat, lut.GetThreatValue(key));
    }

    [Fact]
    public void LUT_OpenFour_XXXX_IsWinning()
    {
        var lut = new DirectionalThreatLUT();

        // Pattern: __XXXX__ (4 consecutive, both sides open - can make 5 either way)
        // Center (position 4) is empty, positions 2,3,5,6 are my stones
        int key = EncodePattern("00 00 01 01 00 01 01 00 00");
        Assert.Equal(DirectionalThreatLUT.WinningMove, lut.GetThreatValue(key));
    }

    [Fact]
    public void LUT_SemiOpenFour_XXXX_IsVeryStrongThreat()
    {
        var lut = new DirectionalThreatLUT();

        // Pattern: _XXXX_XX (4 consecutive stones adjacent to center)
        // Center (4) is empty, positions 2,3 are my stones left, 5,6 are my stones right
        // Position 7 also has my stone (blocking one side for completion)
        // This creates: _XX_XX_X where center can complete one way
        int key = EncodePattern("00 01 01 01 00 01 01 01 00");
        byte value = lut.GetThreatValue(key);

        // This should be VeryStrongThreat or WinningMove depending on configuration
        Assert.True(value >= DirectionalThreatLUT.VeryStrongThreat,
            $"Semi-open four should be very strong threat or winning, got {value}");
    }

    [Fact]
    public void LUT_SemiOpenFour_BlockedAtOneEnd_IsThreat()
    {
        var lut = new DirectionalThreatLUT();

        // Pattern: _XX_X____ (2 stones left of center, 1 stone right)
        // Center (4) is empty, positions 2,3 are my stones left, 5 is my stone right
        // Playing at center creates 3-in-row with room to extend on either side
        int key = EncodePattern("00 00 01 01 00 01 00 00 00");
        byte value = lut.GetThreatValue(key);

        // Should be at least StrongThreat (can extend to 4 or 5)
        Assert.True(value >= DirectionalThreatLUT.StrongThreat,
            $"Three-in-row should be strong threat or better, got {value}");
    }

    [Fact]
    public void LUT_ExactFive_IsWinningMove()
    {
        var lut = new DirectionalThreatLUT();

        // Pattern: _XXXX_X_ (4 stones around center, one more to complete 5)
        // Center (4) is empty, positions 1,2,3 are my stones left, 5 is my stone right
        // Playing at center creates exactly 5-in-row
        int key = EncodePattern("00 01 01 01 00 01 00 00 00");
        Assert.Equal(DirectionalThreatLUT.WinningMove, lut.GetThreatValue(key));
    }

    [Fact]
    public void LUT_SixInRow_Overline_IsInvalid()
    {
        var lut = new DirectionalThreatLUT();

        // Pattern: XXXXX___ (5 stones to left of center)
        // Center (4) is empty, positions 0,1,2,3 are my stones left, position 5 is my stone right
        // Playing at center would create 6-in-row = overline
        int key = EncodePattern("01 01 01 01 00 01 00 00 00");
        Assert.Equal(DirectionalThreatLUT.InvalidWin, lut.GetThreatValue(key));
    }

    [Fact]
    public void LUT_SevenInRow_Overline_IsInvalid()
    {
        var lut = new DirectionalThreatLUT();

        // Pattern: XXXXX_X_ (5 stones left, 1 stone right of center)
        // Playing at center creates 7-in-row
        int key = EncodePattern("01 01 01 01 00 01 01 00 00");
        Assert.Equal(DirectionalThreatLUT.InvalidWin, lut.GetThreatValue(key));
    }

    [Fact]
    public void LUT_SandwichedFive_OXXXXXO_IsInvalid()
    {
        var lut = new DirectionalThreatLUT();

        // Pattern: OXXXXXO (5 stones but both ends blocked by opponent)
        // Center (4) is empty, positions 0-2 are my stones left, 5-7 are my stones right
        // Position -1 doesn't exist, but we simulate with opponent at position 0
        // Actually: position 0 is opponent, 1,2,3 are my stones, 4 is center,
        // 5,6,7 are my stones, 8 is opponent
        int key = EncodePattern("10 01 01 01 00 01 01 01 10");
        Assert.Equal(DirectionalThreatLUT.InvalidWin, lut.GetThreatValue(key));
    }

    /// <summary>
    /// Critical test: oxxxx_x pattern should NOT be a threat
    /// because filling the gap creates 6-in-a-row (overline)
    /// Pattern: oxxxx_x means opponent at left, then 4 stones, gap, then stone
    /// </summary>
    [Fact]
    public void LUT_ComplexPattern_oxxxx_x_IsNoThreat()
    {
        var lut = new DirectionalThreatLUT();

        // Pattern: oxxxx_x where center is the gap between 4 stones and 1 stone
        // Position 0: opponent, positions 1,2,3: my stones left of center
        // Position 4: center (empty)
        // Position 5,6: my stones right of center
        // If we play at center: oxxxx_x becomes oxxxxxx (6 stones) = overline
        int key = EncodePattern("10 01 01 01 00 01 01 00 00");
        byte value = lut.GetThreatValue(key);

        // This should be InvalidWin (overline detected - 3+1+2=6 stones if played)
        Assert.Equal(DirectionalThreatLUT.InvalidWin, value);
    }

    /// <summary>
    /// Test: xxx_o_xxx (gap between 3 stones each side)
    /// Playing at center creates 7-in-row = overline
    /// </summary>
    [Fact]
    public void LUT_Pattern_ThreeGapThree_Overline_IsInvalid()
    {
        var lut = new DirectionalThreatLUT();

        // Pattern: XXX_O_XXX where center is the gap
        // Positions 0,1,2: my stones, position 3: my stone, position 4: center (empty)
        // Positions 5,6,7: my stones
        // Total: 3 + 1 + 3 = 7 stones if center played = overline
        int key = EncodePattern("01 01 01 01 00 01 01 01 00");
        byte value = lut.GetThreatValue(key);

        // Should be InvalidWin (overline)
        Assert.Equal(DirectionalThreatLUT.InvalidWin, value);
    }

    [Fact]
    public void LUT_OpenThree_XXX_IsStrongThreat()
    {
        var lut = new DirectionalThreatLUT();

        // Pattern: __XXX___ (3 consecutive, both sides open)
        // Center (4) is empty, positions 2,3 are my stones left, 5 is my stone right
        int key = EncodePattern("00 00 01 01 00 01 00 00 00");
        byte value = lut.GetThreatValue(key);

        // Open three should be at least StrongThreat
        Assert.True(value >= DirectionalThreatLUT.StrongThreat,
            $"Open three should be at least strong threat (got {value})");
    }

    [Fact]
    public void LUT_BrokenThree_XX_XX_IsThreat()
    {
        var lut = new DirectionalThreatLUT();

        // Pattern: _XX__XX__ (broken three with gap)
        // Center (4) is empty, positions 2,3 are my stones left, positions 6,7 are my stones right
        int key = EncodePattern("00 01 01 01 00 00 01 01 00");
        byte value = lut.GetThreatValue(key);

        // Broken three should be some level of threat
        Assert.True(value >= DirectionalThreatLUT.WeakThreat,
            $"Broken three should be at least weak threat (got {value})");
    }

    [Fact]
    public void LUT_WallAtEdge_NoThreat()
    {
        var lut = new DirectionalThreatLUT();

        // Pattern: WXX______ (2 stones at edge, wall at left edge)
        // Wall represents board boundary
        int key = EncodePattern("11 01 01 00 00 00 00 00 00");
        byte value = lut.GetThreatValue(key);

        // Near edge with wall limits completions
        Assert.True(value < DirectionalThreatLUT.WinningMove,
            $"Pattern at wall edge should not be winning (got {value})");
    }

    [Fact]
    public void LUT_IsWinningMove_ReturnsTrue_ForOpenFour()
    {
        var lut = new DirectionalThreatLUT();

        // Pattern: __XXXX__
        int key = EncodePattern("00 00 01 01 00 01 01 00 00");
        Assert.True(lut.IsWinningMove(key));
    }

    [Fact]
    public void LUT_IsWinningMove_ReturnsFalse_ForEmpty()
    {
        var lut = new DirectionalThreatLUT();

        int key = EncodePattern("00 00 00 00 00 00 00 00 00");
        Assert.False(lut.IsWinningMove(key));
    }

    [Fact]
    public void LUT_IsInvalidMove_ReturnsTrue_ForOverline()
    {
        var lut = new DirectionalThreatLUT();

        // Pattern: XXXXX___ (5 stones to left)
        int key = EncodePattern("01 01 01 01 00 01 00 00 00");
        Assert.True(lut.IsInvalidMove(key));
    }

    [Fact]
    public void LUT_IsInvalidMove_ReturnsTrue_ForSandwichedFive()
    {
        var lut = new DirectionalThreatLUT();

        // Pattern: OXXXXXO
        int key = EncodePattern("10 01 01 01 00 01 01 01 10");
        Assert.True(lut.IsInvalidMove(key));
    }

    [Fact]
    public void LUT_CenterOccupied_NoThreat()
    {
        var lut = new DirectionalThreatLUT();

        // Center is occupied (can't play there)
        int key = EncodePattern("00 00 00 00 01 00 00 00 00");
        Assert.Equal(DirectionalThreatLUT.NoThreat, lut.GetThreatValue(key));
    }

    [Fact]
    public void LUT_CenterWall_NoThreat()
    {
        var lut = new DirectionalThreatLUT();

        // Center is wall (edge of board)
        int key = EncodePattern("00 00 00 00 11 00 00 00 00");
        Assert.Equal(DirectionalThreatLUT.NoThreat, lut.GetThreatValue(key));
    }

    /// <summary>
    /// Helper method to encode pattern string to 18-bit key
    /// Format: "00 01 10 11..." where each 2-bit value represents a cell
    /// 00 = Empty, 01 = My Stone, 10 = Opponent Stone, 11 = Wall
    ///
    /// IMPORTANT: Position 4 (center) should typically be 00 (Empty) for threat evaluation
    /// </summary>
    private static int EncodePattern(string pattern)
    {
        var parts = pattern.Split(' ', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries);
        int key = 0;
        for (int i = 0; i < parts.Length && i < 9; i++)
        {
            int value = Convert.ToInt32(parts[i], 2);
            key |= value << (i * 2);
        }
        return key;
    }
}
