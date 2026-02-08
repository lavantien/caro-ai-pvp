using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Infrastructure.Persistence;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Logging.Abstractions;

namespace Caro.Core.Tests.Helpers;

/// <summary>
/// Shared setup logic for creating opening book generators in tests.
/// Provides methods for creating generators with real stores (SQLite) or mock stores.
/// </summary>
public static class OpeningBookTestSetup
{
    /// <summary>
    /// Create an OpeningBookGenerator with a real SQLite store using a temporary file.
    /// The temporary file is automatically cleaned up when the store is disposed.
    /// </summary>
    public static OpeningBookGenerator CreateGeneratorWithRealStore(
        string? tempFilePath = null,
        IPositionCanonicalizer? canonicalizer = null,
        IOpeningBookValidator? validator = null,
        ILoggerFactory? loggerFactory = null)
    {
        // Use provided temp file or generate one
        tempFilePath ??= Path.Combine(Path.GetTempPath(), $"book_test_{Guid.NewGuid()}.db");

        var store = new SqliteOpeningBookStore(
            $"Data Source={tempFilePath}",
            NullLogger<SqliteOpeningBookStore>.Instance,
            readOnly: false
        );
        store.Initialize();

        canonicalizer ??= new PositionCanonicalizer();
        validator ??= new OpeningBookValidator();
        loggerFactory ??= NullLoggerFactory.Instance;

        return new OpeningBookGenerator(store, canonicalizer, validator, loggerFactory);
    }

    /// <summary>
    /// Create an OpeningBookGenerator with a mock in-memory store.
    /// Ideal for unit tests that don't need SQLite functionality.
    /// </summary>
    public static OpeningBookGenerator CreateGeneratorWithMockStore(
        IPositionCanonicalizer? canonicalizer = null,
        IOpeningBookValidator? validator = null,
        ILoggerFactory? loggerFactory = null,
        IEnumerable<OpeningBookEntry>? initialEntries = null)
    {
        var store = initialEntries != null
            ? new MockOpeningBookStore(initialEntries)
            : new MockOpeningBookStore();

        canonicalizer ??= new MockPositionCanonicalizer();
        validator ??= new MockOpeningBookValidator();
        loggerFactory ??= NullLoggerFactory.Instance;

        return new OpeningBookGenerator(store, canonicalizer, validator, loggerFactory);
    }

    /// <summary>
    /// Create an OpeningBookGenerator with mock components for isolated testing.
    /// All dependencies are mocks for maximum test isolation.
    /// </summary>
    public static OpeningBookGenerator CreateGeneratorWithAllMocks(
        ILoggerFactory? loggerFactory = null)
    {
        return CreateGeneratorWithMockStore(
            canonicalizer: new MockPositionCanonicalizer(),
            validator: new MockOpeningBookValidator(),
            loggerFactory: loggerFactory
        );
    }

    /// <summary>
    /// Create a test board with a center position (9, 9) occupied by Red.
    /// </summary>
    public static Board CreateCenterPosition()
    {
        var board = new Board();
        return board.PlaceStone(9, 9, Player.Red);
    }

    /// <summary>
    /// Create a test board with the specified moves already played.
    /// </summary>
    public static Board CreateBoardWithMoves(params (int x, int y, Player player)[] moves)
    {
        var board = new Board();
        foreach (var (x, y, player) in moves)
        {
            board = board.PlaceStone(x, y, player);
        }
        return board;
    }

    /// <summary>
    /// Create a test board representing the opening sequence:
    /// Red at (9,9), Blue at (9,10), Red at (8,10)
    /// </summary>
    public static Board CreateTypicalOpeningPosition()
    {
        return CreateBoardWithMoves(
            (9, 9, Player.Red),
            (9, 10, Player.Blue),
            (8, 10, Player.Red)
        );
    }

    /// <summary>
    /// Get a temporary file path for SQLite testing.
    /// </summary>
    public static string GetTempDbPath()
    {
        return Path.Combine(Path.GetTempPath(), $"book_test_{Guid.NewGuid()}.db");
    }

    /// <summary>
    /// Clean up a temporary database file if it exists.
    /// </summary>
    public static void CleanupTempDb(string? filePath)
    {
        if (string.IsNullOrEmpty(filePath))
            return;

        try
        {
            if (File.Exists(filePath))
                File.Delete(filePath);

            // Also delete -journal and -wal files if they exist
            string journalPath = $"{filePath}-journal";
            string walPath = $"{filePath}-wal";

            if (File.Exists(journalPath))
                File.Delete(journalPath);

            if (File.Exists(walPath))
                File.Delete(walPath);
        }
        catch
        {
            // Ignore cleanup errors in tests
        }
    }

    /// <summary>
    /// Create an OpeningBookLookupService with all mock dependencies.
    /// </summary>
    public static OpeningBookLookupService CreateLookupService(
        IOpeningBookStore? store = null,
        IPositionCanonicalizer? canonicalizer = null,
        IOpeningBookValidator? validator = null,
        Random? random = null)
    {
        store ??= new MockOpeningBookStore();
        canonicalizer ??= new MockPositionCanonicalizer();
        validator ??= new MockOpeningBookValidator();

        return new OpeningBookLookupService(store, canonicalizer, validator, random);
    }

    /// <summary>
    /// Create an OpeningBook with all mock dependencies.
    /// </summary>
    public static OpeningBook CreateOpeningBook(
        IOpeningBookStore? store = null,
        IPositionCanonicalizer? canonicalizer = null,
        OpeningBookLookupService? lookupService = null)
    {
        store ??= new MockOpeningBookStore();
        canonicalizer ??= new MockPositionCanonicalizer();
        lookupService ??= CreateLookupService(store, canonicalizer);

        return new OpeningBook(store, canonicalizer, lookupService);
    }
}
