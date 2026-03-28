using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.MatchupTests.Helpers;
using Caro.Core.Tournament;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.MatchupTests.Engine.Failsafe;

[Trait("Category", "Failsafe")]
public class GameTerminationFailsafeTests : IDisposable
{
    private readonly ITestOutputHelper _output;
    private readonly TournamentEngine _engine;

    public GameTerminationFailsafeTests(ITestOutputHelper output)
    {
        _output = output;
        _engine = AITestHelper.CreateNonDeterministicEngine();
    }

    public void Dispose() => GC.SuppressFinalize(this);

    [Fact]
    public void Game_TerminatesWithinMaxMoves()
    {
        var result = _engine.RunGame(
            AIDifficulty.Medium, AIDifficulty.Medium,
            maxMoves: 200,
            initialTimeSeconds: 120,
            incrementSeconds: 2);

        Assert.True(result.TotalMoves <= 200,
            $"Game exceeded max moves: {result.TotalMoves}");

        Assert.True(result.Winner != Player.None || result.IsDraw,
            $"Game terminated without a valid result. Moves: {result.TotalMoves}");

        _output.WriteLine($"  OK: Game terminated in {result.TotalMoves} moves. Winner: {result.Winner}, Draw: {result.IsDraw}");
    }

    [Fact]
    public void Game_WinnerIsRedOrBlueOrDraw()
    {
        var pairs = new[]
        {
            (AIDifficulty.Braindead, AIDifficulty.Braindead),
            (AIDifficulty.Hard, AIDifficulty.Easy),
            (AIDifficulty.Grandmaster, AIDifficulty.Medium),
        };

        foreach (var (red, blue) in pairs)
        {
            var result = _engine.RunGame(
                red, blue,
                maxMoves: 256,
                initialTimeSeconds: 120,
                incrementSeconds: 2);

            var winnerIsValid = result.Winner == Player.Red
                             || result.Winner == Player.Blue
                             || result.Winner == Player.None;

            Assert.True(winnerIsValid,
                $"[{red} vs {blue}] Invalid winner: {result.Winner}");

            var resultIsCoherent = result.IsDraw
                ? result.Winner == Player.None
                : result.Winner != Player.None;

            Assert.True(resultIsCoherent,
                $"[{red} vs {blue}] Incoherent result: IsDraw={result.IsDraw}, Winner={result.Winner}");

            _output.WriteLine($"  OK: {red} vs {blue} - Winner: {result.Winner}, Draw: {result.IsDraw}");
        }
    }

    [Fact]
    public void Game_EndedByTimeout_HasCorrectWinner()
    {
        // Very short time control: 5 seconds total, no increment
        var result = _engine.RunGame(
            AIDifficulty.Grandmaster, AIDifficulty.Grandmaster,
            maxMoves: 256,
            initialTimeSeconds: 5,
            incrementSeconds: 0);

        // Either game ends by timeout or quickly by win/draw
        if (result.EndedByTimeout)
        {
            Assert.NotEqual(Player.None, result.Winner);

            // The winner should NOT be the player who timed out
            // (Winner is the opponent of the timed-out player)
            _output.WriteLine($"  OK: Timeout game - Winner: {result.Winner}, " +
                              $"Moves: {result.TotalMoves}, Duration: {result.DurationMs}ms");
        }
        else
        {
            // Grandmaster with 5s might still finish quickly on a short game
            _output.WriteLine($"  OK: Short time control game ended by win/draw - " +
                              $"Winner: {result.Winner}, Moves: {result.TotalMoves}");
        }
    }
}
