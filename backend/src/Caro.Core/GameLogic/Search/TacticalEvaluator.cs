using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic.Search;

/// <summary>
/// Static tactical evaluation methods extracted from MinimaxAI.
/// Provides pattern recognition, threat detection, and pruning safety checks.
/// </summary>
public static partial class TacticalEvaluator
{
    private const int BoardSize = GameConstants.BoardSize;

    /// <summary>
    /// Evaluate tactical importance of a move by detecting patterns
    /// Returns high scores for winning moves, threats, and blocks
    /// Optimized using BitBoard operations
    /// </summary>
    public static int EvaluateTacticalPattern(Board board, int x, int y, Player player)
    {
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var playerBitBoard = board.GetBitBoard(player);
        var opponentBitBoard = board.GetBitBoard(opponent);
        var occupied = playerBitBoard | opponentBitBoard;
        var score = 0;

        // Check all 4 directions for patterns
        foreach (var (dx, dy) in GameConstants.CardinalDirections)
        {
            // Count consecutive stones in both directions (for player)
            var count = 1;
            var openEnds = 0;

            // Check positive direction (using BitBoard)
            for (int i = 1; i <= 4; i++)
            {
                var nx = x + dx * i;
                var ny = y + dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;

                if (playerBitBoard.GetBit(nx, ny))
                {
                    count++;
                }
                else if (!occupied.GetBit(nx, ny))
                {
                    openEnds++;
                    break;
                }
                else
                {
                    break;
                }
            }

            // Check negative direction (using BitBoard)
            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;

                if (playerBitBoard.GetBit(nx, ny))
                {
                    count++;
                }
                else if (!occupied.GetBit(nx, ny))
                {
                    openEnds++;
                    break;
                }
                else
                {
                    break;
                }
            }

            // Score based on pattern
            if (count >= 5)
            {
                score += 10000; // Winning move
            }
            else if (count == 4)
            {
                if (openEnds >= 1)
                    score += 5000; // Open 4 (unstoppable threat)
                else
                    score += 200; // Closed 4
            }
            else if (count == 3)
            {
                if (openEnds == 2)
                    score += 500; // Open 3 (very strong)
                else if (openEnds == 1)
                    score += 100; // Semi-open 3
                else
                    score += 20; // Closed 3
            }
            else if (count == 2)
            {
                if (openEnds == 2)
                    score += 50; // Open 2
            }
        }

        // Check blocking value (how much this blocks opponent)
        foreach (var (dx, dy) in GameConstants.CardinalDirections)
        {
            var count = 1;
            var openEnds = 0;

            // Positive direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x + dx * i;
                var ny = y + dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;

                if (opponentBitBoard.GetBit(nx, ny))
                {
                    count++;
                }
                else if (!occupied.GetBit(nx, ny))
                {
                    openEnds++;
                    break;
                }
                else
                {
                    break;
                }
            }

            // Negative direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;

