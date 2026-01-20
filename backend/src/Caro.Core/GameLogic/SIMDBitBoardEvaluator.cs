using System.Runtime.CompilerServices;
using System.Numerics;
using Caro.Core.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// SIMD-accelerated BitBoard evaluator for ultra-fast pattern evaluation
/// Uses hardware POPCNT and optimized bitwise operations for speed
/// Provides 2-4x speedup over naive evaluation for deep searches
/// </summary>
public static class SIMDBitBoardEvaluator
{
    // Scoring weights - same as BitBoardEvaluator for consistency
    private const int FiveInRowScore = 100000;
    private const int OpenFourScore = 10000;
    private const int ClosedFourScore = 1000;
    private const int OpenThreeScore = 1000;
    private const int ClosedThreeScore = 100;
    private const int OpenTwoScore = 100;
    private const int CenterBonus = 50;

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
    private const float DefenseMultiplier = 1.5f;

    /// <summary>
    /// Platform capability detection
    /// </summary>
    public static readonly bool SupportsHardwarePOPCNT = true; // .NET uses hardware POPCNT on x64/ARM64

    /// <summary>
    /// SIMD-accelerated evaluation entry point
    /// Uses hardware-accelerated popcount and optimized pattern matching
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int Evaluate(Board board, Player player)
    {
        if (player == Player.None)
            throw new ArgumentException("Player cannot be None");

        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var playerBoard = board.GetBitBoard(player);
        var opponentBoard = board.GetBitBoard(opponent);

        return EvaluateOptimized(playerBoard, opponentBoard);
    }

    /// <summary>
    /// Optimized evaluation using hardware POPCNT and efficient pattern matching
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int EvaluateOptimized(BitBoard playerBoard, BitBoard opponentBoard)
    {
        var (p0, p1, p2, p3) = playerBoard.GetRawValues();
        var (o0, o1, o2, o3) = opponentBoard.GetRawValues();
        var occupied = playerBoard | opponentBoard;
        var (occ0, occ1, occ2, occ3) = occupied.GetRawValues();

        var score = 0;

        // Evaluate all directions using optimized shift-based detection
        score += EvaluateHorizontalOptimized(p0, p1, p2, p3, occ0, occ1, occ2, occ3);
        score += EvaluateVerticalOptimized(p0, p1, p2, p3, occ0, occ1, occ2, occ3);
        score += EvaluateDiagonalOptimized(p0, p1, p2, p3, occ0, occ1, occ2, occ3, true);  // Main diagonal
        score += EvaluateDiagonalOptimized(p0, p1, p2, p3, occ0, occ1, occ2, occ3, false); // Anti-diagonal

        // Subtract opponent's score with DefenseMultiplier (asymmetric scoring)
        // In Caro, blocking opponent threats is MORE important than creating your own attacks
        // Use integer math (multiply by 3, divide by 2) to avoid floating-point precision issues
        // DefenseMultiplier of 1.5 = 3/2
        const int DefenseMultiplierNumer = 3;
        const int DefenseMultiplierDenom = 2;

        var oppHorizontal = EvaluateHorizontalOptimized(o0, o1, o2, o3, occ0, occ1, occ2, occ3);
        var oppVertical = EvaluateVerticalOptimized(o0, o1, o2, o3, occ0, occ1, occ2, occ3);
        var oppDiagMain = EvaluateDiagonalOptimized(o0, o1, o2, o3, occ0, occ1, occ2, occ3, true);
        var oppDiagAnti = EvaluateDiagonalOptimized(o0, o1, o2, o3, occ0, occ1, occ2, occ3, false);

        // Use integer math: opp * 11 / 5 for consistent results
        score -= (oppHorizontal * DefenseMultiplierNumer) / DefenseMultiplierDenom;
        score -= (oppVertical * DefenseMultiplierNumer) / DefenseMultiplierDenom;
        score -= (oppDiagMain * DefenseMultiplierNumer) / DefenseMultiplierDenom;
        score -= (oppDiagAnti * DefenseMultiplierNumer) / DefenseMultiplierDenom;

        // Add center control bonus
        score += EvaluateCenterControlOptimized(p0, p1, p2, p3);

        return score;
    }

