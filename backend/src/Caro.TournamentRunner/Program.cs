using Caro.Core.Tournament;
using Caro.Core.Entities;
using Caro.Core.GameLogic;
using System.Text;

namespace Caro.TournamentRunner;

class Program
{
    // Fixed preset for reproducibility
    private const int PresetTimeSeconds = 420;
    private const int PresetIncrementSeconds = 5;
    private const int PresetGamesPerMatchup = 10;

    static async Task Main(string[] args)
    {
        // Find backend directory by searching upward from base directory
        var baseDir = AppDomain.CurrentDomain.BaseDirectory;
        var currentDir = new DirectoryInfo(baseDir);
        DirectoryInfo? backendDir = null;

        // Search upward for backend folder
        while (currentDir != null && currentDir.Parent != null)
        {
            if (currentDir.Name.Equals("backend", StringComparison.OrdinalIgnoreCase))
            {
                backendDir = currentDir;
                break;
            }
            currentDir = currentDir.Parent;
        }

        // Fallback: create in current working directory if backend not found
        var outputPath = backendDir != null
            ? Path.Combine(backendDir.FullName, "test_output.txt")
            : "test_output.txt";

        using var writer = new StreamWriter(outputPath, append: false, Encoding.UTF8)
        {
            AutoFlush = true
        };
        Console.SetOut(writer);
        Console.SetError(writer);

        await ComprehensiveMatchupRunner.RunAsync(PresetTimeSeconds, PresetIncrementSeconds, PresetGamesPerMatchup);
    }
}
