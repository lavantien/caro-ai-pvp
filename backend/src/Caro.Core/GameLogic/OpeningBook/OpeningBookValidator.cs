using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Validates opening book moves against game rules and detects blunders.
/// Enforces Open Rule and validates move legality.
/// </summary>
public sealed class OpeningBookValidator : IOpeningBookValidator
{
    private readonly OpenRuleValidator _openRuleValidator;
    private readonly WinDetector _winDetector;

    public OpeningBookValidator()
    {
        _openRuleValidator = new OpenRuleValidator();
        _winDetector = new WinDetector();
    }

    /// <inheritdoc/>
    public bool IsValidMove(Board board, int x, int y, Player player)
    {
        // Check bounds
        if (x < 0 || x >= board.BoardSize || y < 0 || y >= board.BoardSize)
            return false;

        // Check cell is empty
        var cell = board.GetCell(x, y);
        if (!cell.IsEmpty)
            return false;

        // Check Open Rule for Red's second move
        if (player == Player.Red && !_openRuleValidator.IsValidSecondMove(board, x, y))
            return false;

        return true;
    }

    /// <inheritdoc/>
    public (bool isValid, string reason) ValidateBlunder(Board board, int x, int y, Player player)
    {
        // Basic blunder check - ensure the move doesn't immediately lose
        // Full implementation would use deep search to verify

        // First check if move is valid
        if (!IsValidMove(board, x, y, player))
        {
            return (false, "Move is not valid according to game rules");
        }

        // Check if move creates an immediate win (good, not a blunder)
        var testBoard = board.PlaceStone(x, y, player);
        var winResult = _winDetector.CheckWin(testBoard);
        if (winResult.HasWinner && winResult.Winner == player)
        {
            return (true, "Winning move");
        }

        // Basic tactical check - does this move create a pattern that can be immediately punished?
        // For now, we'll do a simplified check
        // Full implementation would run a deep search from the resulting position

        // Check if move allows opponent to win immediately on their next turn
        var opponent = player == Player.Red ? Player.Blue : Player.Red;

        // This is a placeholder for the full blunder detection
        // The actual implementation would:
        // 1. Make the move
        // 2. Run a search for the opponent's best response
        // 3. Evaluate if the score drops significantly (> 200cp = blunder)

        return (true, "No obvious blunder (full verification requires deep search)");
    }

    /// <inheritdoc/>
    public bool IsWinningMove(Board board, int x, int y, Player player)
    {
        if (!IsValidMove(board, x, y, player))
            return false;

        var testBoard = board.PlaceStone(x, y, player);
        var winResult = _winDetector.CheckWin(testBoard);

        return winResult.HasWinner && winResult.Winner == player;
    }

    /// <inheritdoc/>
    public string? GetInvalidReason(Board board, int x, int y, Player player)
    {
        // Check bounds
        if (x < 0 || x >= board.BoardSize || y < 0 || y >= board.BoardSize)
            return $"Position ({x}, {y}) is outside board bounds";

        // Check cell is empty
        var cell = board.GetCell(x, y);
        if (!cell.IsEmpty)
            return $"Position ({x}, {y}) is already occupied by {cell.Player}";

        // Check Open Rule for Red's second move
        if (player == Player.Red && !_openRuleValidator.IsValidSecondMove(board, x, y))
        {
            return "Red's second move must be at least 3 intersections away from the first red stone (Open Rule)";
        }

        return null; // Move is valid
    }
}
