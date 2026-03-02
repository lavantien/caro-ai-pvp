using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Interface for opening book move validation.
/// Validates moves against game rules and detects blunders.
/// </summary>
public interface IOpeningBookValidator
{
    /// <summary>
    /// Check if a move is valid according to game rules.
    /// Validates bounds, empty cell, and Open Rule if applicable.
    /// </summary>
    bool IsValidMove(Board board, int x, int y, Player player);

    /// <summary>
    /// Validate a move for blunders using deep search.
    /// A blunder is defined as a move that significantly worsens the position.
    /// </summary>
    /// <returns>True if move is safe, false if it's a blunder</returns>
    (bool isValid, string reason) ValidateBlunder(Board board, int x, int y, Player player);

    /// <summary>
    /// Check if placing a stone at the given position creates an immediate win.
    /// </summary>
    bool IsWinningMove(Board board, int x, int y, Player player);

    /// <summary>
    /// Get the reason why a move is invalid.
    /// Returns null if the move is valid.
    /// </summary>
    string? GetInvalidReason(Board board, int x, int y, Player player);
}

/// <summary>
/// Result of a move validation.
/// </summary>
public sealed record ValidationResult(
    bool IsValid,
    string? Reason,
    bool IsBlunder,
    bool IsWinning
);
