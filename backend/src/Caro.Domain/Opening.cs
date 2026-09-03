namespace Caro.Domain;

/// <summary>
/// Seeded two-stone opening placement: red from the center region, blue
/// replying locally. Deterministic per seed. Shared by the API session and
/// the tournament regression fixtures so both derive identical openings.
/// </summary>
public static class Opening
{
    public static ((int X, int Y) Red, (int X, int Y) Blue) SeededPlacements(long seed, int spreadRadius)
    {
        OpeningRng rng = new(seed);
        int low = Constants.Board.Size / 2 - spreadRadius;
        int high = Constants.Board.Size / 2 + spreadRadius - 1;
        int rx = low + rng.Next(high - low + 1);
        int ry = low + rng.Next(high - low + 1);

        int bx = rx - spreadRadius + rng.Next(2 * spreadRadius + 1);
        int by = ry - spreadRadius + rng.Next(2 * spreadRadius + 1);
        bx = Math.Clamp(bx, 0, Constants.Board.Size - 1);
        by = Math.Clamp(by, 0, Constants.Board.Size - 1);
        if (bx == rx && by == ry)
        {
            bx = (bx + 1) % Constants.Board.Size;
        }
        return ((rx, ry), (bx, by));
    }

    // splitmix64 generator: small, deterministic, seedable.
    private sealed class OpeningRng(long seed)
    {
        private ulong _state = (ulong)seed;

        public int Next(int n)
        {
            _state += SplitMix64.GoldenGamma;
            return (int)(SplitMix64.Mix(_state) % (ulong)n);
        }
    }
}
