using Caro.Domain;
using Xunit;

namespace Caro.Domain.Tests;

public class ZobristTests
{
    [Fact]
    public void ZobristKeysAreNonZero()
    {
        for (int x = 0; x < Constants.Board.Size; x++)
        {
            for (int y = 0; y < Constants.Board.Size; y++)
            {
                Assert.NotEqual(0UL, Zobrist.ZobristKey(x, y, Player.Red));
                Assert.NotEqual(0UL, Zobrist.ZobristKey(x, y, Player.Blue));
            }
        }
    }

    [Fact]
    public void ZobristKeysAreDistinct()
    {
        Dictionary<ulong, string> seen = [];
        for (int x = 0; x < Constants.Board.Size; x++)
        {
            for (int y = 0; y < Constants.Board.Size; y++)
            {
                ulong kr = Zobrist.ZobristKey(x, y, Player.Red);
                Assert.False(seen.ContainsKey(kr), $"duplicate key with {x},{y} red (also {seen.GetValueOrDefault(kr)})");
                seen[kr] = $"{x},{y} red";

                ulong kb = Zobrist.ZobristKey(x, y, Player.Blue);
                Assert.False(seen.ContainsKey(kb), $"duplicate key with {x},{y} blue (also {seen.GetValueOrDefault(kb)})");
                seen[kb] = $"{x},{y} blue";
            }
        }
    }

    [Fact]
    public void ZobristDeterministic()
    {
        ulong k1 = Zobrist.ZobristKey(5, 5, Player.Red);
        ulong k2 = Zobrist.ZobristKey(5, 5, Player.Red);
        Assert.Equal(k1, k2);
    }
}
