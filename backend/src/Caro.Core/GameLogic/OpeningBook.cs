using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Opening book for precomputed opening moves.
/// Provides access to opening moves for Hard, Grandmaster, and Experimental difficulties.
/// Uses clean architecture with dependency injection for storage, canonicalization, and validation.
/// </summary>
public sealed class OpeningBook
{
    private const int BoardSize = 19;
    private const int Center = 9;  // (9,9) is center of 19x19 board (0-indexed)

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
    /// Only returns moves for Hard, Grandmaster, and Experimental difficulties.
    /// First move (empty board) always returns center (9,9) for all difficulties.
    /// Book usage is depth-filtered by difficulty in SelectBestMove():
    /// - Hard: moves evaluated at depth ≤ 24 plies (12 moves per side)
    /// - Grandmaster/Experimental: moves evaluated at depth ≤ 32 plies (16 moves per side)
    /// </summary>
    public (int x, int y)? GetBookMove(Board board, Player player, AIDifficulty difficulty, (int x, int y)? lastOpponentMove)
    {
        // Special case: empty board - always return center move for ALL difficulties
        // This maintains backward compatibility with the original implementation
        var redBitBoard = board.GetBitBoard(Player.Red);
        var blueBitBoard = board.GetBitBoard(Player.Blue);
        int stoneCount = redBitBoard.CountBits() + blueBitBoard.CountBits();

        if (stoneCount == 0)
        {
            // Empty board - first move should be center
            return (Center, Center);
        }

        // Check if difficulty supports opening book for subsequent moves
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
    /// </summary>
    private static bool DifficultyUsesBook(AIDifficulty difficulty)
    {
        return difficulty switch
        {
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
