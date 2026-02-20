namespace Caro.Core.Domain.Configuration;

/// <summary>
/// Centralized game constants - single source of truth for game rules.
/// All game logic should reference these constants instead of hardcoding values.
/// </summary>
public static class GameConstants
{
    /// <summary>
    /// Board size (16x16 grid)
    /// </summary>
    public const int BoardSize = 16;

    /// <summary>
    /// Total number of cells on the board (16 * 16 = 256)
    /// </summary>
    public const int TotalCells = BoardSize * BoardSize;

    /// <summary>
    /// Center position index (8 is center of 0-15 range)
    /// </summary>
    public const int CenterPosition = BoardSize / 2;

    /// <summary>
    /// Number of consecutive stones required to win
    /// </summary>
    public const int WinLength = 5;

    /// <summary>
    /// ELO rating system K-factor (determines rating volatility)
    /// </summary>
    public const int EloKFactor = 32;

    /// <summary>
    /// Default ELO rating for new players
    /// </summary>
    public const int DefaultEloRating = 1500;
}
