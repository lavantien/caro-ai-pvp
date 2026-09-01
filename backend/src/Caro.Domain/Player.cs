namespace Caro.Domain;

public enum Player : byte
{
    None = 0,
    Red = 1,
    Blue = 2,
}

public static class Players
{
    public static Player Opponent(this Player p) => p switch
    {
        Player.Red => Player.Blue,
        Player.Blue => Player.Red,
        _ => Player.None,
    };

    public static bool IsValid(this Player p) => p is Player.Red or Player.Blue;

    public static string ToName(this Player p) => p switch
    {
        Player.Red => "red",
        Player.Blue => "blue",
        _ => "none",
    };

    public static bool TryParse(string? s, out Player player)
    {
        switch (s)
        {
            case "red":
                player = Player.Red;
                return true;
            case "blue":
                player = Player.Blue;
                return true;
            case "none":
                player = Player.None;
                return true;
            default:
                player = Player.None;
                return false;
        }
    }
}
