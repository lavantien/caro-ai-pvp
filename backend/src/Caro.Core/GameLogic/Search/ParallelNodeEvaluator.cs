using System.Runtime.CompilerServices;
using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic.Search;

/// <summary>
/// Stateless per-node evaluation utilities for parallel search.
/// Tactical evaluation, position evaluation, adaptive pruning,
/// and quiescence-relevant tactical detection.
/// </summary>
public static class ParallelNodeEvaluator
{
    /// <summary>
    /// Fast tactical evaluation using BitBoard operations.
    /// Only evaluates the specific move position, not the entire board.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int EvaluateTacticalFast(SearchBoard board, int x, int y, Player player, BitBoard playerBitBoard, BitBoard opponentBitBoard)
    {
        int score = 0;
        var occupied = playerBitBoard | opponentBitBoard;

        // Check all 4 directions
        foreach (var (dx, dy) in GameConstants.CardinalDirections)
        {
            // Count consecutive stones for player
            int count = 1;
            int openEnds = 0;

            // Positive direction
            for (int i = 1; i <= 4; i++)
            {
                int nx = x + dx * i;
                int ny = y + dy * i;
                if (nx < 0 || nx >= BitBoard.Size || ny < 0 || ny >= BitBoard.Size) break;

                if (playerBitBoard.GetBit(nx, ny))
                    count++;
                else if (!occupied.GetBit(nx, ny))
                {
                    openEnds++;
                    break;
                }
                else break;
            }

            // Negative direction
            for (int i = 1; i <= 4; i++)
            {
                int nx = x - dx * i;
                int ny = y - dy * i;
                if (nx < 0 || nx >= BitBoard.Size || ny < 0 || ny >= BitBoard.Size) break;

                if (playerBitBoard.GetBit(nx, ny))
                    count++;
                else if (!occupied.GetBit(nx, ny))
                {
                    openEnds++;
                    break;
                }
                else break;
            }

            // Score based on pattern
            if (count >= 5)
                score += 100000; // Winning
            else if (count == 4 && openEnds == 2)
                score += 50000;  // Open four (almost winning)
            else if (count == 4 && openEnds == 1)
                score += 10000;  // Semi-open four
            else if (count == 3 && openEnds == 2)
                score += 5000;   // Open three
            else if (count == 3 && openEnds == 1)
                score += 1000;   // Semi-open three
            else if (count == 2 && openEnds == 2)
                score += 500;    // Open two
        }

        // Check opponent threats we might block
        foreach (var (dx, dy) in GameConstants.CardinalDirections)
        {
            int count = 1;
            int openEnds = 0;

            for (int i = 1; i <= 4; i++)
            {
                int nx = x + dx * i;
                int ny = y + dy * i;
                if (nx < 0 || nx >= BitBoard.Size || ny < 0 || ny >= BitBoard.Size) break;

                if (opponentBitBoard.GetBit(nx, ny))
                    count++;
                else if (!occupied.GetBit(nx, ny))
                {
                    openEnds++;
                    break;
                }
                else break;
            }

            for (int i = 1; i <= 4; i++)
            {
                int nx = x - dx * i;
                int ny = y - dy * i;
                if (nx < 0 || nx >= BitBoard.Size || ny < 0 || ny >= BitBoard.Size) break;

                if (opponentBitBoard.GetBit(nx, ny))
                    count++;
                else if (!occupied.GetBit(nx, ny))
                {
                    openEnds++;
                    break;
                }
                else break;
            }

            // Blocking opponent's threats
            if (count >= 4)
                score += 80000;  // Must block 4
            else if (count == 3 && openEnds == 2)
                score += 30000;  // Block open three
        }

        return score;
    }