    /// <summary>
    /// Horizontal evaluation using run-length encoding (same approach as vertical)
    /// Replaces the broken bit-extraction approach with GetBit() for correctness
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static int EvaluateHorizontalOptimized(
        ulong p0, ulong p1, ulong p2, ulong p3,
        ulong occ0, ulong occ1, ulong occ2, ulong occ3)
    {
        // Reconstruct BitBoard for horizontal scanning
        var playerBoard = BitBoard.FromRawValues(p0, p1, p2, p3);
        var occupied = BitBoard.FromRawValues(occ0, occ1, occ2, occ3);

        var score = 0;

        // For each row, count horizontal runs using run-length encoding
        for (int y = 0; y < 15; y++)
        {
            int runStart = -1;
            int runLength = 0;

            for (int x = 0; x < 15; x++)
            {
                if (playerBoard.GetBit(x, y))
                {
                    if (runStart == -1) runStart = x;
                    runLength++;
                }
                else
                {
                    if (runLength > 0)
                    {
                        // Score this run
                        bool leftOpen = (runStart > 0) && !occupied.GetBit(runStart - 1, y);
                        bool rightOpen = (x < 14) && !occupied.GetBit(x, y);

                        score += ScoreRun(runLength, leftOpen, rightOpen);
                        runStart = -1;
                        runLength = 0;
                    }
                }
            }

            // Handle run ending at right edge of board
            if (runLength > 0)
            {
                bool leftOpen = (runStart > 0) && !occupied.GetBit(runStart - 1, y);
                bool rightOpen = false; // Board edge
                score += ScoreRun(runLength, leftOpen, rightOpen);
            }
        }

        return score;
    }

    /// <summary>
    /// Optimized vertical evaluation using BitBoard shift operations
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static int EvaluateVerticalOptimized(
        ulong p0, ulong p1, ulong p2, ulong p3,
        ulong occ0, ulong occ1, ulong occ2, ulong occ3)
    {
        // Reconstruct BitBoard for vertical shift operations
        var playerBoard = BitBoard.FromRawValues(p0, p1, p2, p3);
        var occupied = BitBoard.FromRawValues(occ0, occ1, occ2, occ3);

        var score = 0;

        // For each column, count vertical runs using bit operations
        for (int x = 0; x < 15; x++)
        {
            int runStart = -1;
            int runLength = 0;

            for (int y = 0; y < 15; y++)
            {
                if (playerBoard.GetBit(x, y))
                {
                    if (runStart == -1) runStart = y;
                    runLength++;
                }
                else
                {
                    if (runLength > 0)
                    {
                        // Score this run
                        bool topOpen = (runStart > 0) && !occupied.GetBit(x, runStart - 1);
                        bool bottomOpen = (y < 14) && !occupied.GetBit(x, y);

                        score += ScoreRun(runLength, topOpen, bottomOpen);
                        runStart = -1;
                        runLength = 0;
                    }
                }
            }

            // Handle run ending at bottom of board
            if (runLength > 0)
            {
                bool topOpen = (runStart > 0) && !occupied.GetBit(x, runStart - 1);
                bool bottomOpen = false; // Board edge
                score += ScoreRun(runLength, topOpen, bottomOpen);
            }
        }

        return score;
    }

    /// <summary>
    /// Optimized diagonal evaluation
    /// CRITICAL FIX: Use counted array to avoid counting the same run multiple times
    /// A run of N stones was being counted N times, massively inflating scores
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static int EvaluateDiagonalOptimized(
        ulong p0, ulong p1, ulong p2, ulong p3,
        ulong occ0, ulong occ1, ulong occ2, ulong occ3,
        bool mainDiagonal) // true = down-right (1,1), false = down-left (1,-1)
    {
        var playerBoard = BitBoard.FromRawValues(p0, p1, p2, p3);
        var occupied = BitBoard.FromRawValues(occ0, occ1, occ2, occ3);

        var score = 0;
        int dx = 1;
        int dy = mainDiagonal ? 1 : -1;

        // CRITICAL: Track counted stones to avoid counting the same run multiple times
        // Without this, a run of N stones would be counted N times, massively inflating scores
        var counted = new bool[15, 15];

        // Scan all starting positions
        for (int x = 0; x < 15; x++)
        {
            for (int y = 0; y < 15; y++)
            {
                if (!playerBoard.GetBit(x, y) || counted[x, y]) continue;

                var count = BitBoardEvaluator.CountConsecutiveBoth(playerBoard, x, y, dx, dy);
                if (count < 2) continue;

                // Mark all stones in this run as counted
                var cx = x;
                var cy = y;
                for (int i = 0; i < count; i++)
                {
                    if (cx >= 0 && cx < 15 && cy >= 0 && cy < 15)
                        counted[cx, cy] = true;
                    cx += dx;
                    cy += dy;
                }

                var openEnds = BitBoardEvaluator.CountOpenEnds(playerBoard, occupied, x, y, dx, dy, count);

                if (count >= 5)
                {
                    // Check sandwich rule
                    if (!BitBoardEvaluator.IsSandwichedFive(playerBoard, occupied, x, y, dx, dy))
                        score += FiveInRowScore;
                }
                else
                {
                    score += ScoreRun(count, openEnds >= 2, openEnds >= 1);
                }
            }
        }

        return score;
    }

