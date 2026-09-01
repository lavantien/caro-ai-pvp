namespace Caro.Domain;

public static class Zobrist
{
    private static readonly ulong[] Table = new ulong[Constants.BoardSize * Constants.BoardSize * 2];
    private static readonly ulong NullMoveKey;

    static Zobrist()
    {
        ulong state = 0x58A2C43F5A3B7E91;
        for (int i = 0; i < Table.Length; i++)
        {
            state += 0x9E3779B97F4A7C15;
            ulong z = state;
            z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9;
            z = (z ^ (z >> 27)) * 0x94D049BB133111EB;
            Table[i] = z ^ (z >> 31);
        }
        state += 0x9E3779B97F4A7C15;
        ulong zn = state;
        zn = (zn ^ (zn >> 30)) * 0xBF58476D1CE4E5B9;
        zn = (zn ^ (zn >> 27)) * 0x94D049BB133111EB;
        NullMoveKey = zn ^ (zn >> 31);
    }

    public static ulong ZobristKey(int x, int y, Player player)
    {
        int playerIndex = 0;
        if (player == Player.Blue)
        {
            playerIndex = 1;
        }
        return Table[x * Constants.BoardSize * 2 + y * 2 + playerIndex];
    }

    public static ulong ZobristNullMove() => NullMoveKey;
}
