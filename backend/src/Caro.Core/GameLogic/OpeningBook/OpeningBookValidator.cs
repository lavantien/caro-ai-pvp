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

        // Use AI search to check opponent's best response
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var moveNumber = testBoard.GetBitBoard(Player.Red).CountBits() + testBoard.GetBitBoard(Player.Blue).CountBits();

        // Use lightweight AI instance for blunder detection
        var ai = new MinimaxAI(ttSizeMb: 16);
        var (oppBestX, oppBestY) = ai.GetBestMove(
            testBoard,
            opponent,
            AIDifficulty.Hard,
            timeRemainingMs: 500,
            moveNumber: moveNumber,
            ponderingEnabled: false,
            parallelSearchEnabled: false
        );

        // Evaluate position after opponent's best response
        var evalBoard = testBoard.PlaceStone(oppBestX, oppBestY, opponent);
        // Score from player's perspective: positive = good for player
        int score = BitBoardEvaluator.Evaluate(evalBoard, player);

        // If score is very negative, opponent has a big advantage = this was a blunder
        if (score < -5000)
        {
            return (false, $"Blunder: opponent gains {Math.Abs(score)}cp advantage after ({oppBestX},{oppBestY})");
        }

        return (true, $"Move verified (opponent best response ({oppBestX},{oppBestY}) yields {score}cp)");
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
