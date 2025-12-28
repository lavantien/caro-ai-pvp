using Caro.Core.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// AI opponent using Minimax algorithm with alpha-beta pruning
/// </summary>
public class MinimaxAI
{
    private readonly BoardEvaluator _evaluator = new();
    private readonly Random _random = new();

    // Search radius around existing stones (optimization)
    private const int SearchRadius = 2;

    /// <summary>
    /// Get the best move for the AI player
    /// </summary>
    public (int x, int y) GetBestMove(Board board, Player player, AIDifficulty difficulty)
    {
        if (player == Player.None)
            throw new ArgumentException("Player cannot be None");

        var depth = (int)difficulty;
        var isMaximizing = true;

        // Get candidate moves (empty cells near existing stones)
        var candidates = GetCandidateMoves(board);

        if (candidates.Count == 0)
        {
            // Board is empty, play center
            return (7, 7);
        }

        // For Easy difficulty, sometimes add randomness
        if (difficulty == AIDifficulty.Easy && _random.Next(100) < 20)
        {
            // 20% chance to play random valid move
            var randomIndex = _random.Next(candidates.Count);
            return candidates[randomIndex];
        }

        var bestScore = int.MinValue;
        var bestMove = candidates[0];
        var alpha = int.MinValue;
        var beta = int.MaxValue;

        foreach (var (x, y) in candidates)
        {
            // Make move
            board.PlaceStone(x, y, player);

            // Evaluate using minimax
            var score = Minimax(board, depth - 1, alpha, beta, !isMaximizing, player);

            // Undo move
            board.GetCell(x, y).Player = Player.None;

            if (score > bestScore)
            {
                bestScore = score;
                bestMove = (x, y);
            }

            alpha = Math.Max(alpha, score);
            if (beta <= alpha)
                break; // Beta cutoff
        }

        return bestMove;
    }

    /// <summary>
    /// Minimax algorithm with alpha-beta pruning
    /// </summary>
    private int Minimax(Board board, int depth, int alpha, int beta, bool isMaximizing, Player aiPlayer)
    {
        // Check terminal states
        var winner = CheckWinner(board);
        if (winner != null)
        {
            return winner == aiPlayer ? 100000 : -100000;
        }

        if (depth == 0)
        {
            return _evaluator.Evaluate(board, aiPlayer);
        }

        var candidates = GetCandidateMoves(board);
        if (candidates.Count == 0)
        {
            return 0; // Draw
        }

        var currentPlayer = isMaximizing ? aiPlayer : (aiPlayer == Player.Red ? Player.Blue : Player.Red);

        if (isMaximizing)
        {
            var maxEval = int.MinValue;
            foreach (var (x, y) in candidates)
            {
                board.PlaceStone(x, y, currentPlayer);
                var eval = Minimax(board, depth - 1, alpha, beta, false, aiPlayer);
                board.GetCell(x, y).Player = Player.None;

                maxEval = Math.Max(maxEval, eval);
                alpha = Math.Max(alpha, eval);
                if (beta <= alpha)
                    break; // Beta cutoff
            }
            return maxEval;
        }
        else
        {
            var minEval = int.MaxValue;
            foreach (var (x, y) in candidates)
            {
                board.PlaceStone(x, y, currentPlayer);
                var eval = Minimax(board, depth - 1, alpha, beta, true, aiPlayer);
                board.GetCell(x, y).Player = Player.None;

                minEval = Math.Min(minEval, eval);
                beta = Math.Min(beta, eval);
                if (beta <= alpha)
                    break; // Alpha cutoff
            }
            return minEval;
        }
    }

    /// <summary>
    /// Get candidate moves (empty cells near existing stones)
    /// </summary>
    private List<(int x, int y)> GetCandidateMoves(Board board)
    {
        var candidates = new List<(int x, int y)>();
        var considered = new bool[15, 15];

        for (int x = 0; x < 15; x++)
        {
            for (int y = 0; y < 15; y++)
            {
                var cell = board.GetCell(x, y);
                if (cell.Player != Player.None)
                {
                    // Check neighboring cells
                    for (int dx = -SearchRadius; dx <= SearchRadius; dx++)
                    {
                        for (int dy = -SearchRadius; dy <= SearchRadius; dy++)
                        {
                            var nx = x + dx;
                            var ny = y + dy;

                            if (nx >= 0 && nx < 15 && ny >= 0 && ny < 15 && !considered[nx, ny])
                            {
                                if (board.GetCell(nx, ny).Player == Player.None)
                                {
                                    candidates.Add((nx, ny));
                                    considered[nx, ny] = true;
                                }
                            }
                        }
                    }
                }
            }
        }

        // If no candidates (empty board), return center
        if (candidates.Count == 0)
        {
            return new List<(int x, int y)> { (7, 7) };
        }

        return candidates;
    }

    /// <summary>
    /// Check if there's a winner on the board
    /// </summary>
    private Player? CheckWinner(Board board)
    {
        // Check all possible 5-in-row sequences
        for (int x = 0; x < 15; x++)
        {
            for (int y = 0; y < 15; y++)
            {
                var cell = board.GetCell(x, y);
                if (cell.Player == Player.None)
                    continue;

                // Check all 4 directions
                var directions = new[] { (1, 0), (0, 1), (1, 1), (1, -1) };
                foreach (var (dx, dy) in directions)
                {
                    if (CheckDirection(board, x, y, dx, dy, cell.Player))
                    {
                        return cell.Player;
                    }
                }
            }
        }

        return null;
    }

    /// <summary>
    /// Check if there's a winning sequence in a direction
    /// </summary>
    private bool CheckDirection(Board board, int startX, int startY, int dx, int dy, Player player)
    {
        for (int i = 1; i < 5; i++)
        {
            var x = startX + i * dx;
            var y = startY + i * dy;

            if (x < 0 || x >= 15 || y < 0 || y >= 15)
                return false;

            if (board.GetCell(x, y).Player != player)
                return false;
        }

        return true;
    }
}
