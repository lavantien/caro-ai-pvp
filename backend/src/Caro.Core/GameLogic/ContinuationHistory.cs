using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Continuation history for move ordering across multiple plies.
/// Tracks move pair statistics: "after playing move A, how good is move B?"
/// Based on Stockfish 18's continuation history implementation.
/// 
/// Uses bounded update formula to prevent overflow:
/// newValue = current + bonus - abs(current * bonus) / MaxScore
/// 
/// Expected ELO gain: +15-25 by improving move ordering.
/// </summary>
public sealed class ContinuationHistory
{
    /// <summary>
    /// Maximum score to prevent overflow in bounded updates.
    /// Matches typical engine continuation history limits.
    /// </summary>
    public const int MaxScore = 30000;

    /// <summary>
    /// Number of plies of continuation history to track.
    /// Stockfish uses 6 plies (plies -1 through -6).
    /// </summary>
    private const int PlyCount = 6;

    /// <summary>
    /// Size of the 19x19 board in cells.
    /// </summary>
    private const int BoardSize = 32 * 32; // 1024 cells

    /// <summary>
    /// Multi-dimensional array tracking continuation history.
    /// Dimensions: [player, prevCell, currentCell]
    /// - player: 3 entries (None, Red, Blue) - we use Player enum directly
    /// - prevCell: The previous move position (0-360)
    /// - currentCell: The current move position being evaluated (0-360)
    /// </summary>
    private readonly short[,,] _history;

    /// <summary>
    /// Create a new continuation history table initialized to zero.
    /// </summary>
    public ContinuationHistory()
    {
        _history = new short[3, BoardSize, BoardSize]; // None, Red, Blue
    }

    /// <summary>
    /// Get the continuation history score for a specific move sequence.
    /// Higher scores indicate this move has been good after the previous move.
    /// </summary>
    /// <param name="player">The player to get history for (Red or Blue)</param>
    /// <param name="prevCell">The previous move position (0-360, or -1 if none)</param>
    /// <param name="currentCell">The current move position being evaluated (0-360)</param>
    /// <returns>History score, bounded to [-MaxScore, MaxScore]. Returns 0 for invalid indices.</returns>
    [System.Runtime.CompilerServices.MethodImpl(System.Runtime.CompilerServices.MethodImplOptions.AggressiveInlining)]
    public int GetScore(Player player, int prevCell, int currentCell)
    {
        // Validate indices
        if (player == Player.None || prevCell < 0 || prevCell >= BoardSize || currentCell < 0 || currentCell >= BoardSize)
            return 0;

        return _history[(int)player, prevCell, currentCell];
    }

    /// <summary>
    /// Update continuation history after a cutoff or search result.
    /// Uses bounded update formula to prevent unbounded growth.
    /// </summary>
    /// <param name="player">The player to update history for</param>
    /// <param name="prevCell">The previous move position (0-360)</param>
    /// <param name="currentCell">The current move position (0-360)</param>
    /// <param name="bonus">The bonus/penalty to apply (positive for good moves, negative for bad)</param>
    public void Update(Player player, int prevCell, int currentCell, int bonus)
    {
        // Validate indices - skip None player
        if (player == Player.None || prevCell < 0 || prevCell >= BoardSize || currentCell < 0 || currentCell >= BoardSize)
            return;

        // Clamp bonus to reasonable range
        int clampedBonus = Math.Clamp(bonus, -MaxScore, MaxScore);

        int playerIndex = (int)player;
        int current = _history[playerIndex, prevCell, currentCell];

        // Bounded update formula:
        // newValue = current + bonus - |current * bonus| / MaxScore
        // This ensures values stay within [-MaxScore, MaxScore]
        int newValue = current + clampedBonus - Math.Abs(current * clampedBonus) / MaxScore;
        newValue = Math.Clamp(newValue, -MaxScore, MaxScore);

        _history[playerIndex, prevCell, currentCell] = (short)newValue;
    }

    /// <summary>
    /// Update continuation history for multiple plies.
    /// Called when a move causes a cutoff, updating history for the sequence leading to it.
    /// </summary>
    /// <param name="player">The player to update history for</param>
    /// <param name="moveHistory">Array of previous move positions (most recent first)</param>
    /// <param name="currentMove">The current move position that caused cutoff</param>
    /// <param name="bonus">The bonus to apply</param>
    public void UpdateMultiple(Player player, int[] moveHistory, int currentMove, int bonus)
    {
        int clampedBonus = Math.Clamp(bonus, -MaxScore, MaxScore);
        int playerIndex = (int)player;

        // Update up to PlyCount previous positions
        for (int i = 0; i < PlyCount && i < moveHistory.Length; i++)
        {
            int prevCell = moveHistory[i];
            if (prevCell < 0 || prevCell >= BoardSize || currentMove < 0 || currentMove >= BoardSize)
                continue;

            int current = _history[playerIndex, prevCell, currentMove];
            int newValue = current + clampedBonus - Math.Abs(current * clampedBonus) / MaxScore;
            newValue = Math.Clamp(newValue, -MaxScore, MaxScore);
            _history[playerIndex, prevCell, currentMove] = (short)newValue;
        }
    }

    /// <summary>
    /// Clear all continuation history scores to zero.
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

    /// <summary>
    /// Get the number of plies tracked.
    /// </summary>
    public static int TrackedPlyCount => PlyCount;
}
