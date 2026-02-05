using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

// Test: Can depth-5 search consistently beat random moves?
var board = new Board();
var random = new Random(42);
var winCounts = new Dictionary<string, int>();

for (int game = 0; game < 100; game++)
{
    board.Clear();
    var gameOver = false;
    int moveCount = 0;

    while (!gameOver && moveCount < 200)
    {
        var player = moveCount % 2 == 0 ? Player.Red : Player.Blue;
        var candidates = board.GetValidMoves();

        (int x, int y) move;

        if (player == Player.Red)
        {
            // Red uses depth-5 search
            var ai = new MinimaxAI();
            var timeAlloc = new TimeAllocation { SoftBoundMs = 5000, HardBoundMs = 5000, OptimalTimeMs = 5000 };
            move = ai.GetBestMoveWithTimeAllocation(
                board, player, AIDifficulty.Medium,
                timeAlloc, moveNumber: moveCount + 1,
                ponderingEnabled: false, parallelSearchEnabled: true);
            Console.WriteLine($"Move {moveCount + 1}: Red (depth-5 AI) -> ({move.x},{move.y})");
        }
        else
        {
            // Blue plays random
            move = candidates[random.Next(candidates.Count)];
            Console.WriteLine($"Move {moveCount + 1}: Blue (random) -> ({move.x},{move.y})");
        }

        board.MakeMove(move.x, move.y, player);
        moveCount++;

        // Check for win
        var winDetector = new WinDetector();
        if (winDetector.CheckWin(board, move.x, move.y))
        {
            var winner = player == Player.Red ? "Red (depth-5)" : "Blue (random)";
            Console.WriteLine($"Game {game + 1}: {winner} wins on move {moveCount}!");
            winCounts.TryAdd(winner, 0);
            winCounts[winner]++;
            gameOver = true;
        }
    }

    if (!gameOver)
    {
        Console.WriteLine($"Game {game + 1}: Draw after {moveCount} moves");
        winCounts.TryAdd("Draw", 0);
        winCounts["Draw"]++;
    }

    Console.WriteLine();
}

Console.WriteLine("=== RESULTS ===");
foreach (var kvp in winCounts.OrderBy(x => x.Key))
{
    Console.WriteLine($"{kvp.Key}: {kvp.Value}");
}
