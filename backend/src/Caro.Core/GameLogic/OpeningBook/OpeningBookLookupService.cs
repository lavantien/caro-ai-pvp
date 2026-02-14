using Caro.Core.Domain.Entities;

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
    private readonly Random? _random;

    // Upper bound for opening phase: 12 moves per side = 24 stones = 24 plies
    // Actual book exit is earlier due to depth filtering in SelectBestMove()
    // - Hard stops at depth 24 (12 of its own moves)
    // - Grandmaster stops at depth 32 (16 of its own moves)
    private const int MaxBookMoves = 12;

    public OpeningBookLookupService(
        IOpeningBookStore store,
        IPositionCanonicalizer canonicalizer,
        IOpeningBookValidator validator,
        Random? random = null)
    {
        _store = store ?? throw new ArgumentNullException(nameof(store));
        _canonicalizer = canonicalizer ?? throw new ArgumentNullException(nameof(canonicalizer));
        _validator = validator ?? throw new ArgumentNullException(nameof(validator));
        _random = random;  // null means use Random.Shared (default behavior)
    }

    // Helper method for random operations
    private int NextRandomInt(int maxValue) => _random?.Next(maxValue) ?? Random.Shared.Next(maxValue);

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

        // Transform back to actual coordinates if needed
        // CRITICAL: Use stored IsNearEdge to decide transformation
        // Moves stored with IsNearEdge=true were NOT transformed during storage
        // so they should NOT be inverse-transformed during retrieval
        (int x, int y) actualMove;
        if (entry.IsNearEdge || canonical.SymmetryApplied == SymmetryType.Identity)
        {
            actualMove = (bestMove.RelativeX, bestMove.RelativeY);
        }
        else
        {
            actualMove = _canonicalizer.TransformToActual(
                (bestMove.RelativeX, bestMove.RelativeY),
                canonical.SymmetryApplied,
                board
            );
        }

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
            // CRITICAL: Use stored IsNearEdge to decide transformation
            // Moves stored with IsNearEdge=true were NOT transformed during storage
            // so they should NOT be inverse-transformed during retrieval
            (int x, int y) actualMove;
            if (entry.IsNearEdge || canonical.SymmetryApplied == SymmetryType.Identity)
            {
                actualMove = (move.RelativeX, move.RelativeY);
            }
            else
            {
                actualMove = _canonicalizer.TransformToActual(
                    (move.RelativeX, move.RelativeY),
                    canonical.SymmetryApplied,
                    board
                );
            }

            if (_validator.IsValidMove(board, actualMove.x, actualMove.y, player))
            {
                validMoves.Add(move);
            }
        }

        return validMoves.ToArray();
    }

    /// <summary>
    /// Check if we're still in the opening phase (fewer than max stones on board).
    /// The limit depends on difficulty: Hard uses 24 stones, Grandmaster uses 32 stones.
    /// </summary>
    public bool IsInOpeningPhase(Board board, AIDifficulty difficulty)
    {
        var redBitBoard = board.GetBitBoard(Player.Red);
        var blueBitBoard = board.GetBitBoard(Player.Blue);
        int stoneCount = redBitBoard.CountBits() + blueBitBoard.CountBits();

        int maxStones = GetMaxBookDepth(difficulty);
        return stoneCount < maxStones;
    }

    /// <summary>
    /// Check if we're still in the opening phase (fewer than 24 stones on board).
    /// This overload uses the default Hard limit for backward compatibility.
    /// </summary>
    public bool IsInOpeningPhase(Board board)
    {
        return IsInOpeningPhase(board, AIDifficulty.Hard);
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
    /// Get maximum book depth allowed for a difficulty level (in plies/half-moves).
    /// This filters which book entries can be used based on their DepthAchieved.
    /// Values are sourced from AIDifficultyConfig for consistency.
    /// Easy: 4 plies (2 moves per side)
    /// Medium: 6 plies (3 moves per side)
    /// Hard: 10 plies (5 moves per side)
    /// Grandmaster: 14 plies (7 moves per side)
    /// Experimental: unlimited (uses all available)
    /// </summary>
    private static int GetMaxBookDepth(AIDifficulty difficulty)
    {
        var settings = AIDifficultyConfig.Instance.GetSettings(difficulty);
        return settings.MaxBookDepth;
    }

    /// <summary>
    /// Select the best move from available book moves.
    /// Selection strategy varies by difficulty.
    /// </summary>
    private BookMove? SelectBestMove(BookMove[] moves, AIDifficulty difficulty)
    {
        if (moves.Length == 0)
            return null;

        // Filter moves by max depth for this difficulty
        int maxDepth = GetMaxBookDepth(difficulty);
        var depthFilteredMoves = moves.Where(m => m.DepthAchieved <= maxDepth).ToArray();

        // If no moves pass depth filter, fall back to all moves
        var candidateMoves = depthFilteredMoves.Length > 0 ? depthFilteredMoves : moves;

        // For Experimental: prioritize verified, forcing moves with highest priority
        if (difficulty == AIDifficulty.Experimental)
        {
            // First try: verified forcing moves
            var best = candidateMoves
                .Where(m => m.IsVerified && m.IsForcing)
                .OrderByDescending(m => m.Priority)
                .ThenByDescending(m => m.Score)
                .FirstOrDefault();

            if (best != null)
                return best;

            // Second try: any verified move
            best = candidateMoves
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
            return candidateMoves
                .Where(m => m.IsVerified)
                .OrderByDescending(m => m.Score)
                .ThenByDescending(m => m.Priority)
                .FirstOrDefault();
        }

        // For Hard: randomly pick from moves with equal highest score
        var verified = candidateMoves.Where(m => m.IsVerified).ToArray();
        if (verified.Length > 0)
        {
            // Find max score, then randomly pick from all moves with that score
            int maxScore = verified.Max(m => m.Score);
            var topMoves = verified.Where(m => m.Score == maxScore).ToArray();
            return topMoves[NextRandomInt(topMoves.Length)];
        }

        // Fallback: highest scoring moves (with random tiebreak)
        int maxFallbackScore = candidateMoves.Max(m => m.Score);
        var topFallbackMoves = candidateMoves.Where(m => m.Score == maxFallbackScore).ToArray();
        return topFallbackMoves[NextRandomInt(topFallbackMoves.Length)];
    }
}
