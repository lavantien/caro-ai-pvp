using System.Runtime.CompilerServices;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Direction-based evaluation methods for BitBoardEvaluator
/// </summary>
public static partial class BitBoardEvaluator
{
    /// <summary>
    /// Evaluate patterns in a specific direction using bit shifts
    /// This is the core high-performance pattern detection
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static int EvaluateDirection(BitBoard playerBoard, BitBoard occupied, int dx, int dy)
    {
        var score = 0;
        Span<bool> counted = stackalloc bool[BitBoard.Size * BitBoard.Size];

        for (int x = 0; x < BitBoard.Size; x++)
        {
            for (int y = 0; y < BitBoard.Size; y++)
            {
                if (!playerBoard.GetBit(x, y) || counted[x * BitBoard.Size + y])
                    continue;

                // Count consecutive stones in this direction
                var count = CountConsecutive(playerBoard, x, y, dx, dy);

                // Mark all stones in this sequence as counted
                var cx = x;
                var cy = y;
                for (int i = 0; i < count; i++)
                {
                    counted[cx * BitBoard.Size + cy] = true;
                    cx += dx;
                    cy += dy;
                }

                // Count open ends
                var openEnds = CountOpenEnds(playerBoard, occupied, x, y, dx, dy, count);

                // Score based on pattern
                if (count >= 5)
                {
                    score += FiveInRowScore;
                }
                else if (count == 4)
                {
                    if (openEnds >= 1)
                        score += OpenFourScore;
                    else
                        score += ClosedFourScore;
                }
                else if (count == 3)
                {
                    if (openEnds == 2)
                        score += OpenThreeScore * 2;
                    else if (openEnds == 1)
                        score += OpenThreeScore;
                    else
                        score += ClosedThreeScore;
                }
                else if (count == 2 && openEnds == 2)
                {
                    score += OpenTwoScore;
                }
            }
        }

        return score;
    }

    /// <summary>
    /// Evaluate patterns in a specific direction with custom scoring parameters
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static int EvaluateDirectionWithParams(BitBoard playerBoard, BitBoard occupied, int dx, int dy,
        int fiveInRowScore, int openFourScore, int closedFourScore, int openThreeScore, int closedThreeScore, int openTwoScore)
    {
        var score = 0;
        Span<bool> counted = stackalloc bool[BitBoard.Size * BitBoard.Size];

        for (int x = 0; x < BitBoard.Size; x++)
        {
            for (int y = 0; y < BitBoard.Size; y++)
            {
                if (!playerBoard.GetBit(x, y) || counted[x * BitBoard.Size + y])
                    continue;

                var count = CountConsecutive(playerBoard, x, y, dx, dy);

                var cx = x;
                var cy = y;
                for (int i = 0; i < count; i++)
                {
                    counted[cx * BitBoard.Size + cy] = true;
                    cx += dx;
                    cy += dy;
                }

                var openEnds = CountOpenEnds(playerBoard, occupied, x, y, dx, dy, count);

                if (count >= 5)
                {
                    score += fiveInRowScore;
                }
                else if (count == 4)
                {
                    if (openEnds >= 1)
                        score += openFourScore;
                    else
                        score += closedFourScore;
                }
                else if (count == 3)
                {
                    if (openEnds == 2)
                        score += openThreeScore * 2;
                    else if (openEnds == 1)
                        score += openThreeScore;
                    else
                        score += closedThreeScore;
                }
                else if (count == 2 && openEnds == 2)
                {
                    score += openTwoScore;
                }
            }
        }

        return score;
    }

    /// <summary>
    /// Shift bitboard by direction (dx, dy)
    /// Returns a new bitboard with bits shifted
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static BitBoard ShiftByDirection(BitBoard board, int dx, int dy)
    {
        var result = board;

        // Shift horizontally
        if (dx > 0)
        {
            for (int i = 0; i < dx; i++)
                result = result.ShiftRight();
        }
        else if (dx < 0)
        {
            for (int i = 0; i < -dx; i++)
                result = result.ShiftLeft();
        }

        // Shift vertically
        if (dy > 0)
        {
            for (int i = 0; i < dy; i++)
                result = result.ShiftDown();
        }
        else if (dy < 0)
        {
            for (int i = 0; i < -dy; i++)
                result = result.ShiftUp();
        }

        return result;
    }

    /// <summary>
    /// Count consecutive stones in a direction starting from (x, y)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int CountConsecutive(BitBoard board, int x, int y, int dx, int dy)
    {
        var count = 0;
        var cx = x;
        var cy = y;

        while (cx >= 0 && cx < BitBoard.Size && cy >= 0 && cy < BitBoard.Size && board.GetBit(cx, cy))
        {
            count++;
            cx += dx;
            cy += dy;
        }

        return count;
    }

    /// <summary>
    /// Count consecutive stones in both directions (positive and negative)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int CountConsecutiveBoth(BitBoard board, int x, int y, int dx, int dy)
    {
        // Count in positive direction
        var count = 1;  // Include starting position
        var cx = x + dx;
        var cy = y + dy;

        while (cx >= 0 && cx < BitBoard.Size && cy >= 0 && cy < BitBoard.Size && board.GetBit(cx, cy))
        {
            count++;
            cx += dx;
            cy += dy;
        }

        // Count in negative direction
        cx = x - dx;
        cy = y - dy;

        while (cx >= 0 && cx < BitBoard.Size && cy >= 0 && cy < BitBoard.Size && board.GetBit(cx, cy))
        {
            count++;
            cx -= dx;
            cy -= dy;
        }

        return count;
    }

    /// <summary>
    /// Count open ends for a sequence
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int CountOpenEnds(BitBoard playerBoard, BitBoard occupied, int x, int y, int dx, int dy, int count)
    {
        var openEnds = 0;

        // Find sequence start and end
        var startX = x;
        var startY = y;

        while (startX - dx >= 0 && startX - dx < BitBoard.Size &&
               startY - dy >= 0 && startY - dy < BitBoard.Size &&
               playerBoard.GetBit(startX - dx, startY - dy))
        {
            startX -= dx;
            startY -= dy;
        }

        var endX = startX + dx * (count - 1);
        var endY = startY + dy * (count - 1);

        // Check before start
        if (startX - dx >= 0 && startX - dx < BitBoard.Size &&
            startY - dy >= 0 && startY - dy < BitBoard.Size &&
            !occupied.GetBit(startX - dx, startY - dy))
        {
            openEnds++;
        }

        // Check after end
        if (endX + dx >= 0 && endX + dx < BitBoard.Size &&
            endY + dy >= 0 && endY + dy < BitBoard.Size &&
            !occupied.GetBit(endX + dx, endY + dy))
        {
            openEnds++;
        }

        return openEnds;
    }
}