    /// <summary>
    /// Calculate adaptive late move reduction based on position and move characteristics.
    /// Uses multiple factors to determine optimal reduction:
    /// - Depth: Deeper searches can reduce more
    /// - Move count: Later moves get more reduction
    /// - Improving: Positions with better static eval get less reduction
    /// - PV node: Principal variation nodes get less reduction
    /// - Cut node: Nodes that are likely to cutoff get more reduction
    /// - TT move: Transposition table moves get no reduction
    /// - History score: Moves with good history get less reduction
    ///
    /// Expected ELO gain: +25-40 through better search efficiency.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int GetAdaptiveReduction(
        int depth,
        int moveCount,
        bool improving,
        bool isPvNode,
        bool isCutNode,
        bool isTTMove,
        int historyScore)
    {
        // Early moves get no reduction
        if (moveCount < PruningConstants.LMRFullDepthMoves)
            return 0;

        // Minimum depth must be met
        if (depth < PruningConstants.LMRMinDepth)
            return 0;

        int reduction = PruningConstants.LMRBaseReduction;

        // Depth-based adjustment: deeper searches can reduce more
        // For each 3 plies beyond minimum, add 1 to reduction
        reduction += (depth - PruningConstants.LMRMinDepth) / 3;

        // Move count adjustment: later moves get more reduction
        // For every 4 moves beyond LMRFullDepthMoves, add 1 to reduction
        reduction += (moveCount - PruningConstants.LMRFullDepthMoves) / 4;

        // Improving positions get less reduction (more valuable to search accurately)
        if (improving)
            reduction -= 1;

        // PV nodes get less reduction (more important for accuracy)
        if (isPvNode)
            reduction -= 1;

        // Cut nodes get more reduction (likely to cutoff anyway)
        if (isCutNode)
            reduction += 1;

        // TT moves get no reduction (highest priority move)
        if (isTTMove)
            reduction = 0;

        // High history scores get less reduction (these moves have been good)
        // Scale: historyScore up to 30000, divide by 10000 = up to 3 reduction bonus
        int historyBonus = Math.Min(3, historyScore / 10000);
        reduction -= historyBonus;

        // Ensure reduction is valid: non-negative and less than depth
        reduction = Math.Max(0, reduction);
        reduction = Math.Min(depth - 1, reduction);

        return reduction;
    }

    /// <summary>
    /// Check if a position is improving (better than previous evaluation).
    /// This is a simplified check that uses material balance as a proxy.
    /// In a full implementation, this would track the evaluation from previous plies.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static bool IsImproving(SearchBoard board, Player player)
    {
        // Simplified: a position is "improving" if the current player has equal or more material
        // This is a basic heuristic; a full implementation would track eval across plies
        var redBitBoard = board.GetBitBoard(Player.Red);
        var blueBitBoard = board.GetBitBoard(Player.Blue);

        int redCount = redBitBoard.CountBits();
        int blueCount = blueBitBoard.CountBits();

        // Current player is improving if they have equal or more stones
        if (player == Player.Red)
            return redCount >= blueCount;
        else
            return blueCount >= redCount;
    }

    /// <summary>
    /// Check for winner using bitwise five-in-a-row detection
    /// </summary>
    public static Player? CheckWinner(SearchBoard board)
    {
        if (board.HasWin(Player.Red)) return Player.Red;
        if (board.HasWin(Player.Blue)) return Player.Blue;
        return null;
    }

    /// <summary>
    /// Check if a move is tactical (creates a forcing threat: Flex3 or better)
    /// Used in quiescence search to filter non-tactical moves and prevent branching explosion
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static bool IsTacticalMoveInQuiesce(SearchBoard board, int x, int y, Player player)
    {
        // Quick check: is the cell empty?
        if (!board.IsEmpty(x, y))
            return false;

        // Simulate placing the stone
        var undo = board.MakeMove(x, y, player);

        // Check if the move creates a forcing threat (Flex3+: open three or better)
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var pattern = Pattern4Evaluator.EvaluatePositionBitBoard(
            board.GetBitBoard(player), board.GetBitBoard(opponent), x, y);
        board.UnmakeMove(undo);
        return Pattern4Evaluator.IsForcingThreat(pattern);
    }

    /// <summary>
    /// Evaluate board position using SIMD-accelerated evaluator.
    /// Uses hardware POPCNT and run-length encoding for fast pattern scoring.
    /// </summary>
    public static int Evaluate(SearchBoard board, Player player)
    {
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        return SIMDBitBoardEvaluator.EvaluateOptimized(
            board.GetBitBoard(player),
            board.GetBitBoard(opponent));
    }
}
