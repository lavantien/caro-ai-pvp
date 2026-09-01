using Caro.Api;
using Caro.Domain;
using Xunit;
using static Caro.Api.Tests.GameSessionTests;

namespace Caro.Api.Tests;

/// <summary>
/// The seeded opening's blue reply can, for rare seeds, clamp onto the red
/// stone; the shift-by-one branch must still produce a legal two-stone
/// opening.
/// </summary>
public class OpeningCollisionTests
{
    private sealed class SplitMix64(long seed)
    {
        private ulong _state = (ulong)seed;

        public int Next(int n)
        {
            _state += 0x9E3779B97F4A7C15;
            ulong z = _state;
            z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9;
            z = (z ^ (z >> 27)) * 0x94D049BB133111EB;
            z ^= z >> 31;
            return (int)(z % (ulong)n);
        }
    }

    // Replicates GameSession.ApplyRandomOpening to predict the collision.
    private static (int Rx, int Ry, int Bx, int By) PredictOpening(long seed)
    {
        SplitMix64 rng = new(seed);
        int low = Constants.BoardSize / 2 - 3;
        int high = Constants.BoardSize / 2 + 2;
        int rx = low + rng.Next(high - low + 1);
        int ry = low + rng.Next(high - low + 1);

        int bx = Math.Clamp(rx - 3 + rng.Next(7), 0, Constants.BoardSize - 1);
        int by = Math.Clamp(ry - 3 + rng.Next(7), 0, Constants.BoardSize - 1);
        if (bx == rx && by == ry)
        {
            bx = (bx + 1) % Constants.BoardSize;
        }
        return (rx, ry, bx, by);
    }

    private static long FindCollisionSeed()
    {
        for (long seed = 1; seed <= 200_000; seed++)
        {
            SplitMix64 rng = new(seed);
            int low = Constants.BoardSize / 2 - 3;
            int high = Constants.BoardSize / 2 + 2;
            int rx = low + rng.Next(high - low + 1);
            int ry = low + rng.Next(high - low + 1);
            int bx = Math.Clamp(rx - 3 + rng.Next(7), 0, Constants.BoardSize - 1);
            int by = Math.Clamp(ry - 3 + rng.Next(7), 0, Constants.BoardSize - 1);
            if (bx == rx && by == ry)
            {
                return seed;
            }
        }
        Assert.Fail("no collision seed in range");
        return 0;
    }

    [Fact]
    public void RandomOpeningShiftsBlueOffCollision()
    {
        long seed = FindCollisionSeed();
        (int rx, int ry, int bx, int by) = PredictOpening(seed);
        Assert.NotEqual((rx, ry), (bx, by));

        GameSession s = NewTestSession();
        s.ApplyRandomOpening(seed);

        Assert.Equal(2, s.GameForTest.MoveNumber);
        Assert.Equal(Player.Red, s.GameForTest.Board.GetPlayerAt(rx, ry));
        Assert.Equal(Player.Blue, s.GameForTest.Board.GetPlayerAt(bx, by));
    }
}
