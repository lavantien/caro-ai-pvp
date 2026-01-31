using Caro.Core.Entities;
using Caro.Core.GameLogic;

// Debug script to test GetOpponentThreatMoves directly
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

// Check ThreatDetector
Console.WriteLine("=== ThreatDetector ===");
var threatDetector = new ThreatDetector();
var threats = threatDetector.DetectThreats(board, Player.Red);
Console.WriteLine($"Found {threats.Count} threats");
foreach (var t in threats)
{
    Console.WriteLine($"  Type={t.Type}, GainSquares.Count={t.GainSquares.Count}");
    foreach (var gs in t.GainSquares)
    {
        Console.WriteLine($"    ({gs.x},{gs.y}) - IsEmpty={board.GetCell(gs.x, gs.y).IsEmpty}");
    }
}

// Check if Priority 1 (immediate win) is finding something
Console.WriteLine("\n=== Priority 1 Check ===");
var winDetector = new WinDetector();
for (int x = 0; x < 15; x++)
{
    for (int y = 0; y < 15; y++)
    {
        if (!board.GetCell(x, y).IsEmpty)
            continue;

        board.PlaceStone(x, y, Player.Red);
        bool isWinningMove = winDetector.CheckWin(board).HasWinner;
        board.GetCell(x, y).Player = Player.None;

        if (isWinningMove)
        {
            Console.WriteLine($"  Found winning move at ({x},{y}) - this would cause early return!");
        }
    }
}

Console.WriteLine("\n=== Conclusion ===");
Console.WriteLine("If no winning moves found above, then GetOpponentThreatMoves should return both (5,5) and (10,10)");
Console.WriteLine("But it only returned (5,5) - so there must be another early return");
