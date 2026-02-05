using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using FluentAssertions;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.Tests.GameLogic;

[Trait("Category", "Debug")]
public class ThreatDetectorDebugTest
{
    private readonly ITestOutputHelper _output;

    public ThreatDetectorDebugTest(ITestOutputHelper output)
    {
        _output = output;
    }

    [Fact]
    public void ThreatDetector_DetectsStraightFour()
    {
        var board = new Board();
        var detector = new ThreatDetector();

        // Blue has four in a row vertically at (3,4), (4,4), (5,4), (6,4)
        board.PlaceStone(3, 4, Player.Blue);
        board.PlaceStone(4, 4, Player.Blue);
        board.PlaceStone(5, 4, Player.Blue);
        board.PlaceStone(6, 4, Player.Blue);

        var threats = detector.DetectThreats(board, Player.Blue);

        _output.WriteLine($"Found {threats.Count} threats");
        foreach (var t in threats)
        {
            _output.WriteLine($"  {t.Type}: {string.Join(", ", t.StonePositions.Select(s => $"({s.x},{s.y})"))}");
            _output.WriteLine($"    Gain squares: {string.Join(", ", t.GainSquares.Select(s => $"({s.x},{s.y})"))}");
        }

        threats.Where(t => t.Type == ThreatType.StraightFour).Count().Should().BeGreaterThan(0,
            "ThreatDetector should detect straight four");
    }

    [Fact]
    public void MinimaxAI_BlocksStraightFour_Grandmaster()
    {
        var board = new Board();

        // Blue has four in a row vertically at (3,4), (4,4), (5,4), (6,4)
        // Red blocked bottom at (7,4)
        board.PlaceStone(3, 4, Player.Blue);
        board.PlaceStone(4, 4, Player.Blue);
        board.PlaceStone(5, 4, Player.Blue);
        board.PlaceStone(6, 4, Player.Blue);
        board.PlaceStone(7, 4, Player.Red);

        var ai = new MinimaxAI();
        var (x, y) = ai.GetBestMove(board, Player.Red, AIDifficulty.Grandmaster);

        _output.WriteLine($"Grandmaster chose: ({x}, {y})");
        _output.WriteLine($"Expected: (2, 4) to block");

        // Should block at (2, 4)
        x.Should().Be(2, "Grandmaster should block Blue's four in a row");
        y.Should().Be(4, "Grandmaster should block Blue's four in a row");
    }
}
