using Caro.Core.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Adaptive depth calculator for AI difficulty levels
/// Grandmaster level uses position-aware depth selection instead of fixed depth
/// This prevents search horizon issues and optimizes computational resources
/// </summary>
public static class AdaptiveDepthCalculator
{
    /// <summary>
    /// Get the search depth for a given difficulty level and board position
    /// For Grandmaster, depth adapts based on position complexity
    /// </summary>
    public static int GetDepth(AIDifficulty difficulty, Board board)
    {
        return difficulty switch
        {
            AIDifficulty.Braindead => 1,
            AIDifficulty.Easy => 3,
            AIDifficulty.Medium => 5,
            AIDifficulty.Hard => 7,
            AIDifficulty.Grandmaster => GetAdaptiveDepth(board),
            _ => 3
        };
    }

    /// <summary>
    /// Calculate adaptive depth for Grandmaster level based on position complexity
    /// Analyzes stone count, threat count, and game phase to determine optimal depth
    /// </summary>
    private static int GetAdaptiveDepth(Board board)
    {
        // Count total stones on board
        int stoneCount = board.Cells.Count(c => !c.IsEmpty);

        // Count threats for both players (simplified detection)
        int threatCount = CountTotalThreats(board);

        // Determine game phase
        bool isOpening = stoneCount < 20;
        bool isMiddlegame = stoneCount >= 20 && stoneCount < 100;
        bool isEndgame = stoneCount >= 100;

        // Adaptive depth selection based on position characteristics
        if (isOpening)
        {
            // Opening: Use moderate depth, positions are less tactical
            // But we need to see tactics early, so still search reasonably deep
            return 9;
        }

        if (threatCount > 5)
        {
            // High tactical complexity: Search deeper to find forcing sequences
            // This covers positions with multiple threats from either player
            return 11;
        }

        if (isEndgame)
        {
            // Endgame: Precision matters, search deep
            return 10;
        }

        if (isMiddlegame)
        {
            // Middlegame: Standard depth for balanced positions
            return 10;
        }

        // Default fallback
        return 10;
    }

    /// <summary>
    /// Count total threats on the board for both players
    /// Simplified threat detection for performance
    /// </summary>
    private static int CountTotalThreats(Board board)
    {
        int totalThreats = 0;

        // Count open threes and broken threes (potential threats)
        // This is a simplified scan - full threat detection is more expensive
        for (int x = 0; x < board.BoardSize; x++)
        {
            for (int y = 0; y < board.BoardSize; y++)
            {
                var cell = board.GetCell(x, y);
                if (cell.IsEmpty)
                    continue;

                var player = cell.Player;
                totalThreats += CountThreatsAtPosition(board, x, y, player);
            }
        }

        return totalThreats;
    }

    /// <summary>
    /// Count potential threats at a specific position
    /// Checks for three-in-a-row patterns (open or broken)
    /// </summary>
    private static int CountThreatsAtPosition(Board board, int startX, int startY, Player player)
    {
        int threats = 0;
        var directions = new (int dx, int dy)[] { (1, 0), (0, 1), (1, 1), (1, -1) };

        foreach (var (dx, dy) in directions)
        {
            // Check for three-in-a-row patterns (simple heuristic)
            int count = 1;
            int emptyBefore = 0;
            int emptyAfter = 0;

            // Count forward
            for (int i = 1; i <= 4; i++)
            {
                int x = startX + dx * i;
                int y = startY + dy * i;
                if (x < 0 || x >= board.BoardSize || y < 0 || y >= board.BoardSize)
                    break;

                var cell = board.GetCell(x, y);
                if (cell.Player == player)
                    count++;
                else if (cell.IsEmpty)
                {
                    emptyAfter++;
                    break;
                }
                else
                    break;
            }

            // Count backward
            for (int i = 1; i <= 4; i++)
            {
                int x = startX - dx * i;
                int y = startY - dy * i;
                if (x < 0 || x >= board.BoardSize || y < 0 || y >= board.BoardSize)
                    break;

                var cell = board.GetCell(x, y);
                if (cell.Player == player)
                    count++;
                else if (cell.IsEmpty)
                {
                    emptyBefore++;
                    break;
                }
                else
                    break;
            }

            // If we have 3+ in a row with at least one open end, it's a threat
            if (count >= 3 && (emptyBefore > 0 || emptyAfter > 0))
                threats++;
        }

        return threats;
    }

    /// <summary>
    /// Get the error rate for a given difficulty level (used for simulating human-like mistakes)
    /// Returns probability (0-1) of making a random/suboptimal move
    /// NOTE: Keep error rates LOW to maintain consistent AI strength hierarchy
    /// Higher error rates introduce randomness that can paradoxically make weaker AIs harder to beat
    /// </summary>
    public static double GetErrorRate(AIDifficulty difficulty)
    {
        return difficulty switch
        {
            AIDifficulty.Braindead => 0.5,  // 50% error rate - intentional weak play
            AIDifficulty.Easy => 0.02,       // 2% error rate - 1 in 50 moves
            AIDifficulty.Medium => 0.005,    // 0.5% error rate - 1 in 200 moves
            AIDifficulty.Hard => 0.0,        // No intentional errors
            AIDifficulty.Grandmaster => 0.0, // No intentional errors
            _ => 0.1
        };
    }

    /// <summary>
    /// Get the parallel search thread count for a given difficulty level
    /// Returns 0 for single-threaded search
    /// </summary>
    public static int GetThreadCount(AIDifficulty difficulty)
    {
        // Conservative thread allocation to avoid TT pollution
        // System.Environment.ProcessorCount is used as baseline
        int processorCount = Environment.ProcessorCount;

        return difficulty switch
        {
            AIDifficulty.Braindead => 0,  // Single-threaded
            AIDifficulty.Easy => 0,       // Single-threaded
            AIDifficulty.Medium => 0,     // Single-threaded
            AIDifficulty.Hard => Math.Max(2, processorCount / 4),  // 2 threads or 25% of CPU
            // Use Lazy SMP formula for Grandmaster: (processorCount/2)-1
            // On 20 threads: (20/2)-1 = 9 helper threads
            AIDifficulty.Grandmaster => Math.Max(2, (processorCount / 2) - 1),
            _ => 0
        };
    }
}
