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

        var outputPath = backendDir != null
            ? Path.Combine(backendDir.FullName, "tournament_results.txt")
            : "tournament_results.txt";

        using var writer = new StreamWriter(outputPath, append: false, Encoding.UTF8)
        {
            AutoFlush = true
        };
        Console.SetOut(writer);
        Console.SetError(writer);

        await ComprehensiveMatchupRunner.RunAsync();
    }
}
