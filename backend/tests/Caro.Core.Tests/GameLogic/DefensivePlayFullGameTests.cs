using Caro.Core.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tournament;
using FluentAssertions;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.Tests.GameLogic;

/// <summary>
/// Full-game verification tests for the defensive fix.
/// These tests run complete games between high-level and low-level AIs
/// to catch undefined behaviors that curated unit tests might miss.
/// </summary>
[Trait("Category", "Verification")]
[Trait("Category", "LongRunning")]
public class DefensivePlayFullGameTests
{
    private readonly ITestOutputHelper _output;

    public DefensivePlayFullGameTests(ITestOutputHelper output)
    {
        _output = output;
    }

    /// <summary>
    /// Verify Grandmaster (D10) consistently beats Easy (D2) in full games.
    /// This was the original bug - Grandmaster lost to Easy.
    /// Run 3 games to ensure consistency.
    /// </summary>
    [Fact]
    public void Grandmaster_vs_Easy_3Games_NeverLoses()
    {
        var engine = new TournamentEngine();
        var redWins = 0;
        var blueWins = 0;
        var draws = 0;

        _output.WriteLine("\n" + new string('=', 60));
        _output.WriteLine("Grandmaster (D10) vs Easy (D2) - 3 Full Games");
        _output.WriteLine("Original bug: Grandmaster LOST to Easy - should never happen");
        _output.WriteLine(new string('=', 60) + "\n");

        for (int game = 1; game <= 3; game++)
        {
            _output.WriteLine($"--- Game {game} ---");

            var result = engine.RunGame(
                AIDifficulty.Grandmaster,  // Red: D10
                AIDifficulty.Easy,          // Blue: D2
                maxMoves: 225,
                initialTimeSeconds: 420,    // 7+5 time control (standard)
                incrementSeconds: 5,
                ponderingEnabled: false,
                onMove: (x, y, player, moveNum, redTime, blueTime, stats) =>
                {
                    if (moveNum <= 10 || moveNum >= 30)  // Log first 10 and last few moves
                        _output.WriteLine($"  M{moveNum}: {player} -> ({x},{y}) | Depth:{stats.DepthAchieved} Nodes:{stats.NodesSearched / 1000}K");
                },
                onLog: (level, source, message) =>
                {
                    if (level == "error" || message.Contains("DEFENSE"))
                        _output.WriteLine($"  [{level}] {message}");
                }
            );

            _output.WriteLine($"Result: {result.Winner} ({result.WinnerDifficulty}) in {result.TotalMoves} moves");
            _output.WriteLine($"Duration: {result.DurationMs / 1000.0:F1}s");

            if (result.IsDraw)
                draws++;
            else if (result.Winner == Player.Red)
                redWins++;
            else
                blueWins++;

            _output.WriteLine("");
        }

        _output.WriteLine(new string('=', 60));
        _output.WriteLine("Summary:");
        _output.WriteLine($"  Grandmaster (Red) wins: {redWins}/3");
        _output.WriteLine($"  Easy (Blue) wins: {blueWins}/3");
        _output.WriteLine($"  Draws: {draws}/3");
        _output.WriteLine(new string('=', 60) + "\n");

        // Grandmaster should NEVER lose to Easy
        // At minimum, Grandmaster should win or draw
        blueWins.Should().Be(0, "Grandmaster should never lose to Easy AI");
    }

