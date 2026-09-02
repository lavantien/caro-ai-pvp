using Caro.Domain;

namespace Caro.Engine;

public enum Pattern4
{
    // Values must stay distinct: the previous enum aliased P4Flex3 with
    // P4Block4 and P4Overline with P4None, which silently corrupted
    // equality-based checks.
    None = 0,
    Flex1 = 1,
    Flex2 = 3,
    Block2 = 4,
    Flex3 = 5,
    Block3 = 6,
    Flex4 = 7,
    Block4 = 8,
    Exactly5 = 9,
    Overline = 10,
}

public struct PlayerPattern4
{
    public int Exactly5Count { get; set; }
    public int Flex4Count { get; set; }
    public int Block4Count { get; set; }
    public int Flex3Count { get; set; }
    public int Block3Count { get; set; }
    public int Flex2Count { get; set; }
    public int Block2Count { get; set; }
}

public static class Pattern4Classifier
{
    internal static readonly (int Dx, int Dy)[] EvalDirs =
    [
        (1, 0),
        (0, 1),
        (1, 1),
        (1, -1),
    ];

    /// <summary>
    /// Classifies the pattern the stone at (x,y) participates in along
    /// (dx,dy), gap-aware: split fours and broken threes count like their
    /// straight equivalents.
    /// </summary>
    internal static Pattern4 ClassifyDirection(SearchBoard sb, int x, int y, int dx, int dy, Player player)
    {
        Span<sbyte> line = stackalloc sbyte[Constants.LineLength];
        PatternWindow.ExtractLine(sb, x, y, player, dx, dy, line);

        PatternWindow.SpanThrough(line, -1, out int lo, out int hi);
        if (hi - lo + 1 > Constants.WinLength)
        {
            return Pattern4.Overline;
        }
        if (PatternWindow.SpanIsFive(line, lo, hi))
        {
            return Pattern4.Exactly5;
        }

        int comps = PatternWindow.LineCompletions(line);
        if (comps == 1)
        {
            return Pattern4.Block4;
        }
        if (comps > 1)
        {
            return Pattern4.Flex4;
        }

        int maxComps = PatternWindow.MaxCompsAfterFill(line);
        if (maxComps == 1)
        {
            return Pattern4.Block3;
        }
        if (maxComps > 1)
        {
            return Pattern4.Flex3;
        }

        // Twos and singles: contiguous counting is sufficient.
        int positive = 0;
        bool positiveOpen = false;
        for (int i = 1; i <= 2; i++)
        {
            int nx = x + dx * i;
            int ny = y + dy * i;
            if (nx < 0 || nx >= Constants.BoardSize || ny < 0 || ny >= Constants.BoardSize)
            {
                break;
            }
            Player p = sb.PlayerAt(nx, ny);
            if (p == player)
            {
                positive++;
            }
            else if (p == Player.None)
            {
                positiveOpen = true;
                break;
            }
            else
            {
                break;
            }
        }

        int negative = 0;
        bool negativeOpen = false;
        for (int i = 1; i <= 2; i++)
        {
            int nx = x - dx * i;
            int ny = y - dy * i;
            if (nx < 0 || nx >= Constants.BoardSize || ny < 0 || ny >= Constants.BoardSize)
            {
                break;
            }
            Player p = sb.PlayerAt(nx, ny);
            if (p == player)
            {
                negative++;
            }
            else if (p == Player.None)
            {
                negativeOpen = true;
                break;
            }
            else
            {
                break;
            }
        }

        int count = 1 + positive + negative;
        if (count >= 3)
        {
            return Pattern4.None;
        }
        int openEnds = 0;
        if (positiveOpen)
        {
            openEnds++;
        }
        if (negativeOpen)
        {
            openEnds++;
        }

        if (count == 2)
        {
            return openEnds switch
            {
                2 => Pattern4.Flex2,
                1 => Pattern4.Block2,
                _ => Pattern4.None,
            };
        }
        if (count == 1)
        {
            return Pattern4.Flex1;
        }
        return Pattern4.None;
    }

