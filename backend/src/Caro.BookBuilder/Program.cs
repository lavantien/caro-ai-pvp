using Caro.Core.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Infrastructure.Persistence;
using Microsoft.Extensions.Logging;

namespace Caro.BookBuilder;

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
                options.SingleLine = false;
                options.TimestampFormat = "HH:mm:ss ";
            });
            builder.SetMinimumLevel(LogLevel.Information);
        });

        var logger = loggerFactory.CreateLogger<Program>();

        Console.WriteLine("Caro Opening Book Builder");
        Console.WriteLine("=========================");
        Console.WriteLine();

        // Parse arguments
        var outputPath = GetArgument(args, "--output", "opening_book.db");
        var maxDepthStr = GetArgument(args, "--max-depth", "12");  // Default to 12 plies (6 moves each)
        var targetDepthStr = GetArgument(args, "--target-depth", "24");  // Default to 24 ply search
        var verifyOnly = args.Contains("--verify-only");

        if (!int.TryParse(maxDepthStr, out int maxDepth))
        {
            Console.WriteLine($"Invalid max-depth: {maxDepthStr}");
            return;
        }

        if (!int.TryParse(targetDepthStr, out int targetDepth))
        {
            Console.WriteLine($"Invalid target-depth: {targetDepthStr}");
            return;
        }

        Console.WriteLine($"Output: {outputPath}");
        Console.WriteLine($"Max Depth: {maxDepth} plies");
        Console.WriteLine($"Target Search Depth: {targetDepth} plies");
        Console.WriteLine();

        // Create store and generator
        var store = new SqliteOpeningBookStore(outputPath, loggerFactory.CreateLogger<SqliteOpeningBookStore>());
        store.Initialize();

        if (verifyOnly)
        {
            Console.WriteLine("Verifying existing book...");
            var stats = store.GetStatistics();
            Console.WriteLine($"Total Entries: {stats.TotalEntries}");
            Console.WriteLine($"Max Depth: {stats.MaxDepth}");
            Console.WriteLine($"Total Moves: {stats.TotalMoves}");
            Console.WriteLine($"Generated At: {stats.GeneratedAt:yyyy-MM-dd HH:mm:ss}");
            Console.WriteLine($"Version: {stats.Version}");

            if (stats.CoverageByDepth != null)
            {
                Console.WriteLine();
                Console.WriteLine("Coverage by Depth:");
                for (int i = 0; i < stats.CoverageByDepth.Length && stats.CoverageByDepth[i] > 0; i++)
                {
                    Console.WriteLine($"  Ply {i}: {stats.CoverageByDepth[i]} positions");
                }
            }
            return;
        }

        var canonicalizer = new PositionCanonicalizer();
        var validator = new OpeningBookValidator();
        var generator = new OpeningBookGenerator(
            store,
            canonicalizer,
            validator
        );

        Console.WriteLine("Starting book generation...");
        Console.WriteLine("Press Ctrl+C to stop.");
        Console.WriteLine();

        var cts = new CancellationTokenSource();
        Console.CancelKeyPress += (s, e) =>
        {
            e.Cancel = true;
            Console.WriteLine("\nCancellation requested...");
            generator.Cancel();
            cts.Cancel();
        };

        try
        {
            // Progress reporting
            var progressTimer = new System.Timers.Timer(5000);
            progressTimer.Elapsed += (s, e) =>
            {
                var progress = generator.GetProgress();
                if (progress.PercentComplete < 100)
                {
                    Console.WriteLine($"[{progress.ElapsedTime:hh\\:mm\\:ss}] {progress.CurrentPhase}: " +
                                    $"Stored: {progress.PositionsStored}, " +
                                    $"Evaluated: {progress.PositionsEvaluated}, " +
                                    $"Progress: {progress.PercentComplete:F1}%");
                }
            };
            progressTimer.Start();

            var result = await generator.GenerateAsync(maxDepth, targetDepth, cts.Token);

            progressTimer.Stop();

            Console.WriteLine();
            Console.WriteLine("Generation Complete!");
            Console.WriteLine($"Positions Generated: {result.PositionsGenerated}");
            Console.WriteLine($"Positions Verified: {result.PositionsVerified}");
            Console.WriteLine($"Total Moves Stored: {result.TotalMovesStored}");
            Console.WriteLine($"Blunders Found: {result.BlundersFound}");
            Console.WriteLine($"Generation Time: {result.GenerationTime:hh\\:mm\\:ss}");

            var stats = store.GetStatistics();
            Console.WriteLine();
            Console.WriteLine("Book Statistics:");
            Console.WriteLine($"Total Entries: {stats.TotalEntries}");
            Console.WriteLine($"Max Depth: {stats.MaxDepth}");
            Console.WriteLine($"Total Moves: {stats.TotalMoves}");

            store.Flush();
        }
        catch (OperationCanceledException)
        {
            Console.WriteLine("\nGeneration was cancelled.");
            var stats = store.GetStatistics();
            Console.WriteLine($"Partial progress: {stats.TotalEntries} positions, {stats.TotalMoves} moves stored.");
        }
        catch (Exception ex)
        {
            Console.WriteLine($"\nError: {ex.Message}");
            Console.WriteLine(ex.StackTrace);
            Environment.Exit(1);
        }
    }

    static string GetArgument(string[] args, string name, string defaultValue)
    {
        for (int i = 0; i < args.Length - 1; i++)
        {
            if (args[i].Equals(name, StringComparison.OrdinalIgnoreCase))
            {
                return args[i + 1];
            }
        }
        return defaultValue;
    }
}
