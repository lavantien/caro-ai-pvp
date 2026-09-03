using Caro.Domain;

namespace Caro.Engine;

/// <summary>
/// Gap-aware threat primitives. A completion is an empty cell whose single
/// fill turns the line through a player's stone into an exact five (Caro
/// rules: exactly five stones, not both ends blocked, no overline
/// extension). Split shapes like XX.XX and .XX.X. participate, unlike plain
/// contiguous counting.
/// </summary>
internal static class PatternWindow
{
    // lineState encodes one cell of the line segment relative to a player.
    public const sbyte LineOpp = -1;
    public const sbyte LineEmpty = 0;
    public const sbyte LineOwn = 1;

    // The window spans offsets -WinLength..+WinLength so any exact-five
    // through the center plus both of its end-check cells is fully visible.
    public const int LineCenter = Constants.Board.WinLength;
    public const int LineLastIndex = Constants.Board.LineLength - 1;
    // A completion cell must share a five with the center, so it can never
    // sit at the window's outermost cells.
    public const int LineFirstPlayable = LineCenter - (Constants.Board.WinLength - 1);
    public const int LineLastPlayable = LineCenter + (Constants.Board.WinLength - 1);

    /// <summary>
    /// Reads the 11 cells centered on (x,y) along (dx,dy) into the caller's
    /// buffer. The center is always reported as the player's own stone, so
    /// callers may query a hypothetical placement without mutating the board.
    /// </summary>
    public static void ExtractLine(SearchBoard sb, int x, int y, Player player, int dx, int dy, Span<sbyte> line)
    {
        for (int off = -LineCenter; off <= LineCenter; off++)
        {
            int i = off + LineCenter;
            if (off == 0)
            {
                line[i] = LineOwn;
                continue;
            }
            int nx = x + dx * off;
            int ny = y + dy * off;
            if (nx < 0 || nx >= Constants.Board.Size || ny < 0 || ny >= Constants.Board.Size)
            {
                line[i] = LineOpp;
                continue;
            }
            Player p = sb.PlayerAt(nx, ny);
            if (p == player)
            {
                line[i] = LineOwn;
            }
            else if (p == Player.None)
            {
                line[i] = LineEmpty;
            }
            else
            {
                line[i] = LineOpp;
            }
        }
    }

    /// <summary>
    /// Returns the maximal run of own stones containing the center, treating
    /// the cell at fillIdx (if empty) as own.
    /// </summary>
    public static void SpanThrough(ReadOnlySpan<sbyte> line, int fillIdx, out int lo, out int hi)
    {
        lo = LineCenter;
        hi = LineCenter;
        while (lo > 0)
        {
            sbyte c = line[lo - 1];
            if (c == LineOwn || (lo - 1 == fillIdx && c == LineEmpty))
            {
                lo--;
            }
            else
            {
                break;
            }
        }
        while (hi < LineLastIndex)
        {
            sbyte c = line[hi + 1];
            if (c == LineOwn || (hi + 1 == fillIdx && c == LineEmpty))
            {
                hi++;
            }
            else
            {
                break;
            }
        }
    }

    /// <summary>
    /// Reports whether the span [lo,hi] is an exact five with at least one
    /// open end (Caro rules).
    /// </summary>
    public static bool SpanIsFive(ReadOnlySpan<sbyte> line, int lo, int hi)
    {
        if (hi - lo + 1 != Constants.Board.WinLength)
        {
            return false;
        }
        bool beforeBlocked = lo == 0 || line[lo - 1] == LineOpp;
        bool afterBlocked = hi == LineLastIndex || line[hi + 1] == LineOpp;
        return !beforeBlocked || !afterBlocked;
    }

    /// <summary>
    /// Counts empty cells whose fill makes an exact five through the center.
    /// Only the cells adjacent to the center's maximal span can ever complete
    /// it, so at most two candidates are tested. Mutates nothing.
    /// </summary>
    public static int LineCompletions(ReadOnlySpan<sbyte> line)
    {
        // A five through the center needs the center plus three more own
        // stones already in the window (the fourth slot is the fill itself).
        int own = 0;
        for (int i = 0; i < line.Length; i++)
        {
            if (line[i] == LineOwn)
            {
                own++;
            }
        }
        if (own < Constants.Board.WinLength - 1)
        {
            return 0;
        }
        SpanThrough(line, -1, out int lo, out int hi);
        int comps = 0;
        for (int i = Math.Max(lo - 1, LineFirstPlayable); i <= Math.Min(hi + 1, LineLastPlayable); i++)
        {
            if (line[i] != LineEmpty)
            {
                continue;
            }
            SpanThrough(line, i, out int l2, out int h2);
            if (SpanIsFive(line, l2, h2))
            {
                comps++;
            }
        }
        return comps;
    }

    /// <summary>Returns the same line from the opponent's perspective.</summary>
    public static void NegateLine(ReadOnlySpan<sbyte> line, Span<sbyte> result)
    {
        for (int i = 0; i < line.Length; i++)
        {
            sbyte v = line[i];
            if (v == LineOwn)
            {
                result[i] = LineOpp;
            }
            else if (v == LineOpp)
            {
                result[i] = LineOwn;
            }
            else
            {
                result[i] = LineEmpty;
            }
        }
    }

    /// <summary>
    /// Assumes (x,y) holds player's stone and appends the empty cells on the
    /// line whose fill makes an exact five through (x,y).
    /// </summary>
    public static void FiveCompletionsInDir(SearchBoard sb, int x, int y, Player player, int dx, int dy, List<Position> output)
    {
        Span<sbyte> line = stackalloc sbyte[Constants.Board.LineLength];
        ExtractLine(sb, x, y, player, dx, dy, line);
        for (int i = LineFirstPlayable; i <= LineLastPlayable; i++)
        {
            if (line[i] != LineEmpty)
            {
                continue;
            }
            SpanThrough(line, i, out int lo, out int hi);
            if (SpanIsFive(line, lo, hi))
            {
                output.Add(new Position(x + dx * (i - LineCenter), y + dy * (i - LineCenter)));
            }
        }
    }

