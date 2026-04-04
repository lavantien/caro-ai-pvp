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

    /// <summary>
    /// Convert ParallelMinimaxSearch.ThreadData to MovePicker.ThreadData.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static MovePicker.ThreadData ConvertToPickerThreadData(ThreadData source)
    {
        var target = new MovePicker.ThreadData
        {
            ThreadIndex = source.ThreadIndex,
            MoveHistoryCount = source.MoveHistoryCount,
            LastOpponentCell = source.LastOpponentCell
        };

        // Copy killer moves
        for (int i = 0; i < 20; i++)
        {
            target.KillerMoves[i, 0] = source.KillerMoves[i, 0];
            target.KillerMoves[i, 1] = source.KillerMoves[i, 1];
        }

        // Copy history tables
        for (int x = 0; x < BitBoard.Size; x++)
        {
            for (int y = 0; y < BitBoard.Size; y++)
            {
                target.HistoryRed[x, y] = source.HistoryRed[x, y];
                target.HistoryBlue[x, y] = source.HistoryBlue[x, y];
            }
        }

        // Copy move history
        for (int i = 0; i < source.MoveHistoryCount && i < source.MoveHistory.Length; i++)
        {
            target.MoveHistory[i] = source.MoveHistory[i];
        }

        return target;
    }

    /// <summary>
    /// <summary>
    /// Legacy move ordering for testing continuation history integration.
    /// Production code uses OrderMovesStaged with MovePicker for better performance.
    /// </summary>
    private List<(int x, int y)> OrderMovesLegacyForTesting(List<(int x, int y)> candidates, int depth, Board board, Player player, (int x, int y)? cachedMove, ThreadData threadData)
    {
        var searchBoard = new SearchBoard(board);
        int count = candidates.Count;
        if (count == 0) return candidates; // Safety check
        if (count == 1) return candidates;

        Span<int> scores = stackalloc int[count];
        var historyTable = player == Player.Red ? threadData.HistoryRed : threadData.HistoryBlue;

        // Check filtering for threats (Optimization from previous fix)
        // Note: For Lazy SMP to work best, we should usually search ALL candidates,
        // but high priority moves must come first.
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var threatMoves = ParallelThreatAnalyzer.GetOpponentThreatMoves(board, opponent, _winDetector);

        for (int i = 0; i < count; i++)
        {
            var (x, y) = candidates[i];
            int score = 0;

            // 1. Mandatory Blocks (Highest Priority - Deterministic)
            if (threatMoves.Contains((x, y)))
                score += 2000000;

            // 2. Hash Move (High Priority)
            if (cachedMove.HasValue && cachedMove.Value == (x, y))
                score += 1000000;

            // 3. Continuation History (Higher priority than killer moves)
            // Weight formula: 2 * mainHistory + sum(continuationHistory[0..2])
            // Continuation history tracks which moves have been good after previous moves
            int currentCell = y * BitBoard.Size + x;
            int continuationScore = 0;
            for (int j = 0; j < threadData.MoveHistoryCount && j < ContinuationHistory.TrackedPlyCount; j++)
            {
                int prevCell = threadData.MoveHistory[j];
                continuationScore += _continuationHistory.GetScore(player, prevCell, currentCell);
            }
            // Weight: continuation history gets bonus up to 300000
            score += Math.Min(continuationScore * 3, 300000);

            // 3b. Counter-Move History (Response to opponent's last move)
            // Tracks which responses have been good after opponent's specific moves
            int counterMoveScore = _counterMoveHistory.GetScore(player, threadData.LastOpponentCell, currentCell);
            // Weight: counter-move history gets bonus up to 150000 (half of continuation)
            score += Math.Min(counterMoveScore * 2, 150000);

            // 4. Winning Move (Tactical)
            // (You can call EvaluateTactical here if you want greater precision)

            // 5. Killer Moves
            if (depth < 20)
            {
                if (threadData.KillerMoves[depth, 0] == (x, y)) score += 500000;
                else if (threadData.KillerMoves[depth, 1] == (x, y)) score += 400000;
            }

            // 6. History Heuristic (weighted 2x as part of composite score)
            score += Math.Min(historyTable[x, y] * 2, MoveOrderingConstants.HistoryScoreMax);

            // 7. Center Preference & Proximity
            int center = board.BoardSize / 2;
            int centerDist = Math.Abs(x - center) + Math.Abs(y - center);
            score += ((board.BoardSize * 2 - 4) - centerDist) * 100;
            score += GetProximityScore(x, y, searchBoard) * 10;

            scores[i] = score;
        }

        return InsertionSort(candidates, scores);
    }

    /// <summary>
    /// Insertion sort with zero allocations
    /// </summary>
    private List<(int x, int y)> InsertionSort(List<(int x, int y)> moves, Span<int> scores)
    {
        for (int i = 1; i < moves.Count; i++)
        {
            int j = i;
            while (j > 0 && scores[j] > scores[j - 1])
            {
                // Swap moves
                var tempMove = moves[j];
                moves[j] = moves[j - 1];
                moves[j - 1] = tempMove;

                // Swap scores
                int tempScore = scores[j];
                scores[j] = scores[j - 1];
                scores[j - 1] = tempScore;

                j--;
            }
        }
        return moves;
    }

    /// <summary>
    /// Zero-allocation in-place sort over spans using insertion sort.
    /// Good for small arrays (typical candidate move counts), no GC pressure.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static void OrderMovesSpan(Span<(int x, int y)> moves, Span<int> scores)
    {
        for (int i = 1; i < moves.Length; i++)
        {
            var keyMove = moves[i];
            int keyScore = scores[i];
            int j = i - 1;
            while (j >= 0 && scores[j] < keyScore)
            {
                moves[j + 1] = moves[j];
                scores[j + 1] = scores[j];
                j--;
            }
            moves[j + 1] = keyMove;
            scores[j + 1] = keyScore;
        }
    }

    /// <summary>
    /// Get candidate moves near existing stones
    /// For empty board, returns center-area moves for the opening
    /// </summary>
    private List<(int x, int y)> GetCandidateMoves(SearchBoard board, int searchRadius)
    {
        var candidates = new List<(int x, int y)>(64);
        int boardSize = board.BoardSize;
        Span<bool> considered = stackalloc bool[boardSize * boardSize];

        var playerBitBoard = board.GetBitBoard(Player.Red);
        var opponentBitBoard = board.GetBitBoard(Player.Blue);
        var occupied = playerBitBoard | opponentBitBoard;

        // Check if board is empty (no stones placed)
        bool boardIsEmpty = true;
        for (int x = 0; x < boardSize && boardIsEmpty; x++)
        {
            for (int y = 0; y < boardSize && boardIsEmpty; y++)
            {
                if (occupied.GetBit(x, y))
                    boardIsEmpty = false;
            }
        }

        // Empty board - return center-area moves for opening
        if (boardIsEmpty)
        {
            // Return center 3x3 area as candidates (standard opening positions)
            int center = boardSize / 2;
            for (int x = center - 1; x <= center + 1; x++)
            {
                for (int y = center - 1; y <= center + 1; y++)
                {
                    candidates.Add((x, y));
                }
            }
            return candidates;
        }

        // Find all cells within searchRadius of existing stones
        for (int x = 0; x < boardSize; x++)
        {
            for (int y = 0; y < boardSize; y++)
            {
                if (occupied.GetBit(x, y))
                {
                    // Add neighbors as candidates
                    for (int dx = -searchRadius; dx <= searchRadius; dx++)
                    {
                        for (int dy = -searchRadius; dy <= searchRadius; dy++)
                        {
                            int nx = x + dx;
                            int ny = y + dy;
                            if (nx >= 0 && nx < boardSize && ny >= 0 && ny < boardSize &&
                                !occupied.GetBit(nx, ny) && !considered[nx * boardSize + ny])
                            {
                                candidates.Add((nx, ny));
                                considered[nx * boardSize + ny] = true;
                            }
                        }
                    }
                }
            }
        }

        return candidates;
    }

    /// <summary>
    /// Calculate proximity score (prefer moves near existing stones)
    /// </summary>
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

    /// <summary>
    /// Record killer move
    /// </summary>
    private void RecordKillerMove(ThreadData threadData, int depth, int x, int y)
    {
        if (depth >= 0 && depth < 20)
        {
            // Shift existing killers
            threadData.KillerMoves[depth, 1] = threadData.KillerMoves[depth, 0];
            threadData.KillerMoves[depth, 0] = (x, y);
        }
    }

    /// <summary>
    /// Record history move
    /// </summary>
    private void RecordHistoryMove(ThreadData threadData, Player player, int x, int y, int depth)
    {
        var table = player == Player.Red ? threadData.HistoryRed : threadData.HistoryBlue;
        table[x, y] += depth * depth;
    }
}
