using Caro.Core.Domain.Entities;

namespace Caro.UCIMockClient;

/// <summary>
/// UCI Mock Client - Runs UCI engine matches for testing the UCI protocol layer.
/// </summary>
class Program
{
    static async Task Main(string[] args)
    {
        var exePath = GetEnginePath(args);

        if (string.IsNullOrEmpty(exePath) || !File.Exists(exePath))
        {
            Console.WriteLine("Error: UCI engine project/executable not found.");
            Console.WriteLine("Usage: Caro.UCIMockClient <engine-path>");
            Console.WriteLine("Example: Caro.UCIMockClient ..\\..\\..\\Caro.UCI\\Caro.UCI.csproj");
            Console.WriteLine("         Caro.UCIMockClient ..\\Caro.UCI\\bin\\Debug\\net10.0\\Caro.UCI.exe");
            Environment.Exit(1);
            return;
        }

        Console.WriteLine("Starting UCI Mock Client Match");
        Console.WriteLine("=============================");
        Console.WriteLine();
        Console.WriteLine($"Engine: {exePath}");
        Console.WriteLine();

        // Parse optional arguments
        int totalGames = 4;
        int initialTimeSeconds = 180;  // 3 minutes
        int incrementSeconds = 2;      // 2 seconds

        for (int i = 0; i < args.Length; i++)
        {
            if (args[i] == "--games" && i + 1 < args.Length)
            {
                if (int.TryParse(args[i + 1], out int games))
                    totalGames = games;
            }
            else if (args[i] == "--time" && i + 1 < args.Length)
            {
                if (int.TryParse(args[i + 1], out int time))
                    initialTimeSeconds = time;
            }
            else if (args[i] == "--inc" && i + 1 < args.Length)
            {
                if (int.TryParse(args[i + 1], out int inc))
                    incrementSeconds = inc;
            }
        }

        // Display match configuration
        Console.WriteLine("Match Configuration");
        Console.WriteLine("-------------------");
        Console.WriteLine($"Bot A: Hard (Skill Level 4)");
        Console.WriteLine($"Bot B: Grandmaster (Skill Level 5)");
        Console.WriteLine($"Time Control: {initialTimeSeconds / 60}+{incrementSeconds}");
        Console.WriteLine($"Games: {totalGames} (alternating colors)");
        Console.WriteLine($"Opening Book: Enabled (depth 24)");
        Console.WriteLine();

        // Create two engine instances
        using var botA = new UCIMockClient(exePath);
        using var botB = new UCIMockClient(exePath);

        // Configure skill levels
        botA.SetSkillLevel(4);    // Hard
        botB.SetSkillLevel(5);    // Grandmaster

        // Configure opening book
        botA.SetOpeningBook(true, 24);
        botB.SetOpeningBook(true, 24);

        try
        {
            // Start both engines
            Console.WriteLine("Starting engine processes...");
            botA.StartEngine();
            botB.StartEngine();
            Console.WriteLine("Engines started, initializing...");
            Console.WriteLine();

            // Initialize UCI protocol
            await botA.InitializeEngineAsync();
            await botB.InitializeEngineAsync();
            Console.WriteLine("Engines initialized.");
            Console.WriteLine();

            // Run the match
            var results = await GameManager.RunMatchAsync(
                botA,
                botB,
                "Hard",
                "Grandmaster",
                totalGames,
                progress: result => DisplayGameResult(result),
                logInfo: info => Console.WriteLine(info)
            );

            // Display final summary
            DisplayFinalSummary(results);
        }
        catch (Exception ex)
        {
            Console.ForegroundColor = ConsoleColor.Red;
            Console.WriteLine($"Error: {ex.Message}");
            Console.WriteLine(ex.StackTrace);
            Console.ResetColor();
            Environment.Exit(1);
        }
        finally
        {
            // Stop engines
            Console.WriteLine();
            Console.WriteLine("Stopping engines...");
            botA.StopEngine();
            botB.StopEngine();
        }

        Console.WriteLine();
        Console.WriteLine("Match completed.");

        // Only wait for key if running interactively
        try
        {
            Console.ReadKey();
        }
        catch (InvalidOperationException)
        {
            // Console input redirected - just exit
        }
    }

