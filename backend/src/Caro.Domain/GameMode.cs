namespace Caro.Domain;

public enum GameMode
{
    PvP = 0,
    PvAI = 1,
    AivAI = 2,
}

public static class GameModes
{
    public static string ToName(this GameMode m) => m switch
    {
        GameMode.PvAI => "pvai",
        GameMode.AivAI => "aivai",
        _ => "pvp",
    };

    public static GameMode Parse(string? s) => s switch
    {
        "pvai" => GameMode.PvAI,
        "aivai" => GameMode.AivAI,
        _ => GameMode.PvP,
    };
}
