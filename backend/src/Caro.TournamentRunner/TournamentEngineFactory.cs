using Caro.Core.GameLogic;
using Caro.Core.Infrastructure.Persistence;
using Caro.Core.Tournament;
using Microsoft.Extensions.Logging.Abstractions;

namespace Caro.TournamentRunner;

/// <summary>
/// Factory for creating TournamentEngine instances with opening book support.
/// </summary>
public static class TournamentEngineFactory
{
    /// <summary>
    /// Create a TournamentEngine with opening book loaded from repo root.
    /// Use for production matchups where book should be enabled.
    /// </summary>
    public static TournamentEngine CreateWithOpeningBook()
    {
        var dbPath = FindOpeningBookPath();
        var store = new SqliteOpeningBookStore(dbPath, NullLogger<SqliteOpeningBookStore>.Instance);
        store.Initialize();

        var canonicalizer = new PositionCanonicalizer();
        var validator = new OpeningBookValidator();
        var lookupService = new OpeningBookLookupService(store, canonicalizer, validator);
        var openingBook = new OpeningBook(store, canonicalizer, lookupService);

        return new TournamentEngine(
            new MinimaxAI(openingBook: openingBook),
            new MinimaxAI(openingBook: openingBook)
        );
    }

    /// <summary>
    /// Find the opening book database path.
    /// Searches in common locations relative to the executable.
    /// </summary>
    private static string FindOpeningBookPath()
    {
        // Search order: repo root relative to bin, cwd, executable dir
        var searchPaths = new[]
        {
            // From bin/Debug/net10.0 back to repo root
            Path.Combine(Directory.GetCurrentDirectory(), "..", "..", "..", "..", "..", "opening_book.db"),
            // Current working directory
            Path.Combine(Directory.GetCurrentDirectory(), "opening_book.db"),
        };

        foreach (var path in searchPaths)
        {
            var fullPath = Path.GetFullPath(path);
            if (File.Exists(fullPath))
                return fullPath;
        }

        throw new FileNotFoundException(
            "opening_book.db not found. Searched: " +
            string.Join(", ", searchPaths.Select(p => Path.GetFullPath(p))));
    }
}
