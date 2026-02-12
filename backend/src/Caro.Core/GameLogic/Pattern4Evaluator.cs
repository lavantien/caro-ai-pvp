using System.Runtime.CompilerServices;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// 4-direction combined pattern evaluation system based on Rapfi's Pattern4.
/// Classifies positions by threat level combining patterns from all directions.
/// 
/// Key improvements over single-direction evaluation:
/// 1. Double-threat detection (e.g., DoubleFlex3, Flex4Flex3)
/// 2. Better tactical awareness through combined classification
/// 3. Caro-specific handling (exactly-5 rule, no overline wins)
/// </summary>
public static class Pattern4Evaluator
{
    // Direction vectors: horizontal, vertical, 2 diagonals
    private static readonly (int dx, int dy)[] Directions = new[]
    {
        (1, 0),   // Horizontal
        (0, 1),   // Vertical
        (1, 1),   // Diagonal down-right
        (1, -1)   // Diagonal down-left
    };

    /// <summary>
    /// Combined pattern classification for a position.
    /// Based on Rapfi's Pattern4 enum with Caro-specific adaptations.
    /// </summary>
    public enum CaroPattern4 : byte
    {
        /// <summary>No significant pattern</summary>
        None = 0,
        /// <summary>Single stone with potential (1 in a row)</summary>
        Flex1 = 1,
        /// <summary>Single blocked stone</summary>
        Block1 = 2,
        /// <summary>Open two (2 in a row with space)</summary>
        Flex2 = 3,
        /// <summary>Blocked two</summary>
        Block2 = 4,
        /// <summary>Open three - creates forcing move (3 in a row, open)</summary>
        Flex3 = 5,
        /// <summary>Blocked three</summary>
        Block3 = 6,
        /// <summary>Open four - winning threat (4 in a row, open)</summary>
        Flex4 = 7,
        /// <summary>Blocked four (still threatening)</summary>
        Block4 = 8,
        /// <summary>Two open threes - winning combination</summary>
        DoubleFlex3 = 9,
        /// <summary>Open four + open three - winning combination</summary>
        Flex4Flex3 = 10,
        /// <summary>Exactly 5 - win condition</summary>
        Exactly5 = 11,
        /// <summary>6+ in a row - invalid in Caro (exactly-5 rule)</summary>
        Overline = 12
    }

    /// <summary>
    /// Single-direction pattern result for pattern combination.
    /// </summary>
    public struct DirectionPattern
    {
        public int Count;           // Number of consecutive stones
        public int OpenEnds;        // Number of open ends (0, 1, or 2)
        public bool HasGap;         // Whether there's a gap in the pattern
        public bool IsOverline;     // Whether this is 6+ (overline)
    }

    /// <summary>
    /// Evaluate combined pattern at a position for a player.
    /// Analyzes all 4 directions and combines into a single classification.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static CaroPattern4 EvaluatePosition(Board board, int x, int y, Player player)
    {
        if (player == Player.None)
            return CaroPattern4.None;

        var cell = board.GetCell(x, y);
        if (!cell.IsEmpty && cell.Player != player)
            return CaroPattern4.None;

        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var playerBoard = board.GetBitBoard(player);
        var opponentBoard = board.GetBitBoard(opponent);

        return EvaluatePositionBitBoard(playerBoard, opponentBoard, x, y);
    }

    /// <summary>
    /// Evaluate combined pattern using BitBoard operations.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static CaroPattern4 EvaluatePositionBitBoard(BitBoard playerBoard, BitBoard opponentBoard, int x, int y)
    {
        // Analyze each direction
        Span<DirectionPattern> patterns = stackalloc DirectionPattern[4];
        int flex3Count = 0;
        int flex4Count = 0;
        bool hasExactly5 = false;
        bool hasOverline = false;

        for (int dir = 0; dir < 4; dir++)
        {
            var (dx, dy) = Directions[dir];
            patterns[dir] = AnalyzeDirection(playerBoard, opponentBoard, x, y, dx, dy);

            // Count significant patterns for double-threat detection
            if (patterns[dir].Count >= 5 && !patterns[dir].IsOverline)
                hasExactly5 = true;
            if (patterns[dir].IsOverline)
                hasOverline = true;
            else if (patterns[dir].Count == 4 && patterns[dir].OpenEnds >= 1)
                flex4Count++;
            else if (patterns[dir].Count == 3 && patterns[dir].OpenEnds == 2)
                flex3Count++;
        }

        // Check win conditions first
        if (hasExactly5)
            return CaroPattern4.Exactly5;
        if (hasOverline && !hasExactly5)
            return CaroPattern4.Overline; // Invalid in Caro

        // Check double-threat combinations (winning)
        if (flex4Count >= 1 && flex3Count >= 1)
            return CaroPattern4.Flex4Flex3;
        if (flex4Count >= 2)
            return CaroPattern4.Flex4; // Double flex4 is already winning
        if (flex3Count >= 2)
            return CaroPattern4.DoubleFlex3;

        // Return highest single-direction threat
        if (flex4Count >= 1)
            return CaroPattern4.Flex4;
        if (flex3Count >= 1)
            return CaroPattern4.Flex3;

        // Check for blocked four, blocked three, etc.
        for (int dir = 0; dir < 4; dir++)
        {
            ref var p = ref patterns[dir];
            if (p.Count == 4 && p.OpenEnds == 0)
                return CaroPattern4.Block4;
            if (p.Count == 3 && p.OpenEnds == 1)
                return CaroPattern4.Block3;
            if (p.Count == 3 && p.OpenEnds == 0)
                return CaroPattern4.Block3;
        }

        // Lower patterns
        for (int dir = 0; dir < 4; dir++)
        {
            ref var p = ref patterns[dir];
            if (p.Count == 2 && p.OpenEnds == 2)
                return CaroPattern4.Flex2;
            if (p.Count == 2 && p.OpenEnds >= 1)
                return CaroPattern4.Block2;
        }

        return CaroPattern4.None;
    }

