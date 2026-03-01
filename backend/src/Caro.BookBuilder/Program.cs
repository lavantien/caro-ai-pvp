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
            Console.WriteLine("  --output <path>       Output database path (default: ../opening_book.db at repo root)");
            Console.WriteLine("  --depth <n>           Maximum book depth in plies (default: 16)");
            Console.WriteLine("  --moves <n>           Moves per position to expand (default: 2)");
            Console.WriteLine("  --self-play <n>       Run n self-play games (engine vs engine) for learning");
            Console.WriteLine("  --time-control <ms>   Time control per move in milliseconds (default: 1000)");
            Console.WriteLine("  --max-moves <n>       Maximum moves per self-play game (default: 100)");
            Console.WriteLine("  --resume              Resume generation from saved progress after interruption");
            Console.WriteLine("  --verify-only         Verify existing book without generation");
            Console.WriteLine("  --debug               Enable verbose logging (default: quiet mode)");
            Console.WriteLine("  --help, -h            Show this help message");
            Console.WriteLine();
            Console.WriteLine("Book Structure:");
            Console.WriteLine("  Configurable moves/position up to specified depth");
            Console.WriteLine();
            Console.WriteLine("Difficulty Coverage:");
            Console.WriteLine("  Easy:         depth 4  (2 moves per side)");
            Console.WriteLine("  Medium:       depth 6  (3 moves per side)");
            Console.WriteLine("  Hard:         depth 10 (5 moves per side)");
            Console.WriteLine("  Grandmaster:  depth 14 (7 moves per side)");
            Console.WriteLine("  Experimental: uses all available book depth");
            Console.WriteLine();
            Console.WriteLine("Examples:");
            Console.WriteLine("  dotnet run --");
            Console.WriteLine("  dotnet run -- --depth 20 --moves 3");
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

        // Book structure: configurable moves/position up to specified depth
        // This covers Easy (4), Medium (6), Hard (10), Grandmaster (14), plus buffer
        int maxBookDepth = GetIntArgument(args, "--depth", 16);
        int movesPerPosition = GetIntArgument(args, "--moves", 2);
        const int TargetSearchDepth = 12;

        // Parse remaining arguments
        // From build output directory, go up 6 levels to reach repo root
        var defaultPath = Path.GetFullPath(Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "..", "..", "..", "opening_book.db"));
        var outputPath = GetArgument(args, "--output", defaultPath);
        var verifyOnly = args.Contains("--verify-only");
        var resumeGeneration = args.Contains("--resume");
        var selfPlayGames = GetIntArgument(args, "--self-play", 0);
        var timeControlMs = GetIntArgument(args, "--time-control", 1000);
        var maxMoves = GetIntArgument(args, "--max-moves", 100);

        // Validate no unrecognized arguments
        ValidateArguments(args);

        Console.WriteLine($"Output: {outputPath}");
        Console.WriteLine($"Book Structure: {movesPerPosition} moves/position up to ply {maxBookDepth}");
        Console.WriteLine();
        Console.WriteLine($"  Plies 0-{maxBookDepth}:  {movesPerPosition} moves/position (covers Easy through Grandmaster+)");
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

        // Self-play mode for learning from engine vs engine games
        if (selfPlayGames > 0)
        {
            Console.WriteLine($"Running {selfPlayGames} self-play games...");
            Console.WriteLine($"Time control: {timeControlMs}ms per move");
            Console.WriteLine($"Max moves per game: {maxMoves}");
            Console.WriteLine();

            var selfPlayGenerator = new SelfPlayGenerator(store, loggerFactory);

            var selfPlayCts = new CancellationTokenSource();
            Console.CancelKeyPress += (s, e) =>
            {
                e.Cancel = true;
                Console.WriteLine("\nCancellation requested...");
                selfPlayCts.Cancel();
            };

            try
            {
                var summary = await selfPlayGenerator.GenerateGamesAsync(
                    selfPlayGames,
                    timeControlMs,
                    maxMoves,
                    selfPlayCts.Token);

                Console.WriteLine();
                Console.WriteLine("=== Self-Play Summary ===");
                Console.WriteLine($"Total Games: {summary.TotalGames}");
                Console.WriteLine($"Red Wins: {summary.RedWins} ({100.0 * summary.RedWins / summary.TotalGames:F1}%)");
                Console.WriteLine($"Blue Wins: {summary.BlueWins} ({100.0 * summary.BlueWins / summary.TotalGames:F1}%)");
                Console.WriteLine($"Draws: {summary.Draws} ({100.0 * summary.Draws / summary.TotalGames:F1}%)");
                Console.WriteLine($"Average Moves/Game: {summary.AverageMoves:F1}");

                store.Flush();
            }
            catch (OperationCanceledException)
            {
                Console.WriteLine("\nSelf-play was cancelled.");
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

        // Handle --resume flag to continue from saved progress
        if (resumeGeneration)
        {
            var savedProgress = store.LoadProgress();
            if (savedProgress != null)
            {
                Console.WriteLine($"Resuming from saved progress:");
                Console.WriteLine($"  Last depth: {savedProgress.CurrentDepth}");
                Console.WriteLine($"  Last batch: {savedProgress.CurrentBatchIndex}");
                Console.WriteLine($"  Phase: {savedProgress.Phase}");
                Console.WriteLine($"  Saved at: {savedProgress.LastUpdatedAt:yyyy-MM-dd HH:mm:ss}");
                Console.WriteLine();
            }
            else
            {
                Console.WriteLine("No saved progress found. Starting fresh generation.");
                Console.WriteLine();
            }
        }

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
            // Progress reporting - 15 second intervals for meaningful monitoring
            var progressTimer = new System.Timers.Timer(15000);  // 15 second intervals
            progressTimer.Elapsed += (s, e) =>
            {
                var progress = generator.GetProgress();
                if (progress.PercentComplete < 100)
                {
                    Console.WriteLine($"[{progress.ElapsedTime:hh\\:mm\\:ss}] Depth {progress.CurrentDepth} " +
                                    $"({progress.PositionsCompletedAtCurrentDepth}/{progress.TotalPositionsAtCurrentDepth} positions)");
                    Console.WriteLine($"  Stored: {progress.PositionsStored:N0}, Evaluated: {progress.PositionsEvaluated:N0}");
                    Console.WriteLine($"  Throughput: {progress.PositionsPerMinute:F1} pos/min, {FormatNodesPerSecond(progress.NodesPerSecond)} nodes/sec");
                    Console.WriteLine($"  Candidates: {progress.CandidatesEvaluated:N0} evaluated, {progress.CandidatesPruned:N0} pruned, {progress.EarlyExits:N0} early exits");
                    Console.WriteLine($"  Write buffer: {progress.WriteBufferFlushes:N0} flushes, peak {progress.MaxWriteBufferSize}/{50}");
                    Console.WriteLine($"  Progress: {progress.PercentComplete:F1}%");
                    Console.WriteLine();
                }
            };
            progressTimer.Start();

            var result = await generator.GenerateAsync(maxBookDepth, TargetSearchDepth, movesPerPosition, cts.Token);

            progressTimer.Stop();

            // Get detailed statistics for final summary
            var detailedStats = generator.GetDetailedStatistics();

            Console.WriteLine();
            Console.WriteLine("=== Book Generation Summary ===");
            Console.WriteLine($"Total Time: {result.GenerationTime:hh\\:mm\\:ss}");
            Console.WriteLine($"Positions: {result.PositionsGenerated:N0} generated, {result.PositionsVerified:N0} verified");
            Console.WriteLine($"Moves: {result.TotalMovesStored:N0} stored");
            Console.WriteLine();

            if (detailedStats != null)
            {
                Console.WriteLine("Throughput:");
                Console.WriteLine($"  Average: {detailedStats.AveragePositionsPerMinute:F1} positions/minute");
                if (detailedStats.PeakPositionsPerMinute > 0)
                {
                    Console.WriteLine($"  Peak: {detailedStats.PeakPositionsPerMinute:F1} positions/minute (depth {detailedStats.PeakPositionsPerMinuteDepth})");
                }
                if (detailedStats.SlowestPositionsPerMinute > 0)
                {
                    Console.WriteLine($"  Slowest: {detailedStats.SlowestPositionsPerMinute:F1} positions/minute (depth {detailedStats.SlowestPositionsPerMinuteDepth})");
                }
                Console.WriteLine();

                Console.WriteLine("Search Statistics:");
                Console.WriteLine($"  Total nodes: {detailedStats.TotalNodesSearched:N0}");
                Console.WriteLine($"  Candidates: {detailedStats.TotalCandidatesEvaluated:N0} evaluated, {detailedStats.TotalCandidatesPruned:N0} pruned ({detailedStats.PruneRate:F1}%)");
                Console.WriteLine($"  Early exits: {detailedStats.TotalEarlyExits:N0} ({detailedStats.EarlyExitRate:F1}%)");
                Console.WriteLine();

                Console.WriteLine("Write Performance:");
                Console.WriteLine($"  Flushes: {detailedStats.WriteBufferFlushes:N0}");
                Console.WriteLine($"  Average batch: {detailedStats.AverageBatchSize:F1} entries");
                Console.WriteLine($"  Peak buffer: {detailedStats.PeakBufferSize}/{detailedStats.BufferCapacity}");
                Console.WriteLine();

                // Show per-depth breakdown for completed depths
                if (detailedStats.DepthStatistics.Count > 0)
                {
                    Console.WriteLine("Per-Depth Statistics:");
                    foreach (var depth in detailedStats.DepthStatistics.Take(15))  // Show first 15 depths
                    {
                        var depthThroughput = depth.Time.TotalMinutes > 0
                            ? depth.Positions / depth.Time.TotalMinutes
                            : 0;
                        Console.WriteLine($"  Depth {depth.Depth,2}: {depth.Positions,5} positions, {depth.MovesStored,5} moves, {depth.Time:mm\\:ss} time, {depthThroughput:F1} pos/min");
                    }
                    if (detailedStats.DepthStatistics.Count > 15)
                    {
                        Console.WriteLine($"  ... and {detailedStats.DepthStatistics.Count - 15} more depths");
                    }
                }
            }

            Console.WriteLine();
            var stats = store.GetStatistics();
            Console.WriteLine("Book Statistics:");
            Console.WriteLine($"Total Entries: {stats.TotalEntries:N0}");
            Console.WriteLine($"Max Depth: {stats.MaxDepth}");
            Console.WriteLine($"Total Moves: {stats.TotalMoves:N0}");

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

    static string FormatNodesPerSecond(double nodesPerSecond)
    {
        if (nodesPerSecond >= 1_000_000)
            return $"{nodesPerSecond / 1_000_000:F1}M";
        if (nodesPerSecond >= 1_000)
            return $"{nodesPerSecond / 1_000:F1}K";
        return $"{nodesPerSecond:F0}";
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

    static int GetIntArgument(string[] args, string name, int defaultValue)
    {
        for (int i = 0; i < args.Length - 1; i++)
        {
            if (args[i].Equals(name, StringComparison.OrdinalIgnoreCase))
            {
                if (int.TryParse(args[i + 1], out int value))
                    return value;
                Console.WriteLine($"Warning: Invalid value for {name}, using default {defaultValue}");
                return defaultValue;
            }
        }
        return defaultValue;
    }

    static void ValidateArguments(string[] args)
    {
        var validArguments = new HashSet<string>(StringComparer.OrdinalIgnoreCase)
        {
            "--output",
            "--depth",
            "--moves",
            "--self-play",
            "--time-control",
            "--max-moves",
            "--resume",
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
