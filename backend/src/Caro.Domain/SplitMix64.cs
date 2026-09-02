namespace Caro.Domain;

/// <summary>
/// splitmix64 step constant and finalizer shared by Zobrist hashing and the
/// seeded opening RNG. Stateless: callers keep their own generator state.
/// </summary>
public static class SplitMix64
{
    public const ulong GoldenGamma = 0x9E3779B97F4A7C15;

    public static ulong Mix(ulong z)
    {
        z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9;
        z = (z ^ (z >> 27)) * 0x94D049BB133111EB;
        return z ^ (z >> 31);
    }
}
