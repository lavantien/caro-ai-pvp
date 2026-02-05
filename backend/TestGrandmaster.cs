using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

var ai = new MinimaxAI();
var board = new Board();

// Test 1: Grandmaster should play center
var move1 = ai.GetBestMove(board, Player.Red, AIDifficulty.Grandmaster, timeRemainingMs: null, moveNumber: 1, ponderingEnabled: false, parallelSearchEnabled: false);
Console.WriteLine($"Move 1 (Grandmaster): ({move1.x}, {move1.y}) - Expected: (9,9) or nearby center");

board.PlaceStone(move1.x, move1.y, Player.Red);

// Test 2: Braindead should also play near center
var move2 = ai.GetBestMove(board, Player.Blue, AIDifficulty.Braindead, timeRemainingMs: null, moveNumber: 2, ponderingEnabled: false, parallelSearchEnabled: false);
Console.WriteLine($"Move 2 (Braindead): ({move2.x}, {move2.y}) - Expected: near center");

board.PlaceStone(move2.x, move2.y, Player.Blue);

// Test 3: Create a threat and see if Grandmaster blocks
board.PlaceStone(9, 10, Player.Red);  // Red creates 2-in-row
board.PlaceStone(8, 10, Player.Blue); // Blue plays elsewhere

// Now Red has a threat at (9,11) to make 3-in-row
var move3 = ai.GetBestMove(board, Player.Blue, AIDifficulty.Grandmaster, timeRemainingMs: null, moveNumber: 5, ponderingEnabled: false, parallelSearchEnabled: false);
Console.WriteLine($"Move 3 (Grandmaster blocking): ({move3.x}, {move3.y}) - Expected: (9,11) to block threat");
Console.WriteLine($"Position 9,11 has Red stone: {board.GetCell(9, 11).Player}");