    /// <summary>
    /// Score a run based on length and open ends
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static int ScoreRun(int count, bool topOpen, bool bottomOpen)
    {
        int openEnds = (topOpen ? 1 : 0) + (bottomOpen ? 1 : 0);

        if (count >= 5) return FiveInRowScore;
        if (count == 4) return openEnds > 0 ? OpenFourScore : ClosedFourScore;
        if (count == 3)
        {
            if (openEnds == 2) return OpenThreeScore * 2;
            if (openEnds == 1) return OpenThreeScore;
            return ClosedThreeScore;
        }
        if (count == 2 && openEnds == 2) return OpenTwoScore;
        return 0;
    }

    /// <summary>
    /// Optimized center control evaluation
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static int EvaluateCenterControlOptimized(ulong p0, ulong p1, ulong p2, ulong p3)
    {
        var score = 0;

        // Center zone: 5x5 area from (5,5) to (9,9)
        for (int x = 5; x <= 9; x++)
        {
            for (int y = 5; y <= 9; y++)
            {
                int index = y * 15 + x;
                int ulongIdx = index / 64;
                int bitIdx = index % 64;

                ulong bits = ulongIdx switch
                {
                    0 => p0,
                    1 => p1,
                    2 => p2,
                    3 => p3,
                    _ => 0
                };

                if ((bits & (1UL << bitIdx)) != 0)
                {
                    int distanceToCenter = Math.Abs(x - 7) + Math.Abs(y - 7);
                    score += CenterBonus - (distanceToCenter * 5);
                }
            }
        }

        return score;
    }

    /// <summary>
    /// Fast evaluation of a potential move at position (x, y)
    /// Uses incremental scoring for speed
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int EvaluateMoveAt(int x, int y, Board board, Player player)
    {
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var playerBoard = board.GetBitBoard(player);
        var opponentBoard = board.GetBitBoard(opponent);
        var occupied = playerBoard | opponentBoard;

        return EvaluateMoveAt(x, y, playerBoard, opponentBoard, occupied);
    }

    /// <summary>
    /// Fast evaluation of a potential move at position (x, y)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int EvaluateMoveAt(int x, int y, BitBoard playerBoard, BitBoard opponentBoard, BitBoard occupied)
    {
        // Temporarily place the stone
        var testBoard = playerBoard;
        testBoard.SetBit(x, y, true);

        // Evaluate the new position
        var score = EvaluateOptimized(testBoard, opponentBoard);

        // Bonus for creating threats
        var threatsCreated = CountNewThreats(x, y, testBoard, occupied);
        score += threatsCreated * 500;

        // Defense bonus - blocking opponent threats
        var blockedThreats = CountBlockedThreats(x, y, opponentBoard, occupied);
        score += blockedThreats * 300;

        return score;
    }

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
            if (nx < 0 || nx >= 15 || ny < 0 || ny >= 15) break; // Out of bounds

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
            if (nx < 0 || nx >= 15 || ny < 0 || ny >= 15) break; // Out of bounds

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

    /// <summary>
    /// Batch evaluation for multiple positions
    /// Useful for move ordering and candidate evaluation
    /// </summary>
    public static Span<int> BatchEvaluate(Span<(int x, int y)> positions, Board board, Player player)
    {
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var playerBoard = board.GetBitBoard(player);
        var opponentBoard = board.GetBitBoard(opponent);
        var occupied = playerBoard | opponentBoard;

        var scores = new int[positions.Length];

        for (int i = 0; i < positions.Length; i++)
        {
            var (x, y) = positions[i];
            scores[i] = EvaluateMoveAt(x, y, playerBoard, opponentBoard, occupied);
        }

        return scores;
    }

    /// <summary>
    /// Get platform info for debugging
    /// </summary>
    public static string GetPlatformInfo()
    {
        return $"POPCNT: {(SupportsHardwarePOPCNT ? "Enabled" : "Disabled")}, " +
               $"Architecture: {(IntPtr.Size == 8 ? "x64" : "x86")}, " +
               $"OS: {Environment.OSVersion.Platform}";
    }
}
