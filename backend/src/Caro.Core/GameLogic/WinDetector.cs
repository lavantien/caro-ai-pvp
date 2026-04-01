namespace Caro.Core.GameLogic;

using System.Runtime.CompilerServices;
using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;

/// <summary>
/// Extension methods for Position
/// </summary>
public static class PositionExtensions
{
    private const int BoardSize = GameConstants.BoardSize;

    /// <summary>
    /// Check if position is within board bounds
    /// </summary>
    public static bool IsValid(this Domain.Entities.Position position) =>
        position.X >= 0 && position.X < BoardSize &&
        position.Y >= 0 && position.Y < BoardSize;

    /// <summary>
    /// Check if raw coordinates are within board bounds
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static bool InBounds(int x, int y, int boardSize) =>
        (uint)x < boardSize && (uint)y < boardSize;

    /// <summary>
    /// Check if board is full (all cells occupied)
    /// </summary>
    public static bool IsFull(this Board board)
    {
        for (int x = 0; x < BoardSize; x++)
        {
            for (int y = 0; y < BoardSize; y++)
            {
                if (board.GetCell(x, y).IsEmpty)
                    return false;
            }
        }
        return true;
    }
}

public class WinDetector
{
    private static readonly (int dx, int dy)[] Directions =
    {
        (1, 0),   // Horizontal
        (0, 1),   // Vertical
        (1, 1),   // Diagonal down-right
        (1, -1)   // Diagonal down-left
    };

    public WinResult CheckWin(Board board)
    {
        for (int x = 0; x < board.BoardSize; x++)
        {
            for (int y = 0; y < board.BoardSize; y++)
            {
                var cell = board.GetCell(x, y);
                if (cell.IsEmpty)
                    continue;

                foreach (var (dx, dy) in Directions)
                {
                    if (CheckLine(board, x, y, dx, dy, out var count))
                    {
                        // Check for blocked ends
                        bool leftBlocked = IsPositionBlocked(board, x - dx, y - dy, cell.Player);
                        bool rightBlocked = IsPositionBlocked(board, x + count * dx, y + count * dy, cell.Player);

                        // Check for overline (more than 5 in a row)
                        bool hasExtension = HasPlayerAt(board, x - dx, y - dy, cell.Player) ||
                                          HasPlayerAt(board, x + count * dx, y + count * dy, cell.Player);

                        // Win only if exactly WinLength (not more) and not both ends blocked
                        if (count == GameConstants.WinLength && !hasExtension && !(leftBlocked && rightBlocked))
                        {
                            // Build winning line
                            var winningLine = new List<Domain.Entities.Position>();
                            for (int i = 0; i < GameConstants.WinLength; i++)
                            {
                                winningLine.Add(new Domain.Entities.Position(x + i * dx, y + i * dy));
                            }

                            return new WinResult
                            {
                                HasWinner = true,
                                Winner = cell.Player,
                                WinningLine = winningLine
                            };
                        }
                    }
                }
            }
        }

        return new WinResult { HasWinner = false };
    }

    /// <summary>
    /// Check for win from the last move position (efficient, no full board scan).
    /// Returns the winning line positions, or empty array if no win.
    /// </summary>
    public static Position[] CheckWinFromMove(Board board, int lastX, int lastY, Player player)
    {
        var boardSize = board.BoardSize;

        foreach (var (dx, dy) in Directions)
        {
            int count = 1;

            int x = lastX + dx;
            int y = lastY + dy;
            while (PositionExtensions.InBounds(x, y, boardSize) &&
                   board.GetCell(x, y).Player == player)
            {
                count++;
                x += dx;
                y += dy;
            }
            int positiveEndX = x;
            int positiveEndY = y;

            x = lastX - dx;
            y = lastY - dy;
            while (PositionExtensions.InBounds(x, y, boardSize) &&
                   board.GetCell(x, y).Player == player)
            {
                count++;
                x -= dx;
                y -= dy;
            }
            int negativeEndX = x;
            int negativeEndY = y;

            if (count == GameConstants.WinLength)
            {
                bool hasPositiveExtension = PositionExtensions.InBounds(positiveEndX, positiveEndY, boardSize) &&
                                            board.GetCell(positiveEndX, positiveEndY).Player == player;
                bool hasNegativeExtension = PositionExtensions.InBounds(negativeEndX, negativeEndY, boardSize) &&
                                            board.GetCell(negativeEndX, negativeEndY).Player == player;

                if (hasPositiveExtension || hasNegativeExtension)
                    continue;

                bool positiveBlocked = IsBlockedAt(board, positiveEndX, positiveEndY, player);
                bool negativeBlocked = IsBlockedAt(board, negativeEndX, negativeEndY, player);

                if (positiveBlocked && negativeBlocked)
                    continue;

                return BuildLine(board, lastX, lastY, dx, dy, player, boardSize);
            }
        }

        return Array.Empty<Position>();
    }

    private static bool IsBlockedAt(Board board, int x, int y, Player player)
    {
        if (!PositionExtensions.InBounds(x, y, board.BoardSize))
            return true;

        var cell = board.GetCell(x, y);
        return !cell.IsEmpty && cell.Player != player;
    }

    private static Position[] BuildLine(Board board, int lastX, int lastY, int dx, int dy, Player player, int boardSize)
    {
        int startX = lastX;
        int startY = lastY;
        int prevX = startX - dx;
        int prevY = startY - dy;
        while (PositionExtensions.InBounds(prevX, prevY, boardSize) &&
               board.GetCell(prevX, prevY).Player == player)
        {
            startX = prevX;
            startY = prevY;
            prevX -= dx;
            prevY -= dy;
        }

        var positions = new Position[GameConstants.WinLength];
        int px = startX;
        int py = startY;
        for (int i = 0; i < GameConstants.WinLength; i++)
        {
            positions[i] = new Position(px, py);
            px += dx;
            py += dy;
        }

        return positions;
    }

    private bool HasPlayerAt(Board board, int x, int y, Player player)
    {
        if (!PositionExtensions.InBounds(x, y, board.BoardSize))
            return false;

        return board.GetCell(x, y).Player == player;
    }

    private bool CheckLine(Board board, int startX, int startY, int dx, int dy, out int count)
    {
        count = 0;
        var player = board.GetCell(startX, startY).Player;
        int x = startX, y = startY;

        while (PositionExtensions.InBounds(x, y, board.BoardSize))
        {
            if (board.GetCell(x, y).Player != player)
                break;
            count++;
            x += dx;
            y += dy;
        }

        return count >= GameConstants.WinLength;
    }

    private bool IsPositionBlocked(Board board, int x, int y, Player player)
    {
        if (!PositionExtensions.InBounds(x, y, board.BoardSize))
            return true;

        var cell = board.GetCell(x, y);
        return !cell.IsEmpty && cell.Player != player;
    }
}

public class WinResult
{
    public bool HasWinner { get; set; }
    public Player Winner { get; set; } = Player.None;
    public List<Domain.Entities.Position> WinningLine { get; set; } = new();
}
