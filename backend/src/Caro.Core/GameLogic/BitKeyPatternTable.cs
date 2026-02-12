using System.Runtime.CompilerServices;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Pattern lookup table for O(1) pattern evaluation using BitKey keys.
/// Each 64-bit key represents a pattern of cells, and this table maps
/// those keys to pattern types and scores.
///
/// Pattern encoding (2 bits per cell, 12-cell window = 24 bits used):
/// - 00 = empty
/// - 01 = Red (current player)
/// - 10 = Blue (opponent)
///
/// The table uses a pre-computed lookup approach with hash-based indexing.
/// </summary>
public static class BitKeyPatternTable
{
    // Pattern window: 12 cells centered on the position (6 on each side)
    private const int PatternBits = 24;  // 12 cells * 2 bits = 24 bits used
    private const int TableSize = 1 << PatternBits;  // 16M entries

    // Scoring weights - aligned with BitBoardEvaluator
    private const int FiveInRowScore = 100000;
    private const int OpenFourScore = 10000;
    private const int ClosedFourScore = 1000;
    private const int OpenThreeScore = 1000;
    private const int ClosedThreeScore = 100;
    private const int OpenTwoScore = 100;

    // Pre-computed pattern tables (lazy initialization)
    private static readonly Lazy<PatternEntry[]> _patternTable = new(InitializePatternTable);

    /// <summary>
    /// Pattern entry containing evaluation info for a pattern.
    /// </summary>
    public readonly struct PatternEntry
    {
        public readonly Pattern4Evaluator.CaroPattern4 PatternType;
        public readonly short Score;
        public readonly byte StoneCount;
        public readonly byte OpenEnds;
        public readonly bool IsWinning;

        public PatternEntry(Pattern4Evaluator.CaroPattern4 patternType, short score, byte stoneCount, byte openEnds, bool isWinning)
        {
            PatternType = patternType;
            Score = score;
            StoneCount = stoneCount;
            OpenEnds = openEnds;
            IsWinning = isWinning;
        }
    }

    /// <summary>
    /// Get the pattern entry for a 64-bit key.
    /// Uses the lower 24 bits for table lookup.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static PatternEntry GetPattern(ulong key)
    {
        int index = (int)(key & 0xFFFFFF);  // Lower 24 bits
        return _patternTable.Value[index];
    }

    /// <summary>
    /// Get the score for a 64-bit key directly.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int GetScore(ulong key)
    {
        return GetPattern(key).Score;
    }

    /// <summary>
    /// Get the pattern type for a 64-bit key.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static Pattern4Evaluator.CaroPattern4 GetPatternType(ulong key)
    {
        return GetPattern(key).PatternType;
    }

    /// <summary>
    /// Initialize the pattern lookup table.
    /// </summary>
    private static PatternEntry[] InitializePatternTable()
    {
        var table = new PatternEntry[TableSize];

        // Initialize all entries
        for (int i = 0; i < TableSize; i++)
        {
            table[i] = AnalyzePattern((ulong)i);
        }

        return table;
    }

    /// <summary>
    /// Analyze a 24-bit pattern and return the pattern entry.
    /// </summary>
    private static PatternEntry AnalyzePattern(ulong pattern)
    {
        // Extract cells from the pattern (12 cells, 2 bits each)
        Span<int> cells = stackalloc int[12];
        for (int i = 0; i < 12; i++)
        {
            cells[i] = (int)((pattern >> (i * 2)) & 0x3);
        }

        // The center of the pattern is at index 6 (position being evaluated)
        // Count consecutive stones of each type through the center
        int redCount = 0;
        int blueCount = 0;
        int centerCell = cells[6];

        // If center is empty, check what patterns could be formed
        if (centerCell == 0)
        {
            // Empty center - return based on nearby patterns
            return new PatternEntry(Pattern4Evaluator.CaroPattern4.None, 0, 0, 0, false);
        }

        // Count consecutive stones of the center's type
        int playerBit = centerCell;
        int count = 1;  // Include center
        int openEnds = 0;

        // Count to the left (lower indices)
        for (int i = 5; i >= 0; i--)
        {
            if (cells[i] == playerBit)
                count++;
            else if (cells[i] == 0)
            {
                openEnds++;
                break;
            }
            else
                break;  // Blocked by opponent
        }

        // Count to the right (higher indices)
        for (int i = 7; i < 12; i++)
        {
            if (cells[i] == playerBit)
                count++;
            else if (cells[i] == 0)
            {
                openEnds++;
                break;
            }
            else
                break;  // Blocked by opponent
        }

        // Determine pattern type and score
        var (patternType, score) = ClassifyPattern(count, openEnds, playerBit == 1);

        return new PatternEntry(
            patternType,
            score,
            (byte)count,
            (byte)openEnds,
            patternType == Pattern4Evaluator.CaroPattern4.Exactly5
        );
    }

