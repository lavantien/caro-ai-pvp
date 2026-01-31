using Caro.Core.Entities;
using Caro.Core.GameLogic;

// Analyze Game 1 to understand the actual threat
var board = new Board();

// Trace the moves
Console.WriteLine("Tracing Game 1 moves:");
board.PlaceStone(7, 7, Player.Red);      // M1: Braindead
Console.WriteLine("M1: R(7,7)");

board.PlaceStone(7, 8, Player.Blue);     // M2: Grandmaster
Console.WriteLine("M2: B(7,8)");

board.PlaceStone(7, 10, Player.Red);    // M3: Braindead
Console.WriteLine("M3: R(7,10)");

board.PlaceStone(6, 8, Player.Blue);     // M4: Grandmaster
Console.WriteLine("M4: B(6,8)");

board.PlaceStone(8, 8, Player.Red);      // M5: Braindead
Console.WriteLine("M5: R(8,8)");

board.PlaceStone(6, 6, Player.Blue);     // M6: Grandmaster (blocking attempt 1)
Console.WriteLine("M6: B(6,6)");

board.PlaceStone(6, 7, Player.Red);      // M7: Braindead
Console.WriteLine("M7: R(6,7)");

board.PlaceStone(9, 9, Player.Blue);     // M8: Grandmaster (blocking attempt 2)
Console.WriteLine("M8: B(9,9)");

board.PlaceStone(8, 7, Player.Red);      // M9: Braindead
Console.WriteLine("M9: R(8,7)");

board.PlaceStone(7, 4, Player.Blue);     // M10: Grandmaster
Console.WriteLine("M10: B(7,4)");

board.PlaceStone(9, 7, Player.Red);      // M11: Braindead
Console.WriteLine("M11: R(9,7)");

Console.WriteLine("\nAfter M11, checking for threats...");

var threatDetector = new ThreatDetector();
var redThreats = threatDetector.DetectThreats(board, Player.Red);
Console.WriteLine($"Red threats: {redThreats.Count}");
foreach (var t in redThreats)
{
    Console.WriteLine($"  Type={t.Type}, Direction={t.Direction}");
    Console.WriteLine($"  Stone positions: {string.Join(", ", t.StonePositions.Select(p => $"({p.x},{p.y}"))}");
    Console.WriteLine($"  GainSquares: {string.Join(", ", t.GainSquares.Select(g => $"({g.x},{g.y})"))}");
}

Console.WriteLine("\nWinDetector check:");
var winDetector = new WinDetector();
var result = winDetector.CheckWin(board);
Console.WriteLine($"HasWinner: {result.HasWinner}, Winner: {result.Winner}");

// Check what happens at each winning square
Console.WriteLine("\nChecking potential winning moves:");
foreach (var (x, y) in new[] { (5,7), (10,7), (5,5), (10,10) })
{
    if (board.GetCell(x, y).IsEmpty)
    {
        board.PlaceStone(x, y, Player.Red);
        var winResult = winDetector.CheckWin(board);
        board.GetCell(x, y).Player = Player.None;
        Console.WriteLine($"  ({x},{y}): IsWin={winResult.HasWinner}, Winner={winResult.Winner}");
    }
}
