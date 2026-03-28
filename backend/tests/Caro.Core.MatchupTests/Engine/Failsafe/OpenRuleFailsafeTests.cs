using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.MatchupTests.Helpers;
using Caro.Core.Tournament;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.MatchupTests.Engine.Failsafe;

[Trait("Category", "Failsafe")]
public class OpenRuleFailsafeTests : IDisposable
{
    private readonly ITestOutputHelper _output;
    private readonly TournamentEngine _engine;

    public OpenRuleFailsafeTests(ITestOutputHelper output)
    {
        _output = output;
        _engine = AITestHelper.CreateNonDeterministicEngine();
    }

    public void Dispose() => GC.SuppressFinalize(this);

    [Fact]
    public void OpenRuleEnforced_AllDifficulties()
    {
        var difficulties = new[]
        {
            AIDifficulty.Braindead, AIDifficulty.Easy, AIDifficulty.Medium,
            AIDifficulty.Hard, AIDifficulty.Grandmaster
        };

        foreach (var diff in difficulties)
        {
            (int firstX, int firstY)? firstRed = null;
            var openRuleViolated = false;

            var result = _engine.RunGame(
                diff, diff,
                maxMoves: 200,
                initialTimeSeconds: 120,
                incrementSeconds: 2,
                onMove: (x, y, player, moveNum, _, _, _) =>
                {
                    if (player == Player.Red && moveNum == 1)
                    {
                        firstRed = (x, y);
                    }
                    else if (player == Player.Red && moveNum == 3 && firstRed.HasValue)
                    {
                        int dx = Math.Abs(x - firstRed.Value.firstX);
                        int dy = Math.Abs(y - firstRed.Value.firstY);

                        // Open rule: dx >= 3 OR dy >= 3
                        if (dx < 3 && dy < 3)
                        {
                            openRuleViolated = true;
                            _output.WriteLine(
                                $"  VIOLATION [{diff}]: Red's 2nd move at ({x},{y}) too close to 1st at " +
                                $"({firstRed.Value.firstX},{firstRed.Value.firstY}), dx={dx}, dy={dy}");
                        }
                        else
                        {
                            _output.WriteLine(
                                $"  OK [{diff}]: Red's 2nd move at ({x},{y}) satisfies open rule " +
                                $"(1st at {firstRed.Value.firstX},{firstRed.Value.firstY}, dx={dx}, dy={dy})");
                        }
                    }
                });

            Assert.False(openRuleViolated,
                $"[{diff}] Open rule violated: Red's 2nd move was within exclusion zone of 1st move. " +
                $"Game completed in {result.TotalMoves} moves.");
        }
    }

    [Fact]
    public void OpenRule_SyntheticValidation()
    {
        var validator = new OpenRuleValidator();

        // Place two stones (Blue first, Red second), then validate Red's second move
        // Board has 2 stones: move #1 Red at (7,7), move #2 Blue at (8,8)
        // Red's second move (move #3) must satisfy open rule
        var board = new Board()
            .PlaceStone(7, 7, Player.Red)
            .PlaceStone(8, 8, Player.Blue);

        // Too close: dx < 3 AND dy < 3
        Assert.False(validator.IsValidSecondMove(board, 8, 7),
            "Move at (8,7) should violate open rule (dx=1, dy=0)");
        Assert.False(validator.IsValidSecondMove(board, 9, 9),
            "Move at (9,9) should violate open rule (dx=2, dy=2)");

        // Valid: dx >= 3 OR dy >= 3
        Assert.True(validator.IsValidSecondMove(board, 10, 7),
            "Move at (10,7) should satisfy open rule (dx=3, dy=0)");
        Assert.True(validator.IsValidSecondMove(board, 7, 10),
            "Move at (7,10) should satisfy open rule (dx=0, dy=3)");
        Assert.True(validator.IsValidSecondMove(board, 4, 4),
            "Move at (4,4) should satisfy open rule (dx=3, dy=3)");
        Assert.True(validator.IsValidSecondMove(board, 0, 0),
            "Move at (0,0) should satisfy open rule (dx=7, dy=7)");

        _output.WriteLine("  OK: All synthetic open rule validations passed");
    }
}
