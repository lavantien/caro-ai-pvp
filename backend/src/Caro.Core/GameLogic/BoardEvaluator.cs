using Caro.Core.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Evaluates board positions for AI decision-making
/// Automatically selects the fastest evaluator based on difficulty level
/// - Low difficulty: BitBoardEvaluator (sufficient)
/// - High difficulty (Professional+): SIMDBitBoardEvaluator (optimized)
/// </summary>
public class BoardEvaluator
{
    // Scoring weights
    private const int FourInRowScore = 10000;
    private const int ThreeInRowScore = 1000;
    private const int TwoInRowScore = 100;
    private const int OneInRowScore = 10;
    private const int CenterBonus = 50;

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
    /// Evaluate with SIMD optimization for high difficulty levels
    /// Automatically falls back to scalar evaluation if SIMD is not beneficial
    /// </summary>
    public int EvaluateOptimized(Board board, Player player, AIDifficulty difficulty)
    {
        if (player == Player.None)
            throw new ArgumentException("Player cannot be None");

        // Use SIMD evaluator for VeryHard and above (D7+)
        return (difficulty >= AIDifficulty.VeryHard)
            ? SIMDBitBoardEvaluator.Evaluate(board, player)
            : BitBoardEvaluator.Evaluate(board, player);
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
