namespace Caro.Domain;

public static partial class Constants
{
    /// <summary>Shared scan directions: horizontal, vertical, both diagonals.</summary>
    public static readonly (int Dx, int Dy)[] Directions =
    [
        (1, 0),
        (0, 1),
        (1, 1),
        (1, -1),
    ];

    /// <summary>
    /// Host-adaptation modes for difficulty-profile thread counts: fixed one
    /// or two threads, or a share of the processor count with cores reserved
    /// for the host. Mapped to concrete counts by
    /// Difficulty.GetDifficultyProfile.
    /// </summary>
    public enum ProfileThreads
    {
        One,
        Two,
        HalfL5,
        L5,
    }

    public readonly record struct DifficultyProfileData(
        int Level,
        string Name,
        double TimeFraction,
        int MaxDepth,
        ProfileThreads Threads,
        bool UseVCF,
        int VCFDepth,
        bool Ponder,
        int TTSizeMB);

    /// <summary>
    /// Per-level strength ladder. Levels are strength-based first (depth
    /// caps, solver sight and parallel gating) and time-fraction scaled
    /// second, so L(k) is stronger than L(k-1) on any host. L3/L4 caps stay
    /// at or below 5: measured at bullet, ID depth past ~6 stops buying
    /// strength in self-play, so the ladder keeps those levels below the
    /// plateau and scales VCF sight instead.
    /// </summary>
    public static readonly List<DifficultyProfileData> DifficultyProfiles =
    [
        new(1, "Novice", 0.05, 2, ProfileThreads.One, false, 0, false, 64),
        new(2, "Beginner", 0.15, 4, ProfileThreads.One, false, 0, false, 64),
        new(3, "Intermediate", 0.40, 4, ProfileThreads.Two, true, 2, false, 256),
        new(4, "Advanced", 0.70, 5, ProfileThreads.HalfL5, true, 4, false, Transposition.DefaultSizeMB),
        new(5, "Grandmaster", 1.0, Search.AbsoluteMaxDepth, ProfileThreads.L5, true, Vcf.SearchDepth, true, Transposition.DefaultSizeMB),
    ];

    public readonly record struct TimeControlData(string Canonical, long InitialTimeMs, int IncrementSeconds);

    /// <summary>
    /// Canonical time-control table for game creation. Keys are the frontend
    /// select values; the bullet/blitz/classical aliases are the Go engine's
    /// legacy inputs and keep resolving. Mirrored by the frontend
    /// timeControlConfig and scripts/lib TIME_CONTROLS lists.
    /// </summary>
    public static readonly Dictionary<string, TimeControlData> TimeControls = new()
    {
        ["1+0"] = new("1+0", 60_000, 0),
        ["bullet"] = new("1+0", 60_000, 0),
        ["3+2"] = new("3+2", 180_000, 2),
        ["blitz"] = new("3+2", 180_000, 2),
        ["3+0"] = new("3+0", 180_000, 0),
        ["10+0"] = new("10+0", 600_000, 0),
        ["15+10"] = new("15+10", 900_000, 10),
        ["classical"] = new("15+10", 900_000, 10),
    };
}
