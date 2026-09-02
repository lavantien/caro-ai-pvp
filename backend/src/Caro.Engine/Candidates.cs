using Caro.Domain;

namespace Caro.Engine;

public static class Candidates
{
    // Seed block for the empty board: a square spanning this many cells per
    // side around the center.
    private const int EmptyBoardSeedSpan = 3;
    private const int DefaultCandidateCapacity = 64;

    public static List<Position> GetCandidates(SearchBoard sb, int radius)
    {
        BitBoard occupied = sb.Occupied();
        if (occupied.IsZero())
        {
            int center = Constants.BoardSize / 2;
            int halfSpan = EmptyBoardSeedSpan / 2;
            List<Position> seed = new(EmptyBoardSeedSpan * EmptyBoardSeedSpan);
            for (int dx = 0; dx < EmptyBoardSeedSpan; dx++)
            {
                for (int dy = 0; dy < EmptyBoardSeedSpan; dy++)
                {
                    seed.Add(new Position(center + dx - halfSpan, center + dy - halfSpan));
                }
            }
            return seed;
        }

        // Stack-allocated dedup: a heap map per node dominated the profile.
        Span<bool> seen = stackalloc bool[Constants.BoardSize * Constants.BoardSize];
        seen.Clear();
        List<Position> result = new(DefaultCandidateCapacity);

        for (int x = 0; x < Constants.BoardSize; x++)
        {
            for (int y = 0; y < Constants.BoardSize; y++)
            {
                if (!occupied.Get(x, y))
                {
                    continue;
                }
                for (int dx = -radius; dx <= radius; dx++)
                {
                    for (int dy = -radius; dy <= radius; dy++)
                    {
                        int nx = x + dx;
                        int ny = y + dy;
                        if (nx < 0 || nx >= Constants.BoardSize || ny < 0 || ny >= Constants.BoardSize)
                        {
                            continue;
                        }
                        int idx = ny * Constants.BoardSize + nx;
                        if (seen[idx] || !sb.IsEmpty(nx, ny))
                        {
                            continue;
                        }
                        seen[idx] = true;
                        result.Add(new Position(nx, ny));
                    }
                }
            }
        }

        return result;
    }

    public static List<Position> GetTacticalCandidates(SearchBoard sb, Player player)
    {
        List<Position> allCandidates = GetCandidates(sb, Constants.MaxSearchRadius);
        if (allCandidates.Count == 0)
        {
            return [];
        }

        Player opponent = player.Opponent();
        List<Position> tactical = new(allCandidates.Count);

        foreach (Position c in allCandidates)
        {
            if (IsTacticalMove(sb, c.X, c.Y, player, opponent))
            {
                tactical.Add(c);
            }
        }

        return tactical;
    }

    internal static bool IsTacticalMove(SearchBoard sb, int x, int y, Player player, Player opponent)
    {
        // Win: creates exactly-5 (Caro-valid)
        sb.MakeMove(x, y, player);
        if (MoveOrdering.WouldWin(sb, x, y, player))
        {
            sb.UnmakeMove();
            return true;
        }
        sb.UnmakeMove();

        // Block: opponent would win here
        sb.MakeMove(x, y, opponent);
        if (MoveOrdering.WouldWin(sb, x, y, opponent))
        {
            sb.UnmakeMove();
            return true;
        }
        sb.UnmakeMove();

        // Four for either side (creating or blocking), plus double threats: a
        // move creating a four-or-open-three shape in two directions at once is
        // forcing, because the opponent cannot answer both lines with one stone.
        // A lone open three stays non-forcing (the opponent may convert or
        // ignore it), so it stays visible to eval and move ordering only.
        Span<sbyte> line = stackalloc sbyte[Constants.LineLength];
        Span<sbyte> oppLine = stackalloc sbyte[Constants.LineLength];
        int ownThreatDirs = 0;
        int oppThreatDirs = 0;
        foreach ((int dx, int dy) in Pattern4Classifier.EvalDirs)
        {
            PatternWindow.ExtractLine(sb, x, y, player, dx, dy, line);
            PatternWindow.NegateLine(line, oppLine);
            if (PatternWindow.LineCompletions(line) >= 1 || PatternWindow.LineCompletions(oppLine) >= 1)
            {
                return true;
            }
            if (PatternWindow.MaxCompsAfterFill(line) >= 2)
            {
                ownThreatDirs++;
            }
            if (PatternWindow.MaxCompsAfterFill(oppLine) >= 2)
            {
                oppThreatDirs++;
            }
        }
        return ownThreatDirs >= 2 || oppThreatDirs >= 2;
    }

    public static List<Position> FilterOpenRule(List<Position> candidates, SearchBoard sb, Player player)
    {
        if (player != Player.Red)
        {
            return candidates;
        }
        // The rule only constrains red's second move: at most two stones exist
        // then. Skip the full-board scan everywhere else.
        if (sb.StoneCount() > 2)
        {
            return candidates;
        }

        int redCount = 0;
        int blueCount = 0;
        int firstRedX = 0;
        int firstRedY = 0;
        for (int bx = 0; bx < Constants.BoardSize; bx++)
        {
            for (int by = 0; by < Constants.BoardSize; by++)
            {
                Player p = sb.PlayerAt(bx, by);
                if (p == Player.Red)
                {
                    redCount++;
                    firstRedX = bx;
                    firstRedY = by;
                }
                else if (p == Player.Blue)
                {
                    blueCount++;
                }
            }
        }

        if (redCount != 1 || blueCount > 1)
        {
            return candidates;
        }

        List<Position> filtered = new(candidates.Count);
        foreach (Position c in candidates)
        {
            int dx = c.X - firstRedX;
            int dy = c.Y - firstRedY;
            if (dx < 0)
            {
                dx = -dx;
            }
            if (dy < 0)
            {
                dy = -dy;
            }
            if (dx >= Constants.OpenRuleMin || dy >= Constants.OpenRuleMin)
            {
                filtered.Add(c);
            }
        }
        return filtered;
    }
}
