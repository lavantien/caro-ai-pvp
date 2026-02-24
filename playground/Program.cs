using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

Console.WriteLine("=== Check Multi-Threat Handling ===\n");

// Build board at M47
var moves = new (int x, int y, Player p)[]
{
    (7,7,Player.Red), (8,8,Player.Blue), (8,10,Player.Red), (12,14,Player.Blue),
    (14,15,Player.Red), (13,14,Player.Blue), (14,14,Player.Red), (14,13,Player.Blue),
    (7,9,Player.Red), (6,8,Player.Blue), (15,12,Player.Red), (7,8,Player.Blue),
    (4,8,Player.Red), (5,8,Player.Blue), (9,8,Player.Red), (13,13,Player.Blue),
    (13,12,Player.Red), (12,13,Player.Blue), (7,0,Player.Red), (11,13,Player.Blue),
    (10,13,Player.Red), (15,13,Player.Blue), (12,12,Player.Red), (14,12,Player.Blue),
    (15,11,Player.Red), (10,12,Player.Blue), (13,15,Player.Red), (12,15,Player.Blue),
    (15,10,Player.Red), (15,9,Player.Blue), (14,11,Player.Red), (11,14,Player.Blue),
    (0,10,Player.Red), (10,14,Player.Blue), (9,14,Player.Red), (11,12,Player.Blue),
    (15,3,Player.Red), (11,11,Player.Blue), (11,10,Player.Red), (9,13,Player.Blue),
    (11,15,Player.Red), (8,14,Player.Blue), (12,10,Player.Red), (7,15,Player.Blue),
    (10,11,Player.Red), (13,10,Player.Blue), (12,9,Player.Red)
};

var board = new Board();
for (int i = 0; i < moves.Length; i++)
    board = board.PlaceStone(moves[i].x, moves[i].y, moves[i].p);

var threatDetector = new ThreatDetector();
var redThreats = threatDetector.DetectThreats(board, Player.Red);

var threeThreats = redThreats.Where(t => 
    t.Type == ThreatType.StraightThree || t.Type == ThreatType.BrokenThree).ToList();

Console.WriteLine($"Three-threats count: {threeThreats.Count}");
foreach (var t in threeThreats)
    Console.WriteLine($"  {t.Type}: {string.Join(", ", t.GainSquares)}");

var allGainSquares = threeThreats.SelectMany(t => t.GainSquares).Distinct().ToList();
Console.WriteLine($"\nDistinct gain squares: {allGainSquares.Count}");
Console.WriteLine($"hasMultipleIndependentThreats = {threeThreats.Count >= 2 && allGainSquares.Count >= 3}");

// Check for counter-attack opportunities
Console.WriteLine("\nCounter-attack opportunities:");
int counterCount = 0;
for (int x = 0; x < 16; x++)
{
    for (int y = 0; y < 16; y++)
    {
        if (!board.GetCell(x, y).IsEmpty) continue;
        var testBoard = board.PlaceStone(x, y, Player.Blue);
        var newThreats = threatDetector.DetectThreats(testBoard, Player.Blue);
        var fourThreats = newThreats.Where(t => 
            t.Type == ThreatType.StraightFour || t.Type == ThreatType.BrokenFour).ToList();
        if (fourThreats.Count > 0)
        {
            Console.WriteLine($"  ({x},{y}) creates {fourThreats.Count} four-threat(s)");
            counterCount++;
        }
    }
}
Console.WriteLine($"Total counter-attack squares: {counterCount}");
