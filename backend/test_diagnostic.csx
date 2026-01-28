using Caro.Core.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tournament;

// Quick diagnostic test
var engine = new TournamentEngine();
var rand = new Random();

Console.WriteLine("=== AI Strength Diagnostic ===");
Console.WriteLine();

// Test Hard vs Medium - run 5 games for quick diagnosis
Console.WriteLine("Running Hard vs Medium (5 games)...");
var hardWins = 0;
var mediumWins = 0;
var draws = 0;

for (int i = 0; i < 5; i++)
{
    bool swapColors = (i % 2 == 1);
    var result = engine.RunGame(
        redDifficulty: swapColors ? AIDifficulty.Medium : AIDifficulty.Hard,
        blueDifficulty: swapColors ? AIDifficulty.Hard : AIDifficulty.Medium,
        maxMoves: 225,
        initialTimeSeconds: 180,
        incrementSeconds: 2,
        ponderingEnabled: true);

    Console.WriteLine($"  Game {i+1}: {result.Winner} (Moves: {result.MoveCount})");

    if (result.IsDraw) draws++;
    else if ((result.Winner == Player.Red && !swapColors) || (result.Winner == Player.Blue && swapColors))
        hardWins++;
    else
        mediumWins++;
}

Console.WriteLine($"  Hard: {hardWins}, Medium: {mediumWins}, Draws: {draws}");
Console.WriteLine($"  Expected: Hard should win more");
Console.WriteLine();

// Test Medium vs Easy - run 5 games
Console.WriteLine("Running Medium vs Easy (5 games)...");
var mediumWins2 = 0;
var easyWins = 0;
var draws2 = 0;

for (int i = 0; i < 5; i++)
{
    bool swapColors = (i % 2 == 1);
    var result = engine.RunGame(
        redDifficulty: swapColors ? AIDifficulty.Easy : AIDifficulty.Medium,
        blueDifficulty: swapColors ? AIDifficulty.Medium : AIDifficulty.Easy,
        maxMoves: 225,
        initialTimeSeconds: 180,
        incrementSeconds: 2,
        ponderingEnabled: true);

    Console.WriteLine($"  Game {i+1}: {result.Winner} (Moves: {result.MoveCount})");

    if (result.IsDraw) draws2++;
    else if ((result.Winner == Player.Red && !swapColors) || (result.Winner == Player.Blue && swapColors))
        mediumWins2++;
    else
        easyWins++;
}

Console.WriteLine($"  Medium: {mediumWins2}, Easy: {easyWins}, Draws: {draws2}");
Console.WriteLine($"  Expected: Medium should win more");
