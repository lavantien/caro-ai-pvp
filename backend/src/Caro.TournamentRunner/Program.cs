using Caro.Core.Tournament;
using Caro.Core.Entities;
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
}
