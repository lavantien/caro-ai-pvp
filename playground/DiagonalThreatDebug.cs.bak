using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

// Debug script to check if ThreatDetector detects the diagonal threat
var board = new Board();

// Red (Braindead) moves
board.PlaceStone(7, 7, Player.Red);
board.PlaceStone(10, 7, Player.Red);
board.PlaceStone(6, 8, Player.Red);
board.PlaceStone(8, 8, Player.Red);
board.PlaceStone(10, 5, Player.Red);
board.PlaceStone(9, 4, Player.Red);
board.PlaceStone(6, 6, Player.Red);   // (6,6)-(7,7)-(8,8) diagonal
board.PlaceStone(9, 9, Player.Red);   // (6,6)-(7,7)-(8,8)-(9,9) - 4 in a row!

// Blue moves
board.PlaceStone(8, 7, Player.Blue);
board.PlaceStone(6, 7, Player.Blue);
board.PlaceStone(8, 6, Player.Blue);
board.PlaceStone(8, 5, Player.Blue);
board.PlaceStone(7, 6, Player.Blue);
board.PlaceStone(7, 8, Player.Blue);
board.PlaceStone(7, 5, Player.Blue);

Console.WriteLine("Checking threats...");
var threatDetector = new ThreatDetector();
var threats = threatDetector.DetectThreats(board, Player.Red);

Console.WriteLine($"Found {threats.Count} threats for Red:");
foreach (var threat in threats)
{
    Console.WriteLine($"  Threat at ({threat.X},{threat.Y}): Type={threat.Type}, Count={threat.StoneCount}, GainSquares={threat.GainSquares.Count}");
    if (threat.GainSquares.Count > 0)
    {
        Console.WriteLine($"    Gain squares: {string.Join(", ", threat.GainSquares.Select(g => $"({g.x},{g.y})"))}");
    }
}

// Check if (5,5) and (10,10) are detected as gain squares
Console.WriteLine($"\nCritical positions:");
Console.WriteLine($"  (5,5) is empty: {board.GetCell(5, 5).IsEmpty}");
Console.WriteLine($"  (10,10) is empty: {board.GetCell(10, 10).IsEmpty}");

// Check specific cells
var cell55 = board.GetCell(5, 5);
var cell1010 = board.GetCell(10, 10);
Console.WriteLine($"  (5,5) - IsEmpty: {cell55.IsEmpty}, Player: {cell55.Player}");
Console.WriteLine($"  (10,10) - IsEmpty: {cell1010.IsEmpty}, Player: {cell1010.Player}");