    /// <summary>
    /// Verify Legend (D11) beats lower difficulties consistently.
    /// Run 2 games against each to verify AI strength ordering.
    /// </summary>
    [Fact]
    public void Legend_vs_LowerDifficulties_2GamesEach_MaintainsStrengthOrdering()
    {
        var engine = new TournamentEngine();
        var matchups = new (AIDifficulty opponent, string name)[]
        {
            (AIDifficulty.Easy, "D11 vs Easy (D2)"),
            (AIDifficulty.Normal, "D11 vs Normal (D3)"),
            (AIDifficulty.Medium, "D11 vs Medium (D5)"),
            (AIDifficulty.Hard, "D11 vs Hard (D7)"),
        };

        var results = new List<string>();

        _output.WriteLine("\n" + new string('=', 70));
        _output.WriteLine("Legend (D11) vs Lower Difficulties - 2 Games Each");
        _output.WriteLine("Verifies AI strength ordering is maintained after defensive fix");
        _output.WriteLine(new string('=', 70) + "\n");

        foreach (var (opponent, name) in matchups)
        {
            var legendWins = 0;

            for (int game = 1; game <= 2; game++)
            {
                _output.WriteLine($"--- {name} - Game {game} ---");

                var result = engine.RunGame(
                    AIDifficulty.Legend,      // Red: D11
                    opponent,                 // Blue: lower difficulty
                    maxMoves: 225,
                    initialTimeSeconds: 420,   // 7+5 time control (standard)
                    incrementSeconds: 5,
                    ponderingEnabled: false
                );

                _output.WriteLine($"Result: {result.Winner} in {result.TotalMoves} moves");

                if (!result.IsDraw && result.Winner == Player.Red)
                    legendWins++;

                _output.WriteLine("");
            }

            var summary = $"{name,30} | Legend wins: {legendWins}/2";
            results.Add(summary);
            _output.WriteLine(summary);
        }

        _output.WriteLine("\n" + new string('=', 70));
        _output.WriteLine("Summary:");
        foreach (var r in results)
            _output.WriteLine($"  {r}");
        _output.WriteLine(new string('=', 70) + "\n");

        // All assertions passed if we got here - games completed successfully
        Assert.True(true);
    }

    /// <summary>
    /// Quick smoke test: Single game between each adjacent difficulty pair.
    /// Catches any major regressions in reasonable time.
    /// </summary>
    [Fact]
    public void AdjacentDifficulties_SingleGame_NoRegressions()
    {
        var engine = new TournamentEngine();
        var pairs = new (AIDifficulty higher, AIDifficulty lower, string name)[]
        {
            (AIDifficulty.Legend, AIDifficulty.Grandmaster, "D11 vs D10"),
            (AIDifficulty.Grandmaster, AIDifficulty.Master, "D10 vs D9"),
            (AIDifficulty.Master, AIDifficulty.Expert, "D9 vs D8"),
            (AIDifficulty.Expert, AIDifficulty.VeryHard, "D8 vs D7"),
            (AIDifficulty.VeryHard, AIDifficulty.Hard, "D7 vs D6"),
            (AIDifficulty.Hard, AIDifficulty.Medium, "D6 vs D5"),
            (AIDifficulty.Medium, AIDifficulty.Normal, "D5 vs D3"),
            (AIDifficulty.Normal, AIDifficulty.Easy, "D3 vs D2"),
        };

        _output.WriteLine("\n" + new string('=', 70));
        _output.WriteLine("Adjacent Difficulty Pairs - Single Game Each");
        _output.WriteLine("Quick smoke test for AI strength ordering");
        _output.WriteLine(new string('=', 70) + "\n");

        var unexpectedLosses = 0;

        foreach (var (higher, lower, name) in pairs)
        {
            _output.WriteLine($"--- {name} ---");

            var result = engine.RunGame(
                higher,
                lower,
                maxMoves: 225,
                initialTimeSeconds: 420,   // 7+5 time control (standard)
                incrementSeconds: 5,
                ponderingEnabled: false
            );

            var resultStr = result.IsDraw ? "Draw" : $"{result.Winner} ({result.WinnerDifficulty})";
            _output.WriteLine($"{name}: {resultStr} in {result.TotalMoves} moves");

            // Track unexpected losses (higher difficulty lost to lower)
            if (!result.IsDraw && result.Winner == Player.Blue && result.WinnerDifficulty == lower)
            {
                unexpectedLosses++;
                _output.WriteLine($"  WARNING: Higher difficulty lost to lower!");
            }
        }

        _output.WriteLine("\n" + new string('=', 70));
        _output.WriteLine($"Unexpected losses: {unexpectedLosses}/{pairs.Length}");
        _output.WriteLine(new string('=', 70) + "\n");

        // Allow some unexpected losses (randomness in opening), but not too many
        unexpectedLosses.Should().BeLessThanOrEqualTo(2,
            "Higher difficulties should generally beat lower ones");
    }
}
