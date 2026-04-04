using System.Runtime.CompilerServices;
using Caro.Core.Domain.Configuration;

namespace Caro.Core.GameLogic;

/// <summary>
/// Threat detection partial class for SIMDBitBoardEvaluator
/// </summary>
public static partial class SIMDBitBoardEvaluator
{
    /// <summary>
    /// Count new threats created by placing a stone at (x, y)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static int CountNewThreats(int x, int y, BitBoard playerBoard, BitBoard occupied)
    {
        int threats = 0;
        var directions = new[] { (1, 0), (0, 1), (1, 1), (1, -1) };

        foreach (var (dx, dy) in directions)
        {
            var count = BitBoardEvaluator.CountConsecutiveBoth(playerBoard, x, y, dx, dy);
            var openEnds = BitBoardEvaluator.CountOpenEnds(playerBoard, occupied, x, y, dx, dy, count);

            if (count == 4 && openEnds > 0) threats += 5;  // Straight Four
            if (count == 3 && openEnds == 2) threats += 3; // Open Three
            if (count == 3 && openEnds == 1) threats += 1; // Closed Three
        }

        return threats;
    }

    /// <summary>
    /// Count opponent threats blocked by placing at (x, y)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static int CountBlockedThreats(int x, int y, BitBoard opponentBoard, BitBoard occupied)
    {
        int blocked = 0;
        var directions = new[] { (1, 0), (0, 1), (1, 1), (1, -1) };

        foreach (var (dx, dy) in directions)
        {
            // Check if this position blocks an opponent threat
            var count = BitBoardEvaluator.CountConsecutiveBoth(opponentBoard, x, y, dx, dy);
            var openEnds = BitBoardEvaluator.CountOpenEnds(opponentBoard, occupied, x, y, dx, dy, count);

            if (count == 3 && openEnds == 2) blocked += 4; // Blocking open three is valuable
            if (count == 4 && openEnds > 0) blocked += 10; // Blocking four is critical

            // CRITICAL FIX: Also check for broken four patterns (__xx_x__) that would become five if not blocked
            // If placing at (x, y) creates/extends opponent's pattern to 4 with a gap, must block
            var brokenFourCount = CountBrokenFourPatterns(opponentBoard, occupied, x, y, dx, dy);
            if (brokenFourCount > 0) blocked += brokenFourCount * 15; // High priority - broken four is almost as bad as open four
        }

        return blocked;
    }

    /// <summary>
    /// Detects broken four patterns (__xx_x__) where opponent has 4 stones with a gap
    /// If the gap is filled, it becomes 5-in-a-row (a win). Must block!
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static int CountBrokenFourPatterns(BitBoard opponentBoard, BitBoard occupied, int x, int y, int dx, int dy)
    {
        int blockedPatterns = 0;

        // Check all pattern variations centered around (x, y)
        // Pattern 1: __XX_X (gap after 2 stones) - opponent would win if they fill the gap
        // Pattern 2: X__XX (gap before last 2 stones)
        // Pattern 3: X_X__X (middle gap)
        // etc.

        // We simulate: what if opponent plays at (x, y)? Would they have 4 stones with potential to win?
        // Check up to 4 cells in each direction to see the full pattern

        // Look for patterns where opponent has stones that would form 4 with this gap filled
        int totalStones = 0;
        int gapCount = 0;

        // Check positive direction (dx, dy)
        for (int i = 1; i <= 4; i++)
        {
            int nx = x + dx * i;
            int ny = y + dy * i;
            if (nx < 0 || nx >= GameConstants.BoardSize || ny < 0 || ny >= GameConstants.BoardSize) break; // Out of bounds

            if (opponentBoard.GetBit(nx, ny))
            {
                totalStones++;
            }
            else if (!occupied.GetBit(nx, ny))
            {
                gapCount++;
            }
            else
            {
                break; // Blocked by current player
            }
        }

        // Check negative direction (-dx, -dy)
        for (int i = 1; i <= 4; i++)
        {
            int nx = x - dx * i;
            int ny = y - dy * i;
            if (nx < 0 || nx >= GameConstants.BoardSize || ny < 0 || ny >= GameConstants.BoardSize) break; // Out of bounds

            if (opponentBoard.GetBit(nx, ny))
            {
                totalStones++;
            }
            else if (!occupied.GetBit(nx, ny))
            {
                gapCount++;
            }
            else
            {
                break; // Blocked by current player
            }
        }

        // If opponent would have 4 stones (including this gap position) with few gaps, it's a threat
        // A broken four like __XX_X__ has 4 stones + 1 gap + 2 empty = 7 positions
        if (totalStones >= 3 && gapCount <= 3)
        {
            // This looks like a broken four pattern
            blockedPatterns++;
        }

        return blockedPatterns;
    }
}
