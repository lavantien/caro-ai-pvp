using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.GameLogic.TimeManagement;
using System.Reflection;

// Test to understand why Grandmaster fails to block Braindead's open threes
// Reproduce Game 1 scenario from tournament EXACTLY

Console.WriteLine("=== Game 1 Move 7 Scenario Test ===\n");

// Game 1 moves from tournament:
// M1: R(8,8) by Grandmaster
// M2: B(8,7) by Braindead
// M3: R(5,8) by Grandmaster
// M4: B(7,7) by Braindead
// M5: R(8,6) by Grandmaster
// M6: B(9,7) by Braindead - creates open three at row 7
// M7: R(2,9) by Grandmaster - FAILED TO BLOCK!
// M8: B(6,7) by Braindead - creates open four
// M9: R(5,7) by Grandmaster - still not blocking
// M10: B(10,7) by Braindead - Wn (five in a row)

// Build board state AFTER move 6 (before Grandmaster's move 7)
var board = new Board()
    .PlaceStone(8, 8, Player.Red)   // GM's M1
    .PlaceStone(8, 7, Player.Blue)  // BD's M2
    .PlaceStone(5, 8, Player.Red)   // GM's M3
    .PlaceStone(7, 7, Player.Blue)  // BD's M4
    .PlaceStone(8, 6, Player.Red)   // GM's M5
    .PlaceStone(9, 7, Player.Blue); // BD's M6 - open three created!

Console.WriteLine("Board state after move 6 (GM to move):");
Console.WriteLine("Red stones: (8,8), (5,8), (8,6)");
Console.WriteLine("Blue stones: (8,7), (7,7), (9,7) - OPEN THREE at row 7!");
Console.WriteLine();

// Detect Blue's threats
var threatDetector = new ThreatDetector();
var blueThreats = threatDetector.DetectThreats(board, Player.Blue);
Console.WriteLine($"Blue's threats: {blueThreats.Count}");
foreach (var threat in blueThreats)
{
    Console.WriteLine($"  {threat.Type}: gain squares = [{string.Join(", ", threat.GainSquares.Select(g => $"({g.x},{g.y})"))}]");
}

var blueOpenThrees = blueThreats.Where(t => t.Type == ThreatType.StraightThree).ToList();
Console.WriteLine($"\nBlue's open threes: {blueOpenThrees.Count}");
foreach (var threat in blueOpenThrees)
{
    Console.WriteLine($"  Gain squares (blocking squares): [{string.Join(", ", threat.GainSquares.Select(g => $"({g.x},{g.y})"))}]");
}
Console.WriteLine();

// Now let's trace through the parallel search logic manually
Console.WriteLine("=== Tracing ParallelSearch blocking logic ===");

var parallelSearch = new ParallelMinimaxSearch();
var getOpenThreeBlocksMethod = typeof(ParallelMinimaxSearch).GetMethod("GetOpenThreeBlocks",
    System.Reflection.BindingFlags.NonPublic | System.Reflection.BindingFlags.Instance);

// Call GetOpenThreeBlocks to see what it returns
var openThreeBlocks = (List<(int x, int y)>)getOpenThreeBlocksMethod!.Invoke(parallelSearch, new object[] { board, Player.Blue })!;
Console.WriteLine($"GetOpenThreeBlocks returns: [{string.Join(", ", openThreeBlocks.Select(b => $"({b.x},{b.y})"))}]");

// Get candidate moves
var getCandidatesMethod = typeof(ParallelMinimaxSearch).GetMethod("GetCandidateMoves",
    System.Reflection.BindingFlags.NonPublic | System.Reflection.BindingFlags.Instance);
var candidates = (List<(int x, int y)>)getCandidatesMethod!.Invoke(parallelSearch, new object[] { board })!;
Console.WriteLine($"\nCandidates before prioritization (first 10): [{string.Join(", ", candidates.Take(10).Select(c => $"({c.x},{c.y})"))}]");

// Simulate the prioritization logic from GetBestMoveWithStats
foreach (var block in openThreeBlocks)
{
    candidates.Remove(block);
    candidates.Insert(0, block);
}
Console.WriteLine($"Candidates AFTER prioritization (first 10): [{string.Join(", ", candidates.Take(10).Select(c => $"({c.x},{c.y})"))}]");

// Check if blocking squares are at front
bool blockingAtFront = candidates.Count >= 2 &&
    ((candidates[0].x == 6 && candidates[0].y == 7) || (candidates[0].x == 10 && candidates[0].y == 7)) &&
    ((candidates[1].x == 6 && candidates[1].y == 7) || (candidates[1].x == 10 && candidates[1].y == 7));
Console.WriteLine($"Blocking squares at front: {blockingAtFront}");

// Now test what move Grandmaster would make
Console.WriteLine("\n=== Testing Grandmaster's move selection at move 7 ===");

var ai = new MinimaxAI();
var sw = System.Diagnostics.Stopwatch.StartNew();

// Use realistic time allocation for move 7 in 180+2 time control
var move = ai.GetBestMove(board, Player.Red, AIDifficulty.Grandmaster,
    timeRemainingMs: 178000, moveNumber: 7, ponderingEnabled: true, parallelSearchEnabled: true);
sw.Stop();

var depthField = typeof(MinimaxAI).GetField("_depthAchieved", BindingFlags.NonPublic | BindingFlags.Instance);
var nodesField = typeof(MinimaxAI).GetField("_nodesSearched", BindingFlags.NonPublic | BindingFlags.Instance);
var moveTypeField = typeof(MinimaxAI).GetField("_moveType", BindingFlags.NonPublic | BindingFlags.Instance);
var threadCountField = typeof(MinimaxAI).GetField("_lastThreadCount", BindingFlags.NonPublic | BindingFlags.Instance);
var parallelDiagField = typeof(MinimaxAI).GetField("_lastParallelDiagnostics", BindingFlags.NonPublic | BindingFlags.Instance);

Console.WriteLine($"Grandmaster's move: ({move.x},{move.y})");
Console.WriteLine($"  Depth: {depthField?.GetValue(ai)}");
Console.WriteLine($"  Nodes: {nodesField?.GetValue(ai)}");
Console.WriteLine($"  Time: {sw.ElapsedMilliseconds}ms");
Console.WriteLine($"  MoveType: {moveTypeField?.GetValue(ai)}");
Console.WriteLine($"  Threads: {threadCountField?.GetValue(ai)}");
Console.WriteLine($"  Parallel Diag: {parallelDiagField?.GetValue(ai)}");

// The correct blocking move should be (6,7) or (10,7) to block Blue's open three
bool isBlocking = (move.x == 6 && move.y == 7) || (move.x == 10 && move.y == 7);
Console.WriteLine($"  Is blocking move: {isBlocking}");

if (!isBlocking)
{
    Console.WriteLine("\n*** ERROR: Grandmaster failed to block Blue's open three! ***");
    Console.WriteLine($"Expected blocking move at (6,7) or (10,7), but got ({move.x},{move.y})");
}
else
{
    Console.WriteLine("\n*** SUCCESS: Grandmaster correctly blocked the open three! ***");
}
