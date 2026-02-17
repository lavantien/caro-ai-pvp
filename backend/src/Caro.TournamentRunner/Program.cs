using Caro.Core.Tournament;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using System.Text;

namespace Caro.TournamentRunner;

class Program
{
    static async Task Main(string[] args)
    {
        var baseDir = AppDomain.CurrentDomain.BaseDirectory;
        var currentDir = new DirectoryInfo(baseDir);
        DirectoryInfo? backendDir = null;

        while (currentDir != null && currentDir.Parent != null)
        {
            if (currentDir.Name.Equals("backend", StringComparison.OrdinalIgnoreCase))
            {
                backendDir = currentDir;
                break;
            }
            currentDir = currentDir.Parent;
        }

        var defaultOutputPath = backendDir != null
            ? Path.Combine(backendDir.FullName, "tournament_results.txt")
            : "tournament_results.txt";

        var testSuiteArg = args.FirstOrDefault(a => a.StartsWith("--test-suite="));
        if (testSuiteArg != null)
        {
            var suiteName = testSuiteArg.Split('=')[1];
            var outputPath = args
                .FirstOrDefault(a => a.StartsWith("--output="))
                ?.Split('=')[1] ?? defaultOutputPath;

            var runner = new TestSuiteRunner();
            runner.Run(suiteName, outputPath);
            return;
        }

        if (args.Contains("--help") || args.Contains("-h"))
        {
            PrintHelp();
            return;
        }

        if (args.Contains("--quick"))
        {
            QuickSmokeTest.Run();
            return;
        }

        if (args.Contains("--comprehensive"))
        {
            var options = ParseComprehensiveOptions(args);
            await ComprehensiveMatchupRunner.RunAsync(options);
            return;
        }

        if (args.Contains("--color-swap-test"))
        {
            ColorSwapTest.Run();
            return;
        }

        using var writer = new StreamWriter(defaultOutputPath, append: false, Encoding.UTF8)
        {
            AutoFlush = true
        };
        Console.SetOut(writer);
        Console.SetError(writer);

        await GrandmasterVsBraindeadRunner.RunAsync();
    }

    private static void PrintHelp()
    {
        Console.WriteLine("Caro Tournament Runner - AI Matchup Testing");
        Console.WriteLine();
        Console.WriteLine("Usage: dotnet run -- [options]");
        Console.WriteLine();
        Console.WriteLine("Options:");
        Console.WriteLine("  --quick                          Run quick smoke test (5 matchups x 2 games)");
        Console.WriteLine("  --comprehensive                  Run full comprehensive test suite");
        Console.WriteLine();
        Console.WriteLine("Comprehensive Test Options:");
        Console.WriteLine("  --matchups=<list>                Comma-separated list of matchups");
        Console.WriteLine("                                   Format: Diff1vsDiff2,Diff3vsDiff4,...");
        Console.WriteLine("                                   Default: all standard matchups");
        Console.WriteLine("  --time=<initial>+<increment>     Time control in seconds (e.g., 180+2)");
        Console.WriteLine("                                   Default: 180+2 (3+2 blitz)");
        Console.WriteLine("  --games=<n>                      Games per matchup");
        Console.WriteLine("                                   Default: 20");
        Console.WriteLine("  --no-pondering                   Disable pondering");
        Console.WriteLine("  --no-parallel                    Disable parallel search");
        Console.WriteLine();
        Console.WriteLine("Available Difficulties:");
        Console.WriteLine("  Braindead, Easy, Medium, Hard, Grandmaster, Experimental");
        Console.WriteLine();
        Console.WriteLine("Examples:");
        Console.WriteLine("  dotnet run -- --quick");
        Console.WriteLine("  dotnet run -- --comprehensive");
        Console.WriteLine("  dotnet run -- --comprehensive --matchups=EasyvsBraindead,GMvsHard --time=120+1 --games=10");
        Console.WriteLine("  dotnet run -- --comprehensive --matchups=GMvsGM --time=300+5 --games=5");
        Console.WriteLine();
        Console.WriteLine("Other Options:");
        Console.WriteLine("  --test-suite=<name>              Run a predefined test suite");
        Console.WriteLine("  --color-swap-test                Run color swap validation test");
        Console.WriteLine("  --help, -h                       Show this help message");
    }

    private static ComprehensiveOptions ParseComprehensiveOptions(string[] args)
    {
        var options = new ComprehensiveOptions();

        // Parse matchups
        var matchupsArg = args.FirstOrDefault(a => a.StartsWith("--matchups="));
        if (matchupsArg != null)
        {
            var matchupsList = matchupsArg.Split('=')[1];
            foreach (var matchup in matchupsList.Split(','))
            {
                var parsed = ParseMatchup(matchup.Trim());
                if (parsed != null)
                {
                    options.Matchups.Add(parsed.Value);
                }
            }
        }

        // Parse time control
        var timeArg = args.FirstOrDefault(a => a.StartsWith("--time="));
        if (timeArg != null)
        {
            var timeStr = timeArg.Split('=')[1];
            var parts = timeStr.Split('+');
            if (parts.Length == 2 && int.TryParse(parts[0], out var initial) && int.TryParse(parts[1], out var increment))
            {
                options.InitialTimeSeconds = initial;
                options.IncrementSeconds = increment;
            }
        }

        // Parse games per matchup
        var gamesArg = args.FirstOrDefault(a => a.StartsWith("--games="));
        if (gamesArg != null && int.TryParse(gamesArg.Split('=')[1], out var games))
        {
            options.GamesPerMatchup = games;
        }

        // Parse boolean flags
        options.EnablePondering = !args.Contains("--no-pondering");
        options.EnableParallel = !args.Contains("--no-parallel");

        return options;
    }

    private static (AIDifficulty First, AIDifficulty Second)? ParseMatchup(string matchup)
    {
        // Support formats like "EasyvsBraindead", "GMvsHard", "GrandmastervsMedium"
        var vsIndex = matchup.IndexOf("vs", StringComparison.OrdinalIgnoreCase);
        if (vsIndex <= 0) return null;

        var firstStr = matchup.Substring(0, vsIndex);
        var secondStr = matchup.Substring(vsIndex + 2);

        var first = ParseDifficulty(firstStr);
        var second = ParseDifficulty(secondStr);

        if (first == null || second == null) return null;

        return (first.Value, second.Value);
    }

    private static AIDifficulty? ParseDifficulty(string str)
    {
        return str.ToLowerInvariant() switch
        {
            "braindead" or "bd" => AIDifficulty.Braindead,
            "easy" => AIDifficulty.Easy,
            "medium" or "med" => AIDifficulty.Medium,
            "hard" => AIDifficulty.Hard,
            "grandmaster" or "gm" => AIDifficulty.Grandmaster,
            "experimental" or "exp" => AIDifficulty.Experimental,
            _ => null
        };
    }
}
