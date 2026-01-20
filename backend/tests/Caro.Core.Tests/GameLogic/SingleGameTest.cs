using Caro.Core.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Tournament;
using FluentAssertions;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.Tests.GameLogic;

[Trait("Category", "Verification")]
public class SingleGameTest
{
    private readonly ITestOutputHelper _output;

    public SingleGameTest(ITestOutputHelper output)
    {
        _output = output;
    }

    [Fact]
    public void SingleGame_Grandmaster_vs_Easy_7Plus5_ShouldNotLose()
    {
        var engine = new TournamentEngine();

        var result = engine.RunGame(
            AIDifficulty.Grandmaster,  // Red: D10
            AIDifficulty.Easy,          // Blue: D2
            maxMoves: 50,
            initialTimeSeconds: 420,    // 7+5 time control (standard)
            incrementSeconds: 5,
            ponderingEnabled: false
        );

        _output.WriteLine($"Winner: {result.Winner} ({result.WinnerDifficulty})");
        _output.WriteLine($"Total Moves: {result.TotalMoves}");
        _output.WriteLine($"Duration: {result.DurationMs / 1000.0:F1}s");

        // Grandmaster should not lose to Easy
        if (result.Winner == Player.Blue && result.WinnerDifficulty == AIDifficulty.Easy)
        {
            _output.WriteLine("\n*** FAILED: Grandmaster lost to Easy! ***");
            Assert.Fail("Grandmaster should never lose to Easy AI");
        }
    }
}
