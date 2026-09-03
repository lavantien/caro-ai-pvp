using Caro.Domain;

namespace Caro.Engine;

internal sealed class VCFSolver(SearchBoard sb, Player attacker, TimeMonitor monitor, int startDepth)
{
    private int _winX;
    private int _winY;
    private bool _timedOut;
    private long _nodes;
    private int _chainDepth;

    public bool Search(int depth)
    {
        if (monitor.ShouldStop())
        {
            _timedOut = true;
            return false;
        }
        if (depth <= 0)
        {
            return false;
        }

        List<Position> candidates = Candidates.GetCandidates(sb, Constants.Board.MaxSearchRadius);

        foreach (Position c in candidates)
        {
            if (monitor.ShouldStop())
            {
                _timedOut = true;
                return false;
            }

            sb.MakeMove(c.X, c.Y, attacker);
            _nodes++;

            if (MoveOrdering.WouldWin(sb, c.X, c.Y, attacker))
            {
                sb.UnmakeMove();
                _winX = c.X;
                _winY = c.Y;
                _chainDepth = startDepth - depth + 1;
                return true;
            }

            List<Position> blocks = Vcf.FindFourBlocks(sb, c.X, c.Y, attacker);
            if (blocks.Count == 0)
            {
                sb.UnmakeMove();
                continue;
            }

            // Opponent may have a winning response outside the blocking squares.
            if (Vcf.OpponentHasImmediateWin(sb, attacker.Opponent()))
            {
                sb.UnmakeMove();
                continue;
            }

            bool allWin = true;
            foreach (Position block in blocks)
            {
                sb.MakeMove(block.X, block.Y, attacker.Opponent());

                if (MoveOrdering.WouldWin(sb, block.X, block.Y, attacker.Opponent()))
                {
                    allWin = false;
                    sb.UnmakeMove();
                    break;
                }
                if (!Search(depth - 1))
                {
                    allWin = false;
                    sb.UnmakeMove();
                    break;
                }
                sb.UnmakeMove();
            }

            sb.UnmakeMove();

            if (_timedOut)
            {
                return false;
            }

            if (allWin)
            {
                // _chainDepth already counts the full forced chain from the
                // winning recursion; only the move itself is decided here.
                _winX = c.X;
                _winY = c.Y;
                return true;
            }
        }

        return false;
    }

    public (int X, int Y) WinMove => (_winX, _winY);

    public bool TimedOut => _timedOut;

    public long NodesSearched => _nodes;

    public int ChainDepth => _chainDepth;
}

public static class Vcf
{
    public static (int X, int Y, VCFResult Result) SolveVCF(
        Board b,
        Player player,
        long allocatedMs,
        CancellationToken ctx)
    {
        VcfSearchResult r = SolveVCFWithDepth(b, player, Constants.Vcf.SearchDepth, allocatedMs, ctx);
        return (r.X, r.Y, r.Result);
    }

    /// <summary>
    /// Bounds the forcing chain length: depth counts attacker moves, so
    /// depth 1 sees only immediate fours. It is the per-level tactical sight
    /// knob behind DifficultyProfile.VCFDepth.
    /// </summary>
    public static VcfSearchResult SolveVCFWithDepth(
        Board b,
        Player player,
        int depth,
        long allocatedMs,
        CancellationToken ctx)
    {
        SearchBoard sb = new(b);
        using TimeMonitor monitor = new(allocatedMs, ctx);

        if (depth <= 0)
        {
            depth = Constants.Vcf.SearchDepth;
        }
        VCFSolver v = new(sb, player, monitor, depth);

        if (v.Search(depth))
        {
            (int x, int y) = v.WinMove;
            return new VcfSearchResult(x, y, VCFResult.Win, v.NodesSearched, v.ChainDepth);
        }
        if (v.TimedOut)
        {
            return new VcfSearchResult(-1, -1, VCFResult.Timeout, v.NodesSearched, v.ChainDepth);
        }
        return new VcfSearchResult(-1, -1, VCFResult.NoWin, v.NodesSearched, v.ChainDepth);
    }

    internal static bool OpponentHasImmediateWin(SearchBoard sb, Player opponent)
    {
        List<Position> candidates = Candidates.GetCandidates(sb, Constants.Board.MaxSearchRadius);
        foreach (Position c in candidates)
        {
            sb.MakeMove(c.X, c.Y, opponent);
            bool wins = MoveOrdering.WouldWin(sb, c.X, c.Y, opponent);
            sb.UnmakeMove();
            if (wins)
            {
                return true;
            }
        }
        return false;
    }

    /// <summary>
    /// Returns the cells the opponent must play to block a four created by
    /// placing attacker at (x,y): every empty cell whose fill would complete
    /// an exact five for the attacker, gapped or straight. Returns empty if
    /// no four was created.
    /// </summary>
    internal static List<Position> FindFourBlocks(SearchBoard sb, int x, int y, Player attacker)
    {
        List<Position> blocks = [];
        HashSet<Position> seen = [];
        foreach ((int dx, int dy) in Constants.Directions)
        {
            PatternWindow.FiveCompletionsInDir(sb, x, y, attacker, dx, dy, blocks);
        }
        // Dedup after collection; FiveCompletionsInDir appends per direction.
        List<Position> deduped = [];
        foreach (Position c in blocks)
        {
            if (seen.Add(c))
            {
                deduped.Add(c);
            }
        }
        return deduped;
    }
}
