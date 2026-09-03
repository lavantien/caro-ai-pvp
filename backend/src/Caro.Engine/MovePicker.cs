using Caro.Domain;

namespace Caro.Engine;

internal struct ScoredMove(Position pos, int score)
{
    public Position Pos = pos;
    public int Score = score;
}

/// <summary>
/// Staged move picker: TT move, then winning, must-block, threat,
/// killer/counter, and quiet stages, each deduplicated against everything
/// already yielded.
/// </summary>
public sealed class MovePicker
{
    private const int StageTTMove = 0;
    private const int StageWinning = 1;
    private const int StageMustBlock = 2;
    private const int StageThreat = 3;
    private const int StageKillerCounter = 4;
    private const int StageQuiet = 5;
    private const int StageDone = 6;

    private readonly List<Position> _candidates;
    private readonly SearchBoard _sb;
    private readonly Player _player;
    private readonly int _depth;
    private readonly Position? _ttMove;
    private readonly SearchHeuristics _heuristics;
    private readonly Position _prevMove;
    private int _stage;
    private int _index;
    private List<Position>? _staged;
    private readonly ulong[] _yielded = new ulong[4];
    private bool _lastTactical;

    public MovePicker(List<Position> candidates, SearchBoard sb, Player player, int depth,
        Position? ttMove, SearchHeuristics heuristics, Position prevMove)
    {
        _candidates = candidates;
        _sb = sb;
        _player = player;
        _depth = depth;
        _ttMove = ttMove;
        _heuristics = heuristics;
        _prevMove = prevMove;
        _stage = StageTTMove;
    }

    /// <summary>
    /// Reports whether the move from the most recent Next call came from a
    /// forcing stage (winning, must-block, or threat). Reducing such moves in
    /// LMR hides exactly the refutations the search must resolve.
    /// </summary>
    public bool LastMoveTactical() => _lastTactical;

    private void MarkYielded(Position p)
    {
        int c = EngineMath.PosToCell(p.X, p.Y);
        _yielded[c / 64] |= 1UL << (c % 64);
    }

    private bool AlreadyYielded(Position p)
    {
        int c = EngineMath.PosToCell(p.X, p.Y);
        return (_yielded[c / 64] & (1UL << (c % 64))) != 0;
    }

    /// <summary>Returns the next move to search, or false when done.</summary>
    public bool Next(out Position move)
    {
        while (true)
        {
            if (_stage == StageTTMove)
            {
                _stage = StageWinning;
                if (_ttMove.HasValue)
                {
                    foreach (Position c in _candidates)
                    {
                        if (c == _ttMove.Value)
                        {
                            MarkYielded(c);
                            _lastTactical = false;
                            move = c;
                            return true;
                        }
                    }
                }
                continue;
            }

            if (_staged == null)
            {
                _staged = GenerateStage();
                _index = 0;
            }

            if (_index < _staged.Count)
            {
                Position m = _staged[_index];
                _index++;
                if (AlreadyYielded(m))
                {
                    continue;
                }
                MarkYielded(m);
                _lastTactical = _stage >= StageWinning && _stage <= StageThreat;
                move = m;
                return true;
            }

            _staged = null;
            _stage++;
            if (_stage >= StageDone)
            {
                move = default;
                return false;
            }
        }
    }

    private List<Position> GenerateStage() => _stage switch
    {
        StageWinning => GenWinning(),
        StageMustBlock => GenMustBlock(),
        StageThreat => GenThreats(),
        StageKillerCounter => GenKillerCounter(),
        StageQuiet => GenQuiet(),
        _ => [],
    };

    private List<Position> GenMustBlock()
    {
        Player opponent = _player.Opponent();
        List<Position> result = [];
        foreach (Position c in _candidates)
        {
            if (_ttMove.HasValue && c == _ttMove.Value)
            {
                continue;
            }
            _sb.MakeMove(c.X, c.Y, opponent);
            if (MoveOrdering.WouldWin(_sb, c.X, c.Y, opponent))
            {
                result.Add(c);
            }
            _sb.UnmakeMove();
        }
        return result;
    }

