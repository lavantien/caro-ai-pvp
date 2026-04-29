using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Evaluates board positions for AI decision-making.
/// </summary>
public class BoardEvaluator
{
    /// <summary>
    /// Evaluate the board for a given player using the default evaluator
    /// Positive score = good for player
    /// Negative score = good for opponent
    /// </summary>
    public int Evaluate(Board board, Player player)
    {
        if (player == Player.None)
            throw new ArgumentException("Player cannot be None");

        // Use BitBoardEvaluator for compatibility
        return BitBoardEvaluator.Evaluate(board, player);
    }

    /// <summary>
    /// Evaluate the SearchBoard for a given player using the default evaluator.
    /// High-performance path for search that avoids immutable Board overhead.
    /// </summary>
    public int Evaluate(SearchBoard board, Player player)
    {
        if (player == Player.None)
            throw new ArgumentException("Player cannot be None");

        return BitBoardEvaluator.Evaluate(board, player);
    }

    /// <summary>
    /// Evaluate with custom parameters (for SPSA tuning)
    /// </summary>
    public int Evaluate(Board board, Player player, TunableParameters parameters)
    {
        if (player == Player.None)
            throw new ArgumentException("Player cannot be None");

        return BitBoardEvaluator.EvaluateWithParameters(board, player, parameters);
    }

    /// <summary>
    /// Evaluate SearchBoard with custom parameters (for SPSA tuning)
    /// </summary>
    public int Evaluate(SearchBoard board, Player player, TunableParameters parameters)
    {
        if (player == Player.None)
            throw new ArgumentException("Player cannot be None");

        return BitBoardEvaluator.EvaluateWithParameters(board, player, parameters);
    }

    /// <summary>
    /// Evaluate with SIMD optimization for high difficulty levels
    /// Automatically falls back to scalar evaluation if SIMD is not beneficial
    /// </summary>
    public int EvaluateOptimized(Board board, Player player)
    {
        if (player == Player.None)
            throw new ArgumentException("Player cannot be None");

        return SIMDBitBoardEvaluator.Evaluate(board, player);
    }

    /// <summary>
    /// Fast evaluation of a potential move at position (x, y)
    /// Uses incremental scoring for move ordering
    /// </summary>
    public static int EvaluateMoveAt(int x, int y, Board board, Player player)
    {
        if (player == Player.None)
            throw new ArgumentException("Player cannot be None");

        return SIMDBitBoardEvaluator.EvaluateMoveAt(x, y, board, player);
    }
}
