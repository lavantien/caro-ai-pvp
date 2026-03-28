using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.MatchupTests.Helpers;
using Caro.Core.Tournament;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.MatchupTests.Engine.Failsafe;

[Trait("Category", "Failsafe")]
public class OverlineRuleFailsafeTests : IDisposable
{
    private readonly ITestOutputHelper _output;
    private readonly TournamentEngine _engine;

    public OverlineRuleFailsafeTests(ITestOutputHelper output)
    {
        _output = output;
        _engine = AITestHelper.CreateNonDeterministicEngine();
    }

    public void Dispose() => GC.SuppressFinalize(this);

    [Fact]
    public void NoOverlineWins_BraindeadVsBraindead()
    {
        var result = RunGameAndCheckNoOverline(AIDifficulty.Braindead, AIDifficulty.Braindead);
        Assert.True(result.noOverlineDeclared, result.message);
    }

    [Fact]
    public void NoOverlineWins_HardVsMedium()
    {
        var result = RunGameAndCheckNoOverline(AIDifficulty.Hard, AIDifficulty.Medium);
        Assert.True(result.noOverlineDeclared, result.message);
    }

    [Fact]
    public void NoOverlineWins_GrandmasterVsBraindead()
    {
        var result = RunGameAndCheckNoOverline(AIDifficulty.Grandmaster, AIDifficulty.Braindead);
        Assert.True(result.noOverlineDeclared, result.message);
    }

    private (bool noOverlineDeclared, string message) RunGameAndCheckNoOverline(
        AIDifficulty redDiff, AIDifficulty blueDiff)
    {
        var gameResult = _engine.RunGame(
            redDiff, blueDiff,
            maxMoves: 256,
            initialTimeSeconds: 120,
            incrementSeconds: 2);

        var board = gameResult.FinalBoard;

        // Scan for overlines (6+ consecutive same-color stones)
        var overlines = ScanForOverlines(board);

        if (overlines.Count > 0 && !gameResult.IsDraw && gameResult.Winner != Player.None)
        {
            var overlineStr = string.Join("; ", overlines);
            var msg = $"Overline detected on final board but game declared {gameResult.Winner} as winner. " +
                      $"Overlines: {overlineStr}. Matchup: {redDiff} vs {blueDiff}";
            _output.WriteLine($"  FAIL: {msg}");
            return (false, msg);
        }

        _output.WriteLine($"  OK: {redDiff} vs {blueDiff} - Winner: {gameResult.Winner}, " +
                          $"Overlines found: {overlines.Count}, Moves: {gameResult.TotalMoves}");
        return (true, "");
    }

    private static List<string> ScanForOverlines(Board board)
    {
        var overlines = new List<string>();
        var directions = new (int dx, int dy)[] { (1, 0), (0, 1), (1, 1), (1, -1) };

        for (int x = 0; x < board.BoardSize; x++)
        {
            for (int y = 0; y < board.BoardSize; y++)
            {
                var cell = board.GetCell(x, y);
                if (cell.IsEmpty) continue;

                foreach (var (dx, dy) in directions)
                {
                    int count = 1;
                    int nx = x + dx, ny = y + dy;
                    while (nx >= 0 && nx < board.BoardSize && ny >= 0 && ny < board.BoardSize &&
                           board.GetCell(nx, ny).Player == cell.Player)
                    {
                        count++;
                        nx += dx;
                        ny += dy;
                    }

                    if (count >= 6)
                    {
                        overlines.Add($"{cell.Player} has {count}-in-a-row at ({x},{y}) dir=({dx},{dy})");
                    }
                }
            }
        }

        return overlines;
    }
}
