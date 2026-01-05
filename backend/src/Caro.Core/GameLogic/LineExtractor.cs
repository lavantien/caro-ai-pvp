using System.Runtime.CompilerServices;

namespace Caro.Core.GameLogic;

/// <summary>
/// Extracts 9-cell context keys from bitboard positions for DirectionalThreatLUT
///
/// The 9-cell window includes: Target cell + 4 cells left + 4 cells right
/// This provides full context for threat evaluation in any direction.
///
/// Each cell is encoded as 2 bits:
/// - 00: Empty
/// - 01: My Stone
/// - 10: Opponent Stone
/// - 11: Wall/Edge (out of bounds)
///
/// Result is an 18-bit key (9 cells × 2 bits) for O(1) LUT lookup.
/// </summary>
public static class LineExtractor
{
    // Cell encoding constants
    private const int EMPTY = 0b00;
    private const int MY_STONE = 0b01;
    private const int OPP_STONE = 0b10;
    private const int WALL = 0b11;

    /// <summary>
    /// Extract 9-cell horizontal line key centered at (x, y)
    /// Window: (x-4, y) to (x+4, y)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int ExtractHorizontalKey(ulong myStones, ulong oppStones, int x, int y)
    {
        int key = 0;

        for (int i = -4; i <= 4; i++)
        {
            int cx = x + i;
            int cellKey = GetCellKey(cx, y, myStones, oppStones);
            key |= cellKey << ((i + 4) * 2);
        }

        return key;
    }

    /// <summary>
    /// Extract 9-cell vertical line key centered at (x, y)
    /// Window: (x, y-4) to (x, y+4)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int ExtractVerticalKey(ulong myStones, ulong oppStones, int x, int y)
    {
        int key = 0;

        for (int i = -4; i <= 4; i++)
        {
            int cy = y + i;
            int cellKey = GetCellKey(x, cy, myStones, oppStones);
            key |= cellKey << ((i + 4) * 2);
        }

        return key;
    }

    /// <summary>
    /// Extract 9-cell diagonal (\) key centered at (x, y)
    /// Window: (x-4, y-4) to (x+4, y+4) - top-left to bottom-right
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int ExtractDiagonalDownKey(ulong myStones, ulong oppStones, int x, int y)
    {
        int key = 0;

        for (int i = -4; i <= 4; i++)
        {
            int cx = x + i;
            int cy = y + i;
            int cellKey = GetCellKey(cx, cy, myStones, oppStones);
            key |= cellKey << ((i + 4) * 2);
        }

        return key;
    }

    /// <summary>
    /// Extract 9-cell anti-diagonal (/) key centered at (x, y)
    /// Window: (x-4, y+4) to (x+4, y-4) - bottom-left to top-right
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int ExtractDiagonalUpKey(ulong myStones, ulong oppStones, int x, int y)
    {
        int key = 0;

        for (int i = -4; i <= 4; i++)
        {
            int cx = x + i;
            int cy = y - i;
            int cellKey = GetCellKey(cx, cy, myStones, oppStones);
            key |= cellKey << ((i + 4) * 2);
        }

        return key;
    }

    /// <summary>
    /// Get 2-bit cell key for a specific position
    /// Returns: WALL if out of bounds, MY_STONE/OPP_STONE/EMPTY otherwise
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static int GetCellKey(int x, int y, ulong myStones, ulong oppStones)
    {
        // Check bounds - return WALL if out of bounds
        if (x < 0 || x >= 15 || y < 0 || y >= 15)
            return WALL;

        int idx = y * 15 + x;
        ulong mask = 1UL << idx;

        // Check my stones first
        if ((myStones & mask) != 0)
            return MY_STONE;

        // Check opponent stones
        if ((oppStones & mask) != 0)
            return OPP_STONE;

        // Otherwise empty
        return EMPTY;
    }

    /// <summary>
    /// Extract all 4 directional keys for a position in a single call
    /// Returns tuple: (horizontal, vertical, diagonalDown, diagonalUp)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static (int h, int v, int dd, int du) ExtractAllKeys(ulong myStones, ulong oppStones, int x, int y)
    {
        return (
            ExtractHorizontalKey(myStones, oppStones, x, y),
            ExtractVerticalKey(myStones, oppStones, x, y),
            ExtractDiagonalDownKey(myStones, oppStones, x, y),
            ExtractDiagonalUpKey(myStones, oppStones, x, y)
        );
    }

    /// <summary>
    /// Extract all 4 directional keys assuming perspective of a specific player
    /// The player's stones become "MyStone" in the encoding
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static (int h, int v, int dd, int du) ExtractAllKeysForPlayer(
        ulong playerStones, ulong opponentStones, int x, int y)
    {
        return ExtractAllKeys(playerStones, opponentStones, x, y);
    }
}