    /// <summary>
    /// Returns the largest completion count reachable by filling a single
    /// empty cell on the line.
    /// </summary>
    public static int MaxCompsAfterFill(ReadOnlySpan<sbyte> line)
    {
        int own = 0;
        for (int i = 0; i < line.Length; i++)
        {
            if (line[i] == LineOwn)
            {
                own++;
            }
        }
        if (own < Constants.Board.WinLength - 2)
        {
            return 0;
        }
        int best = 0;
        Span<sbyte> filled = stackalloc sbyte[Constants.Board.LineLength];
        for (int i = LineFirstPlayable; i <= LineLastPlayable; i++)
        {
            if (line[i] != LineEmpty)
            {
                continue;
            }
            if (SeparatedByOpp(line, i))
            {
                continue;
            }
            line.CopyTo(filled);
            filled[i] = LineOwn;
            int c = LineCompletions(filled);
            if (c > best)
            {
                best = c;
            }
        }
        return best;
    }

    /// <summary>
    /// Reports whether an opponent stone sits strictly between the center
    /// and cell i, making it unable to join the center's span.
    /// </summary>
    public static bool SeparatedByOpp(ReadOnlySpan<sbyte> line, int i)
    {
        if (i < LineCenter)
        {
            for (int j = i + 1; j < LineCenter; j++)
            {
                if (line[j] == LineOpp)
                {
                    return true;
                }
            }
        }
        else
        {
            for (int j = LineCenter + 1; j < i; j++)
            {
                if (line[j] == LineOpp)
                {
                    return true;
                }
            }
        }
        return false;
    }
}

/// <summary>Describes what a hypothetical stone at (x,y) creates.</summary>
internal struct PlacementThreats
{
    public int Comp0;
    public int Comp1;
    public int Comp2;
    public int Comp3;
    public bool Flex3;

    public readonly bool OpenFour() => Comp0 >= 2 || Comp1 >= 2 || Comp2 >= 2 || Comp3 >= 2;

    public readonly bool Four() => Comp0 >= 1 || Comp1 >= 1 || Comp2 >= 1 || Comp3 >= 1;
}

internal static class PlacementAnalysis
{
    /// <summary>Computes only the per-direction completion counts. Cheap: no flex-three reachability scan.</summary>
    public static void PlacementComps(SearchBoard sb, int x, int y, Player player, Span<int> comps)
    {
        Span<sbyte> line = stackalloc sbyte[Constants.Board.LineLength];
        for (int i = 0; i < Pattern4Classifier.EvalDirs.Length; i++)
        {
            (int dx, int dy) = Pattern4Classifier.EvalDirs[i];
            PatternWindow.ExtractLine(sb, x, y, player, dx, dy, line);
            comps[i] = PatternWindow.LineCompletions(line);
        }
    }

    /// <summary>Computes completions plus the flex-three flag. Callers that only need four-ness should use PlacementComps instead.</summary>
    public static PlacementThreats AnalyzePlacement(SearchBoard sb, int x, int y, Player player)
    {
        Span<int> comps = stackalloc int[4];
        PlacementComps(sb, x, y, player, comps);
        PlacementThreats pt = new()
        {
            Comp0 = comps[0],
            Comp1 = comps[1],
            Comp2 = comps[2],
            Comp3 = comps[3],
        };
        Span<sbyte> line = stackalloc sbyte[Constants.Board.LineLength];
        for (int i = 0; i < Pattern4Classifier.EvalDirs.Length; i++)
        {
            if (comps[i] == 0 && !pt.Flex3)
            {
                (int dx, int dy) = Pattern4Classifier.EvalDirs[i];
                PatternWindow.ExtractLine(sb, x, y, player, dx, dy, line);
                pt.Flex3 = PatternWindow.MaxCompsAfterFill(line) >= 2;
            }
        }
        return pt;
    }

    /// <summary>
    /// Reports whether placing player at (x,y) creates an open four: at
    /// least two distinct winning completions (straight .XXXX. or split
    /// XX.XX shapes alike).
    /// </summary>
    public static bool CreatesOpenFour(SearchBoard sb, int x, int y, Player player)
    {
        Span<int> comps = stackalloc int[4];
        PlacementComps(sb, x, y, player, comps);
        return comps[0] >= 2 || comps[1] >= 2 || comps[2] >= 2 || comps[3] >= 2;
    }

    /// <summary>
    /// Reports whether placing player at (x,y) creates any four: a shape one
    /// move away from an exact five, gapped or straight.
    /// </summary>
    public static bool CreatesFourType(SearchBoard sb, int x, int y, Player player)
    {
        Span<int> comps = stackalloc int[4];
        PlacementComps(sb, x, y, player, comps);
        return comps[0] >= 1 || comps[1] >= 1 || comps[2] >= 1 || comps[3] >= 1;
    }

    /// <summary>
    /// Reports whether placing player at (x,y) creates an open three: a
    /// shape (straight or broken) that can become an open four next move.
    /// </summary>
    public static bool CreatesOpenThree(SearchBoard sb, int x, int y, Player player)
    {
        Span<sbyte> line = stackalloc sbyte[Constants.Board.LineLength];
        foreach ((int dx, int dy) in Pattern4Classifier.EvalDirs)
        {
            PatternWindow.ExtractLine(sb, x, y, player, dx, dy, line);
            if (PatternWindow.MaxCompsAfterFill(line) >= 2)
            {
                return true;
            }
        }
        return false;
    }
}
