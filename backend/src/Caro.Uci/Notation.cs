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
        if (x < 0 || x >= 16 || y < 0 || y >= 16)
        {
            x = 0;
            y = 0;
            return false;
        }
        return true;
    }
}
