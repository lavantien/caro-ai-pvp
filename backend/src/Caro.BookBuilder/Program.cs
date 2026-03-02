using Caro.Core.Domain.Configuration;
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
            ShowHelp();
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

        // Validate no unrecognized arguments
        ValidateArguments(args);

        // Determine which mode to run
        if (args.Contains("--full-pipeline"))
        {
            await RunFullPipelineAsync(args, loggerFactory, logger);
        }
        else if (args.Contains("--verify-staging"))
        {
            await RunVerifyStagingAsync(args, loggerFactory, logger);
        }
        else if (args.Contains("--integrate"))
        {
            RunIntegrate(args, loggerFactory, logger);
        }
        else if (args.Contains("--staging"))
        {
            await RunStagingAsync(args, loggerFactory, logger);
        }
        else if (args.Contains("--tune"))
        {
            await RunTuningAsync(args, loggerFactory, logger);
        }
        else if (args.Contains("--export-binary"))
        {
            RunExportBinary(args, loggerFactory, logger);
        }
        else if (args.Contains("--import-binary"))
        {
            RunImportBinary(args, loggerFactory, logger);
        }
        else
        {
            // Legacy mode - traditional book generation or self-play
            await RunLegacyAsync(args, loggerFactory, logger);
        }
    }

    static void ShowHelp()
    {
        Console.WriteLine("Caro Opening Book Builder");
        Console.WriteLine();
        Console.WriteLine("Usage: dotnet run -- [options]");
        Console.WriteLine();
        Console.WriteLine("=== SEPARATED PIPELINE (Recommended) ===");
        Console.WriteLine();
        Console.WriteLine("Phase 1: Self-Play Generation (Actor)");
        Console.WriteLine("  --staging <path>          Run self-play games to staging database");
        Console.WriteLine("  --games <n>               Number of games (default: 8192 = 2^13)");
        Console.WriteLine("  --base-time <ms>          Base time per player (default: 60000 = 1 min)");
        Console.WriteLine("  --increment <ms>          Time increment per move (default: 0)");
        Console.WriteLine("  --threads <n>             Parallel games (default: CPU cores)");
        Console.WriteLine("  --buffer <n>              Games before commit (default: 4096 = 2^12)");
        Console.WriteLine();
        Console.WriteLine("Phase 2: Verification (Critic)");
        Console.WriteLine("  --verify-staging <path>   Verify staging database with deep search");
        Console.WriteLine("  --time <ms>               Time per position (default: 2048 = 2^11 ms)");
        Console.WriteLine("                            Survival zone (ply 8-16) gets 4096ms");
        Console.WriteLine("  --output <path>           Output verified database");
        Console.WriteLine("  --threads <n>             Parallel threads (default: cores/2)");
        Console.WriteLine();
        Console.WriteLine("Phase 3: Integration");
        Console.WriteLine("  --integrate <path>        Integrate verified moves into main book");
        Console.WriteLine("  --book <path>             Main opening book path (default: opening_book.db)");
        Console.WriteLine("  --batch <n>               Batch size (default: 65536 = 2^16)");
        Console.WriteLine();
        Console.WriteLine("Convenience Command");
        Console.WriteLine("  --full-pipeline           Run all phases in sequence");
        Console.WriteLine("  --games <n>               Self-play games (default: 8192)");
        Console.WriteLine("  --base-time <ms>          Self-play base time (default: 60000 = 1 min)");
        Console.WriteLine("  --increment <ms>          Self-play increment (default: 0)");
        Console.WriteLine("  --verify-time <ms>        Verification time (default: 2048)");
        Console.WriteLine("  --threads <n>             Parallel games (default: CPU cores)");
        Console.WriteLine();
        Console.WriteLine("=== BINARY FORMAT ===");
        Console.WriteLine();
        Console.WriteLine("Export/Import:");
        Console.WriteLine("  --export-binary <path>   Export book to binary format (.cobook)");
        Console.WriteLine("  --import-binary <path>  Import book from binary format");
        Console.WriteLine("  --output <path>         Output path (for import)");
        Console.WriteLine("  --verify-only           Validate binary file only (with --import-binary)");
        Console.WriteLine();
        Console.WriteLine("=== SPSA PARAMETER TUNING ===");
        Console.WriteLine();
        Console.WriteLine("  --tune                     Run SPSA parameter tuning");
        Console.WriteLine("  --iterations <n>           Number of SPSA iterations (default: 50)");
        Console.WriteLine("  --games-per-eval <n>       Games per evaluation (default: 256)");
        Console.WriteLine("  --preset <name>            SPSA preset: Default, Aggressive, Conservative");
        Console.WriteLine("  --base-time <ms>           Base time per player (default: 10000)");
        Console.WriteLine("  --output <path>            Output optimized parameters to JSON file");
        Console.WriteLine();
        Console.WriteLine("Binary format (.cobook):");
        Console.WriteLine("  - ~4x smaller than SQLite");
        Console.WriteLine("  - ~10x faster load time");
        Console.WriteLine("  - Uses varint encoding for compactness");
        Console.WriteLine("  - Includes checksum for integrity");
        Console.WriteLine();
        Console.WriteLine("=== LEGACY MODE ===");
        Console.WriteLine();
        Console.WriteLine("Traditional Generation:");
        Console.WriteLine("  --output <path>           Output database path");
        Console.WriteLine("  --depth <n>               Maximum book depth in plies (default: 16)");
        Console.WriteLine("  --moves <n>               Moves per position to expand (default: 2)");
        Console.WriteLine("  --resume                  Resume generation from saved progress");
        Console.WriteLine();
        Console.WriteLine("Legacy Self-Play:");
        Console.WriteLine("  --self-play <n>           Run n self-play games (legacy, use --staging)");
        Console.WriteLine("  --time-control <ms>       Time control per move (default: 1000)");
        Console.WriteLine("  --max-moves <n>           Maximum moves per game (default: 100)");
        Console.WriteLine();
        Console.WriteLine("=== OTHER ===");
        Console.WriteLine();
        Console.WriteLine("  --verify-only             Verify existing book without generation");
        Console.WriteLine("  --debug                   Enable verbose logging");
        Console.WriteLine("  --help, -h                Show this help message");
        Console.WriteLine();
        Console.WriteLine("=== THRESHOLDS (All Powers of 2) ===");
        Console.WriteLine();
        Console.WriteLine("Statistical:");
        Console.WriteLine("  MinPlayCount = 512 (2^9)    - Filters fluke wins");
        Console.WriteLine("  MinWinRate = 0.625 (5/8)    - Winning line indicator");
        Console.WriteLine("  MaxWinRateForLoss = 0.375   - Losing line indicator");
        Console.WriteLine("  MinConsensusRate = 0.8125   - Self-play vs deep search consensus");
        Console.WriteLine();
        Console.WriteLine("Score (centipawns):");
        Console.WriteLine("  MaxScoreDelta = 512 (2^9)   - Pruning threshold");
        Console.WriteLine("  InclusionScoreDelta = 256   - Inclusion range");
        Console.WriteLine("  MaxMovesPerPosition = 4     - Variety without bloat");
        Console.WriteLine();
        Console.WriteLine("=== EXAMPLES ===");
        Console.WriteLine();
        Console.WriteLine("  # Separated Pipeline (Recommended)");
        Console.WriteLine("  dotnet run -- --staging staging.db --games 8192");
        Console.WriteLine("  dotnet run -- --verify-staging staging.db --output verified.db");
        Console.WriteLine("  dotnet run -- --integrate verified.db --book opening_book.db");
        Console.WriteLine();
        Console.WriteLine("  # Full Pipeline (All phases)");
        Console.WriteLine("  dotnet run -- --full-pipeline --games 8192 --threads 8");
        Console.WriteLine();
        Console.WriteLine("  # Legacy");
        Console.WriteLine("  dotnet run -- --depth 20 --moves 3");
        Console.WriteLine("  dotnet run -- --verify-only");
    }

    #region Separated Pipeline Commands

    /// <summary>
    /// Phase 1: Self-Play Generation (Actor) - Record moves to staging database.
    /// </summary>
    static async Task RunStagingAsync(string[] args, ILoggerFactory loggerFactory, ILogger<Program> logger)
    {
        var stagingPath = GetArgument(args, "--staging", "staging.db");
        var gameCount = GetIntArgument(args, "--games", 8192);  // 2^13
        var baseTimeMs = GetIntArgument(args, "--base-time", 60000);   // 1 min default
        var incrementMs = GetIntArgument(args, "--increment", 0);      // No increment
        var threads = GetIntArgument(args, "--threads", Environment.ProcessorCount);
        var buffer = GetIntArgument(args, "--buffer", 4096);     // 2^12
        var maxPly = GetIntArgument(args, "--max-ply", 16);

        Console.WriteLine("=== Phase 1: Self-Play Generation (Actor) ===");
        Console.WriteLine($"Staging database: {stagingPath}");
        Console.WriteLine($"Games: {gameCount}");
        Console.WriteLine($"Time control: {baseTimeMs / 60000}+{incrementMs / 1000}");
        Console.WriteLine($"Max ply to record: {maxPly}");
        Console.WriteLine($"Buffer size: {buffer}");
        Console.WriteLine();

        using var stagingStore = new StagingBookStore(
            stagingPath,
            loggerFactory.CreateLogger<StagingBookStore>(),
            buffer);

        stagingStore.Initialize();

        var canonicalizer = new PositionCanonicalizer();
        var selfPlayGenerator = new SelfPlayGenerator(
            stagingStore,
            canonicalizer,
            loggerFactory);

        var cts = new CancellationTokenSource();
        Console.CancelKeyPress += (s, e) =>
        {
            e.Cancel = true;
            Console.WriteLine("\nCancellation requested...");
            cts.Cancel();
        };

        try
        {
            var summary = await selfPlayGenerator.GenerateGamesAsync(
                gameCount,
                baseTimeMs: baseTimeMs,
                incrementMs: incrementMs,
                maxMoves: 200,
                maxPly: maxPly,
                workerCount: threads,
                cancellationToken: cts.Token);

            Console.WriteLine();
            Console.WriteLine("=== Self-Play Summary ===");
            Console.WriteLine($"Total Games: {summary.TotalGames}");
            Console.WriteLine($"Red Wins: {summary.RedWins} ({100.0 * summary.RedWins / Math.Max(1, summary.TotalGames):F1}%)");
            Console.WriteLine($"Blue Wins: {summary.BlueWins} ({100.0 * summary.BlueWins / Math.Max(1, summary.TotalGames):F1}%)");
            Console.WriteLine($"Draws: {summary.Draws} ({100.0 * summary.Draws / Math.Max(1, summary.TotalGames):F1}%)");
            Console.WriteLine($"Average Moves/Game: {summary.AverageMoves:F1}");
            Console.WriteLine($"Staging Moves Recorded: {summary.StagingMovesRecorded}");

            stagingStore.Flush();

            var positionCount = stagingStore.GetPositionCount();
            Console.WriteLine($"Total positions in staging: {positionCount}");
        }
        catch (OperationCanceledException)
        {
            Console.WriteLine("\nSelf-play was cancelled.");
        }
    }

    /// <summary>
    /// Phase 2: Verification (Critic) - Deep search verification of staging moves.
    /// </summary>
    static async Task RunVerifyStagingAsync(string[] args, ILoggerFactory loggerFactory, ILogger<Program> logger)
    {
        var stagingPath = GetArgument(args, "--verify-staging", "staging.db");
        var timeMs = GetIntArgument(args, "--time", 2048);       // 2^11
        var outputPath = GetArgument(args, "--output", "verified.db");
        var threads = GetIntArgument(args, "--threads", Math.Max(4, Environment.ProcessorCount / 2));
        var maxPly = GetIntArgument(args, "--max-ply", 16);

        Console.WriteLine("=== Phase 2: Verification (Critic) ===");
        Console.WriteLine($"Input staging: {stagingPath}");
        Console.WriteLine($"Output verified: {outputPath}");
        Console.WriteLine($"Time per position: {timeMs}ms (survival zone: 4096ms)");
        Console.WriteLine($"Max ply: {maxPly}");
        Console.WriteLine();

        // Read from staging (read-only)
        using var stagingStore = new StagingBookStore(
            stagingPath,
            loggerFactory.CreateLogger<StagingBookStore>());

        var positionCount = stagingStore.GetPositionCount();
        Console.WriteLine($"Positions in staging: {positionCount}");

        var verifier = new MoveVerifier(
            stagingStore,
            new PositionCanonicalizer(),
            loggerFactory);

        var cts = new CancellationTokenSource();
        Console.CancelKeyPress += (s, e) =>
        {
            e.Cancel = true;
            Console.WriteLine("\nCancellation requested...");
            cts.Cancel();
        };

        try
        {
            var summary = await verifier.VerifyStagingAsync(
                timeMs,
                maxPly,
                cts.Token);

            Console.WriteLine();
            Console.WriteLine("=== Verification Summary ===");
            Console.WriteLine($"Positions Processed: {summary.TotalPositionsProcessed}");
            Console.WriteLine($"Filtered (Low Play Count): {summary.FilteredLowPlayCount}");
            Console.WriteLine($"Filtered (Unclear Result): {summary.FilteredUnclearResult}");
            Console.WriteLine($"Moves Verified: {summary.TotalMovesVerified}");
            Console.WriteLine($"VCF Solved: {summary.VcfSolvedCount}");
            Console.WriteLine($"Consensus Rate: {summary.ConsensusRate:P1}");
            Console.WriteLine($"Duration: {summary.Duration}");

            // Show thresholds
            var thresholds = MoveVerifier.GetThresholds();
            Console.WriteLine();
            Console.WriteLine("=== Verification Thresholds ===");
            Console.WriteLine($"MinPlayCount: {thresholds.MinPlayCount} (2^9)");
            Console.WriteLine($"MinWinRate: {thresholds.MinWinRate} (5/8)");
            Console.WriteLine($"MaxWinRateForLoss: {thresholds.MaxWinRateForLoss} (3/8)");
            Console.WriteLine($"MinConsensusRate: {thresholds.MinConsensusRate} (13/16)");
            Console.WriteLine($"MaxScoreDelta: {thresholds.MaxScoreDelta} cp (2^9)");
            Console.WriteLine($"InclusionScoreDelta: {thresholds.InclusionScoreDelta} cp (2^8)");
            Console.WriteLine($"MaxMovesPerPosition: {thresholds.MaxMovesPerPosition} (2^2)");

            // TODO: Save verified moves to output database
            // For now, just log the count
            Console.WriteLine();
            Console.WriteLine($"Verified moves ready for integration: {summary.VerifiedMoves.Count}");
        }
        catch (OperationCanceledException)
        {
            Console.WriteLine("\nVerification was cancelled.");
        }
    }

    /// <summary>
    /// Phase 3: Integration - Merge verified moves into main book.
    /// </summary>
    static void RunIntegrate(string[] args, ILoggerFactory loggerFactory, ILogger<Program> logger)
    {
        var verifiedPath = GetArgument(args, "--integrate", "verified.db");
        var bookPath = GetArgument(args, "--book", GetDefaultBookPath());
        var batchSize = GetIntArgument(args, "--batch", 65536);  // 2^16

        Console.WriteLine("=== Phase 3: Book Integration ===");
        Console.WriteLine($"Input verified: {verifiedPath}");
        Console.WriteLine($"Main book: {bookPath}");
        Console.WriteLine($"Batch size: {batchSize}");
        Console.WriteLine();

        // TODO: Load verified moves from verified database
        // For now, show placeholder
        Console.WriteLine("Note: Full integration requires verified database with VerifiedMove records.");
        Console.WriteLine("This is a placeholder - implement verified.db loading when available.");

        var store = new SqliteOpeningBookStore(
            bookPath,
            loggerFactory.CreateLogger<SqliteOpeningBookStore>(),
            readOnly: false);

        store.Initialize();

        // Placeholder - in production, load from verified.db
        var verifiedMoves = new List<VerifiedMove>();

        if (verifiedMoves.Count > 0)
        {
            var summary = store.IntegrateVerifiedMoves(verifiedMoves, batchSize);

            Console.WriteLine();
            Console.WriteLine("=== Integration Summary ===");
            Console.WriteLine($"Total Positions: {summary.TotalPositions}");
            Console.WriteLine($"Positions Integrated: {summary.PositionsIntegrated}");
            Console.WriteLine($"Positions Filtered: {summary.PositionsFiltered}");
            Console.WriteLine($"Moves Integrated: {summary.MovesIntegrated}");
            Console.WriteLine($"Batches Processed: {summary.BatchesProcessed}");

            store.Flush();
        }
        else
        {
            Console.WriteLine("No verified moves to integrate.");
        }
    }

    /// <summary>
    /// Convenience command: Run all phases in sequence.
    /// </summary>
    static async Task RunFullPipelineAsync(string[] args, ILoggerFactory loggerFactory, ILogger<Program> logger)
    {
        var gameCount = GetIntArgument(args, "--games", 8192);
        var baseTimeMs = GetIntArgument(args, "--base-time", 60000);    // 1 min for self-play
        var incrementMs = GetIntArgument(args, "--increment", 0);       // No increment
        var verifyTimeMs = GetIntArgument(args, "--verify-time", 2048);
        var threads = GetIntArgument(args, "--threads", Environment.ProcessorCount);
        var bookPath = GetArgument(args, "--book", GetDefaultBookPath());

        var stagingPath = "staging.db";
        var verifiedPath = "verified.db";

        Console.WriteLine("=== Full Pipeline: All Phases ===");
        Console.WriteLine($"Games: {gameCount}");
        Console.WriteLine($"Self-play time control: {baseTimeMs / 60000}+{incrementMs / 1000}");
        Console.WriteLine($"Verification time: {verifyTimeMs}ms");
        Console.WriteLine($"Threads: {threads}");
        Console.WriteLine($"Final book: {bookPath}");
        Console.WriteLine();

        var totalStopwatch = System.Diagnostics.Stopwatch.StartNew();

        try
        {
            // Phase 1: Self-Play
            Console.WriteLine();
            Console.WriteLine(">>> Phase 1: Self-Play Generation <<<");
            Console.WriteLine();

            using (var stagingStore = new StagingBookStore(
                stagingPath,
                loggerFactory.CreateLogger<StagingBookStore>()))
            {
                stagingStore.Initialize();

                var selfPlayGenerator = new SelfPlayGenerator(
                    stagingStore,
                    new PositionCanonicalizer(),
                    loggerFactory);

                var selfPlaySummary = await selfPlayGenerator.GenerateGamesAsync(
                    gameCount,
                    baseTimeMs: baseTimeMs,
                    incrementMs: incrementMs,
                    maxMoves: 200,
                    maxPly: 16,
                    workerCount: threads);

                Console.WriteLine($"Phase 1 complete: {selfPlaySummary.StagingMovesRecorded} moves to staging");
            }

            // Phase 2: Verification
            Console.WriteLine();
            Console.WriteLine(">>> Phase 2: Verification <<<");
            Console.WriteLine();

            using (var stagingStore = new StagingBookStore(
                stagingPath,
                loggerFactory.CreateLogger<StagingBookStore>()))
            {
                var verifier = new MoveVerifier(
                    stagingStore,
                    new PositionCanonicalizer(),
                    loggerFactory);

                var verificationSummary = await verifier.VerifyStagingAsync(verifyTimeMs, 16);

                Console.WriteLine($"Phase 2 complete: {verificationSummary.TotalMovesVerified} moves verified");
                Console.WriteLine($"VCF solved: {verificationSummary.VcfSolvedCount}");
                Console.WriteLine($"Consensus: {verificationSummary.ConsensusRate:P1}");

                // Phase 3: Integration
                Console.WriteLine();
                Console.WriteLine(">>> Phase 3: Integration <<<");
                Console.WriteLine();

                var store = new SqliteOpeningBookStore(
                    bookPath,
                    loggerFactory.CreateLogger<SqliteOpeningBookStore>(),
                    readOnly: false);

                store.Initialize();

                var integrationSummary = store.IntegrateVerifiedMoves(
                    verificationSummary.VerifiedMoves,
                    batchSize: 65536);

                Console.WriteLine($"Phase 3 complete: {integrationSummary.MovesIntegrated} moves integrated");

                store.Flush();
            }

            // Cleanup temporary databases
            Console.WriteLine();
            Console.WriteLine(">>> Cleanup <<<");
            try
            {
                if (File.Exists(stagingPath)) File.Delete(stagingPath);
                if (File.Exists(verifiedPath)) File.Delete(verifiedPath);
                Console.WriteLine("Temporary databases cleaned up.");
            }
            catch (Exception ex)
            {
                Console.WriteLine($"Warning: Could not clean up temporary files: {ex.Message}");
            }

            totalStopwatch.Stop();

            Console.WriteLine();
            Console.WriteLine("=== Full Pipeline Summary ===");
            Console.WriteLine($"Total Time: {totalStopwatch.Elapsed:hh\\:mm\\:ss}");
            Console.WriteLine($"Final book: {bookPath}");
        }
        catch (Exception ex)
        {
            Console.WriteLine($"\nPipeline error: {ex.Message}");
            Console.WriteLine("Temporary databases preserved for debugging.");
            Environment.Exit(1);
        }
    }

    #endregion

    #region Legacy Mode

    /// <summary>
    /// Legacy mode - traditional book generation or self-play.
    /// </summary>
    static async Task RunLegacyAsync(string[] args, ILoggerFactory loggerFactory, ILogger<Program> logger)
    {
        // Book structure: configurable moves/position up to specified depth
        int maxBookDepth = GetIntArgument(args, "--depth", 16);
        int movesPerPosition = GetIntArgument(args, "--moves", 2);
        const int TargetSearchDepth = 12;

        var outputPath = GetArgument(args, "--output", GetDefaultBookPath());
        var verifyOnly = args.Contains("--verify-only");
        var resumeGeneration = args.Contains("--resume");
        var selfPlayGames = GetIntArgument(args, "--self-play", 0);
        var timeControlMs = GetIntArgument(args, "--time-control", 1000);
        var maxMoves = GetIntArgument(args, "--max-moves", 100);

        Console.WriteLine($"Output: {outputPath}");
        Console.WriteLine($"Book Structure: {movesPerPosition} moves/position up to ply {maxBookDepth}");
        Console.WriteLine();
        Console.WriteLine($"  Plies 0-{maxBookDepth}:  {movesPerPosition} moves/position");
        Console.WriteLine();

        // Create store (write mode)
        var store = new SqliteOpeningBookStore(
            outputPath,
            loggerFactory.CreateLogger<SqliteOpeningBookStore>(),
            readOnly: false);

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

        // Legacy self-play mode (deprecated - use --staging instead)
        if (selfPlayGames > 0)
        {
            Console.WriteLine("WARNING: --self-play is deprecated. Use --staging for separated pipeline.");
            Console.WriteLine($"Running {selfPlayGames} self-play games (legacy mode)...");
            Console.WriteLine();

            // Use staging store in non-separated mode
            var stagingPath = outputPath + ".staging";
            using var stagingStore = new StagingBookStore(
                stagingPath,
                loggerFactory.CreateLogger<StagingBookStore>());

            var selfPlayGenerator = new SelfPlayGenerator(
                stagingStore,
                new PositionCanonicalizer(),
                loggerFactory,
                store);

            var cts = new CancellationTokenSource();
            Console.CancelKeyPress += (s, e) =>
            {
                e.Cancel = true;
                Console.WriteLine("\nCancellation requested...");
                cts.Cancel();
            };

            try
            {
                var summary = await selfPlayGenerator.GenerateGamesAsync(
                    selfPlayGames,
                    baseTimeMs: timeControlMs,  // Legacy: treat as base time
                    incrementMs: 5000,
                    maxMoves: maxMoves,
                    maxPly: 16,
                    cancellationToken: cts.Token);

                Console.WriteLine();
                Console.WriteLine("=== Self-Play Summary ===");
                Console.WriteLine($"Total Games: {summary.TotalGames}");
                Console.WriteLine($"Red Wins: {summary.RedWins}");
                Console.WriteLine($"Blue Wins: {summary.BlueWins}");
                Console.WriteLine($"Draws: {summary.Draws}");
                Console.WriteLine($"Average Moves/Game: {summary.AverageMoves:F1}");
                Console.WriteLine($"Staging Moves Recorded: {summary.StagingMovesRecorded}");
            }
            catch (OperationCanceledException)
            {
                Console.WriteLine("\nSelf-play was cancelled.");
            }

            return;
        }

        // Traditional book generation
        var canonicalizer = new PositionCanonicalizer();
        var validator = new OpeningBookValidator();
        var generator = new OpeningBookGenerator(
            store,
            canonicalizer,
            validator,
            loggerFactory);

        if (resumeGeneration)
        {
            var savedProgress = store.LoadProgress();
            if (savedProgress != null)
            {
                Console.WriteLine($"Resuming from saved progress:");
                Console.WriteLine($"  Last depth: {savedProgress.CurrentDepth}");
                Console.WriteLine($"  Last batch: {savedProgress.CurrentBatchIndex}");
                Console.WriteLine($"  Phase: {savedProgress.Phase}");
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

        var cts2 = new CancellationTokenSource();
        Console.CancelKeyPress += (s, e) =>
        {
            e.Cancel = true;
            Console.WriteLine("\nCancellation requested...");
            generator.Cancel();
            cts2.Cancel();
        };

        try
        {
            var progressTimer = new System.Timers.Timer(15000);
            progressTimer.Elapsed += (s, e) =>
            {
                var progress = generator.GetProgress();
                if (progress.PercentComplete < 100)
                {
                    Console.WriteLine($"[{progress.ElapsedTime:hh\\:mm\\:ss}] Depth {progress.CurrentDepth} " +
                                    $"({progress.PositionsCompletedAtCurrentDepth}/{progress.TotalPositionsAtCurrentDepth})");
                    Console.WriteLine($"  Progress: {progress.PercentComplete:F1}%");
                }
            };
            progressTimer.Start();

            var result = await generator.GenerateAsync(maxBookDepth, TargetSearchDepth, movesPerPosition, cts2.Token);

            progressTimer.Stop();

            Console.WriteLine();
            Console.WriteLine("=== Book Generation Summary ===");
            Console.WriteLine($"Total Time: {result.GenerationTime:hh\\:mm\\:ss}");
            Console.WriteLine($"Positions: {result.PositionsGenerated:N0} generated");
            Console.WriteLine($"Moves: {result.TotalMovesStored:N0} stored");

            var stats = store.GetStatistics();
            Console.WriteLine();
            Console.WriteLine("Book Statistics:");
            Console.WriteLine($"Total Entries: {stats.TotalEntries:N0}");
            Console.WriteLine($"Max Depth: {stats.MaxDepth}");

            store.Flush();
        }
        catch (OperationCanceledException)
        {
            Console.WriteLine("\nGeneration was cancelled.");
        }
        catch (Exception ex)
        {
            Console.WriteLine($"\nError: {ex.Message}");
            Environment.Exit(1);
        }
    }

    #endregion

    #region Binary Export/Import

    /// <summary>
    /// Export opening book to binary format (.cobook).
    /// </summary>
    static void RunExportBinary(string[] args, ILoggerFactory loggerFactory, ILogger<Program> logger)
    {
        var bookPath = GetArgument(args, "--export-binary", GetDefaultBookPath());
        var outputPath = GetArgument(args, "--output", "book.cobook");

        Console.WriteLine("=== Binary Export ===");
        Console.WriteLine($"Input book: {bookPath}");
        Console.WriteLine($"Output: {outputPath}");
        Console.WriteLine();

        if (!File.Exists(bookPath))
        {
            Console.WriteLine($"Error: Book file not found: {bookPath}");
            Environment.Exit(1);
        }

        var store = new SqliteOpeningBookStore(
            bookPath,
            loggerFactory.CreateLogger<SqliteOpeningBookStore>(),
            readOnly: true);

        store.Initialize();

        var entries = store.GetAllEntries().ToList();
        var exporter = new BinaryBookExporter();
        var result = exporter.Export(entries, outputPath);

        Console.WriteLine();
        Console.WriteLine("=== Export Summary ===");
        Console.WriteLine($"Entries exported: {result.EntriesExported:N0}");
        Console.WriteLine($"Total moves: {result.TotalMoves:N0}");
        Console.WriteLine($"File size: {result.BytesWritten:N0} bytes ({result.BytesWritten / 1024.0:F1} KB)");
        var bytesPerEntry = result.EntriesExported > 0 ? (double)result.BytesWritten / result.EntriesExported : 0;
        var bytesPerMove = result.TotalMoves > 0 ? (double)result.BytesWritten / result.TotalMoves : 0;
        Console.WriteLine($"Bytes per entry: {bytesPerEntry:F1}");
        Console.WriteLine($"Bytes per move: {bytesPerMove:F1}");
        Console.WriteLine($"Max depth: {result.MaxDepth}");
        Console.WriteLine($"Duration: {result.Duration.TotalMilliseconds}ms");
    }

    /// <summary>
    /// Import opening book from binary format (.cobook).
    /// </summary>
    static void RunImportBinary(string[] args, ILoggerFactory loggerFactory, ILogger<Program> logger)
    {
        var inputPath = GetArgument(args, "--import-binary", "book.cobook");
        var outputPath = GetArgument(args, "--output", GetDefaultBookPath());
        var verifyOnly = args.Contains("--verify-only");

        Console.WriteLine("=== Binary Import ===");
        Console.WriteLine($"Input: {inputPath}");
        Console.WriteLine($"Output: {outputPath}");
        Console.WriteLine();

        var importer = new BinaryBookImporter();

        if (verifyOnly)
        {
            var validationResult = importer.Validate(inputPath);
            Console.WriteLine("=== Validation Result ===");
            if (validationResult.IsValid)
            {
                Console.WriteLine("Status: Valid");
                Console.WriteLine($"Entries: {validationResult.EntryCount:N0}");
                Console.WriteLine($"Total moves: {validationResult.TotalMoves:N0}");
            }
            else
            {
                Console.WriteLine($"Status: Invalid");
                Console.WriteLine($"Error: {validationResult.ErrorMessage}");
                Environment.Exit(1);
            }
            return;
        }

        var importResult = importer.Import(inputPath, verifyChecksum: true);

        var store = new SqliteOpeningBookStore(
            outputPath,
            loggerFactory.CreateLogger<SqliteOpeningBookStore>(),
            readOnly: false);

        store.Initialize();

        // Batch insert for performance
        store.StoreEntriesBatch(importResult.Entries);
        store.Flush();

        Console.WriteLine();
        Console.WriteLine("=== Import Summary ===");
        Console.WriteLine($"Entries imported: {importResult.Entries.Count:N0}");
        Console.WriteLine($"Total moves: {importResult.TotalMoves:N0}");
        Console.WriteLine($"Max depth: {importResult.MaxDepth}");
        Console.WriteLine($"Duration: {importResult.Duration.TotalMilliseconds}ms");
    }

    #endregion

    #region SPSA Tuning

    /// <summary>
    /// Run SPSA parameter tuning
    /// </summary>
    static async Task RunTuningAsync(string[] args, ILoggerFactory loggerFactory, ILogger<Program> logger)
    {
        var iterations = GetIntArgument(args, "--iterations", 50);
        var gamesPerEval = GetIntArgument(args, "--games-per-eval", 256);
        var baseTimeMs = GetIntArgument(args, "--base-time", 10000);
        var preset = GetArgument(args, "--preset", "Default");
        var outputPath = GetArgument(args, "--output", "");

        Console.WriteLine("=== SPSA Parameter Tuning ===");
        Console.WriteLine($"Iterations: {iterations}");
        Console.WriteLine($"Games per evaluation: {gamesPerEval}");
        Console.WriteLine($"Base time: {baseTimeMs}ms");
        Console.WriteLine($"Preset: {preset}");
        Console.WriteLine();

        // Get SPSA parameters from preset
        var spsaConfig = preset.ToLowerInvariant() switch
        {
            "aggressive" => SPSAParameters.Aggressive,
            "conservative" => SPSAParameters.Conservative,
            _ => SPSAParameters.Default
        };

        Console.WriteLine($"SPSA Config: A={spsaConfig.A}, C={spsaConfig.C}, " +
                          $"alpha={spsaConfig.Alpha}, gamma={spsaConfig.Gamma}");
        Console.WriteLine();

        var tuningService = new SPSATuningService(loggerFactory);

        var cts = new CancellationTokenSource();
        Console.CancelKeyPress += (s, e) =>
        {
            e.Cancel = true;
            Console.WriteLine("\nCancellation requested...");
            cts.Cancel();
        };

        try
        {
            var result = await tuningService.RunTuningAsync(
                iterations,
                gamesPerEval,
                spsaConfig,
                baseTimeMs,
                cts.Token);

            Console.WriteLine();
            Console.WriteLine("=== Tuning Summary ===");
            Console.WriteLine($"Total iterations: {result.Iterations}");
            Console.WriteLine($"Duration: {result.Duration:hh\\:mm\\:ss}");
            Console.WriteLine($"Initial win rate: {result.InitialWinRate:P1}");
            Console.WriteLine($"Final win rate: {result.FinalWinRate:P1}");
            Console.WriteLine();

            // Display final parameters
            var finalParams = new TunableParameters();
            finalParams.ApplyFromArray(result.FinalParameters);
            Console.WriteLine("Final parameters:");
            Console.WriteLine($"  FiveInRowScore: {finalParams.FiveInRowScore:F0}");
            Console.WriteLine($"  OpenFourScore: {finalParams.OpenFourScore:F0}");
            Console.WriteLine($"  ClosedFourScore: {finalParams.ClosedFourScore:F0}");
            Console.WriteLine($"  OpenThreeScore: {finalParams.OpenThreeScore:F0}");
            Console.WriteLine($"  ClosedThreeScore: {finalParams.ClosedThreeScore:F0}");
            Console.WriteLine($"  OpenTwoScore: {finalParams.OpenTwoScore:F0}");
            Console.WriteLine($"  CenterBonus: {finalParams.CenterBonus:F0}");
            Console.WriteLine($"  DefenseMultiplier: {finalParams.DefenseMultiplier:F2}");

            // Save to file if output specified
            if (!string.IsNullOrEmpty(outputPath))
            {
                finalParams.Save(outputPath);
                Console.WriteLine();
                Console.WriteLine($"Parameters saved to: {outputPath}");
            }

            // Show parameter changes
            Console.WriteLine();
            Console.WriteLine("=== Parameter Changes ===");
            var defaults = new TunableParameters();
            Console.WriteLine($"FiveInRowScore: {EvaluationConstants.FiveInRowScore} -> {finalParams.FiveInRowScore:F0}");
            Console.WriteLine($"OpenFourScore: {EvaluationConstants.OpenFourScore} -> {finalParams.OpenFourScore:F0}");
            Console.WriteLine($"ClosedFourScore: {EvaluationConstants.ClosedFourScore} -> {finalParams.ClosedFourScore:F0}");
            Console.WriteLine($"OpenThreeScore: {EvaluationConstants.OpenThreeScore} -> {finalParams.OpenThreeScore:F0}");
            Console.WriteLine($"ClosedThreeScore: {EvaluationConstants.ClosedThreeScore} -> {finalParams.ClosedThreeScore:F0}");
            Console.WriteLine($"OpenTwoScore: {EvaluationConstants.OpenTwoScore} -> {finalParams.OpenTwoScore:F0}");
            Console.WriteLine($"CenterBonus: {EvaluationConstants.CenterBonus} -> {finalParams.CenterBonus:F0}");
            var defMult = (double)EvaluationConstants.DefenseMultiplierNumerator / EvaluationConstants.DefenseMultiplierDenominator;
            Console.WriteLine($"DefenseMultiplier: {defMult:F2} -> {finalParams.DefenseMultiplier:F2}");
        }
        catch (OperationCanceledException)
        {
            Console.WriteLine("\nTuning was cancelled.");
        }
        catch (Exception ex)
        {
            Console.WriteLine($"\nError: {ex.Message}");
            Environment.Exit(1);
        }
    }

    #endregion

    #region Helpers

    static string GetDefaultBookPath()
    {
        return Path.GetFullPath(Path.Combine(
            AppContext.BaseDirectory,
            "..", "..", "..", "..", "..", "..",
            "opening_book.db"));
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
            // Separated pipeline
            "--staging",
            "--verify-staging",
            "--integrate",
            "--full-pipeline",
            "--games",
            "--time",
            "--base-time",
            "--increment",
            "--threads",
            "--buffer",
            "--batch",
            "--verify-time",
            "--book",
            "--max-ply",
            // SPSA tuning
            "--tune",
            "--iterations",
            "--games-per-eval",
            "--preset",
            // Binary format
            "--export-binary",
            "--import-binary",
            "--verify-only",
            // Legacy
            "--output",
            "--depth",
            "--moves",
            "--self-play",
            "--time-control",
            "--max-moves",
            "--resume",
            // General
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

    #endregion
}
