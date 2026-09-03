namespace Caro.Domain;

public static class Zobrist
{
    private const ulong Seed = 0x58A2C43F5A3B7E91;

    private static readonly ulong[] Table = new ulong[Constants.Board.Size * Constants.Board.Size * 2];
    private static readonly ulong NullMoveKey;

    static Zobrist()
    {
        ulong state = Seed;
        for (int i = 0; i < Table.Length; i++)
        {
            state += SplitMix64.GoldenGamma;
            Table[i] = SplitMix64.Mix(state);
        }
        state += SplitMix64.GoldenGamma;
        NullMoveKey = SplitMix64.Mix(state);
    }

    public static ulong ZobristKey(int x, int y, Player player)
    {
        int playerIndex = 0;
        if (player == Player.Blue)
        {
            playerIndex = 1;
        }
        return Table[x * Constants.Board.Size * 2 + y * 2 + playerIndex];
    }

    public static ulong ZobristNullMove() => NullMoveKey;
}
