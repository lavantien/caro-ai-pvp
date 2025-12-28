namespace Caro.Core.GameLogic;

using Caro.Core.Entities;

public struct Position(int x, int y)
{
    public int X { get; set; } = x;
    public int Y { get; set; } = y;
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

                        // Win only if exactly 5 (not 6+) and not both ends blocked
                        if (count == 5 && !hasExtension && !(leftBlocked && rightBlocked))
                        {
                            // Build winning line
                            var winningLine = new List<Position>();
                            for (int i = 0; i < 5; i++)
                            {
                                winningLine.Add(new Position(x + i * dx, y + i * dy));
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

        return count >= 5;
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
    public List<Position> WinningLine { get; set; } = new();
}