    private List<Position> GenWinning()
    {
        List<Position> result = [];
        foreach (Position c in _candidates)
        {
            if (_ttMove.HasValue && c == _ttMove.Value)
            {
                continue;
            }
            _sb.MakeMove(c.X, c.Y, _player);
            if (MoveOrdering.WouldWin(_sb, c.X, c.Y, _player))
            {
                result.Add(c);
            }
            _sb.UnmakeMove();
        }
        return result;
    }

    private List<Position> GenThreats()
    {
        List<ScoredMove> result = [];
        foreach (Position c in _candidates)
        {
            if (_ttMove.HasValue && c == _ttMove.Value)
            {
                continue;
            }
            int score = ThreatScore(c.X, c.Y);
            if (score > 0)
            {
                result.Add(new ScoredMove(c, score));
            }
        }
        result.Sort((a, b) => b.Score.CompareTo(a.Score));
        List<Position> output = new(result.Count);
        foreach (ScoredMove s in result)
        {
            output.Add(s.Pos);
        }
        return output;
    }

    private int ThreatScore(int x, int y)
    {
        PlacementThreats own = PlacementAnalysis.AnalyzePlacement(_sb, x, y, _player);
        int score = 0;
        if (own.OpenFour())
        {
            score += Constants.Ordering.OwnOpenFourScore;
        }
        else if (own.Four())
        {
            score += Constants.Ordering.OwnFourScore;
        }
        if (own.Flex3)
        {
            score += Constants.Ordering.OwnFlex3Score;
        }

        Player opponent = _player.Opponent();
        PlacementThreats theirs = PlacementAnalysis.AnalyzePlacement(_sb, x, y, opponent);
        if (theirs.OpenFour())
        {
            score += Constants.Ordering.OppOpenFourScore;
        }
        else if (theirs.Four())
        {
            score += Constants.Ordering.OppFourScore;
        }
        if (theirs.Flex3)
        {
            score += Constants.Ordering.OppFlex3Score;
        }
        return score;
    }

    private List<Position> GenKillerCounter()
    {
        List<Position> result = [];
        for (int slot = 0; slot < 2; slot++)
        {
            if (_depth < 0 || _depth >= Constants.History.MaxKillerDepth)
            {
                continue;
            }
            Position k = _heuristics.KillerAt(_depth, slot);
            if (k.X < 0 || k.X >= Constants.Board.Size || k.Y < 0 || k.Y >= Constants.Board.Size)
            {
                continue;
            }
            if (!_sb.IsEmpty(k.X, k.Y))
            {
                continue;
            }
            result.Add(k);
        }

        if (_prevMove.X >= 0 && _prevMove.Y >= 0)
        {
            Position cm = _heuristics.CounterMoveFor(_player, _prevMove.X, _prevMove.Y);
            if (cm.X >= 0 && cm.X < Constants.Board.Size && cm.Y >= 0 && cm.Y < Constants.Board.Size)
            {
                if (_sb.IsEmpty(cm.X, cm.Y))
                {
                    result.Add(cm);
                }
            }
        }

        return result;
    }

    private List<Position> GenQuiet()
    {
        List<ScoredMove> scored = new(_candidates.Count);
        foreach (Position c in _candidates)
        {
            int score = _heuristics.HistoryScore(_player, c.X, c.Y) * Constants.Ordering.HistoryMultiplier;
            if (score > Constants.Ordering.HistoryScoreCap)
            {
                score = Constants.Ordering.HistoryScoreCap;
            }
            score += _heuristics.KillerScore(_depth, c);
            score += _heuristics.ContHistoryScore(_player, _prevMove.X, _prevMove.Y, c.X, c.Y);

            int center = Constants.Board.Size / 2;
            int dist = EngineMath.Abs(c.X - center) + EngineMath.Abs(c.Y - center);
            score += (Constants.Ordering.CenterDistScaleBase - dist) * Constants.Ordering.CenterWeight;

            score += MoveOrdering.ProximityScore(_sb, c.X, c.Y) * Constants.Ordering.ProximityWeight;

            scored.Add(new ScoredMove(c, score));
        }

        scored.Sort((a, b) => b.Score.CompareTo(a.Score));

        List<Position> output = new(scored.Count);
        foreach (ScoredMove s in scored)
        {
            output.Add(s.Pos);
        }
        return output;
    }
}