    /// <summary>
    /// Classify a pattern based on stone count and open ends.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static (Pattern4Evaluator.CaroPattern4 PatternType, short Score) ClassifyPattern(int count, int openEnds, bool isRed)
    {
        // Score multiplier (defensive: opponent threats scored higher)
        short multiplier = isRed ? (short)1 : (short)-1;

        if (count >= 5)
        {
            // Exactly 5 is a win in Caro
            return (Pattern4Evaluator.CaroPattern4.Exactly5, (short)(FiveInRowScore * multiplier));
        }

        if (count == 4)
        {
            if (openEnds >= 2)
                return (Pattern4Evaluator.CaroPattern4.Flex4, (short)(OpenFourScore * multiplier));
            if (openEnds == 1)
                return (Pattern4Evaluator.CaroPattern4.Block4, (short)(ClosedFourScore * multiplier));
            return (Pattern4Evaluator.CaroPattern4.Block4, (short)(ClosedFourScore / 2 * multiplier));
        }

        if (count == 3)
        {
            if (openEnds >= 2)
                return (Pattern4Evaluator.CaroPattern4.Flex3, (short)(OpenThreeScore * multiplier));
            if (openEnds == 1)
                return (Pattern4Evaluator.CaroPattern4.Block3, (short)(ClosedThreeScore * multiplier));
            return (Pattern4Evaluator.CaroPattern4.Block3, (short)(ClosedThreeScore / 2 * multiplier));
        }

        if (count == 2)
        {
            if (openEnds >= 2)
                return (Pattern4Evaluator.CaroPattern4.Flex2, (short)(OpenTwoScore * multiplier));
            if (openEnds == 1)
                return (Pattern4Evaluator.CaroPattern4.Block2, (short)(OpenTwoScore / 2 * multiplier));
            return (Pattern4Evaluator.CaroPattern4.Block2, 0);
        }

        if (count == 1)
        {
            if (openEnds >= 2)
                return (Pattern4Evaluator.CaroPattern4.Flex1, (short)10);
            return (Pattern4Evaluator.CaroPattern4.Block1, 0);
        }

        return (Pattern4Evaluator.CaroPattern4.None, 0);
    }

    /// <summary>
    /// Evaluate all four directions at a position and return combined score.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int EvaluatePosition(BitKeyBoard board, int x, int y)
    {
        var (h, v, d, ad) = board.GetAllKeysAt(x, y);

        int score = 0;
        score += GetScore(h);
        score += GetScore(v);
        score += GetScore(d);
        score += GetScore(ad);

        return score;
    }

