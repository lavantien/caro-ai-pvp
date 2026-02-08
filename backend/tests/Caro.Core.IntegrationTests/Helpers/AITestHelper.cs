using Caro.Core.GameLogic;
using Caro.Core.Infrastructure.Persistence;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Logging.Abstractions;

namespace Caro.Core.IntegrationTests.Helpers;

/// <summary>
/// Helper class for creating MinimaxAI instances with proper DI in tests.
/// Encapsulates the OpeningBook setup logic.
/// </summary>
public static class AITestHelper
{
    /// <summary>
    /// Create a MinimaxAI instance with a minimal opening book setup for testing.
    /// Uses a SQLite in-memory database for tests that don't specifically test opening book functionality.
    /// Uses non-deterministic Random (Random.Shared).
    /// </summary>
    public static MinimaxAI CreateAI(int ttSizeMb = 256, ILogger<MinimaxAI>? logger = null)
    {
        return CreateAI(random: null, ttSizeMb, logger);
    }

    /// <summary>
    /// Create a MinimaxAI instance with deterministic random source for testing.
    /// Pass a seeded Random instance for reproducible test results.
    /// </summary>
    public static MinimaxAI CreateAI(Random? random, int ttSizeMb = 256, ILogger<MinimaxAI>? logger = null)
    {
        // For tests that don't need a persistent SQLite opening book, use an in-memory database
        // This provides fast, isolated tests without file I/O overhead
        var store = new SqliteOpeningBookStore(
            "file::memory:?cache=shared",  // In-memory SQLite database
            NullLogger<SqliteOpeningBookStore>.Instance,
            readOnly: false
        );
        store.Initialize();
        var canonicalizer = new PositionCanonicalizer();
        var validator = new OpeningBookValidator();
        var lookupService = new OpeningBookLookupService(store, canonicalizer, validator, random);
        var openingBook = new OpeningBook(store, canonicalizer, lookupService);

        return new MinimaxAI(ttSizeMb, logger, openingBook, random);
    }

    /// <summary>
    /// Create a MinimaxAI instance with a fixed random seed for deterministic tests.
    /// </summary>
    public static MinimaxAI CreateDeterministicAI(int seed = 42, int ttSizeMb = 256, ILogger<MinimaxAI>? logger = null)
    {
        return CreateAI(new Random(seed), ttSizeMb, logger);
    }

    /// <summary>
    /// Create a MinimaxAI instance without an opening book.
    /// Useful for tests that specifically test AI behavior without book interference.
    /// </summary>
    public static MinimaxAI CreateAIWithoutBook(int ttSizeMb = 256, ILogger<MinimaxAI>? logger = null, Random? random = null)
    {
        return new MinimaxAI(ttSizeMb, logger, openingBook: null, random);
    }
}