    /// <summary>
    /// Analyze a single direction from a position.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static DirectionPattern AnalyzeDirection(BitBoard playerBoard, BitBoard opponentBoard, int x, int y, int dx, int dy)
    {
        var pattern = new DirectionPattern();
        var occupied = playerBoard | opponentBoard;

        // Count consecutive stones in positive direction
        int count = 0;
        int px = x, py = y;

        // If there's a stone at (x,y), start counting from there
        if (playerBoard.GetBit(x, y))
        {
            count = 1;
            // Count forward
            px = x + dx;
            py = y + dy;
            while (IsValidPosition(px, py) && playerBoard.GetBit(px, py))
            {
                count++;
                px += dx;
                py += dy;
            }
            // Count backward
            px = x - dx;
            py = y - dy;
            while (IsValidPosition(px, py) && playerBoard.GetBit(px, py))
            {
                count++;
                px -= dx;
                py -= dy;
            }
        }

        pattern.Count = count;

        // Check for overline (6+)
        pattern.IsOverline = count >= 6;

        // Count open ends
        if (count > 0)
        {
            // Find the ends of the sequence
            int startX = x, startY = y;
            int endX = x, endY = y;

            // Find start (go backward from x,y)
            int sx = x - dx, sy = y - dy;
            while (IsValidPosition(sx, sy) && playerBoard.GetBit(sx, sy))
            {
                startX = sx;
                startY = sy;
                sx -= dx;
                sy -= dy;
            }

            // Find end (go forward from x,y)
            int ex = x + dx, ey = y + dy;
            while (IsValidPosition(ex, ey) && playerBoard.GetBit(ex, ey))
            {
                endX = ex;
                endY = ey;
                ex += dx;
                ey += dy;
            }

            // Check open ends
            bool openStart = IsValidPosition(startX - dx, startY - dy) && !occupied.GetBit(startX - dx, startY - dy);
            bool openEnd = IsValidPosition(endX + dx, endY + dy) && !occupied.GetBit(endX + dx, endY + dy);

            pattern.OpenEnds = (openStart ? 1 : 0) + (openEnd ? 1 : 0);
        }

        return pattern;
    }

    /// <summary>
    /// Get score for a pattern type (for move ordering and evaluation).
    /// Higher scores indicate more valuable patterns.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int GetPatternScore(CaroPattern4 pattern)
    {
        return pattern switch
        {
            CaroPattern4.Exactly5 => 1000000,
            CaroPattern4.Flex4Flex3 => 500000,
            CaroPattern4.DoubleFlex3 => 400000,
            CaroPattern4.Flex4 => 100000,
            CaroPattern4.Block4 => 10000,
            CaroPattern4.Flex3 => 5000,
            CaroPattern4.Block3 => 500,
            CaroPattern4.Flex2 => 100,
            CaroPattern4.Block2 => 50,
            CaroPattern4.Flex1 => 10,
            CaroPattern4.Block1 => 5,
            CaroPattern4.Overline => -1000, // Penalty in Caro
            _ => 0
        };
    }

    /// <summary>
    /// Check if a pattern is a winning threat (requires immediate response).
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static bool IsWinningThreat(CaroPattern4 pattern)
    {
        return pattern >= CaroPattern4.Flex4 && pattern <= CaroPattern4.Exactly5;
    }

    /// <summary>
    /// Check if a pattern is a forcing threat (creates immediate pressure).
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static bool IsForcingThreat(CaroPattern4 pattern)
    {
        return pattern >= CaroPattern4.Flex3;
    }

    /// <summary>
    /// Find all positions with the specified pattern type for a player.
    /// </summary>
    public static List<(int x, int y, CaroPattern4 pattern)> FindPatternPositions(Board board, Player player, CaroPattern4 minPattern = CaroPattern4.Flex3)
    {
        var result = new List<(int x, int y, CaroPattern4 pattern)>();
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var playerBoard = board.GetBitBoard(player);
        var opponentBoard = board.GetBitBoard(opponent);

        for (int x = 0; x < BitBoard.Size; x++)
        {
            for (int y = 0; y < BitBoard.Size; y++)
            {
                // Check player's existing stones
                if (playerBoard.GetBit(x, y))
                {
                    var pattern = EvaluatePositionBitBoard(playerBoard, opponentBoard, x, y);
                    if (pattern >= minPattern)
                    {
                        result.Add((x, y, pattern));
                    }
                }
            }
        }

        // Sort by pattern score (descending)
        result.Sort((a, b) => GetPatternScore(b.pattern).CompareTo(GetPatternScore(a.pattern)));
        return result;
    }

    /// <summary>
    /// Check if a position is valid on the board.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static bool IsValidPosition(int x, int y)
    {
        return x >= 0 && x < BitBoard.Size && y >= 0 && y < BitBoard.Size;
    }
}
