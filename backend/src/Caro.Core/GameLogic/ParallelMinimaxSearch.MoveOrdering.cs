using System.Runtime.CompilerServices;
using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic.Search;

namespace Caro.Core.GameLogic;

public sealed partial class ParallelMinimaxSearch
{
    /// <summary>
    /// Order moves using fast zero-allocation scoring.
    /// PERFORMANCE FIX: Previous MovePicker implementation scanned entire board (225 cells)
    /// 3 times per call, causing 100x NPS slowdown vs sequential search.
    /// This version only evaluates the candidate moves themselves.
    /// </summary>
    private List<(int x, int y)> OrderMovesStaged(
        List<(int x, int y)> candidates,
        int depth,
        SearchBoard board,
        Player player,
        (int x, int y)? cachedMove,
        ThreadData threadData)
    {
        int count = candidates.Count;
        if (count <= 1) return candidates;

        // Use stack allocation for scores (zero heap allocation)
        Span<int> scores = stackalloc int[count];
        var historyTable = player == Player.Red ? threadData.HistoryRed : threadData.HistoryBlue;
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var playerBitBoard = board.GetBitBoard(player);
        var opponentBitBoard = board.GetBitBoard(opponent);

        for (int i = 0; i < count; i++)
        {
            var (x, y) = candidates[i];
            int score = 0;

            // 1. TT Move (highest priority)
            if (cachedMove.HasValue && cachedMove.Value == (x, y))
            {
                scores[i] = MoveOrderingConstants.TtMoveScore * 10;
                continue;
            }

            // 2. Tactical evaluation using BitBoard (fast)
            score += ParallelNodeEvaluator.EvaluateTacticalFast(board, x, y, player, playerBitBoard, opponentBitBoard);

            // 3. Killer moves
            if (depth >= 0 && depth < 20)
            {
                if (threadData.KillerMoves[depth, 0] == (x, y))
                    score += MoveOrderingConstants.KillerScore1;
                else if (threadData.KillerMoves[depth, 1] == (x, y))
                    score += MoveOrderingConstants.KillerScore2;
            }

            // 4. History heuristic
            score += Math.Min(historyTable[x, y] * 2, MoveOrderingConstants.HistoryScoreMax);

            // 5. Center preference
            int center = board.BoardSize / 2;
            int centerDist = Math.Abs(x - center) + Math.Abs(y - center);
            score += ((board.BoardSize * 2 - 4) - centerDist) * 100;

            // 6. Nearby stones bonus
            score += GetProximityScore(x, y, board) * 10;

            scores[i] = score;
        }

        // Insertion sort (fast for small arrays)
        for (int i = 1; i < count; i++)
        {
            int j = i;
            while (j > 0 && scores[j] > scores[j - 1])
            {
                var tmpC = candidates[j]; candidates[j] = candidates[j - 1]; candidates[j - 1] = tmpC;
                int tmpS = scores[j]; scores[j] = scores[j - 1]; scores[j - 1] = tmpS;
                j--;
            }
        }

        return candidates;
    }

    /// <summary>
    /// Zero-allocation Span-based version of OrderMovesStaged.
    /// Scores and sorts candidates in-place using the same logic.
    /// </summary>
    private void OrderMovesStagedSpan(
        Span<(int x, int y)> candidates,
        int depth,
        SearchBoard board,
        Player player,
        (int x, int y)? cachedMove,
        ThreadData threadData)
    {
        int count = candidates.Length;
        if (count <= 1) return;

        Span<int> scores = stackalloc int[count];
        var historyTable = player == Player.Red ? threadData.HistoryRed : threadData.HistoryBlue;
        var playerBitBoard = board.GetBitBoard(player);
        var opponentBitBoard = board.GetBitBoard(player == Player.Red ? Player.Blue : Player.Red);

        for (int i = 0; i < count; i++)
        {
            var (x, y) = candidates[i];
            int score = 0;

            if (cachedMove.HasValue && cachedMove.Value == (x, y))
            {
                scores[i] = MoveOrderingConstants.TtMoveScore * 10;
                continue;
            }

            score += ParallelNodeEvaluator.EvaluateTacticalFast(board, x, y, player, playerBitBoard, opponentBitBoard);

            if (depth >= 0 && depth < 20)
            {
                if (threadData.KillerMoves[depth, 0] == (x, y))
                    score += MoveOrderingConstants.KillerScore1;
                else if (threadData.KillerMoves[depth, 1] == (x, y))
                    score += MoveOrderingConstants.KillerScore2;
            }

            score += Math.Min(historyTable[x, y] * 2, MoveOrderingConstants.HistoryScoreMax);

            int center = board.BoardSize / 2;
            int centerDist = Math.Abs(x - center) + Math.Abs(y - center);
            score += ((board.BoardSize * 2 - 4) - centerDist) * 100;

            score += GetProximityScore(x, y, board) * 10;

            scores[i] = score;
        }

        // Insertion sort descending by score
        for (int i = 1; i < count; i++)
        {
            var keyMove = candidates[i];
            int keyScore = scores[i];
            int j = i - 1;
            while (j >= 0 && scores[j] < keyScore)
            {
                candidates[j + 1] = candidates[j];
                scores[j + 1] = scores[j];
                j--;
            }
            candidates[j + 1] = keyMove;
            scores[j + 1] = keyScore;
        }
    }

    /// Calculate proximity score (prefer moves near existing stones)
    private int GetProximityScore(int x, int y, SearchBoard board)
    {
        int boardSize = board.BoardSize;
        var playerBitBoard = board.GetBitBoard(Player.Red);
        var opponentBitBoard = board.GetBitBoard(Player.Blue);
        int score = 0;

        for (int dx = -2; dx <= 2; dx++)
        {
            for (int dy = -2; dy <= 2; dy++)
            {
                int nx = x + dx;
                int ny = y + dy;
                if (nx >= 0 && nx < boardSize && ny >= 0 && ny < boardSize)
                {
                    if (playerBitBoard.GetBit(nx, ny)) score += 3;
                    if (opponentBitBoard.GetBit(nx, ny)) score += 2;
                }
            }
        }

        return score;
    }

    /// Record killer move
    private void RecordKillerMove(ThreadData threadData, int depth, int x, int y)
    {
        if (depth >= 0 && depth < 20)
        {
            threadData.KillerMoves[depth, 1] = threadData.KillerMoves[depth, 0];
            threadData.KillerMoves[depth, 0] = (x, y);
        }
    }

    /// Record history move
    private void RecordHistoryMove(ThreadData threadData, Player player, int x, int y, int depth)
    {
        var table = player == Player.Red ? threadData.HistoryRed : threadData.HistoryBlue;
        table[x, y] += depth * depth;
    }
}
