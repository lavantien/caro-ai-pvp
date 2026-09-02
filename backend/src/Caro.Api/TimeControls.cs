namespace Caro.Api;

/// <summary>
/// Canonical time-control table for game creation. Keys are the frontend
/// select values; the bullet/blitz/classical aliases are the Go engine's
/// legacy inputs and keep resolving. Mirrored by the frontend
/// timeControlConfig and scripts/lib TIME_CONTROLS lists.
/// </summary>
internal static class TimeControls
{
    public const string Default = "7+5";
    public const long DefaultInitialTimeMs = 420_000;
    public const int DefaultIncrementSeconds = 5;

    private static readonly Dictionary<string, (string Canonical, long InitialTimeMs, int IncrementSeconds)> Table =
        new()
        {
            ["1+0"] = ("1+0", 60_000, 0),
            ["bullet"] = ("1+0", 60_000, 0),
            ["3+2"] = ("3+2", 180_000, 2),
            ["blitz"] = ("3+2", 180_000, 2),
            ["3+0"] = ("3+0", 180_000, 0),
            ["10+0"] = ("10+0", 600_000, 0),
            ["15+10"] = ("15+10", 900_000, 10),
            ["classical"] = ("15+10", 900_000, 10),
        };

    public static (string Canonical, long InitialTimeMs, int IncrementSeconds) Resolve(string? requested) =>
        requested is not null && Table.TryGetValue(requested, out (string Canonical, long InitialTimeMs, int IncrementSeconds) entry)
            ? entry
            : (Default, DefaultInitialTimeMs, DefaultIncrementSeconds);
}
