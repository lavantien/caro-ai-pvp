using Caro.Core.Domain.Entities;

namespace Caro.Core.Domain.Interfaces;

/// <summary>
/// Interface for board position evaluation.
/// Evaluates a board position from the perspective of a given player.
/// </summary>
public interface IBoardEvaluator
{
    /// <summary>
    /// Evaluate the board position for the given player.
    /// Returns a score where positive values favor the player and negative values favor the opponent.
    /// </summary>
    /// <param name="board">The board to evaluate</param>
    /// <param name="player">The player to evaluate for</param>
    /// <returns>A score (higher is better for the player)</returns>
    int Evaluate(IBoard board, Player player);

    /// <summary>
    /// Evaluate the board position and check if it's terminal.
    /// </summary>
    /// <param name="board">The board to evaluate</param>
    /// <param name="player">The player to evaluate for</param>
    /// <returns>A tuple containing the score and whether the position is terminal</returns>
    (int score, bool isTerminal) EvaluateWithTerminalCheck(IBoard board, Player player);
}
