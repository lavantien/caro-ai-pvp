using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic.Search;

/// <summary>
/// Static tactical evaluation methods extracted from MinimaxAI.
/// Provides pattern recognition, threat detection, and pruning safety checks.
/// </summary>
public static class TacticalEvaluator
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
        var directions = new[] { (1, 0), (0, 1), (1, 1), (1, -1) };

        foreach (var (dx, dy) in directions)
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
        foreach (var (dx, dy) in directions)
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

        var directions = new[] { (1, 0), (0, 1), (1, 1), (1, -1) };

        foreach (var (dx, dy) in directions)
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
    /// Check if position is tactical (has threats) - should not use reduced depth
    /// Tactical positions have: 3+ in a row, or multiple threats nearby
    /// </summary>
    public static bool IsTacticalPosition(Board board)
    {
        // Check for 3+ in a row
        for (int x = 0; x < BoardSize; x++)
        {
            for (int y = 0; y < BoardSize; y++)
            {
                var cell = board.GetCell(x, y);
                if (cell.Player == Player.None)
                    continue;

                // Check horizontal
                var count = 1;
                for (int dy = 1; dy <= 4 && y + dy < BoardSize; dy++)
                {
                    if (board.GetCell(x, y + dy).Player == cell.Player)
                        count++;
                    else
                        break;
                }
                if (count >= 3)
                    return true;

                // Check vertical
                count = 1;
                for (int dx = 1; dx <= 4 && x + dx < BoardSize; dx++)
                {
                    if (board.GetCell(x + dx, y).Player == cell.Player)
                        count++;
                    else
                        break;
                }
                if (count >= 3)
                    return true;

                // Check diagonal (down-right)
                count = 1;
                for (int i = 1; i <= 4 && x + i < BoardSize && y + i < BoardSize; i++)
                {
                    if (board.GetCell(x + i, y + i).Player == cell.Player)
                        count++;
                    else
                        break;
                }
                if (count >= 3)
                    return true;

                // Check diagonal (down-left)
                count = 1;
                for (int i = 1; i <= 4 && x + i < BoardSize && y - i >= 0; i++)
                {
                    if (board.GetCell(x + i, y - i).Player == cell.Player)
                        count++;
                    else
                        break;
                }
                if (count >= 3)
                    return true;
            }
        }

        return false;  // Not tactical
    }

    /// <summary>
    /// Check if position is tactical using SearchBoard.
    /// </summary>
    public static bool IsTacticalPosition(SearchBoard board)
    {
        var redBits = board.GetBitBoard(Player.Red);
        var blueBits = board.GetBitBoard(Player.Blue);

        // Quick check using bitboard operations
        // Check for 3+ in a row in any direction for either player
        return HasThreeInRow(redBits) || HasThreeInRow(blueBits);
    }

    /// <summary>
    /// Check if a specific move is tactical (creates threats or blocks opponent)
    /// Used for LMR - tactical moves should not use reduced depth
    /// </summary>
    public static bool IsTacticalMove(Board board, int x, int y, Player player)
    {
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var playerBitBoard = board.GetBitBoard(player);
        var opponentBitBoard = board.GetBitBoard(opponent);
        var occupied = playerBitBoard | opponentBitBoard;
        var directions = new[] { (1, 0), (0, 1), (1, 1), (1, -1) };

        foreach (var (dx, dy) in directions)
        {
            // Check if this move creates threat for player
            var playerCount = 1;
            var playerOpenEnds = 0;

            for (int i = 1; i <= 4; i++)
            {
                var nx = x + dx * i;
                var ny = y + dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;
                if (playerBitBoard.GetBit(nx, ny)) playerCount++;
                else if (!occupied.GetBit(nx, ny)) { playerOpenEnds++; break; }
                else break;
            }
            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;
                if (playerBitBoard.GetBit(nx, ny)) playerCount++;
                else if (!occupied.GetBit(nx, ny)) { playerOpenEnds++; break; }
                else break;
            }

            // Creating 3+ with open ends is tactical
            if (playerCount >= 3 && playerOpenEnds >= 1)
                return true;
            if (playerCount >= 4)
                return true;

            // Check if this move blocks opponent threat
            var oppCount = 1;
            var oppOpenEnds = 0;

            for (int i = 1; i <= 4; i++)
            {
                var nx = x + dx * i;
                var ny = y + dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;
                if (opponentBitBoard.GetBit(nx, ny)) oppCount++;
                else if (!occupied.GetBit(nx, ny)) { oppOpenEnds++; break; }
                else break;
            }
            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;
                if (opponentBitBoard.GetBit(nx, ny)) oppCount++;
                else if (!occupied.GetBit(nx, ny)) { oppOpenEnds++; break; }
                else break;
            }

            // Blocking 3+ with open ends is tactical (must block)
            if (oppCount >= 3 && oppOpenEnds >= 1)
                return true;
            if (oppCount >= 4)
                return true;
        }

        return false;  // Not a tactical move
    }

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
            else if (count >= 3 && openEnds >= 2) threatCount++;
        }

        return threatCount >= 2;
    }

    /// <summary>
    /// Check if a move at (x, y) creates or blocks critical threats
    /// These moves should NEVER be pruned as they're tactically significant
    /// </summary>
    public static bool IsCriticalMove(Board board, int x, int y, Player player)
    {
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var playerBitBoard = board.GetBitBoard(player);
        var opponentBitBoard = board.GetBitBoard(opponent);
        var occupied = playerBitBoard | opponentBitBoard;

        var directions = new[] { (1, 0), (0, 1), (1, 1), (1, -1) };

        foreach (var (dx, dy) in directions)
        {
            // Check if this move creates threats for current player
            var count = 1; // Include the placed stone

            // Count in positive direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x + dx * i;
                var ny = y + dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;
                if (playerBitBoard.GetBit(nx, ny)) count++;
                else if (!occupied.GetBit(nx, ny)) break;
                else break;
            }

            // Count in negative direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;
                if (playerBitBoard.GetBit(nx, ny)) count++;
                else if (!occupied.GetBit(nx, ny)) break;
                else break;
            }

            // Critical: creates 4+ or open 3
            if (count >= 4) return true; // Potential winning move
            if (count == 3)
            {
                // Check if both ends are open
                bool leftOpen = x - dx >= 0 && x - dx < BoardSize && y - dy >= 0 && y - dy < BoardSize
                               && !occupied.GetBit(x - dx, y - dy);
                bool rightOpen = x + dx * 3 >= 0 && x + dx * 3 < BoardSize && y + dy * 3 >= 0 && y + dy * 3 < BoardSize
                                && !occupied.GetBit(x + dx * 3, y + dy * 3);
                if (leftOpen && rightOpen) return true; // Creates open three
            }

            // Check if this move blocks opponent threats
            var oppCount = 1;

            // Count opponent stones in positive direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x + dx * i;
                var ny = y + dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;
                if (opponentBitBoard.GetBit(nx, ny)) oppCount++;
                else if (!occupied.GetBit(nx, ny)) break;
                else break;
            }

            // Count opponent stones in negative direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= BoardSize || ny < 0 || ny >= BoardSize) break;
                if (opponentBitBoard.GetBit(nx, ny)) oppCount++;
                else if (!occupied.GetBit(nx, ny)) break;
                else break;
            }

            // Critical: blocks opponent's 4 or open 3
            if (oppCount >= 4) return true; // Blocks winning threat
            if (oppCount == 3)
            {
                // Check if this blocks an open three
                var leftOpen = x - dx >= 0 && x - dx < BoardSize && y - dy >= 0 && y - dy < BoardSize
                              && !occupied.GetBit(x - dx, y - dy);
                var rightOpen = x + dx * 3 >= 0 && x + dx * 3 < BoardSize && y + dy * 3 >= 0 && y + dy * 3 < BoardSize
                               && !occupied.GetBit(x + dx * 3, y + dy * 3);
                if (leftOpen && rightOpen) return true; // Blocks open three
            }
        }

        return false;
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
        var directions = new[] { (1, 0), (0, 1), (1, 1), (1, -1) };

        foreach (var (dx, dy) in directions)
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
        foreach (var (dx, dy) in directions)
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
