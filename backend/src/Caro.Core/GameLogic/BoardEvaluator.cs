using Caro.Core.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Evaluates board positions for AI decision-making
/// </summary>
public class BoardEvaluator
{
    // Scoring weights
    private const int FourInRowScore = 10000;
    private const int ThreeInRowScore = 1000;
    private const int TwoInRowScore = 100;
    private const int OneInRowScore = 10;
    private const int CenterBonus = 50;

    // Direction vectors: horizontal, vertical, 2 diagonals
    private static readonly (int dx, int dy)[] Directions = new[]
    {
        (1, 0),   // Horizontal
        (0, 1),   // Vertical
        (1, 1),   // Diagonal down-right
        (1, -1)   // Diagonal down-left
    };

    /// <summary>
    /// Evaluate the board for a given player
    /// Positive score = good for player
    /// Negative score = good for opponent
    /// </summary>
    public int Evaluate(Board board, Player player)
    {
        if (player == Player.None)
            throw new ArgumentException("Player cannot be None");

        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var score = 0;

        // Evaluate all lines (horizontal, vertical, diagonal)
        score += EvaluateAllLines(board, player, opponent);

        // Center control bonus
        score += EvaluateCenterControl(board, player);

        return score;
    }

    private int EvaluateAllLines(Board board, Player player, Player opponent)
    {
        var score = 0;

        // Scan all possible starting positions for lines
        for (int x = 0; x < 15; x++)
        {
            for (int y = 0; y < 15; y++)
            {
                // Check each direction from each position
                foreach (var (dx, dy) in Directions)
                {
                    // Evaluate line for player
                    score += EvaluateLine(board, x, y, dx, dy, player);

                    // Evaluate line for opponent (subtracts from score)
                    score -= EvaluateLine(board, x, y, dx, dy, opponent);
                }
            }
        }

        return score;
    }

    private int EvaluateLine(Board board, int startX, int startY, int dx, int dy, Player player)
    {
        var count = 0;
        var openEnds = 0;
        var blocked = 0;

        // Count consecutive stones for player in this direction
        for (int i = 0; i < 5; i++)
        {
            var x = startX + i * dx;
            var y = startY + i * dy;

            // Check bounds
            if (x < 0 || x >= 15 || y < 0 || y >= 15)
            {
                blocked++;
                break;
            }

            var cell = board.GetCell(x, y);

            if (cell.Player == player)
            {
                count++;
            }
            else if (cell.Player == Player.None)
            {
                openEnds++;
                break; // Empty cell ends the streak
            }
            else
            {
                blocked++;
                break; // Opponent stone ends the streak
            }
        }

        // Check if the start of the line is open
        if (startX - dx >= 0 && startX - dx < 15 &&
            startY - dy >= 0 && startY - dy < 15)
        {
            var beforeCell = board.GetCell(startX - dx, startY - dy);
            if (beforeCell.Player == Player.None)
                openEnds++;
        }

        // Score based on count and open ends
        return ScoreSequence(count, openEnds, blocked);
    }

    private int ScoreSequence(int count, int openEnds, int blocked)
    {
        // If blocked on both ends, very low value
        if (blocked >= 2)
            return count switch
            {
                4 => 100,  // Blocked 4-in-row (can't win)
                3 => 10,   // Blocked 3-in-row
                _ => 0
            };

        return count switch
        {
            4 when openEnds > 0 => FourInRowScore,  // 4 in a row, can win
            3 => openEnds switch
            {
                2 => ThreeInRowScore * 2,   // Open on both ends (very dangerous)
                1 => ThreeInRowScore,       // Open on one end
                _ => 50
            },
            2 when openEnds > 0 => TwoInRowScore,
            1 when openEnds > 0 => OneInRowScore,
            _ => 0
        };
    }

    private int EvaluateCenterControl(Board board, Player player)
    {
        var score = 0;

        // Value center 5x5 zone more
        for (int x = 5; x <= 9; x++)
        {
            for (int y = 5; y <= 9; y++)
            {
                if (board.GetCell(x, y).Player == player)
                {
                    // Center cell (7,7) gets highest bonus
                    var distanceToCenter = Math.Abs(x - 7) + Math.Abs(y - 7);
                    score += CenterBonus - (distanceToCenter * 5);
                }
            }
        }

        return score;
    }
}
