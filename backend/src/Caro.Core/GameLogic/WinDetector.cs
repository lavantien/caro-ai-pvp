namespace Caro.Core.GameLogic;

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

    private bool HasPlayerAt(Board board, int x, int y, Player player)
    {
        if (x < 0 || x >= board.BoardSize || y < 0 || y >= board.BoardSize)
            return false;

        return board.GetCell(x, y).Player == player;
    }

    private bool CheckLine(Board board, int startX, int startY, int dx, int dy, out int count)
    {
        count = 0;
        var player = board.GetCell(startX, startY).Player;
        int x = startX, y = startY;

        while (x >= 0 && x < board.BoardSize && y >= 0 && y < board.BoardSize)
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
        if (x < 0 || x >= board.BoardSize || y < 0 || y >= board.BoardSize)
            return true;  // Edge of board counts as blocked

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
