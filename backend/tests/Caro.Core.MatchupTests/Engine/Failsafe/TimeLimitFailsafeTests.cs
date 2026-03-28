using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.MatchupTests.Helpers;
using Caro.Core.Tournament;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.MatchupTests.Engine.Failsafe;

[Trait("Category", "Failsafe")]
public class TimeLimitFailsafeTests : IDisposable
{
    private readonly ITestOutputHelper _output;
    private readonly TournamentEngine _engine;

    private static readonly AIDifficulty[] AllDifficulties =
    {
        AIDifficulty.Braindead, AIDifficulty.Easy, AIDifficulty.Medium,
        AIDifficulty.Hard, AIDifficulty.Grandmaster
    };

    public TimeLimitFailsafeTests(ITestOutputHelper output)
    {
        _output = output;
        _engine = AITestHelper.CreateNonDeterministicEngine();
    }

    public void Dispose() => GC.SuppressFinalize(this);

    [Fact]
    public void Braindead_MovesWithinTimeBudget()
    {
        var initialTimeSeconds = 30;
        var initialTimeMs = initialTimeSeconds * 1000L;

        var result = _engine.RunGame(
            AIDifficulty.Braindead, AIDifficulty.Braindead,
            maxMoves: 100,
            initialTimeSeconds: initialTimeSeconds,
            incrementSeconds: 1,
            onMove: (x, y, player, moveNum, redTimeMs, blueTimeMs, stats) =>
            {
                // Braindead should never come close to using all its time
                // TimeMultiplier = 0.05, so it uses ~5% of allocated time per move
                var remainingMs = player == Player.Red ? redTimeMs : blueTimeMs;
                Assert.True(remainingMs > 0,
                    $"[Braindead] {player} ran out of time at move #{moveNum}. Remaining: {remainingMs}ms");

                if (moveNum <= 5)
                {
                    _output.WriteLine($"  Move #{moveNum} {player}: remaining {remainingMs}ms");
                }
            });

        _output.WriteLine($"  OK: Braindead game completed in {result.TotalMoves} moves. EndedByTimeout: {result.EndedByTimeout}");
    }

    [Fact]
    public void Grandmaster_MovesWithinTimeBudget()
    {
        var initialTimeSeconds = 120;

        var result = _engine.RunGame(
            AIDifficulty.Grandmaster, AIDifficulty.Grandmaster,
            maxMoves: 200,
            initialTimeSeconds: initialTimeSeconds,
            incrementSeconds: 5,
            onMove: (x, y, player, moveNum, redTimeMs, blueTimeMs, stats) =>
            {
                var remainingMs = player == Player.Red ? redTimeMs : blueTimeMs;
                Assert.True(remainingMs > 0,
                    $"[Grandmaster] {player} ran out of time at move #{moveNum}. Remaining: {remainingMs}ms");
            });

        _output.WriteLine($"  OK: Grandmaster game completed in {result.TotalMoves} moves. EndedByTimeout: {result.EndedByTimeout}");
    }

    [Fact]
    public void ShortTimeControl_GameCompletes()
    {
        var result = _engine.RunGame(
            AIDifficulty.Easy, AIDifficulty.Easy,
            maxMoves: 256,
            initialTimeSeconds: 10,
            incrementSeconds: 0);

        // Game should complete (either by win, draw, or timeout - all valid)
        Assert.True(result.Winner != Player.None || result.IsDraw,
            $"Game did not produce a valid result. Winner: {result.Winner}, IsDraw: {result.IsDraw}");

        _output.WriteLine($"  OK: Short time control game - Winner: {result.Winner}, " +
                          $"Moves: {result.TotalMoves}, Timeout: {result.EndedByTimeout}");
    }
}
