using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic.Search;

/// <summary>
/// TacticalEvaluator partial class - Safety check methods.
/// Pruning safety checks, emergency defense, and null-move verification.
/// </summary>
public static partial class TacticalEvaluator
{
    /// <summary>
    /// Check if a move is emergency defense (must block immediate threat)
    /// Returns true if this move blocks opponent's open-4 or double-open-3 threats.
    /// This is priority #2 in move ordering (after Hash Move, before general threats).
    /// Zero-allocation, very fast - runs at every node.
    /// </summary>
    public static bool IsEmergencyDefense(Board board, int x, int y, Player player)
    {
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var opponentBitBoard = board.GetBitBoard(opponent);
        var playerBitBoard = board.GetBitBoard(player);
        var occupied = playerBitBoard | opponentBitBoard;

        // Temporarily place stone to check if it blocks threats
        playerBitBoard.SetBit(x, y, true);

        // Check all 4 directions for blocking patterns
        var directions = new[] { (1, 0), (0, 1), (1, 1), (1, -1) };

        foreach (var (dx, dy) in directions)
        {
            // Count opponent consecutive stones if we DON'T block
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

            // Emergency if blocking open-4 (4 with open end)
            if (count == 4 && openEnds >= 1)
            {
                playerBitBoard.SetBit(x, y, false);  // Undo before returning
                return true;
            }
        }

        playerBitBoard.SetBit(x, y, false);  // Undo
        return false;
    }

    /// <summary>
    /// Emergency defense check for SearchBoard.
    /// Returns true if this move blocks opponent's immediate winning threat.
    /// </summary>
    public static bool IsEmergencyDefense(SearchBoard board, int x, int y, Player player)
    {
        var opponent = player == Player.Red ? Player.Blue : Player.Red;

        // Check if opponent would win by playing at (x, y)
        if (board.IsWinningMove(x, y, opponent))
            return true;

        // Check for double threats (multiple open 3s or open 4s)
        var opponentBits = board.GetBitBoard(opponent);
        var threatCount = 0;

        var directions = new[] { (1, 0), (0, 1), (1, 1), (1, -1) };
        foreach (var (dx, dy) in directions)
        {
            var count = 1;
            var openEnds = 0;

            // Check positive direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x + dx * i;
                var ny = y + dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;
                if (opponentBits.GetBit(nx, ny)) count++;
                else if (board.IsEmpty(nx, ny)) { openEnds++; break; }
                else break;
            }

            // Check negative direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;
                if (opponentBits.GetBit(nx, ny)) count++;
                else if (board.IsEmpty(nx, ny)) { openEnds++; break; }
                else break;
            }

            // Open 4 or open 3 is a threat
            if (count >= 4 && openEnds >= 1) threatCount++;
            else if (count >= 3 && openEnds == 2) threatCount++;
        }

        return threatCount >= 2;
    }

    /// <summary>
    /// Check if futility pruning is safe for this position
    /// Returns false if the position is tactical or has high uncertainty
    /// </summary>
    public static bool IsFutilitySafe(Board board, int depth, int alpha, int beta)
    {
        // Don't use futility in PV nodes
        if (beta - alpha > 1) return false;

        // Don't use futility at shallow depths
        if (depth < PruningConstants.FutilityMinDepth) return false;

        // Don't use futility if position is tactical
        if (IsTacticalPosition(board)) return false;

        return true;
    }

    /// <summary>
    /// Check if a BitBoard has 3+ consecutive stones in any direction.
    /// </summary>
    public static bool HasThreeInRow(BitBoard bits)
    {
        // Check horizontal: shift right 3 times and AND
        var h1 = bits;
        var h2 = h1.ShiftRight();
        var h3 = h2.ShiftRight();
        if ((h1 & h2 & h3).IsEmpty == false)
            return true;

        // Check vertical: shift down 3 times and AND
        var v1 = bits;
        var v2 = v1.ShiftDown();
        var v3 = v2.ShiftDown();
        if ((v1 & v2 & v3).IsEmpty == false)
            return true;

        // Check diagonal \
        var d1 = bits;
        var d2 = d1.ShiftDownRight();
        var d3 = d2.ShiftDownRight();
        if ((d1 & d2 & d3).IsEmpty == false)
            return true;

        // Check diagonal /
        var a1 = bits;
        var a2 = a1.ShiftDownLeft();
        var a3 = a2.ShiftDownLeft();
        if ((a1 & a2 & a3).IsEmpty == false)
            return true;

        return false;
    }

    /// <summary>
    /// Verify if null-move is safe (avoid zugzwang positions)
    /// In Caro, null-move is generally safe except in very tight tactical positions
    /// </summary>
    public static bool IsNullMoveSafe(Board board, Player player)
    {
        var playerBitBoard = board.GetBitBoard(player);
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var opponentBitBoard = board.GetBitBoard(opponent);
        var occupied = playerBitBoard | opponentBitBoard;

        // Check if position is "quiet" enough for null-move
        // Count stones on board - if too few, null-move is risky
        int totalStones = playerBitBoard.CountBits() + opponentBitBoard.CountBits();
        if (totalStones < 10) return false;  // Early game, too volatile

        // Check for immediate threats (4-in-row, open 3s)
        // If there are threats, null-move is unsafe (might miss tactical sequences)
        foreach (var (dx, dy) in new[] { (1, 0), (0, 1), (1, 1), (1, -1) })
        {
            for (int x = 0; x < BoardSize; x++)
            {
                for (int y = 0; y < BoardSize; y++)
                {
                    if (!opponentBitBoard.GetBit(x, y)) continue;

                    var count = BitBoardEvaluator.CountConsecutiveBoth(opponentBitBoard, x, y, dx, dy);
                    var openEnds = BitBoardEvaluator.CountOpenEnds(opponentBitBoard, occupied, x, y, dx, dy, count);

                    // Opponent has 4-in-row or open 3 - too dangerous for null-move
                    if (count == 4 && openEnds > 0) return false;
                    if (count == 3 && openEnds == 2) return false;
                }
            }
        }

        return true;
    }

    /// <summary>
    /// Verify if null-move is safe for SearchBoard (high-performance path).
    /// </summary>
    public static bool IsNullMoveSafe(SearchBoard board, Player player)
    {
        var playerBitBoard = board.GetBitBoard(player);
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var opponentBitBoard = board.GetBitBoard(opponent);
        var occupied = playerBitBoard | opponentBitBoard;

        // Count stones on board
        int totalStones = playerBitBoard.CountBits() + opponentBitBoard.CountBits();
        if (totalStones < 10) return false;

        // Check for immediate threats
        foreach (var (dx, dy) in new[] { (1, 0), (0, 1), (1, 1), (1, -1) })
        {
            for (int x = 0; x < BoardSize; x++)
            {
                for (int y = 0; y < BoardSize; y++)
                {
                    if (!opponentBitBoard.GetBit(x, y)) continue;

                    var count = BitBoardEvaluator.CountConsecutiveBoth(opponentBitBoard, x, y, dx, dy);
                    var openEnds = BitBoardEvaluator.CountOpenEnds(opponentBitBoard, occupied, x, y, dx, dy, count);

                    if (count == 4 && openEnds > 0) return false;
                    if (count == 3 && openEnds == 2) return false;
                }
            }
        }

        return true;
    }
}
