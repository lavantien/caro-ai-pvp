namespace Caro.Domain;

public readonly struct WinResult
{
    public bool HasWinner { get; init; }
    public Player Winner { get; init; }
    public Position[]? WinningLine { get; init; }

    public static WinResult None => default;
}

public static class WinDetector
{
    public static WinResult CheckWin(Board b)
    {
        for (int x = 0; x < Constants.Board.Size; x++)
        {
            for (int y = 0; y < Constants.Board.Size; y++)
            {
                Player p = b.GetPlayerAt(x, y);
                if (p == Player.None)
                {
                    continue;
                }
                WinResult result = CheckWinFrom(b, x, y, p);
                if (result.HasWinner)
                {
                    return result;
                }
            }
        }
        return WinResult.None;
    }

    public static WinResult CheckWinFromMove(Board b, int x, int y)
    {
        Player p = b.GetPlayerAt(x, y);
        if (p == Player.None)
        {
            return WinResult.None;
        }
        return CheckWinFrom(b, x, y, p);
    }

    private static WinResult CheckWinFrom(Board b, int x, int y, Player player)
    {
        foreach ((int dx, int dy) in Constants.Directions)
        {
            int positive = 0;
            for (int i = 1; i <= Constants.Board.WinLength; i++)
            {
                int nx = x + dx * i;
                int ny = y + dy * i;
                if (nx < 0 || nx >= Constants.Board.Size || ny < 0 || ny >= Constants.Board.Size)
                {
                    break;
                }
                if (b.GetPlayerAt(nx, ny) != player)
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
                if (nx < 0 || nx >= Constants.Board.Size || ny < 0 || ny >= Constants.Board.Size)
                {
                    break;
                }
                if (b.GetPlayerAt(nx, ny) != player)
                {
                    break;
                }
                negative++;
            }

            int total = 1 + positive + negative;
            if (total != Constants.Board.WinLength)
            {
                continue;
            }

            // Caro: both ends blocked = no win
            int afterX = x + dx * (positive + 1);
            int afterY = y + dy * (positive + 1);
            int beforeX = x - dx * (negative + 1);
            int beforeY = y - dy * (negative + 1);

            bool afterBlocked = afterX < 0 || afterX >= Constants.Board.Size
                || afterY < 0 || afterY >= Constants.Board.Size
                || b.GetPlayerAt(afterX, afterY) != Player.None;
            bool beforeBlocked = beforeX < 0 || beforeX >= Constants.Board.Size
                || beforeY < 0 || beforeY >= Constants.Board.Size
                || b.GetPlayerAt(beforeX, beforeY) != Player.None;

            if (afterBlocked && beforeBlocked)
            {
                continue;
            }

            int startX = x - dx * negative;
            int startY = y - dy * negative;
            Position[] line = new Position[Constants.Board.WinLength];
            for (int i = 0; i < Constants.Board.WinLength; i++)
            {
                line[i] = new Position(startX + dx * i, startY + dy * i);
            }
            return new WinResult { HasWinner = true, Winner = player, WinningLine = line };
        }
        return WinResult.None;
    }
}
