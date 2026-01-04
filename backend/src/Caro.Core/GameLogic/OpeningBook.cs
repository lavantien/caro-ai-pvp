using Caro.Core.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Opening book with pre-computed good moves for early game
/// Saves computation time for first 5-10 moves
/// </summary>
public sealed class OpeningBook
{
    /// <summary>
    /// Pre-computed good opening moves for Red (first player)
    /// Center-focused moves with good territorial control
    /// </summary>
    private static readonly (int x, int y)[] RedOpenings = new (int x, int y)[]
    {
        // Absolute center (best opening move)
        (7, 7),

        // Near-center diagonals (second best)
        (6, 6), (6, 8), (8, 6), (8, 8),

        // Near-center orthogonals
        (6, 7), (7, 6), (7, 8), (8, 7),

        // Slightly further out but still strong
        (5, 5), (5, 7), (5, 9),
        (7, 5), (7, 9),
        (9, 5), (9, 7), (9, 9),

        // Extension moves
        (4, 4), (4, 7), (4, 10),
        (7, 4), (7, 10),
        (10, 4), (10, 7), (10, 10),
    };

    /// <summary>
    /// Pre-computed responses for Blue (second player)
    /// Balanced responses to Red's moves
    /// </summary>
    private static readonly (int x, int y, int respondToX, int respondToY)[] BlueResponses = new (int, int, int, int)[]
    {
        // If Red played center, respond near center
        (6, 6, 7, 7), (6, 8, 7, 7), (8, 6, 7, 7), (8, 8, 7, 7),

        // Diagonal responses
        (5, 5, 6, 6), (5, 9, 6, 8), (9, 5, 8, 6), (9, 9, 8, 8),

        // Orthogonal responses
        (6, 7, 7, 7), (7, 6, 7, 7), (7, 8, 7, 7), (8, 7, 7, 7),

        // Extension responses
        (5, 7, 6, 7), (7, 5, 7, 6), (7, 9, 7, 8), (9, 7, 8, 7),
    };

    private readonly Random _random = new();

    /// <summary>
    /// Get a good opening move from the book
    /// Returns null if no book move is available (board not in opening phase)
    /// </summary>
    /// <param name="board">Current board state</param>
    /// <param name="player">Player to move</param>
    /// <param name="lastOpponentMove">Last move by opponent (for Blue responses)</param>
    /// <returns>Move from book, or null if book exhausted</returns>
    public (int x, int y)? GetBookMove(Board board, Player player, (int x, int y)? lastOpponentMove)
    {
        if (player == Player.None)
            return null;

        // Count stones on board to determine game phase
        var redBitBoard = board.GetBitBoard(Player.Red);
        var blueBitBoard = board.GetBitBoard(Player.Blue);
        int stoneCount = redBitBoard.CountBits() + blueBitBoard.CountBits();

        // Book only for first 4 moves per player (8 stones total)
        // More conservative to avoid interfering with tactical positions
        if (stoneCount >= 8)
            return null;

        if (player == Player.Red)
        {
            return GetRedOpeningMove(board, stoneCount);
        }
        else
        {
            return GetBlueResponseMove(board, stoneCount, lastOpponentMove);
        }
    }

    /// <summary>
    /// Get opening move for Red (first player)
    /// </summary>
    private (int x, int y)? GetRedOpeningMove(Board board, int stoneCount)
    {
        // Find the first available move from our preferred openings
        foreach (var (x, y) in RedOpenings)
        {
            if (board.GetCell(x, y).IsEmpty)
            {
                // Add slight randomness for variety in early moves
                if (stoneCount < 4 && _random.Next(100) < 30)
                {
                    // Occasionally pick a different good move
                    continue;
                }
                return (x, y);
            }
        }

        return null; // Book exhausted
    }

    /// <summary>
    /// Get response move for Blue (second player)
    /// </summary>
    private (int x, int y)? GetBlueResponseMove(Board board, int stoneCount, (int x, int y)? lastOpponentMove)
    {
        if (lastOpponentMove.HasValue)
        {
            var (lastX, lastY) = lastOpponentMove.Value;

            // Try to find a direct response to the last move
            foreach (var (x, y, rx, ry) in BlueResponses)
            {
                if (rx == lastX && ry == lastY && board.GetCell(x, y).IsEmpty)
                {
                    return (x, y);
                }
            }
        }

        // Fall back to general good positions
        foreach (var (x, y, _, _) in BlueResponses)
        {
            if (board.GetCell(x, y).IsEmpty)
            {
                return (x, y);
            }
        }

        return null; // Book exhausted
    }

    /// <summary>
    /// Check if we're still in the opening phase
    /// </summary>
    public bool IsInOpeningPhase(Board board)
    {
        var redBitBoard = board.GetBitBoard(Player.Red);
        var blueBitBoard = board.GetBitBoard(Player.Blue);
        int stoneCount = redBitBoard.CountBits() + blueBitBoard.CountBits();
        return stoneCount < 8;
    }

    /// <summary>
    /// Get the number of remaining book moves
    /// </summary>
    public int GetRemainingBookMoves(Board board, Player player)
    {
        int count = 0;
        for (int x = 0; x < 15; x++)
        {
            for (int y = 0; y < 15; y++)
            {
                if (board.GetCell(x, y).IsEmpty)
                {
                    if (player == Player.Red)
                    {
                        if (RedOpenings.Any(m => m.x == x && m.y == y))
                            count++;
                    }
                    else
                    {
                        if (BlueResponses.Any(m => m.Item1 == x && m.Item2 == y))
                            count++;
                    }
                }
            }
        }
        return count;
    }
}
