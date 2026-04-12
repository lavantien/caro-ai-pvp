using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic.Pondering;
using Caro.Core.GameLogic.Search;
using Microsoft.Extensions.Logging;

namespace Caro.Core.GameLogic;

public partial class MinimaxAI
{
    /// <summary>
    /// Try to use a ponder hit result instead of performing a full search.
    /// Returns null if ponder miss or ponder result is unusable.
    /// </summary>
    private (int x, int y)? TryPonderHit(Board board, Player player, List<(int x, int y)> candidates, bool ponderingEnabled)
    {
        if (!ponderingEnabled || !_ponderer.IsPondering || _lastPV.IsEmpty)
            return null;

        var lastOppMove = QuickWinChecker.GetLastOpponentMove(board, player);
        if (!lastOppMove.HasValue)
            return null;

        var (ponderState, _) = _ponderer.HandleOpponentMove(lastOppMove.Value.x, lastOppMove.Value.y);

        if (ponderState != PonderState.PonderHit)
            return null;

        // PONDER HIT - opponent played expected move!
        // CRITICAL FIX: Still check for immediate wins and threats before using ponder result.
        foreach (var (cx, cy) in candidates)
        {
            if (_threatDetector.IsWinningMove(board, cx, cy, player))
            {
                _ponderer.StopPondering();
                _depthAchieved = 1;
                _nodesSearched = 1;
                _lastAllocatedTimeMs = 0;
                _moveType = MoveType.ImmediateWin;
                return (cx, cy);
            }
        }

        // Check if opponent has an immediate winning threat we must block
        var ponderOppPlayer = player == Player.Red ? Player.Blue : Player.Red;
        var ponderOpponentWinningSquares = new List<(int x, int y)>();
        for (int x = 0; x < BoardSize; x++)
        {
            for (int y = 0; y < BoardSize; y++)
            {
                if (board.GetCell(x, y).Player == Player.None)
                {
                    if (_threatDetector.IsWinningMove(board, x, y, ponderOppPlayer))
                    {
                        ponderOpponentWinningSquares.Add((x, y));
                    }
                }
            }
        }

        // If there are immediate threats, must block - don't use ponder result
        if (ponderOpponentWinningSquares.Count > 0)
            return null;

        // No immediate threats - safe to use ponder result
        var ponderResult = _ponderer.GetPonderHitResult();

        if (ponderResult.BestMove.HasValue && ponderResult.Depth > 0)
        {
            var ponderMove = ponderResult.BestMove.Value;
            if (board.GetCell(ponderMove.x, ponderMove.y).IsEmpty)
            {
                _depthAchieved = ponderResult.Depth;
                _nodesSearched = ponderResult.NodesSearched;
                _lastAllocatedTimeMs = ponderResult.TimeSpentMs;
                _moveType = MoveType.Normal;
                return ponderMove;
            }
        }

        return null;
    }
}
