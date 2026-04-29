using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic.Search;

/// <summary>
/// Stateless tactical utilities for pre-search quick wins and board scanning.
/// These methods detect immediate tactical patterns before the full search runs.
/// </summary>
public static class QuickWinChecker
{
    private const int BoardSize = GameConstants.BoardSize;

    /// <summary>
    /// Find a move that creates an immediate winning threat for the given player.
    /// Scans for any empty square that completes a verified 5-in-a-row.
    /// Used by safeguard when opponent has multiple independent threats that can't all be blocked.
    /// </summary>
    public static (int x, int y)? FindOurWinningMove(Board board, Player player, ThreatDetector threatDetector)
    {
        for (int x = 0; x < BoardSize; x++)
        {
            for (int y = 0; y < BoardSize; y++)
            {
                if (board.GetCell(x, y).Player == Player.None)
                {
                    if (threatDetector.IsWinningMove(board, x, y, player))
                    {
                        return (x, y);
                    }
                }
            }
        }
        return null;
    }

    /// <summary>
    /// Find any empty square on the board. Used as a final fallback when
    /// the proposed move is occupied and no threats exist.
    /// Prefers squares near the invalid move's location for aesthetic reasons.
    /// </summary>
    public static (int x, int y) FindAnyEmptySquare(Board board, (int x, int y) invalidMove)
    {
        // First try near the invalid move's location
        for (int dx = -1; dx <= 1; dx++)
        {
            for (int dy = -1; dy <= 1; dy++)
            {
                int nx = invalidMove.x + dx;
                int ny = invalidMove.y + dy;
                if (nx >= 0 && nx < BoardSize && ny >= 0 && ny < BoardSize)
                {
                    if (board.GetCell(nx, ny).Player == Player.None)
                    {
                        return (nx, ny);
                    }
                }
            }
        }

        // Scan the entire board for any empty square
        for (int x = 0; x < BoardSize; x++)
        {
            for (int y = 0; y < BoardSize; y++)
            {
                if (board.GetCell(x, y).Player == Player.None)
                {
                    return (x, y);
                }
            }
        }

        // Board is completely full (draw) - return center as absolute fallback
        return (BoardSize / 2, BoardSize / 2);
    }

    /// <summary>
    /// Get ALL legal moves (every empty cell on the board).
    /// Used for error rate simulation - true random moves, not tactical moves.
    /// </summary>
    public static List<(int x, int y)> GetAllLegalMoves(Board board)
    {
        var legalMoves = new List<(int x, int y)>(64);

        for (int x = 0; x < BoardSize; x++)
        {
            for (int y = 0; y < BoardSize; y++)
            {
                if (board.GetCell(x, y).Player == Player.None)
                {
                    legalMoves.Add((x, y));
                }
            }
        }

        return legalMoves;
    }

    /// <summary>
    /// PROACTIVE DEFENSE: Find squares that block opponent's open threes.
    /// An open three is 3 stones in a row with BOTH ends open (not blocked).
    /// Open threes become open fours on the next move, which are unblockable.
    /// We should block open threes BEFORE they become open fours.
    /// </summary>
    public static List<(int x, int y)> FindOpenThreeBlocks(Board board, Player opponent)
    {
        var blocks = new List<(int x, int y)>();
        var directions = GameConstants.CardinalDirections;

        // Scan for open threes in all 4 directions
        for (int x = 0; x < BoardSize; x++)
        {
            for (int y = 0; y < BoardSize; y++)
            {
                if (board.GetCell(x, y).Player != opponent)
                    continue;

                foreach (var (dx, dy) in directions)
                {
                    // Check if this stone is the START of a 3-in-a-row
                    int prevX = x - dx;
                    int prevY = y - dy;

                    // Skip if not the start (previous cell is also opponent's stone)
                    if (prevX >= 0 && prevX < BoardSize && prevY >= 0 && prevY < BoardSize)
                    {
                        if (board.GetCell(prevX, prevY).Player == opponent)
                            continue;
                    }

                    // Count consecutive opponent stones
                    int count = 0;
                    int currX = x, currY = y;
                    while (currX >= 0 && currX < BoardSize && currY >= 0 && currY < BoardSize &&
                           board.GetCell(currX, currY).Player == opponent)
                    {
                        count++;
                        currX += dx;
                        currY += dy;
                    }

                    // Only interested in exactly 3 consecutive stones
                    if (count != 3)
                        continue;

                    // Check if both ends are open (empty)
                    int endX = currX;
                    int endY = currY;
                    bool endOpen = endX >= 0 && endX < BoardSize && endY >= 0 && endY < BoardSize &&
                                   board.GetCell(endX, endY).Player == Player.None;

                    int startX = x - dx;
                    int startY = y - dy;
                    bool startOpen = startX >= 0 && startX < BoardSize && startY >= 0 && startY < BoardSize &&
                                     board.GetCell(startX, startY).Player == Player.None;

                    // Open three: 3 in a row with both ends open
                    if (startOpen && endOpen)
                    {
                        if (!blocks.Contains((startX, startY)))
                            blocks.Add((startX, startY));
                        if (!blocks.Contains((endX, endY)))
                            blocks.Add((endX, endY));
                    }
                }
            }
        }

        return blocks;
    }

    /// <summary>
    /// Get the last move made by the opponent.
    /// Scans for the most recently placed opponent stone.
    /// </summary>
    public static (int x, int y)? GetLastOpponentMove(Board board, Player currentPlayer)
    {
        var opponent = currentPlayer == Player.Red ? Player.Blue : Player.Red;

        // Find the most recent opponent move by checking all occupied cells
        for (int x = 0; x < BoardSize; x++)
        {
            for (int y = 0; y < BoardSize; y++)
            {
                if (board.GetCell(x, y).Player == opponent)
                {
                    return (x, y);
                }
            }
        }

        return null;
    }
}
