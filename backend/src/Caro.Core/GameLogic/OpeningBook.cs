using Caro.Core.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Opening book - MINIMAL IMPLEMENTATION
/// Only handles first move at center of 19x19 board.
/// All other moves use actual search to expose bugs for fixing.
/// </summary>
public sealed class OpeningBook
{
    private const int BoardSize = 19;
    private const int Center = 9;  // (9,9) is center of 19x19 board (0-indexed)

    /// <summary>
    /// Get a good opening move from the book
    /// Only returns center (9,9) for first move of the game.
    /// All other moves return null to use actual search.
    /// </summary>
    public (int x, int y)? GetBookMove(Board board, Player player, AIDifficulty difficulty, (int x, int y)? lastOpponentMove)
    {
        // Only provide center move for the very first move of the game (empty board)
        var redBitBoard = board.GetBitBoard(Player.Red);
        var blueBitBoard = board.GetBitBoard(Player.Blue);
        int stoneCount = redBitBoard.CountBits() + blueBitBoard.CountBits();

        if (stoneCount == 0)
        {
            // Empty board - first move should be center
            return (Center, Center);
        }

        // All other moves: use actual search (don't hide bugs)
        return null;
    }

    /// <summary>
    /// Check if we're still in the opening phase
    /// </summary>
    public bool IsInOpeningPhase(Board board)
    {
        var redBitBoard = board.GetBitBoard(Player.Red);
        var blueBitBoard = board.GetBitBoard(Player.Blue);
        int stoneCount = redBitBoard.CountBits() + blueBitBoard.CountBits();
        return stoneCount == 0;  // Only first move
    }

    /// <summary>
    /// Get the number of remaining book moves
    /// </summary>
    public int GetRemainingBookMoves(Board board, Player player)
    {
        var redBitBoard = board.GetBitBoard(Player.Red);
        var blueBitBoard = board.GetBitBoard(Player.Blue);
        int stoneCount = redBitBoard.CountBits() + blueBitBoard.CountBits();
        return stoneCount == 0 ? 1 : 0;
    }
}
