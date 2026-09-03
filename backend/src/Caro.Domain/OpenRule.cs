namespace Caro.Domain;

public static class OpenRule
{
    public static bool IsValidSecondMove(Board b, int x, int y)
    {
        if (b.IsEmpty())
        {
            return true;
        }

        int redCount = 0;
        int blueCount = 0;
        int firstRedX = 0;
        int firstRedY = 0;
        for (int bx = 0; bx < Constants.Board.Size; bx++)
        {
            for (int by = 0; by < Constants.Board.Size; by++)
            {
                Player p = b.GetPlayerAt(bx, by);
                if (p == Player.Red)
                {
                    redCount++;
                    firstRedX = bx;
                    firstRedY = by;
                }
                else if (p == Player.Blue)
                {
                    blueCount++;
                }
            }
        }

        if (redCount != 1 || blueCount > 1)
        {
            return true;
        }

        int dx = x - firstRedX;
        int dy = y - firstRedY;
        if (dx < 0)
        {
            dx = -dx;
        }
        if (dy < 0)
        {
            dy = -dy;
        }
        return dx >= Constants.Board.OpenRuleMin || dy >= Constants.Board.OpenRuleMin;
    }
}