                if (opponentBitBoard.GetBit(nx, ny))
                {
                    count++;
                }
                else if (!occupied.GetBit(nx, ny))
                {
                    openEnds++;
                    break;
                }
                else
                {
                    break;
                }
            }

            // Score blocking value
            if (count >= 4)
            {
                if (openEnds >= 1)
                    score += 4000; // Must block (opponent has winning threat)
            }
            else if (count == 3)
            {
                if (openEnds == 2)
                    score += 300; // Block open 3
                else if (openEnds == 1)
                    score += 80; // Block semi-open 3
            }
        }

        return score;
    }

    /// <summary>
    /// Evaluate tactical pattern for SearchBoard.
    /// Uses bitboard operations for efficiency.
    /// </summary>
    public static int EvaluateTacticalPattern(SearchBoard board, int x, int y, Player player)
    {
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var playerBitBoard = board.GetBitBoard(player);
        var opponentBitBoard = board.GetBitBoard(opponent);
        var occupied = playerBitBoard | opponentBitBoard;
        var score = 0;

        foreach (var (dx, dy) in GameConstants.CardinalDirections)
        {
            // Count consecutive stones in both directions (for player)
            var count = 1;
            var openEnds = 0;

            for (int i = 1; i <= 4; i++)
            {
                var nx = x + dx * i;
                var ny = y + dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;

                if (playerBitBoard.GetBit(nx, ny))
                {
                    count++;
                }
                else if (!occupied.GetBit(nx, ny))
                {
                    openEnds++;
                    break;
                }
                else
                {
                    break;
                }
            }

            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;

                if (playerBitBoard.GetBit(nx, ny))
                {
                    count++;
                }
                else if (!occupied.GetBit(nx, ny))
                {
                    openEnds++;
                    break;
                }
                else
                {
                    break;
                }
            }

            // Score based on pattern
            if (count >= 5) score += 100000;  // Winning move
            else if (count == 4 && openEnds >= 1) score += 10000;  // Open 4
            else if (count == 3 && openEnds == 2) score += 5000;   // Open 3 (double threat)
            else if (count == 3 && openEnds == 1) score += 500;    // Half-open 3
            else if (count == 2 && openEnds == 2) score += 100;    // Open 2

            // Also check blocking value (opponent patterns)
            var oppCount = 1;
            var oppOpenEnds = 0;

            for (int i = 1; i <= 4; i++)
            {
                var nx = x + dx * i;
                var ny = y + dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;

                if (opponentBitBoard.GetBit(nx, ny))
                {
                    oppCount++;
                }
                else if (!occupied.GetBit(nx, ny))
                {
                    oppOpenEnds++;
                    break;
                }
                else
                {
                    break;
                }
            }

            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;

                if (opponentBitBoard.GetBit(nx, ny))
                {
                    oppCount++;
                }
                else if (!occupied.GetBit(nx, ny))
                {
                    oppOpenEnds++;
                    break;
                }
                else
                {
                    break;
                }
            }

            // Blocking is slightly less valuable than attacking
            if (oppCount >= 5) score += 90000;  // Block win
            else if (oppCount == 4 && oppOpenEnds >= 1) score += 9000;  // Block open 4
            else if (oppCount == 3 && oppOpenEnds == 2) score += 4000;   // Block open 3
        }

        return score;
    }

    /// <summary>
    /// Estimate the maximum possible gain from a move at (x, y)
    /// Used for futility pruning - if max gain < alpha - margin, skip search
    /// </summary>
    public static int EstimateMaxGain(Board board, int x, int y, Player player)
    {
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var playerBitBoard = board.GetBitBoard(player);
        var opponentBitBoard = board.GetBitBoard(opponent);
        var occupied = playerBitBoard | opponentBitBoard;

        int maxGain = 0;

        foreach (var (dx, dy) in GameConstants.CardinalDirections)
        {
            // Count consecutive stones after placing this stone
            var count = 1;

            // Positive direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x + dx * i;
                var ny = y + dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;
                if (playerBitBoard.GetBit(nx, ny)) count++;
                else if (!occupied.GetBit(nx, ny)) break;
                else break;
            }

            // Negative direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;
                if (playerBitBoard.GetBit(nx, ny)) count++;
                else if (!occupied.GetBit(nx, ny)) break;
                else break;
            }

            // Score based on potential
            if (count >= 5) maxGain += 100000;
            else if (count == 4) maxGain += 10000;
            else if (count == 3) maxGain += 1000;
            else if (count == 2) maxGain += 100;
            else if (count == 1) maxGain += 10;
        }

        // Add blocking value
        foreach (var (dx, dy) in GameConstants.CardinalDirections)
        {
            var count = 1;

            // Positive direction (opponent)
            for (int i = 1; i <= 4; i++)
            {
                var nx = x + dx * i;
                var ny = y + dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;
                if (opponentBitBoard.GetBit(nx, ny)) count++;
                else if (!occupied.GetBit(nx, ny)) break;
                else break;
            }

            // Negative direction (opponent)
            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;
                if (opponentBitBoard.GetBit(nx, ny)) count++;
                else if (!occupied.GetBit(nx, ny)) break;
                else break;
            }

            if (count >= 4) maxGain += 10000;
            else if (count == 3) maxGain += 1000;
        }

        return maxGain;
    }
}
