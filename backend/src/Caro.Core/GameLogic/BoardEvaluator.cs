using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Evaluates board positions for AI decision-making.
/// </summary>
public class BoardEvaluator
{
    // Scoring weights from centralized EvaluationConstants
    private const int FourInRowScore = EvaluationConstants.FourInRowScore;
    private const int ThreeInRowScore = EvaluationConstants.ThreeInRowScore;
    private const int TwoInRowScore = EvaluationConstants.TwoInRowScore;
    private const int OneInRowScore = EvaluationConstants.OneInRowScore;
    private const int CenterBonus = EvaluationConstants.CenterBonus;

    // Direction vectors: horizontal, vertical, 2 diagonals
    private static readonly (int dx, int dy)[] Directions = new[]
    {
        (1, 0),   // Horizontal
        (0, 1),   // Vertical
        (1, 1),   // Diagonal down-right
        (1, -1)   // Diagonal down-left
    };

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
