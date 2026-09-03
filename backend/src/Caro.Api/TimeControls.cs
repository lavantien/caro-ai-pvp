using Caro.Domain;

namespace Caro.Api;

/// <summary>
/// Resolves game-creation time controls against the central table in
/// Constants.TimeControls; unknown or missing values fall back to
/// Constants.TimeControl defaults.
/// </summary>
internal static class TimeControls
{
    public static (string Canonical, long InitialTimeMs, int IncrementSeconds) Resolve(string? requested) =>
        requested is not null && Constants.TimeControls.TryGetValue(requested, out Constants.TimeControlData entry)
            ? (entry.Canonical, entry.InitialTimeMs, entry.IncrementSeconds)
            : (Constants.TimeControl.Default, Constants.TimeControl.DefaultInitialTimeMs, Constants.TimeControl.DefaultIncrementSeconds);
}
