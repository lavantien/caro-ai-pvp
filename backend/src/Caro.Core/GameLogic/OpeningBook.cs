using Caro.Core.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Opening book facade for backward compatibility with existing MinimaxAI code.
/// Provides access to precomputed opening moves for Hard, Grandmaster, and Experimental difficulties.
/// Uses clean architecture with dependency injection for storage, canonicalization, and validation.
/// </summary>
public sealed class OpeningBook
{
    private const int BoardSize = 19;
    private const int Center = 9;  // (9,9) is center of 19x19 board (0-indexed)

    private readonly OpeningBookLookupService _lookupService;
    private readonly IOpeningBookStore _store;
    private readonly IPositionCanonicalizer _canonicalizer;

    // Lazy initialization of services
    private static readonly OpeningBook _instance = new();
    private static readonly object _lock = new();

    /// <summary>
    /// Get the singleton instance of the opening book.
    /// </summary>
    public static OpeningBook Instance => _instance;

    /// <summary>
    /// Private constructor for singleton pattern.
    /// Services are initialized lazily on first access.
    /// </summary>
    private OpeningBook()
    {
        // Initialize with default implementations
        _store = new InMemoryOpeningBookStore();
        _store.Initialize();

        _canonicalizer = new PositionCanonicalizer();
        var validator = new OpeningBookValidator();

        _lookupService = new OpeningBookLookupService(_store, _canonicalizer, validator);
    }

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
    /// First move (empty board) always returns center (9,9).
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

        // Check if still in opening phase (first 12 moves)
        if (!_lookupService.IsInOpeningPhase(board))
            return null;

        // Query the book for a move
        return _lookupService.GetBookMove(board, player, difficulty);
    }

    /// <summary>
    /// Check if we're still in the opening phase.
    /// Opening phase is defined as the first 12 moves (24 stones).
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
    /// Load opening book from a SQLite database file.
    /// This replaces the in-memory store with a SQLite-backed store.
    /// </summary>
    /// <param name="databasePath">Path to the SQLite database file.</param>
    public static void LoadFromFile(string databasePath)
    {
        lock (_lock)
        {
            // Note: For simplicity, we use in-memory store by default.
            // To use SQLite, the application needs to inject a SqliteOpeningBookStore.
            // This is a placeholder for future enhancement.

            // The actual implementation would require:
            // 1. Resetting the singleton instance
            // 2. Creating a new SqliteOpeningBookStore
            // 3. Reconstructing the lookup service

            throw new NotImplementedException(
                "Loading from file requires DI container integration. " +
                "Use SqliteOpeningBookStore directly with OpeningBook constructor for now.");
        }
    }

    /// <summary>
    /// Get book statistics.
    /// </summary>
    public BookStatistics GetStatistics() => _store.GetStatistics();
}
