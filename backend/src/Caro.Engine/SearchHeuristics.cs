using Caro.Domain;

namespace Caro.Engine;

public sealed class SearchHeuristics
{
    internal const int MaxKillerDepth = 64;
    private const int KillerPrimaryScore = 500_000;
    private const int KillerSecondaryScore = 400_000;
    private const int HistoryMax = 1_000_000;
    private const int ContHistMax = 30_000;
    private const int BoardCells = Constants.Board.Size * Constants.Board.Size;
    private const int ContHistBonusScale = 300;

    private readonly Position[,] _killerMoves = new Position[MaxKillerDepth, 2];
    private readonly int[,] _historyRed = new int[Constants.Board.Size, Constants.Board.Size];
    private readonly int[,] _historyBlue = new int[Constants.Board.Size, Constants.Board.Size];
    // Flattened continuation history [2][256][256]: one array copy per Clone.
    private readonly int[] _contHistory = new int[2 * BoardCells * BoardCells];
    private readonly Position[] _counterMove = new Position[2 * BoardCells];

    public void RecordKiller(int depth, Position pos)
    {
        if (depth < 0 || depth >= MaxKillerDepth)
        {
            return;
        }
        _killerMoves[depth, 1] = _killerMoves[depth, 0];
        _killerMoves[depth, 0] = pos;
    }

    public bool IsKiller(int depth, Position pos)
    {
        if (depth < 0 || depth >= MaxKillerDepth)
        {
            return false;
        }
        return _killerMoves[depth, 0] == pos || _killerMoves[depth, 1] == pos;
    }

    internal Position KillerAt(int depth, int slot)
    {
        if (depth < 0 || depth >= MaxKillerDepth)
        {
            return new Position(-1, -1);
        }
        return _killerMoves[depth, slot];
    }

    public int KillerScore(int depth, Position pos)
    {
        if (depth < 0 || depth >= MaxKillerDepth)
        {
            return 0;
        }
        if (_killerMoves[depth, 0] == pos)
        {
            return KillerPrimaryScore;
        }
        if (_killerMoves[depth, 1] == pos)
        {
            return KillerSecondaryScore;
        }
        return 0;
    }

    public void RecordHistory(Player player, int x, int y, int depth)
    {
        if (x < 0 || x >= Constants.Board.Size || y < 0 || y >= Constants.Board.Size)
        {
            return;
        }
        int[,] table = player == Player.Blue ? _historyBlue : _historyRed;
        table[x, y] += depth * depth;
        if (table[x, y] > HistoryMax)
        {
            table[x, y] = HistoryMax;
        }
    }

    public int HistoryScore(Player player, int x, int y)
    {
        if (x < 0 || x >= Constants.Board.Size || y < 0 || y >= Constants.Board.Size)
        {
            return 0;
        }
        return player == Player.Red ? _historyRed[x, y] : _historyBlue[x, y];
    }

    public void Clear()
    {
        Array.Clear(_killerMoves);
        Array.Clear(_historyRed);
        Array.Clear(_historyBlue);
        Array.Clear(_contHistory);
        Array.Clear(_counterMove);
    }

    /// <summary>
    /// Keeps game-level ordering knowledge across moves: history tables
    /// halve so recent cutoffs dominate, while killers and counter moves
    /// persist until naturally replaced. Positions evolve slowly in caro, so
    /// the previous move's ordering is a strong prior for the next search.
    /// </summary>
    public void AgeForNewMove()
    {
        for (int x = 0; x < Constants.Board.Size; x++)
        {
            for (int y = 0; y < Constants.Board.Size; y++)
            {
                _historyRed[x, y] /= 2;
                _historyBlue[x, y] /= 2;
            }
        }
        for (int i = 0; i < _contHistory.Length; i++)
        {
            _contHistory[i] /= 2;
        }
    }

    /// <summary>
    /// Snapshots the heuristics for a search worker that must not write to
    /// the shared instance.
    /// </summary>
    public SearchHeuristics Clone()
    {
        SearchHeuristics c = new();
        Array.Copy(_killerMoves, c._killerMoves, _killerMoves.Length);
        Array.Copy(_historyRed, c._historyRed, _historyRed.Length);
        Array.Copy(_historyBlue, c._historyBlue, _historyBlue.Length);
        Array.Copy(_contHistory, c._contHistory, _contHistory.Length);
        Array.Copy(_counterMove, c._counterMove, _counterMove.Length);
        return c;
    }

    public void RecordContHistory(Player player, int prevX, int prevY, int x, int y, int depth)
    {
        if (prevX < 0 || prevY < 0 || x < 0 || y < 0)
        {
            return;
        }
        int pi = EngineMath.PlayerIdx(player);
        int prevCell = EngineMath.PosToCell(prevX, prevY);
        int cell = EngineMath.PosToCell(x, y);
        int bonus = depth * depth * ContHistBonusScale / 100;
        int idx = ContIndex(pi, prevCell, cell);
        _contHistory[idx] += bonus;
        if (_contHistory[idx] > ContHistMax)
        {
            _contHistory[idx] = ContHistMax;
        }
    }

    public int ContHistoryScore(Player player, int prevX, int prevY, int x, int y)
    {
        if (prevX < 0 || prevY < 0 || x < 0 || y < 0)
        {
            return 0;
        }
        int pi = EngineMath.PlayerIdx(player);
        return _contHistory[ContIndex(pi, EngineMath.PosToCell(prevX, prevY), EngineMath.PosToCell(x, y))];
    }

    public void RecordCounterMove(Player player, int oppX, int oppY, int x, int y)
    {
        if (oppX < 0 || oppY < 0 || x < 0 || y < 0)
        {
            return;
        }
        int pi = EngineMath.PlayerIdx(player);
        _counterMove[pi * BoardCells + EngineMath.PosToCell(oppX, oppY)] = new Position(x, y);
    }

    public Position CounterMoveFor(Player player, int oppX, int oppY)
    {
        if (oppX < 0 || oppY < 0)
        {
            return new Position(-1, -1);
        }
        int pi = EngineMath.PlayerIdx(player);
        return _counterMove[pi * BoardCells + EngineMath.PosToCell(oppX, oppY)];
    }

    private static int ContIndex(int playerIdx, int prevCell, int cell) =>
        playerIdx * BoardCells * BoardCells + prevCell * BoardCells + cell;
}

internal static class EngineMath
{
    public static int PosToCell(int x, int y) => y * Constants.Board.Size + x;

    public static int PlayerIdx(Player p) => p == Player.Blue ? 1 : 0;

    public static int Abs(int x) => x < 0 ? -x : x;
}
