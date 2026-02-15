using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Opening book for precomputed opening moves.
/// Provides access to opening moves for Easy, Medium, Hard, Grandmaster, and Experimental difficulties.
/// Uses clean architecture with dependency injection for storage, canonicalization, and validation.
/// </summary>
public sealed class OpeningBook
{
    private const int BoardSize = GameConstants.BoardSize;
    private const int Center = GameConstants.CenterPosition;  // (16,16) is center of 32x32 board (0-indexed)

    private readonly OpeningBookLookupService _lookupService;
    private readonly IOpeningBookStore _store;
    private readonly IPositionCanonicalizer _canonicalizer;

    /// <summary>
    /// Constructor with dependency injection for testing.
    /// </summary>
    public OpeningBook(IOpeningBookStore store, IPositionCanonicalizer canonicalizer, OpeningBookLookupService lookupService)
    {
        _store = store ?? throw new ArgumentNullException(nameof(store));
        _canonicalizer = canonicalizer ?? throw new ArgumentNullException(nameof(canonicalizer));
        _lookupService = lookupService ?? throw new ArgumentNullException(nameof(lookupService));
    }

    /// <summary>
    /// Get a good opening move from the book.
    /// Returns moves for Easy, Medium, Hard, Grandmaster, and Experimental difficulties.
    /// Book usage is depth-filtered by difficulty in SelectBestMove() (from AIDifficultyConfig):
    /// - Easy: 4 plies, Medium: 6 plies, Hard: 10 plies
    /// - Grandmaster: 14 plies, Experimental: unlimited
    /// First move is not hardcoded - the opening book or AI decides naturally.
    /// </summary>
    public (int x, int y)? GetBookMove(Board board, Player player, AIDifficulty difficulty, (int x, int y)? lastOpponentMove)
    {
        // Check if difficulty supports opening book
        if (!DifficultyUsesBook(difficulty))
            return null;

        // Check if still in opening phase (difficulty-dependent)
        if (!_lookupService.IsInOpeningPhase(board, difficulty))
            return null;

        // Query the book for a move
        return _lookupService.GetBookMove(board, player, difficulty);
    }

    /// <summary>
    /// Check if we're still in the opening phase.
    /// Opening phase is defined as having fewer than 24 stones on the board
    /// (up to 12 moves per side, or 24 plies). This is a loose upper bound;
    /// actual book usage ends earlier based on depth filtering in SelectBestMove().
    /// </summary>
    public bool IsInOpeningPhase(Board board, AIDifficulty difficulty)
    {
        return _lookupService.IsInOpeningPhase(board, difficulty);
    }

    /// <summary>
    /// Check if we're still in the opening phase (uses Hard limit by default).
    /// </summary>
    public bool IsInOpeningPhase(Board board)
    {
        return _lookupService.IsInOpeningPhase(board);
    }

    /// <summary>
    /// Get the number of remaining book moves.
    /// </summary>
    public int GetRemainingBookMoves(Board board, Player player)
    {
        return _lookupService.GetRemainingBookMoves(board, player);
    }

    /// <summary>
    /// Check if a specific difficulty level uses the opening book.
    /// Easy+ all use the book with different depth limits.
    /// </summary>
    private static bool DifficultyUsesBook(AIDifficulty difficulty)
    {
        return difficulty switch
        {
            AIDifficulty.Easy => true,
            AIDifficulty.Medium => true,
            AIDifficulty.Hard => true,
            AIDifficulty.Grandmaster => true,
            AIDifficulty.Experimental => true,
            _ => false
        };
    }

    /// <summary>
    /// Get the underlying store for testing purposes.
    /// </summary>
    internal IOpeningBookStore GetStore() => _store;

    /// <summary>
    /// Get the underlying canonicalizer for testing purposes.
    /// </summary>
    internal IPositionCanonicalizer GetCanonicalizer() => _canonicalizer;

    /// <summary>
    /// Get book statistics.
    /// </summary>
    public BookStatistics GetStatistics() => _store.GetStatistics();
}
