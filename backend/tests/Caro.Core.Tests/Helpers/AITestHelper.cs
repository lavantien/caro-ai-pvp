using Caro.Core.GameLogic;
using Microsoft.Extensions.Logging;

namespace Caro.Core.Tests.Helpers;

/// <summary>
/// Helper class for creating MinimaxAI instances with proper DI in tests.
/// Encapsulates the OpeningBook setup logic.
/// </summary>
public static class AITestHelper
{
    /// <summary>
    /// Create a MinimaxAI instance with a minimal opening book setup for testing.
    /// Uses an in-memory store for tests that don't specifically test opening book functionality.
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
        // For tests that don't need SQLite opening book, create a minimal setup
        // Using InMemoryOpeningBookStore for fast, isolated tests
        var store = new InMemoryOpeningBookStore();
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
}
