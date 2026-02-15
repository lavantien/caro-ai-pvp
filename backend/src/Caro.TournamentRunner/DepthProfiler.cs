using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tournament;

namespace Caro.TournamentRunner;

/// <summary>
/// Quick profiler to check what depths each AI level reaches with different time controls.
/// </summary>
public class DepthProfiler
{
    public static void Run(string[] args)
    {
        var engine = TournamentEngineFactory.CreateWithOpeningBook();
        var difficulties = new[]
        {
            AIDifficulty.Braindead,
            AIDifficulty.Easy,
            AIDifficulty.Medium,
            AIDifficulty.Hard,
            AIDifficulty.Grandmaster
        };

        var timeControls = new[]
        {
            (name: "30s", initial: 30, increment: 0),
            (name: "3+2", initial: 180, increment: 2),
            (name: "7+5", initial: 420, increment: 5)
        };

        Console.WriteLine("=== AI Depth Profiler ===");
        Console.WriteLine();

        foreach (var (tcName, initialTime, increment) in timeControls)
        {
            Console.WriteLine($"--- Time Control: {tcName} ({initialTime}s + {increment}s/move) ---");

            foreach (var difficulty in difficulties)
            {
                // Run a quick game and collect depth statistics via callback
                var depths = new List<int>();
                var nodes = new List<long>();
                var redDepths = new List<int>();
                var blueDepths = new List<int>();

                var result = engine.RunGame(
                    redDifficulty: difficulty,
                    blueDifficulty: difficulty,
                    maxMoves: 100,  // Short game for quick testing
                    initialTimeSeconds: initialTime,
                    incrementSeconds: increment,
                    ponderingEnabled: true,
                    onMove: (x, y, player, moveNumber, redTimeMs, blueTimeMs, stats) =>
                    {
                        if (stats != null)
                        {
                            depths.Add(stats.DepthAchieved);
                            nodes.Add(stats.NodesSearched);

                            if (player == Player.Red)
                                redDepths.Add(stats.DepthAchieved);
                            else
                                blueDepths.Add(stats.DepthAchieved);
                        }
                    });

                if (depths.Count > 0)
                {
                    var avgDepth = depths.Average();
                    var maxDepth = depths.Max();
                    var minDepth = depths.Min();
                    var avgNodes = nodes.Average();
                    var totalNodes = nodes.Sum();
                    var redAvg = redDepths.Count > 0 ? redDepths.Average() : 0;
                    var blueAvg = blueDepths.Count > 0 ? blueDepths.Average() : 0;

                    Console.WriteLine($"  {difficulty,-12}: AvgDepth={avgDepth,5:F1} (R:{redAvg:F1} B:{blueAvg:F1}) Min={minDepth,2} Max={maxDepth,2} Nodes={totalNodes / 1000000.0:F1}M");
                }
                else
                {
                    Console.WriteLine($"  {difficulty,-12}: No data");
                }
            }

            Console.WriteLine();
        }
    }
}
