using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Adaptive depth calculator based on position complexity.
///
/// DEPRECATED: Use TimeBudgetDepthManager for depth calculation.
/// The primary mechanism is time-budgeted iterative deepening that scales
/// with machine capability rather than hardcoded depths.
/// </summary>
public static class AdaptiveDepthCalculator
{
    /// <summary>
    /// Calculate adaptive depth based on position complexity.
    /// Analyzes stone count, threat count, and game phase to determine optimal depth.
    /// </summary>
    public static int GetAdaptiveDepth(Board board)
    {
        int stoneCount = board.Cells.Count(c => !c.IsEmpty);
        int threatCount = CountTotalThreats(board);

        bool isOpening = stoneCount < 20;
        bool isMiddlegame = stoneCount >= 20 && stoneCount < 100;
        bool isEndgame = stoneCount >= 100;

        if (isOpening)
            return 7;

        if (threatCount > 5)
            return 8;

        if (isEndgame)
            return 8;

        if (isMiddlegame)
            return 7;

        return 7;
    }

    private static int CountTotalThreats(Board board)
    {
        int totalThreats = 0;

        for (int x = 0; x < board.BoardSize; x++)
        {
            for (int y = 0; y < board.BoardSize; y++)
            {
                var cell = board.GetCell(x, y);
                if (cell.IsEmpty)
                    continue;

                var player = cell.Player;
                totalThreats += CountThreatsAtPosition(board, x, y, player);
            }
        }

        return totalThreats;
    }

    private static int CountThreatsAtPosition(Board board, int startX, int startY, Player player)
    {
        int threats = 0;
        var directions = new (int dx, int dy)[] { (1, 0), (0, 1), (1, 1), (1, -1) };

        foreach (var (dx, dy) in directions)
        {
            int count = 1;
            int emptyBefore = 0;
            int emptyAfter = 0;

            for (int i = 1; i <= 4; i++)
            {
                int x = startX + dx * i;
                int y = startY + dy * i;
                if (x < 0 || x >= board.BoardSize || y < 0 || y >= board.BoardSize)
                    break;

                var cell = board.GetCell(x, y);
                if (cell.Player == player)
                    count++;
                else if (cell.IsEmpty)
                {
                    emptyAfter++;
                    break;
                }
                else
                    break;
            }

            for (int i = 1; i <= 4; i++)
            {
                int x = startX - dx * i;
                int y = startY - dy * i;
                if (x < 0 || x >= board.BoardSize || y < 0 || y >= board.BoardSize)
                    break;

                var cell = board.GetCell(x, y);
                if (cell.Player == player)
                    count++;
                else if (cell.IsEmpty)
                {
                    emptyBefore++;
                    break;
                }
                else
                    break;
            }

            if (count >= 3 && (emptyBefore > 0 || emptyAfter > 0))
                threats++;
        }

        return threats;
    }
}
