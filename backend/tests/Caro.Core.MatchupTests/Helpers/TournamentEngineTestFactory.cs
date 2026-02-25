using Caro.Core.GameLogic;
using Caro.Core.Infrastructure.Persistence;
using Caro.Core.Tournament;
using Microsoft.Extensions.Logging.Abstractions;

namespace Caro.Core.MatchupTests.Helpers;

/// <summary>
/// Factory for creating TournamentEngine instances with opening book support.
/// </summary>
public static class TournamentEngineTestFactory
{
    /// <summary>
    /// Create a TournamentEngine with opening book loaded from repo root.
    /// Use for production matchups where book should be enabled.
    /// </summary>
    public static TournamentEngine CreateWithOpeningBook()
    {
        var dbPath = OpeningBookPathResolver.FindOpeningBookPath();
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
}
