using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Counter-move history for move ordering based on opponent moves.
/// Tracks which moves are good responses to specific opponent moves.
/// Complements continuation history (which tracks our own previous moves).
/// Based on Stockfish 18's counter-move history implementation.
/// 
/// Uses bounded update formula to prevent overflow:
/// newValue = current + bonus - abs(current * bonus) / MaxScore
/// 
/// Memory overhead: 2 * 361 * 361 * 2 bytes = ~500KB
/// Expected ELO gain: +10-20 by improving move ordering for opponent responses.
/// </summary>
public sealed class CounterMoveHistory
{
    /// <summary>
    /// Maximum score to prevent overflow in bounded updates.
    /// Matches ContinuationHistory limits for consistency.
    /// </summary>
    public const int MaxScore = 30000;

    /// <summary>
    /// Total cells on the board for array sizing.
    /// </summary>
    private const int BoardSize = GameConstants.TotalCells;

    /// <summary>
    /// Multi-dimensional array tracking counter-move history.
    /// Dimensions: [player, opponentCell, ourCell]
    /// - player: 3 entries (None, Red, Blue) - we use Player enum directly
    /// - opponentCell: The opponent's previous move position (0-360)
    /// - ourCell: Our response move position being evaluated (0-360)
    /// </summary>
    private readonly short[,,] _history;

    /// <summary>
    /// Create a new counter-move history table initialized to zero.
    /// </summary>
    public CounterMoveHistory()
    {
        _history = new short[3, BoardSize, BoardSize]; // None, Red, Blue
    }

    /// <summary>
    /// Get the counter-move history score for a specific response to an opponent move.
    /// Higher scores indicate this move has been a good response to the opponent's move.
    /// </summary>
    /// <param name="player">The player to get history for (Red or Blue)</param>
    /// <param name="opponentCell">The opponent's previous move position (0-360, or -1 if none)</param>
    /// <param name="ourCell">Our response move position being evaluated (0-360)</param>
    /// <returns>History score, bounded to [-MaxScore, MaxScore]. Returns 0 for invalid indices.</returns>
    [System.Runtime.CompilerServices.MethodImpl(System.Runtime.CompilerServices.MethodImplOptions.AggressiveInlining)]
    public int GetScore(Player player, int opponentCell, int ourCell)
    {
        // Validate indices
        if (player == Player.None || opponentCell < 0 || opponentCell >= BoardSize || ourCell < 0 || ourCell >= BoardSize)
            return 0;

        return _history[(int)player, opponentCell, ourCell];
    }

    /// <summary>
    /// Update counter-move history after a cutoff or search result.
    /// Uses bounded update formula to prevent unbounded growth.
    /// </summary>
    /// <param name="player">The player to update history for</param>
    /// <param name="opponentCell">The opponent's previous move position (0-360)</param>
    /// <param name="ourCell">Our response move position that caused cutoff (0-360)</param>
    /// <param name="bonus">The bonus/penalty to apply (positive for good moves, negative for bad)</param>
    public void Update(Player player, int opponentCell, int ourCell, int bonus)
    {
        // Validate indices - skip None player
        if (player == Player.None || opponentCell < 0 || opponentCell >= BoardSize || ourCell < 0 || ourCell >= BoardSize)
            return;

        // Clamp bonus to reasonable range
        int clampedBonus = Math.Clamp(bonus, -MaxScore, MaxScore);

        int playerIndex = (int)player;
        int current = _history[playerIndex, opponentCell, ourCell];

        // Bounded update formula:
        // newValue = current + bonus - |current * bonus| / MaxScore
        // This ensures values stay within [-MaxScore, MaxScore]
        int newValue = current + clampedBonus - Math.Abs(current * clampedBonus) / MaxScore;
        newValue = Math.Clamp(newValue, -MaxScore, MaxScore);

        _history[playerIndex, opponentCell, ourCell] = (short)newValue;
    }

    /// <summary>
    /// Clear all counter-move history scores to zero.
    /// Should be called at the start of a new game.
    /// </summary>
    public void Clear()
    {
        Array.Clear(_history, 0, _history.Length);
    }

    /// <summary>
    /// Get the board size for validation purposes.
    /// </summary>
    public static int BoardCellCount => BoardSize;
}
