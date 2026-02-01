namespace Caro.Core.Domain.Entities;

/// <summary>
/// Represents a player in the Caro game.
/// None represents an empty cell or no current player.
/// </summary>
public enum Player
{
    /// <summary>No player (empty cell)</summary>
    None,

    /// <summary>Red player (moves first)</summary>
    Red,

    /// <summary>Blue player (moves second)</summary>
    Blue
}

/// <summary>
/// Extension methods for Player enum
/// </summary>
public static class PlayerExtensions
{
    /// <summary>
    /// Get the opponent of the given player
    /// </summary>
    public static Player Opponent(this Player player) => player switch
    {
        Player.Red => Player.Blue,
        Player.Blue => Player.Red,
        _ => Player.None
    };

    /// <summary>
    /// Check if the player is valid (not None)
    /// </summary>
    public static bool IsValid(this Player player) => player is Player.Red or Player.Blue;

    /// <summary>
    /// Check if the player is Red
    /// </summary>
    public static bool IsRed(this Player player) => player == Player.Red;

    /// <summary>
    /// Check if the player is Blue
    /// </summary>
    public static bool IsBlue(this Player player) => player == Player.Blue;
}
