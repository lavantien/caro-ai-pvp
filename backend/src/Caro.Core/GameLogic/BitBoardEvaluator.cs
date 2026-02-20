using System.Runtime.CompilerServices;
using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// High-performance board evaluator using BitBoard operations
/// Leverages bitwise operations and hardware POPCNT for fast pattern detection
/// </summary>
public static class BitBoardEvaluator
{
    // Import scoring weights from centralized constants
    // Local aliases for readability within this file
    private const int FiveInRowScore = EvaluationConstants.FiveInRowScore;
    private const int OpenFourScore = EvaluationConstants.OpenFourScore;
    private const int ClosedFourScore = EvaluationConstants.ClosedFourScore;
    private const int OpenThreeScore = EvaluationConstants.OpenThreeScore;
    private const int ClosedThreeScore = EvaluationConstants.ClosedThreeScore;
    private const int OpenTwoScore = EvaluationConstants.OpenTwoScore;
    private const int CenterBonus = EvaluationConstants.CenterBonus;

    /// <summary>
    /// Defense multiplier for asymmetric scoring.
    /// In Caro, blocking opponent threats is MORE important than creating your own.
    /// This multiplier ensures opponent threats are weighted higher than equivalent player threats.
    /// Rationale: In fast time controls, safer to be "paranoid" and block early than miss a VCF.
    /// Effect: Opponent Open 4 = -15,000, My Open 4 = +10,000 -> AI prioritizes blocking.
    ///
    /// NOTE: Reduced from 2.2x to 1.5x to prevent second-mover (Blue) advantage.
    /// 2.2x was too aggressive and caused Blue to consistently win regardless of difficulty difference.
    /// </summary>
    private const float DefenseMultiplier = (float)EvaluationConstants.DefenseMultiplierNumerator / EvaluationConstants.DefenseMultiplierDenominator;

    // Direction vectors: horizontal, vertical, 2 diagonals
    private static readonly (int dx, int dy)[] Directions = new[]
    {
        (1, 0),   // Horizontal
        (0, 1),   // Vertical
        (1, 1),   // Diagonal down-right
        (1, -1)   // Diagonal down-left
    };

    /// <summary>
    /// Evaluate the board for a given player using BitBoard operations
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int Evaluate(Board board, Player player)
    {
        if (player == Player.None)
            throw new ArgumentException("Player cannot be None");

        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var playerBoard = board.GetBitBoard(player);
        var opponentBoard = board.GetBitBoard(opponent);

        return EvaluateBitBoard(playerBoard, opponentBoard);
    }

    /// <summary>
    /// Evaluate the SearchBoard for a given player using BitBoard operations.
    /// High-performance path for search that avoids immutable Board overhead.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int Evaluate(SearchBoard board, Player player)
    {
        if (player == Player.None)
            throw new ArgumentException("Player cannot be None");

        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var playerBoard = board.GetBitBoard(player);
        var opponentBoard = board.GetBitBoard(opponent);

        return EvaluateBitBoard(playerBoard, opponentBoard);
    }

    /// <summary>
    /// Evaluate using only BitBoard operations (fastest path)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int EvaluateBitBoard(BitBoard playerBoard, BitBoard opponentBoard)
    {
        var score = 0;
        var occupied = playerBoard | opponentBoard;

        // Evaluate all directions using shift-based pattern detection
        score += EvaluateDirection(playerBoard, occupied, 1, 0);   // Horizontal
        score += EvaluateDirection(playerBoard, occupied, 0, 1);   // Vertical
        score += EvaluateDirection(playerBoard, occupied, 1, 1);   // Diagonal
        score += EvaluateDirection(playerBoard, occupied, 1, -1);  // Anti-diagonal

        // Subtract opponent's threats with DefenseMultiplier (asymmetric scoring)
        // In Caro, blocking opponent threats is MORE important than creating your own attacks
        // Use integer math (multiply by 3, divide by 2) to avoid floating-point precision issues
        // DefenseMultiplier of 1.5 = 3/2
        const int DefenseMultiplierNumer = 3;
        const int DefenseMultiplierDenom = 2;

        var oppHorizontal = EvaluateDirection(opponentBoard, occupied, 1, 0);
        var oppVertical = EvaluateDirection(opponentBoard, occupied, 0, 1);
        var oppDiagonal = EvaluateDirection(opponentBoard, occupied, 1, 1);
        var oppAntiDiagonal = EvaluateDirection(opponentBoard, occupied, 1, -1);

        // Use integer math: opp * 3 / 2 for consistent results
        score -= (oppHorizontal * DefenseMultiplierNumer) / DefenseMultiplierDenom;
        score -= (oppVertical * DefenseMultiplierNumer) / DefenseMultiplierDenom;
        score -= (oppDiagonal * DefenseMultiplierNumer) / DefenseMultiplierDenom;
        score -= (oppAntiDiagonal * DefenseMultiplierNumer) / DefenseMultiplierDenom;

        // Add center control bonus
        score += EvaluateCenterControl(playerBoard);

        return score;
    }

    /// <summary>
    /// Evaluate patterns in a specific direction using bit shifts
    /// This is the core high-performance pattern detection
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static int EvaluateDirection(BitBoard playerBoard, BitBoard occupied, int dx, int dy)
    {
        var score = 0;
        var counted = new bool[BitBoard.Size, BitBoard.Size];

        for (int x = 0; x < BitBoard.Size; x++)
        {
            for (int y = 0; y < BitBoard.Size; y++)
            {
                if (!playerBoard.GetBit(x, y) || counted[x, y])
                    continue;

                // Count consecutive stones in this direction
                var count = CountConsecutive(playerBoard, x, y, dx, dy);

                // Mark all stones in this sequence as counted
                var cx = x;
                var cy = y;
                for (int i = 0; i < count; i++)
                {
                    counted[cx, cy] = true;
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

    /// <summary>
    /// Evaluate center control using BitBoard
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static int EvaluateCenterControl(BitBoard playerBoard)
    {
        var score = 0;

        // Center zone: 5x5 area from (5,5) to (9,9)
        for (int x = 5; x <= 9; x++)
        {
            for (int y = 5; y <= 9; y++)
            {
                if (playerBoard.GetBit(x, y))
                {
                    // Center cell (7,7) gets highest bonus
                    var distanceToCenter = Math.Abs(x - 7) + Math.Abs(y - 7);
                    score += CenterBonus - (distanceToCenter * 5);
                }
            }
        }

        return score;
    }
}
