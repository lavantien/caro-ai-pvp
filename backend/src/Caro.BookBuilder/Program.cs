using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Infrastructure.Persistence;
using Microsoft.Extensions.Logging;

namespace Caro.BookBuilder;

class Program
{
    static async Task Main(string[] args)
    {
        // Parse simple flags first (help and debug)
        var showHelp = args.Contains("--help") || args.Contains("-h");
        var debugLogging = args.Contains("--debug");

        if (showHelp)
        {
            Console.WriteLine("Caro Opening Book Builder");
            Console.WriteLine();
            Console.WriteLine("Usage: dotnet run -- [options]");
            Console.WriteLine();
            Console.WriteLine("Options:");
            Console.WriteLine("  --output <path>       Output database path (default: opening_book.db)");
            Console.WriteLine("  --verify-only         Verify existing book without generation");
            Console.WriteLine("  --debug               Enable verbose logging (default: quiet mode)");
            Console.WriteLine("  --help, -h            Show this help message");
            Console.WriteLine();
            Console.WriteLine("Book Structure:");
            Console.WriteLine("  Plies 0-14:   4 moves per position (early game + survival zone)");
            Console.WriteLine("  Plies 15-24:  3 moves per position (Hard difficulty)");
            Console.WriteLine("  Plies 25-32:  2 moves per position (Grandmaster)");
            Console.WriteLine("  Plies 33-40:  1 move per position (Experimental - main line)");
            Console.WriteLine();
            Console.WriteLine("Examples:");
            Console.WriteLine("  dotnet run --");
            Console.WriteLine("  dotnet run -- --output custom_book.db --debug");
            Console.WriteLine("  dotnet run -- --verify-only");
            return;
        }

        // Setup logging
        using var loggerFactory = LoggerFactory.Create(builder =>
        {
            builder.AddSimpleConsole(options =>
            {
                options.IncludeScopes = false;
                options.SingleLine = false;
                options.TimestampFormat = "HH:mm:ss ";
            });
            // Default to Warning (quiet), use --debug for verbose output
            builder.SetMinimumLevel(debugLogging ? LogLevel.Debug : LogLevel.Warning);
        });

        var logger = loggerFactory.CreateLogger<Program>();

        Console.WriteLine("Caro Opening Book Builder");
        Console.WriteLine("=========================");
        Console.WriteLine();

        // Hardcoded book structure: 4-3-2-1 tapered beam up to ply 40
        const int MaxBookDepth = 40;
        const int TargetSearchDepth = 32;

        // Parse remaining arguments
        var outputPath = GetArgument(args, "--output", "opening_book.db");
        var verifyOnly = args.Contains("--verify-only");

        // Validate no unrecognized arguments
        ValidateArguments(args);

        Console.WriteLine($"Output: {outputPath}");
        Console.WriteLine("Book Structure: 4-3-2-1 tapered beam up to ply 40");
        Console.WriteLine();
        Console.WriteLine("  Plies 0-14:  4 moves/position");
        Console.WriteLine("  Plies 15-24: 3 moves/position");
        Console.WriteLine("  Plies 25-32: 2 moves/position");
        Console.WriteLine("  Plies 33-40: 1 move/position");
        Console.WriteLine();

        // Create store and generator (write mode - only place that modifies the database)
        var store = new SqliteOpeningBookStore(outputPath, loggerFactory.CreateLogger<SqliteOpeningBookStore>(), readOnly: false);
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
            validator,
            loggerFactory
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
            var progressTimer = new System.Timers.Timer(60000);  // 60 second intervals
            progressTimer.Elapsed += (s, e) =>
            {
                var progress = generator.GetProgress();
                if (progress.PercentComplete < 100)
                {
                    Console.WriteLine($"[{progress.ElapsedTime:hh\\:mm\\:ss}] " +
                                    $"Depth {progress.CurrentDepth} " +
                                    $"({progress.PositionsCompletedAtCurrentDepth}/{progress.TotalPositionsAtCurrentDepth} positions): " +
                                    $"Stored: {progress.PositionsStored}, " +
                                    $"Evaluated: {progress.PositionsEvaluated}, " +
                                    $"Progress: {progress.PercentComplete:F1}%");
                }
            };
            progressTimer.Start();

            var result = await generator.GenerateAsync(MaxBookDepth, TargetSearchDepth, cts.Token);

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

    static void ValidateArguments(string[] args)
    {
        var validArguments = new HashSet<string>(StringComparer.OrdinalIgnoreCase)
        {
            "--output",
            "--verify-only",
            "--debug",
            "--help",
            "-h"
        };

        for (int i = 0; i < args.Length; i++)
        {
            var arg = args[i];
            if (arg.StartsWith("--") || arg.StartsWith("-"))
            {
                if (!validArguments.Contains(arg))
                {
                    Console.WriteLine($"Error: Unrecognized argument '{arg}'");
                    Console.WriteLine();
                    Console.WriteLine("Run 'dotnet run -- --help' for usage information.");
                    Environment.Exit(1);
                }
            }
        }
    }
}