    /// <summary>
    /// Get combined pattern classification for all four directions.
    /// This is used for double-threat detection.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static (Pattern4Evaluator.CaroPattern4 Combined, int ThreatCount) GetCombinedPattern(BitKeyBoard board, int x, int y)
    {
        var (h, v, d, ad) = board.GetAllKeysAt(x, y);

        var hPattern = GetPatternType(h);
        var vPattern = GetPatternType(v);
        var dPattern = GetPatternType(d);
        var adPattern = GetPatternType(ad);

        // Count threats (Flex3 and above)
        int threatCount = 0;
        if (IsThreat(hPattern)) threatCount++;
        if (IsThreat(vPattern)) threatCount++;
        if (IsThreat(dPattern)) threatCount++;
        if (IsThreat(adPattern)) threatCount++;

        // Check for winning combinations
        bool hasFlex4 = hPattern == Pattern4Evaluator.CaroPattern4.Flex4 || vPattern == Pattern4Evaluator.CaroPattern4.Flex4 ||
                        dPattern == Pattern4Evaluator.CaroPattern4.Flex4 || adPattern == Pattern4Evaluator.CaroPattern4.Flex4;
        bool hasFlex3 = hPattern == Pattern4Evaluator.CaroPattern4.Flex3 || vPattern == Pattern4Evaluator.CaroPattern4.Flex3 ||
                        dPattern == Pattern4Evaluator.CaroPattern4.Flex3 || adPattern == Pattern4Evaluator.CaroPattern4.Flex3;
        bool hasFive = hPattern == Pattern4Evaluator.CaroPattern4.Exactly5 || vPattern == Pattern4Evaluator.CaroPattern4.Exactly5 ||
                       dPattern == Pattern4Evaluator.CaroPattern4.Exactly5 || adPattern == Pattern4Evaluator.CaroPattern4.Exactly5;

        if (hasFive)
            return (Pattern4Evaluator.CaroPattern4.Exactly5, threatCount);

        // Double threat detection
        if (hasFlex4 && hasFlex3)
            return (Pattern4Evaluator.CaroPattern4.Flex4Flex3, threatCount);

        int flex3Count = CountPattern(hPattern, vPattern, dPattern, adPattern, Pattern4Evaluator.CaroPattern4.Flex3);
        if (flex3Count >= 2)
            return (Pattern4Evaluator.CaroPattern4.DoubleFlex3, threatCount);

        if (hasFlex4)
            return (Pattern4Evaluator.CaroPattern4.Flex4, threatCount);

        if (hasFlex3)
            return (Pattern4Evaluator.CaroPattern4.Flex3, threatCount);

        // Return the highest pattern found
        Pattern4Evaluator.CaroPattern4 highest = MaxPattern(hPattern, vPattern, dPattern, adPattern);
        return (highest, threatCount);
    }

    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static bool IsThreat(Pattern4Evaluator.CaroPattern4 pattern)
    {
        return pattern >= Pattern4Evaluator.CaroPattern4.Flex3;
    }

    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static int CountPattern(Pattern4Evaluator.CaroPattern4 p1, Pattern4Evaluator.CaroPattern4 p2, Pattern4Evaluator.CaroPattern4 p3, Pattern4Evaluator.CaroPattern4 p4, Pattern4Evaluator.CaroPattern4 target)
    {
        int count = 0;
        if (p1 == target) count++;
        if (p2 == target) count++;
        if (p3 == target) count++;
        if (p4 == target) count++;
        return count;
    }

    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static Pattern4Evaluator.CaroPattern4 MaxPattern(Pattern4Evaluator.CaroPattern4 p1, Pattern4Evaluator.CaroPattern4 p2, Pattern4Evaluator.CaroPattern4 p3, Pattern4Evaluator.CaroPattern4 p4)
    {
        Pattern4Evaluator.CaroPattern4 max = p1;
        if (p2 > max) max = p2;
        if (p3 > max) max = p3;
        if (p4 > max) max = p4;
        return max;
    }

    /// <summary>
    /// Check if a move at (x, y) would create a winning position.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static bool IsWinningMove(BitKeyBoard board, int x, int y, Player player)
    {
        // Temporarily place the stone
        var testBoard = board.Clone();
        testBoard.SetBit(x, y, player);

        var (combined, _) = GetCombinedPattern(testBoard, x, y);
        return combined == Pattern4Evaluator.CaroPattern4.Exactly5;
    }

    /// <summary>
    /// Check if a move at (x, y) would create a double threat.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static bool IsDoubleThreatMove(BitKeyBoard board, int x, int y, Player player)
    {
        // Temporarily place the stone
        var testBoard = board.Clone();
        testBoard.SetBit(x, y, player);

        var (combined, threatCount) = GetCombinedPattern(testBoard, x, y);
        return combined == Pattern4Evaluator.CaroPattern4.DoubleFlex3 || combined == Pattern4Evaluator.CaroPattern4.Flex4Flex3 || threatCount >= 2;
    }
}
