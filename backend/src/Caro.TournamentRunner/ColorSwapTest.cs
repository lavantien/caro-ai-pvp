using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tournament;

namespace Caro.TournamentRunner;

public class ColorSwapTest
{
    public static void Run()
    {
        var engine = TournamentEngineFactory.CreateWithOpeningBook();
        const int games = 4;

        for (int i = 0; i < games; i++)
        {
            bool swapColors = (i % 2 == 1);
            // Grandmaster always plays as BotA, Braindead as BotB
            // swapColors determines which color each bot plays
            var redDiff = AIDifficulty.Grandmaster;  // BotA's difficulty
            var blueDiff = AIDifficulty.Braindead;   // BotB's difficulty

            var actualRed = swapColors ? AIDifficulty.Braindead : AIDifficulty.Grandmaster;
            var actualBlue = swapColors ? AIDifficulty.Grandmaster : AIDifficulty.Braindead;

            Console.WriteLine($"=== Game {i + 1}: swapColors={swapColors}, Actual: Red={actualRed}, Blue={actualBlue} ===");

            var result = engine.RunGame(
                redDifficulty: redDiff,  // Always Grandmaster (BotA)
                blueDifficulty: blueDiff,  // Always Braindead (BotB)
                maxMoves: 50,
                initialTimeSeconds: 30,
                incrementSeconds: 0,
                ponderingEnabled: true,
                parallelSearchEnabled: true,
                swapColors: swapColors,
                onLog: (level, source, message) =>
                {
                    if (level == "debug" || level == "warn" || level == "error")
                    {
                        Console.WriteLine($"    [{level.ToUpper()}] {source}: {message}");
                    }
                }
            );

            Console.WriteLine($"Result: {result.WinnerDifficulty} ({result.Winner}) won in {result.TotalMoves} moves");
            Console.WriteLine();
        }
    }
}
