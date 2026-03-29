using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.MatchupTests.Helpers;
using Caro.Core.Tournament;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.MatchupTests.Engine.Failsafe;

[Trait("Category", "Failsafe")]
public class BraindeadErrorRateFailsafeTests : IDisposable
{
    private readonly ITestOutputHelper _output;
    private readonly TournamentEngine _engine;

    public BraindeadErrorRateFailsafeTests(ITestOutputHelper output)
    {
        _output = output;
        _engine = AITestHelper.CreateNonDeterministicEngine();
    }

    public void Dispose() => GC.SuppressFinalize(this);

    [Fact]
    public void Braindead_ErrorRateOccurs()
    {
        var errorMoves = 0;
        var totalMoves = 0;

        for (int game = 0; game < 5; game++)
        {
            var result = _engine.RunGame(
                AIDifficulty.Braindead, AIDifficulty.Braindead,
                maxMoves: 200,
                initialTimeSeconds: 120,
                incrementSeconds: 2,
                onMove: (x, y, player, moveNum, _, _, stats) =>
                {
                    totalMoves++;
                    if (stats?.MoveType == MoveType.ErrorRate)
                    {
                        errorMoves++;
                    }
                });

            _output.WriteLine($"  Game {game + 1}: {result.TotalMoves} moves, Winner: {result.Winner}");
        }

        // Braindead has 10% error rate, so over 5 games (~500+ moves total)
        // we should see at least 1 error move
        Assert.True(errorMoves >= 1,
            $"Expected at least 1 ErrorRate move across 5 Braindead games, got {errorMoves}/{totalMoves}");

        _output.WriteLine($"  OK: {errorMoves} error moves out of {totalMoves} total " +
                          $"({(double)errorMoves / totalMoves * 100:F1}%)");
    }

    [Fact]
    public void Grandmaster_NoErrorRate()
    {
        var errorMoves = 0;

        var result = _engine.RunGame(
            AIDifficulty.Grandmaster, AIDifficulty.Grandmaster,
            maxMoves: 200,
            initialTimeSeconds: 120,
            incrementSeconds: 2,
            onMove: (x, y, player, moveNum, _, _, stats) =>
            {
                if (stats?.MoveType == MoveType.ErrorRate)
                {
                    errorMoves++;
                    _output.WriteLine($"  UNEXPECTED: Grandmaster made ErrorRate move #{moveNum}");
                }
            });

        Assert.Equal(0, errorMoves);

        _output.WriteLine($"  OK: Grandmaster game completed with 0 error moves in {result.TotalMoves} moves");
    }

    [Fact]
    public void Braindead_ErrorRateAtLeast10Percent()
    {
        var settings = AIDifficultyConfig.Instance.GetSettings(AIDifficulty.Braindead);
        Assert.True(settings.ErrorRate >= 0.10,
            $"Braindead error rate should be at least 10%, got {settings.ErrorRate}");
        _output.WriteLine($"  OK: Braindead error rate is {settings.ErrorRate:P0}%");
    }
}
