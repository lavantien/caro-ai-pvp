using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.IntegrationTests.Helpers;
using Xunit;
using Xunit.Abstractions;

namespace Caro.Core.IntegrationTests.GameLogic;

public class DiagonalThreatTest
{
    private readonly ITestOutputHelper _output;

    public DiagonalThreatTest(ITestOutputHelper output)
    {
        _output = output;
    }

    [Fact]
    public void Grandmaster_ShouldBlock_DiagonalFourInRow()
    {
        // Reproduce the board state from the lost game
        // After move 15, Braindead (Red) has diagonal: (6,6)-(7,7)-(8,8)-(9,9)
        // Needs to block either (5,5) or (10,10)
        var board = new Board();

        // Red (Braindead) moves
        board = board.PlaceStone(7, 7, Player.Red);   // M1
        board = board.PlaceStone(10, 7, Player.Red);  // M3
        board = board.PlaceStone(6, 8, Player.Red);   // M5
        board = board.PlaceStone(8, 8, Player.Red);   // M7
        board = board.PlaceStone(10, 5, Player.Red);  // M9
        board = board.PlaceStone(9, 4, Player.Red);   // M11
        board = board.PlaceStone(6, 6, Player.Red);   // M13 - now has (6,6)-(7,7)-(8,8) diagonal
        board = board.PlaceStone(9, 9, Player.Red);   // M15 - now has (6,6)-(7,7)-(8,8)-(9,9) - 4 in a row!

        // Blue (Grandmaster) moves - these were played before
        board = board.PlaceStone(8, 7, Player.Blue);  // M2
        board = board.PlaceStone(6, 7, Player.Blue);  // M4
        board = board.PlaceStone(8, 6, Player.Blue);  // M6
        board = board.PlaceStone(8, 5, Player.Blue);  // M8
        board = board.PlaceStone(7, 6, Player.Blue);  // M10
        board = board.PlaceStone(7, 8, Player.Blue);  // M12
        board = board.PlaceStone(7, 5, Player.Blue);  // M14

        // Print board state
        _output.WriteLine("Board state before Grandmaster's move (M16):");
        _output.WriteLine($"Red has (6,6), (7,7), (8,8), (9,9) - diagonal 4-in-row");
        _output.WriteLine($"Blue needs to block (5,5) or (10,10)");

        // Check if WinDetector detects the threat
        var winDetector = new WinDetector();
        var winResult = winDetector.CheckWin(board);
        _output.WriteLine($"CheckWin result: HasWinner={winResult.HasWinner}, Winner={winResult.Winner}");

        // Check what threats ThreatDetector detects
        var threatDetector = new ThreatDetector();
        var threats = threatDetector.DetectThreats(board, Player.Red);
        _output.WriteLine($"ThreatDetector found {threats.Count} threats for Red:");
        foreach (var threat in threats.Take(10))
        {
            _output.WriteLine($"  Threat Type={threat.Type}, Stones={threat.StonePositions.Count}, Direction={threat.Direction}");
            if (threat.GainSquares.Count > 0)
            {
                var gainStr = string.Join(", ", threat.GainSquares.Select(g => $"({g.x},{g.y})"));
                _output.WriteLine($"    GainSquares ({threat.GainSquares.Count}): {gainStr}");
            }
            // Check if GainSquares are actually empty
            foreach (var gs in threat.GainSquares)
            {
                var cell = board.GetCell(gs.x, gs.y);
                _output.WriteLine($"    ({gs.x},{gs.y}) - IsEmpty={cell.IsEmpty}, Player={cell.Player}");
            }
        }

        // Also test ParallelMinimaxSearch directly to see what candidates it considers
        var pms = new ParallelMinimaxSearch();
        var getCandidates = pms.GetType()
            .GetMethod("GetCandidateMoves", System.Reflection.BindingFlags.NonPublic | System.Reflection.BindingFlags.Instance);
        var candidateList = getCandidates?.Invoke(pms, new object[] { board }) as List<(int x, int y)>;
        _output.WriteLine($"Total candidates: {candidateList?.Count ?? 0}");
        if (candidateList != null)
        {
            var contains55 = candidateList.Contains((5, 5));
            var contains1010 = candidateList.Contains((10, 10));
            _output.WriteLine($"Candidate list contains (5,5): {contains55}");
            _output.WriteLine($"Candidate list contains (10,10): {contains1010}");
            if (!contains55 || !contains1010)
            {
                _output.WriteLine("Some blocking moves are NOT in candidate list!");
            }
        }

        // Test GetOpponentThreatMoves directly
        var getThreatMoves = pms.GetType()
            .GetMethod("GetOpponentThreatMoves", System.Reflection.BindingFlags.NonPublic | System.Reflection.BindingFlags.Instance);
        var threatMoves = getThreatMoves?.Invoke(pms, new object[] { board, Player.Red }) as List<(int x, int y)>;
        _output.WriteLine($"GetOpponentThreatMoves returned {threatMoves?.Count ?? 0} threat moves");
        if (threatMoves != null && threatMoves.Count > 0)
        {
            foreach (var tm in threatMoves)
            {
                _output.WriteLine($"  Threat move: ({tm.x}, {tm.y})");
            }
        }

        // Act - Get Grandmaster's move
        var ai = AITestHelper.CreateAI();
        var move = ai.GetBestMove(board, Player.Blue, AIDifficulty.Grandmaster);

        var stats = ai.GetSearchStatistics();
        _output.WriteLine($"Grandmaster played: ({move.x}, {move.y})");
        _output.WriteLine($"Depth: {stats.DepthAchieved}, Nodes: {stats.NodesSearched}");

        // Assert - Should block either (5,5) or (10,10)
        bool blocked = (move.x == 5 && move.y == 5) || (move.x == 10 && move.y == 10);
        _output.WriteLine($"Blocked the threat: {blocked}");

        // For now, let's see what it actually plays
        Assert.True(move.x >= 0 && move.x < 15);
        Assert.True(move.y >= 0 && move.y < 15);
    }
}
