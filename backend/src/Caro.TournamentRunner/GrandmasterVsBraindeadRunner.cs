using Caro.Core.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tournament;

namespace Caro.TournamentRunner;

/// <summary>
/// Grandmaster vs Braindead baseline: 10 games, alternating colors, full parallel, full pondering
/// </summary>
public class GrandmasterVsBraindeadRunner
{
    private static void LogWrite(string? message = null)
    {
        Console.WriteLine(message);
    }

    public static async Task RunAsync()
    {
        var engine = new TournamentEngine();
        const int initialTimeSeconds = 420;  // 7 minutes
        const int incrementSeconds = 5;      // +5 seconds per move
        const int gamesPerMatchup = 10;

        LogWrite("═══════════════════════════════════════════════════════════════════");
        LogWrite("  GRANDMASTER VS BRAINDEAD: 7+5 Time Control");
        LogWrite("  10 Games (alternating colors)");
        LogWrite("  Full parallel search, Full pondering");
        LogWrite("═══════════════════════════════════════════════════════════════════");
        LogWrite();

        var redWins = 0;
        var blueWins = 0;
        var draws = 0;
        var totalMoves = 0;
        var totalTimeMs = 0L;
        var grandmasterWins = 0;
        var braindeadWins = 0;

        for (int game = 1; game <= gamesPerMatchup; game++)
        {
            // Alternate colors each game: even games swap colors
            // Game 1: Grandmaster=Red, Braindead=Blue
            // Game 2: Grandmaster=Blue, Braindead=Red
            // Difficulties stay constant - swapColors controls which bot plays which color
            bool swapColors = (game % 2 == 0);
            var redDiff = AIDifficulty.Grandmaster;  // BotA's difficulty
            var blueDiff = AIDifficulty.Braindead;   // BotB's difficulty

            var swapNote = swapColors ? " (swapped)" : "";
            LogWrite($"  Game {game}/{gamesPerMatchup}: Red={redDiff,-12}{swapNote} Blue={blueDiff,-12}");

            var moveCount = 0;
            var gameStartMs = DateTime.UtcNow.Ticks / 10000;

            var result = engine.RunGame(
                redDifficulty: redDiff,
                blueDifficulty: blueDiff,
                maxMoves: 361,
                initialTimeSeconds: initialTimeSeconds,
                incrementSeconds: incrementSeconds,
                ponderingEnabled: true,
                parallelSearchEnabled: true,
                swapColors: swapColors,  // Use swapColors parameter to control color assignment
                onMove: (x, y, player, moveNumber, redTimeMs, blueTimeMs, stats) =>
                {
                    // Determine actual difficulty based on which bot is playing which color
                    // When swapped: Red slot has Braindead, Blue slot has Grandmaster
                    var actualDiff = swapColors
                        ? (player == Player.Red ? AIDifficulty.Braindead : AIDifficulty.Grandmaster)
                        : (player == Player.Red ? AIDifficulty.Grandmaster : AIDifficulty.Braindead);
                    LogWrite(GameStatsFormatter.FormatMoveLine(game, moveNumber, x, y, player, actualDiff, stats));
                    moveCount = moveNumber;
                },
                onLog: (level, source, message) =>
                {
                    if (level == "warn" || level == "error")
                    {
                        LogWrite($"    [{level.ToUpper()}] {source}: {message}");
                    }
                });

            var gameDurationMs = DateTime.UtcNow.Ticks / 10000 - gameStartMs;
            totalMoves += moveCount;
            totalTimeMs += gameDurationMs;

            if (result.IsDraw)
            {
                draws++;
                LogWrite(GameStatsFormatter.FormatGameResult(game, AIDifficulty.Easy, moveCount, gameDurationMs / 1000.0, isDraw: true));
            }
            else
            {
                var winner = result.Winner;
                // Determine actual winning difficulty based on swapColors
                var winningDiff = swapColors
                    ? (winner == Player.Red ? AIDifficulty.Braindead : AIDifficulty.Grandmaster)
                    : (winner == Player.Red ? AIDifficulty.Grandmaster : AIDifficulty.Braindead);

                if (winner == Player.Red)
                {
                    if (swapColors)
                        blueWins++;
                    else
                        redWins++;
                }
                else
                {
                    if (swapColors)
                        redWins++;
                    else
                        blueWins++;
                }

                // Track wins by difficulty (not just color)
                if (winningDiff == AIDifficulty.Grandmaster)
                    grandmasterWins++;
                else
                    braindeadWins++;

                LogWrite(GameStatsFormatter.FormatGameResult(game, winningDiff, moveCount, gameDurationMs / 1000.0, winnerColor: winner));
            }

            LogWrite();
        }

        var total = redWins + blueWins + draws;
        var gmWinRate = (double)grandmasterWins / total;

        LogWrite("  ───────────────────────────────────────────────────────────────────");
        LogWrite($"  SUMMARY: Grandmaster {grandmasterWins} - {braindeadWins} Braindead - {draws} draws");
        LogWrite($"  Grandmaster win rate: {gmWinRate:P1}");
        LogWrite($"  Avg moves: {(double)totalMoves / total:F1} | Avg time: {totalTimeMs / total / 1000:F1}s/game");
        LogWrite();
        LogWrite("═══════════════════════════════════════════════════════════════════");
        LogWrite("  BASELINE COMPLETE");
        LogWrite("═══════════════════════════════════════════════════════════════════");
    }
}
