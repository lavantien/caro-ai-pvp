using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.MatchupTests.Helpers;
using Caro.Core.Tournament;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.MatchupTests.Engine.Failsafe;

[Trait("Category", "Failsafe")]
public class DrawDetectionFailsafeTests : IDisposable
{
    private readonly ITestOutputHelper _output;
    private readonly TournamentEngine _engine;

    public DrawDetectionFailsafeTests(ITestOutputHelper output)
    {
        _output = output;
        _engine = AITestHelper.CreateNonDeterministicEngine();
    }

    public void Dispose() => GC.SuppressFinalize(this);

    [Fact]
    public void BoardFullGame_EndsInDraw_NotIllegalMove()
    {
        var result = _engine.RunGame(
            AIDifficulty.Medium, AIDifficulty.Medium,
            maxMoves: 256,
            initialTimeSeconds: 120,
            incrementSeconds: 2);

        _output.WriteLine($"  Moves: {result.TotalMoves}, Winner: {result.Winner}, Draw: {result.IsDraw}");

        var resultIsCoherent = result.IsDraw
            ? result.Winner == Player.None
            : result.Winner != Player.None;
        Assert.True(resultIsCoherent,
            $"Incoherent result: IsDraw={result.IsDraw}, Winner={result.Winner}, TotalMoves={result.TotalMoves}. " +
            "If draw, should have Winner=None. If win, should have Winner=Red or Blue.");

        if (result.TotalMoves >= 250)
        {
            Assert.True(result.IsDraw,
                $"Game reached {result.TotalMoves} moves on 16x16 board but was NOT detected as draw. " +
                $"Winner: {result.Winner}");
            Assert.Equal(Player.None, result.Winner);
        }
    }
}