    /// <summary>
    /// Get the engine path from command line arguments or find it automatically.
    /// </summary>
    static string? GetEnginePath(string[] args)
    {
        // First try command line argument
        if (args.Length > 0 && !args[0].StartsWith("--"))
        {
            return args[0];
        }

        // Try to find the Caro.UCI project or executable
        // Prefer .csproj for 'dotnet run' which handles native DLLs correctly
        var possiblePaths = new[]
        {
            @"..\Caro.UCI\Caro.UCI.csproj",  // Project file - uses 'dotnet run'
            @"..\Caro.UCI\bin\Debug\net10.0\Caro.UCI.exe",
            @"..\Caro.UCI\bin\Release\net10.0\Caro.UCI.exe",
            @"..\..\Caro.UCI\Caro.UCI.csproj",
            @"..\..\Caro.UCI\bin\Debug\net10.0\Caro.UCI.exe",
            @"..\..\Caro.UCI\bin\Release\net10.0\Caro.UCI.exe",
            @".\Caro.UCI.exe",
            @"Caro.UCI.exe"
        };

        foreach (var path in possiblePaths)
        {
            if (File.Exists(path))
                return Path.GetFullPath(path);
        }

        return null;
    }

    /// <summary>
    /// Display the result of a single game.
    /// </summary>
    static void DisplayGameResult(GameResult result)
    {
        Console.WriteLine();
        Console.WriteLine($"Game {result.GameNumber} Result: {result.ResultString} in {result.TotalMoves} moves ({result.DurationMs / 1000.0:F1}s)");

        if (result.Moves.Count > 0)
        {
            Console.WriteLine("Move summary:");
            var redMoves = result.Moves.Where(m => m.Player == Player.Red).ToList();
            var blueMoves = result.Moves.Where(m => m.Player == Player.Blue).ToList();

            if (redMoves.Count > 0)
            {
                var avgRedTime = redMoves.Average(m => m.MoveTimeMs);
                var maxRedTime = redMoves.Max(m => m.MoveTimeMs);
                Console.WriteLine($"  Red: {redMoves.Count} moves, avg {avgRedTime:F0}ms, max {maxRedTime}ms");
            }

            if (blueMoves.Count > 0)
            {
                var avgBlueTime = blueMoves.Average(m => m.MoveTimeMs);
                var maxBlueTime = blueMoves.Max(m => m.MoveTimeMs);
                Console.WriteLine($"  Blue: {blueMoves.Count} moves, avg {avgBlueTime:F0}ms, max {maxBlueTime}ms");
            }
        }

        if (result.EndedByTimeout)
        {
            Console.ForegroundColor = ConsoleColor.Yellow;
            Console.WriteLine("  (Ended by timeout)");
            Console.ResetColor();
        }
    }

    /// <summary>
    /// Display the final match summary.
    /// </summary>
    static void DisplayFinalSummary(List<GameResult> results)
    {
        Console.WriteLine();
        Console.WriteLine("===============================");
        Console.WriteLine("Final Results");
        Console.WriteLine("===============================");
        Console.WriteLine();

        var hardWins = results.Count(r => r.Winner == Player.Red && r.RedBotName == "Hard") +
                       results.Count(r => r.Winner == Player.Blue && r.BlueBotName == "Hard");
        var grandmasterWins = results.Count(r => r.Winner == Player.Red && r.RedBotName == "Grandmaster") +
                              results.Count(r => r.Winner == Player.Blue && r.BlueBotName == "Grandmaster");
        var draws = results.Count(r => r.Winner == Player.None);

        Console.WriteLine($"Grandmaster: {grandmasterWins} win{(grandmasterWins != 1 ? "s" : "")}");
        Console.WriteLine($"Hard: {hardWins} win{(hardWins != 1 ? "s" : "")}");
        if (draws > 0)
        {
            Console.WriteLine($"Draws: {draws}");
        }

        Console.WriteLine();

        // Detailed per-game results
        Console.WriteLine("Game-by-Game Results:");
        Console.WriteLine("----------------------");
        foreach (var result in results)
        {
            var winner = result.Winner == Player.None ? "Draw" :
                         result.Winner == Player.Red ? result.RedBotName : result.BlueBotName;
            Console.WriteLine($"Game {result.GameNumber}: {winner} ({result.TotalMoves} moves, {result.DurationMs / 1000.0:F1}s)");
        }

        Console.WriteLine();

        // Statistics
        var allMoves = results.SelectMany(r => r.Moves).ToList();
        if (allMoves.Count > 0)
        {
            var avgTime = allMoves.Average(m => m.MoveTimeMs);
            var maxTime = allMoves.Max(m => m.MoveTimeMs);
            var minTime = allMoves.Min(m => m.MoveTimeMs);

            Console.WriteLine("Move Time Statistics:");
            Console.WriteLine($"  Average: {avgTime:F0}ms");
            Console.WriteLine($"  Min: {minTime}ms");
            Console.WriteLine($"  Max: {maxTime}ms");
        }

        var totalTimeMs = results.Sum(r => r.DurationMs);
        Console.WriteLine($"Total match duration: {totalTimeMs / 1000.0:F1}s ({totalTimeMs / 60000.0:F1} minutes)");
    }
}
