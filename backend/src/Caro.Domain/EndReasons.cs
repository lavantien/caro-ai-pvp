namespace Caro.Domain;

/// <summary>
/// Values reported in GameState.EndReason and the JSON API; Abandoned is the
/// winner sentinel persisted for deleted games. API clients and the match DB
/// depend on these exact strings.
/// </summary>
public static class EndReasons
{
    public const string Win = "win";
    public const string Timeout = "timeout";
    public const string Draw = "draw";
    public const string Abandoned = "abandoned";
}
