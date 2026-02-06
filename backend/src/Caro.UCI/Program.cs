using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.GameLogic.UCI;
using Caro.Core.Infrastructure.Persistence;
using Microsoft.Extensions.Logging;

namespace Caro.UCI;

/// <summary>
/// Main entry point for the UCI engine.
/// Implements the Universal Chess Interface protocol for Caro.
/// </summary>
class Program
{
    static async Task Main(string[] args)
    {
        // Setup logging
        using var loggerFactory = LoggerFactory.Create(builder =>
        {
            builder.AddSimpleConsole(options =>
            {
                options.IncludeScopes = false;
                options.SingleLine = true;
                options.TimestampFormat = "HH:mm:ss ";
            });
            builder.SetMinimumLevel(LogLevel.Warning);  // Quiet by default
        });

        var logger = loggerFactory.CreateLogger<Program>();

        // Initialize components
        var dbPath = Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "..", "..", "..", "opening_book.db");
        var store = new SqliteOpeningBookStore(dbPath, loggerFactory.CreateLogger<SqliteOpeningBookStore>(), readOnly: true);
        store.Initialize();

        var canonicalizer = new PositionCanonicalizer();
        var validator = new OpeningBookValidator();
        var lookupService = new OpeningBookLookupService(store, canonicalizer, validator);
        var openingBook = new OpeningBook(store, canonicalizer, lookupService);

        var ai = new MinimaxAI(logger: loggerFactory.CreateLogger<MinimaxAI>(), openingBook: openingBook);
        var protocol = new UCIProtocol(ai, logger);

        // Handle graceful shutdown
        var cts = new CancellationTokenSource();
        Console.CancelKeyPress += (s, e) =>
        {
            e.Cancel = true;
            protocol.Stop();
            cts.Cancel();
        };

        // Main UCI loop
        await protocol.RunAsync(Console.In, Console.Out, cts.Token);
    }
}
