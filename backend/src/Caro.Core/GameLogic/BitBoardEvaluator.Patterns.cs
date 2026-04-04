using System.Runtime.CompilerServices;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Pattern detection methods for BitBoardEvaluator
/// </summary>
public static partial class BitBoardEvaluator
{
    /// <summary>
    /// Check if a five-in-row is sandwiched (OXXXXXO pattern)
    /// Sandwiched fives don't count as wins in Caro
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static bool IsSandwichedFive(BitBoard playerBoard, BitBoard occupied, int x, int y, int dx, int dy)
    {
        // Check if there are opponent stones on both ends
        // Find the start of the sequence
        var startX = x;
        var startY = y;

        while (startX - dx >= 0 && startX - dx < BitBoard.Size &&
               startY - dy >= 0 && startY - dy < BitBoard.Size &&
               playerBoard.GetBit(startX - dx, startY - dy))
        {
            startX -= dx;
            startY -= dy;
        }

        // Check if position before start has opponent stone
        var beforeBlocked = startX - dx >= 0 && startX - dx < BitBoard.Size &&
                           startY - dy >= 0 && startY - dy < BitBoard.Size &&
                           !playerBoard.GetBit(startX - dx, startY - dy) &&
                           occupied.GetBit(startX - dx, startY - dy);

        // Find the end of the sequence
        var endX = x;
        var endY = y;

        while (endX + dx >= 0 && endX + dx < BitBoard.Size &&
               endY + dy >= 0 && endY + dy < BitBoard.Size &&
               playerBoard.GetBit(endX + dx, endY + dy))
        {
            endX += dx;
            endY += dy;
        }

        // Check if position after end has opponent stone
        var afterBlocked = endX + dx >= 0 && endX + dx < BitBoard.Size &&
                          endY + dy >= 0 && endY + dy < BitBoard.Size &&
                          !playerBoard.GetBit(endX + dx, endY + dy) &&
                          occupied.GetBit(endX + dx, endY + dy);

        return beforeBlocked && afterBlocked;
    }

    /// <summary>
    /// Detect a specific threat pattern on the board
    /// </summary>
    public static bool DetectPattern(BitBoard playerBoard, BitBoard occupied, ThreatType threatType, out List<(int x, int y)> positions)
    {
        positions = new List<(int x, int y)>();

        foreach (var (dx, dy) in Directions)
        {
            // Scan the board
            for (int x = 0; x < BitBoard.Size; x++)
            {
                for (int y = 0; y < BitBoard.Size; y++)
                {
                    if (!playerBoard.GetBit(x, y))
                        continue;

                    var count = CountConsecutiveBoth(playerBoard, x, y, dx, dy);
                    var openEnds = CountOpenEnds(playerBoard, occupied, x, y, dx, dy, count);

                    bool matches = threatType switch
                    {
                        ThreatType.StraightFour => count == 4 && openEnds > 0,
                        ThreatType.StraightThree => count == 3 && openEnds > 0,
                        _ => false
                    };

                    if (matches)
                    {
                        positions.Add((x, y));
                    }
                }
            }
        }

        return positions.Count > 0;
    }

    /// <summary>
    /// Detect all threats on the board
    /// </summary>
    public static List<(ThreatType type, int x, int y)> DetectAllThreats(BitBoard playerBoard, BitBoard occupied)
    {
        var threats = new List<(ThreatType, int, int)>();

        foreach (var (dx, dy) in Directions)
        {
            for (int x = 0; x < BitBoard.Size; x++)
            {
                for (int y = 0; y < BitBoard.Size; y++)
                {
                    if (!playerBoard.GetBit(x, y))
                        continue;

                    var count = CountConsecutiveBoth(playerBoard, x, y, dx, dy);
                    var openEnds = CountOpenEnds(playerBoard, occupied, x, y, dx, dy, count);

                    if (count == 4 && openEnds > 0)
                    {
                        threats.Add((ThreatType.StraightFour, x, y));
                    }
                    else if (count == 3 && openEnds > 0)
                    {
                        threats.Add((ThreatType.StraightThree, x, y));
                    }
                }
            }
        }

        return threats;
    }
}