    /// <summary>
    /// Classifies all 4-direction patterns for a single stone. Only
    /// processes each line once (from the starting stone) by skipping
    /// directions where a same-color stone precedes the current one. Shapes
    /// below four whose cluster is anchored by a same-color stone two cells
    /// back are also skipped to avoid double counting gapped clusters (XX.X
    /// anchors at its leftmost stone).
    /// </summary>
    public static PlayerPattern4 ClassifyStone(SearchBoard sb, int x, int y, Player player)
    {
        PlayerPattern4 pp = default;
        foreach ((int dx, int dy) in EvalDirs)
        {
            int px = x - dx;
            int py = y - dy;
            if (px >= 0 && px < Constants.BoardSize && py >= 0 && py < Constants.BoardSize
                && sb.PlayerAt(px, py) == player)
            {
                continue;
            }

            int p2x = x - 2 * dx;
            int p2y = y - 2 * dy;
            bool clusterAnchored = p2x >= 0 && p2x < Constants.BoardSize && p2y >= 0 && p2y < Constants.BoardSize
                && sb.PlayerAt(p2x, p2y) == player;

            Pattern4 @class = ClassifyDirection(sb, x, y, dx, dy, player);
            if (clusterAnchored && @class != Pattern4.Exactly5 && @class != Pattern4.Flex4)
            {
                continue;
            }
            switch (@class)
            {
                case Pattern4.Exactly5: pp.Exactly5Count++; break;
                case Pattern4.Flex4: pp.Flex4Count++; break;
                case Pattern4.Block4: pp.Block4Count++; break;
                case Pattern4.Flex3: pp.Flex3Count++; break;
                case Pattern4.Block3: pp.Block3Count++; break;
                case Pattern4.Flex2: pp.Flex2Count++; break;
                case Pattern4.Block2: pp.Block2Count++; break;
            }
        }
        return pp;
    }

    /// <summary>Classifies all patterns for a player across the entire board.</summary>
    public static PlayerPattern4 ClassifyBoard(SearchBoard sb, Player player)
    {
        PlayerPattern4 total = default;
        for (int x = 0; x < Constants.BoardSize; x++)
        {
            for (int y = 0; y < Constants.BoardSize; y++)
            {
                if (sb.PlayerAt(x, y) != player)
                {
                    continue;
                }
                PlayerPattern4 pp = ClassifyStone(sb, x, y, player);
                total.Exactly5Count += pp.Exactly5Count;
                total.Flex3Count += pp.Flex3Count;
                total.Flex4Count += pp.Flex4Count;
                total.Block4Count += pp.Block4Count;
                total.Block3Count += pp.Block3Count;
                total.Flex2Count += pp.Flex2Count;
                total.Block2Count += pp.Block2Count;
            }
        }
        return total;
    }

    /// <summary>Returns true if a single move creates two or more open threes.</summary>
    internal static bool HasDoubleFlex3(SearchBoard sb, int x, int y, Player player)
    {
        sb.MakeMove(x, y, player);
        try
        {
            int flex3Count = 0;
            foreach ((int dx, int dy) in EvalDirs)
            {
                if (ClassifyDirection(sb, x, y, dx, dy, player) == Pattern4.Flex3)
                {
                    flex3Count++;
                }
            }
            return flex3Count >= 2;
        }
        finally
        {
            sb.UnmakeMove();
        }
    }

    /// <summary>Returns true if a single move creates both open four and open three.</summary>
    internal static bool HasFlex4PlusFlex3(SearchBoard sb, int x, int y, Player player)
    {
        sb.MakeMove(x, y, player);
        try
        {
            bool flex4 = false;
            bool flex3 = false;
            foreach ((int dx, int dy) in EvalDirs)
            {
                Pattern4 p = ClassifyDirection(sb, x, y, dx, dy, player);
                if (p == Pattern4.Flex4)
                {
                    flex4 = true;
                }
                if (p == Pattern4.Flex3)
                {
                    flex3 = true;
                }
            }
            return flex4 && flex3;
        }
        finally
        {
            sb.UnmakeMove();
        }
    }
}