public static class MoveOrdering
{
    /// <summary>All-at-once fallback ordering for the root search.</summary>
    public static List<Position> OrderMoves(
        List<Position> candidates,
        SearchBoard board,
        Player player,
        int depth,
        Position? ttMove,
        SearchHeuristics heuristics)
    {
        if (candidates.Count <= 1)
        {
            return candidates;
        }

        MovePicker picker = new(candidates, board, player, depth, ttMove, heuristics, new Position(-1, -1));
        List<Position> result = [];
        while (picker.Next(out Position m))
        {
            result.Add(m);
        }
        return result;
    }

    internal static bool WouldWin(SearchBoard sb, int x, int y, Player player)
    {
        int[] dirs = [1, 0, 0, 1, 1, 1, 1, -1];
        for (int d = 0; d < 4; d++)
        {
            int dx = dirs[d * 2];
            int dy = dirs[d * 2 + 1];
            int positive = 0;
            for (int i = 1; i <= Constants.Board.WinLength; i++)
            {
                int nx = x + dx * i;
                int ny = y + dy * i;
                if (nx < 0 || nx >= Constants.Board.Size || ny < 0 || ny >= Constants.Board.Size || sb.PlayerAt(nx, ny) != player)
                {
                    break;
                }
                positive++;
            }
            int negative = 0;
            for (int i = 1; i <= Constants.Board.WinLength; i++)
            {
                int nx = x - dx * i;
                int ny = y - dy * i;
                if (nx < 0 || nx >= Constants.Board.Size || ny < 0 || ny >= Constants.Board.Size || sb.PlayerAt(nx, ny) != player)
                {
                    break;
                }
                negative++;
            }

            if (1 + positive + negative != Constants.Board.WinLength)
            {
                continue;
            }

            int afterX = x + dx * (positive + 1);
            int afterY = y + dy * (positive + 1);
            int beforeX = x - dx * (negative + 1);
            int beforeY = y - dy * (negative + 1);

            bool afterBlocked = afterX < 0 || afterX >= Constants.Board.Size || afterY < 0 || afterY >= Constants.Board.Size ||
                (sb.PlayerAt(afterX, afterY) != Player.None && sb.PlayerAt(afterX, afterY) != player);
            bool beforeBlocked = beforeX < 0 || beforeX >= Constants.Board.Size || beforeY < 0 || beforeY >= Constants.Board.Size ||
                (sb.PlayerAt(beforeX, beforeY) != Player.None && sb.PlayerAt(beforeX, beforeY) != player);

            if (afterBlocked && beforeBlocked)
            {
                continue;
            }
            return true;
        }
        return false;
    }

    internal static int ProximityScore(SearchBoard sb, int x, int y)
    {
        int score = 0;
        for (int dx = -Constants.Board.MaxSearchRadius; dx <= Constants.Board.MaxSearchRadius; dx++)
        {
            for (int dy = -Constants.Board.MaxSearchRadius; dy <= Constants.Board.MaxSearchRadius; dy++)
            {
                int nx = x + dx;
                int ny = y + dy;
                if (nx >= 0 && nx < Constants.Board.Size && ny >= 0 && ny < Constants.Board.Size)
                {
                    Player p = sb.PlayerAt(nx, ny);
                    if (p == Player.Red || p == Player.Blue)
                    {
                        score += Constants.Ordering.NeighborStoneScore;
                    }
                }
            }
        }
        return score;
    }
}
