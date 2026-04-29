using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic.Search;

/// <summary>
/// Manages search heuristics for move ordering: killer moves, history tables, butterfly tables.
/// These heuristics improve alpha-beta pruning efficiency by iterative deepening search.
/// </summary>
public class SearchHeuristics
{
    private const int BoardSize = GameConstants.BoardSize;

    // Killer heuristic: track best moves at each depth
    private readonly (int x, int y)[,] _killerMoves = new (int x, int y)[SearchConstants.MaxKillerDepth, SearchConstants.MaxKillerMoves];

    // History heuristic: track moves that cause cutoffs across all depths
    // Two tables: one for Red, one for Blue (each move can be good for different players)
    private readonly int[,] _historyRed = new int[BoardSize, BoardSize];
    private readonly int[,] _historyBlue = new int[BoardSize, BoardSize];

    // Butterfly heuristic: track moves that cause beta cutoffs (complements history)
    private readonly int[,] _butterflyRed = new int[BoardSize, BoardSize];
    private readonly int[,] _butterflyBlue = new int[BoardSize, BoardSize];

    public SearchHeuristics()
    {
        // Arrays are zero-initialized by default (C# default for int)
    }

    /// <summary>
    /// Record a killer move at the given depth.
    /// Killer moves are the most recent cutoff moves at each depth level.
    /// </summary>
    public void RecordKillerMove(int depth, int x, int y)
    {
        // Shift existing killer moves
        for (int i = SearchConstants.MaxKillerMoves - 1; i > 0; i--)
        {
            _killerMoves[depth, i] = _killerMoves[depth, i - 1];
        }
        _killerMoves[depth, 0] = (x, y);
    }

    /// <summary>
    /// Record a move that caused a cutoff in the history table.
    /// Higher depth = more significant = larger bonus.
    /// </summary>
    public void RecordHistoryMove(Player player, int x, int y, int depth)
    {
        var bonus = depth * depth;
        var butterflyBonus = depth * depth * 2;

        if (player == Player.Red)
        {
            _historyRed[x, y] += bonus;
            _butterflyRed[x, y] += butterflyBonus;
        }
        else
        {
            _historyBlue[x, y] += bonus;
            _butterflyBlue[x, y] += butterflyBonus;
        }
    }

    /// <summary>
    /// Get the history score for a move.
    /// </summary>
    public int GetHistoryScore(Player player, int x, int y)
    {
        return player == Player.Red ? _historyRed[x, y] : _historyBlue[x, y];
    }

    /// <summary>
    /// Check if a move is a killer move at the given depth.
    /// Returns true if the move matches any killer move slot.
    /// </summary>
    public bool IsKillerMove(int depth, int x, int y)
    {
        if (depth < 0 || depth >= SearchConstants.MaxKillerDepth)
            return false;

        for (int k = 0; k < SearchConstants.MaxKillerMoves; k++)
        {
            if (_killerMoves[depth, k].x == x && _killerMoves[depth, k].y == y)
            {
                return true;
            }
        }
        return false;
    }

    /// <summary>
    /// Get the killer moves at a given depth.
    /// </summary>
    public (int x, int y)[] GetKillerMoves(int depth)
    {
        if (depth < 0 || depth >= SearchConstants.MaxKillerDepth)
            return [];

        var moves = new (int x, int y)[SearchConstants.MaxKillerMoves];
        for (int k = 0; k < SearchConstants.MaxKillerMoves; k++)
        {
            moves[k] = _killerMoves[depth, k];
        }
        return moves;
    }

    /// <summary>
    /// Get the butterfly score for a move.
    /// </summary>
    public int GetButterflyScore(Player player, int x, int y)
    {
        return player == Player.Red ? _butterflyRed[x, y] : _butterflyBlue[x, y];
    }

    /// <summary>
    /// Clear history tables (call at start of new game).
    /// </summary>
    public void ClearHistory()
    {
        Array.Clear(_historyRed, 0, _historyRed.Length);
        Array.Clear(_historyBlue, 0, _historyBlue.Length);
        Array.Clear(_butterflyRed, 0, _butterflyRed.Length);
        Array.Clear(_butterflyBlue, 0, _butterflyBlue.Length);
    }

    /// <summary>
    /// Clear killer moves (call when resetting position-specific state).
    /// </summary>
    public void ClearKillers()
    {
        for (int d = 0; d < SearchConstants.MaxKillerDepth; d++)
        {
            for (int k = 0; k < SearchConstants.MaxKillerMoves; k++)
            {
                _killerMoves[d, k] = (0, 0);
            }
        }
    }
}
