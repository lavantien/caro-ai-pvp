namespace Caro.Core.Domain.Entities;

/// <summary>
/// Game mode types for Caro matches.
/// </summary>
public enum GameMode
{
    PvP = 0,
    PvAI = 1,
    AivAI = 2
}

public static class GameModeExtensions
{
    public static string ToLowerString(this GameMode mode) => mode switch
    {
        GameMode.PvP => "pvp",
        GameMode.PvAI => "pvai",
        GameMode.AivAI => "aivai",
        _ => "pvp"
    };
}
