using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic.UCI;

namespace Caro.Core.GameLogic.UCI;

/// <summary>
/// Parses UCI position commands and applies them to a board.
/// 
/// UCI position format:
/// - "position startpos" - Start from empty board
/// - "position startpos moves h9 j10 i9" - Start from empty, then apply moves
/// 
/// Caro uses 19x19 board with Red (equivalent to White) moving first.
/// UCI "White" maps to Red, "Black" maps to Blue.
/// </summary>
public static class UCIPositionConverter
{
    /// <summary>
    /// Parse a UCI position command and return the resulting board.
    /// </summary>
    /// <param name="positionCommand">Full UCI position command</param>
    /// <returns>Board with all moves applied</returns>
    public static Board ParsePosition(string positionCommand)
    {
        if (string.IsNullOrWhiteSpace(positionCommand))
            throw new ArgumentException("Position command cannot be empty");

        var parts = positionCommand.Split(' ', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries);

        if (parts.Length == 0 || parts[0] != "position")
            throw new ArgumentException($"Invalid position command: '{positionCommand}'");

        // Start with empty board
        var board = new Board();
        Player currentPlayer = Player.Red;  // Red always moves first

        // Find "moves" keyword index
        int movesIndex = -1;
        for (int i = 0; i < parts.Length; i++)
        {
            if (parts[i] == "moves")
            {
                movesIndex = i + 1;
                break;
            }
        }

        // Apply each move in sequence
        if (movesIndex > 0 && movesIndex < parts.Length)
        {
            for (int i = movesIndex; i < parts.Length; i++)
            {
                var moveStr = parts[i];
                var position = UCIMoveNotation.FromUCI(moveStr);

                // Validate the move is to an empty cell
                if (!board.IsEmpty(position.X, position.Y))
                    throw new InvalidOperationException($"Invalid position: cell {moveStr} is already occupied");

                board = board.PlaceStone(position.X, position.Y, currentPlayer);
                currentPlayer = currentPlayer == Player.Red ? Player.Blue : Player.Red;
            }
        }

        return board;
    }

    /// <summary>
    /// Apply a sequence of UCI moves to an existing board.
    /// </summary>
    /// <param name="board">Starting board</param>
    /// <param name="moves">Array of UCI move strings</param>
    /// <param name="startingPlayer">Player to move first (default: Red)</param>
    /// <returns>Board with moves applied and the next player to move</returns>
    public static (Board Board, Player NextPlayer) ApplyMoves(Board board, string[] moves, Player startingPlayer = Player.Red)
    {
        var currentBoard = board;
        var currentPlayer = startingPlayer;

        foreach (var move in moves)
        {
            var position = UCIMoveNotation.FromUCI(move);

            if (!currentBoard.IsEmpty(position.X, position.Y))
                throw new InvalidOperationException($"Cannot apply move {move}: cell already occupied");

            currentBoard = currentBoard.PlaceStone(position.X, position.Y, currentPlayer);
            currentPlayer = currentPlayer == Player.Red ? Player.Blue : Player.Red;
        }

        return (currentBoard, currentPlayer);
    }

    /// <summary>
    /// Convert a list of moves to UCI notation.
    /// </summary>
    public static string[] MovesToUCI(IEnumerable<(int x, int y, Player player)> moves)
    {
        return moves.Select(m => UCIMoveNotation.ToUCI(m.x, m.y)).ToArray();
    }

    /// <summary>
    /// Build a UCI position command from moves.
    /// </summary>
    public static string BuildPositionCommand(params string[] moves)
    {
        if (moves.Length == 0)
            return "position startpos";

        return "position startpos moves " + string.Join(" ", moves);
    }
}
