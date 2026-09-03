using Caro.Domain;

namespace Caro.Uci;

public static class Notation
{
    public static string MoveToString(int x, int y) =>
        string.Concat((char)('a' + y), (char)('a' + x));

    public static bool TryParseMove(string s, out int x, out int y)
    {
        x = 0;
        y = 0;
        if (s.Length < 2)
        {
            return false;
        }
        y = s[0] - 'a';
        x = s[1] - 'a';
        if (x < 0 || x >= Constants.Board.Size || y < 0 || y >= Constants.Board.Size)
        {
            x = 0;
            y = 0;
            return false;
        }
        return true;
    }
}
