using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

namespace Caro.Core.Tests.Helpers;

/// <summary>
/// Basic implementation of IOpeningBookValidator for testing.
/// Provides simple rule validation without complex blunder detection.
/// </summary>
public sealed class MockOpeningBookValidator : IOpeningBookValidator
{
    private const int BoardSize = 19;

    /// <summary>
    /// Check if a move is valid (bounds check and empty cell check).
    /// </summary>
    public bool IsValidMove(Board board, int x, int y, Player player)
    {
        // Check bounds
        if (x < 0 || x >= BoardSize || y < 0 || y >= BoardSize)
            return false;

        // Check if cell is empty
        return board.IsEmpty(x, y);
    }

    /// <summary>
    /// Validate for blunders (always returns valid for simplicity).
    /// Tests can override this behavior if needed.
    /// </summary>
    public (bool isValid, string reason) ValidateBlunder(Board board, int x, int y, Player player)
    {
        // For testing, always consider moves non-blunders
        // Override in derived classes or use a real validator for blunder testing
        return (true, string.Empty);
    }

    /// <summary>
    /// Check if placing a stone creates an immediate 5-in-a-row win.
    /// </summary>
    public bool IsWinningMove(Board board, int x, int y, Player player)
    {
        // Check if move is valid first
        if (!IsValidMove(board, x, y, player))
            return false;

        // Create a new board with the move and check for win
        var testBoard = board.PlaceStone(x, y, player);
        var winDetector = new WinDetector();
        var result = winDetector.CheckWin(testBoard);

        return result.HasWinner && result.Winner == player;
    }

    /// <summary>
    /// Get the reason why a move is invalid.
    /// </summary>
    public string? GetInvalidReason(Board board, int x, int y, Player player)
    {
        // Check bounds
        if (x < 0 || x >= BoardSize || y < 0 || y >= BoardSize)
            return $"Position ({x}, {y}) is out of bounds";

        // Check if cell is occupied
        if (!board.IsEmpty(x, y))
            return $"Cell ({x}, {y}) is already occupied by {board.GetCell(x, y).Player}";

        return null; // Move is valid
    }
}
