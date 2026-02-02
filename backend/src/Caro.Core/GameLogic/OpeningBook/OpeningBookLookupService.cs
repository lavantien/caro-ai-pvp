using Caro.Core.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Main opening book lookup service combining storage, canonicalization, and validation.
/// Orchestrates the process of finding book moves for a given position.
/// </summary>
public sealed class OpeningBookLookupService
{
    private readonly IOpeningBookStore _store;
    private readonly IPositionCanonicalizer _canonicalizer;
    private readonly IOpeningBookValidator _validator;

    // Maximum number of moves to be considered in opening phase (12 per side = 24 stones)
    private const int MaxBookMoves = 12;

    public OpeningBookLookupService(
        IOpeningBookStore store,
        IPositionCanonicalizer canonicalizer,
        IOpeningBookValidator validator)
    {
        _store = store ?? throw new ArgumentNullException(nameof(store));
        _canonicalizer = canonicalizer ?? throw new ArgumentNullException(nameof(canonicalizer));
        _validator = validator ?? throw new ArgumentNullException(nameof(validator));
    }

    /// <summary>
    /// Get best move from opening book for current position.
    /// Returns null if position not in book, difficulty doesn't support book, or move fails validation.
    /// </summary>
    public (int x, int y)? GetBookMove(
        Board board,
        Player player,
        AIDifficulty difficulty)
    {
        // Check if difficulty supports opening book
        if (!DifficultyUsesBook(difficulty))
            return null;

        // Canonicalize position
        var canonical = _canonicalizer.Canonicalize(board);

        // Look up in book
        var entry = _store.GetEntry(canonical.CanonicalHash, canonical.Player);
        if (entry == null || entry.Moves.Length == 0)
            return null;

        // Select best move from available options
        var bestMove = SelectBestMove(entry.Moves, difficulty);
        if (bestMove == null)
            return null;

        // Transform back to actual coordinates if symmetry was applied
        var actualMove = _canonicalizer.TransformToActual(
            (bestMove.RelativeX, bestMove.RelativeY),
            canonical.SymmetryApplied,
            board
        );

        // Verify move is valid according to game rules
        if (!_validator.IsValidMove(board, actualMove.x, actualMove.y, player))
            return null;

        return actualMove;
    }

    /// <summary>
    /// Get all book moves for the current position.
    /// Useful for analysis or when variety is desired.
    /// </summary>
    public BookMove[] GetAllBookMoves(Board board, Player player, AIDifficulty difficulty)
    {
        if (!DifficultyUsesBook(difficulty))
            return Array.Empty<BookMove>();

        var canonical = _canonicalizer.Canonicalize(board);
        var entry = _store.GetEntry(canonical.CanonicalHash, canonical.Player);

        if (entry == null)
            return Array.Empty<BookMove>();

        // Filter valid moves and transform to actual coordinates
        var validMoves = new List<BookMove>();

        foreach (var move in entry.Moves)
        {
            var actualMove = _canonicalizer.TransformToActual(
                (move.RelativeX, move.RelativeY),
                canonical.SymmetryApplied,
                board
            );

            if (_validator.IsValidMove(board, actualMove.x, actualMove.y, player))
            {
                validMoves.Add(move);
            }
        }

        return validMoves.ToArray();
    }

    /// <summary>
    /// Check if we're still in the opening phase.
    /// Opening phase is defined as having fewer than MaxBookMoves per side played.
    /// </summary>
    public bool IsInOpeningPhase(Board board)
    {
        var redBitBoard = board.GetBitBoard(Player.Red);
        var blueBitBoard = board.GetBitBoard(Player.Blue);
        int stoneCount = redBitBoard.CountBits() + blueBitBoard.CountBits();

        return stoneCount < MaxBookMoves * 2;
    }

    /// <summary>
    /// Get the number of remaining book moves for the current position.
    /// </summary>
    public int GetRemainingBookMoves(Board board, Player player)
    {
        var canonical = _canonicalizer.Canonicalize(board);
        var entry = _store.GetEntry(canonical.CanonicalHash, canonical.Player);

        if (entry == null)
            return 0;

        int currentMoveNumber = (board.GetBitBoard(Player.Red).CountBits() +
                                board.GetBitBoard(Player.Blue).CountBits() + 1) / 2;

        return Math.Max(0, MaxBookMoves - currentMoveNumber);
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
    /// Select the best move from available book moves.
    /// Selection strategy varies by difficulty.
    /// </summary>
    private static BookMove? SelectBestMove(BookMove[] moves, AIDifficulty difficulty)
    {
        if (moves.Length == 0)
            return null;

        // For Experimental: prioritize verified, forcing moves with highest priority
        if (difficulty == AIDifficulty.Experimental)
        {
            // First try: verified forcing moves
            var best = moves
                .Where(m => m.IsVerified && m.IsForcing)
                .OrderByDescending(m => m.Priority)
                .ThenByDescending(m => m.Score)
                .FirstOrDefault();

            if (best != null)
                return best;

            // Second try: any verified move
            best = moves
                .Where(m => m.IsVerified)
                .OrderByDescending(m => m.Priority)
                .ThenByDescending(m => m.Score)
                .FirstOrDefault();

            if (best != null)
                return best;
        }

        // For Grandmaster: verified moves with highest score
        if (difficulty == AIDifficulty.Grandmaster)
        {
            return moves
                .Where(m => m.IsVerified)
                .OrderByDescending(m => m.Score)
                .ThenByDescending(m => m.Priority)
                .FirstOrDefault();
        }

        // For Hard: any verified move, or highest score if none verified
        var verified = moves.Where(m => m.IsVerified).ToArray();
        if (verified.Length > 0)
        {
            return verified.OrderByDescending(m => m.Score).FirstOrDefault();
        }

        // Fallback: highest scoring move
        return moves.OrderByDescending(m => m.Score).FirstOrDefault();
    }
}
