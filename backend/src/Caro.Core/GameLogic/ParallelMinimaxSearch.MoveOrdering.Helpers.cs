using System.Runtime.CompilerServices;
using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic.Search;

namespace Caro.Core.GameLogic;

public sealed partial class ParallelMinimaxSearch
{
    /// Convert ParallelMinimaxSearch.ThreadData to MovePicker.ThreadData.
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

    /// Legacy move ordering for testing continuation history integration.
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
            int currentCell = y * BitBoard.Size + x;
            int continuationScore = 0;
            for (int j = 0; j < threadData.MoveHistoryCount && j < ContinuationHistory.TrackedPlyCount; j++)
            {
                int prevCell = threadData.MoveHistory[j];
                continuationScore += _continuationHistory.GetScore(player, prevCell, currentCell);
            }
            score += Math.Min(continuationScore * 3, 300000);

            // 3b. Counter-Move History (Response to opponent's last move)
            int counterMoveScore = _counterMoveHistory.GetScore(player, threadData.LastOpponentCell, currentCell);
            score += Math.Min(counterMoveScore * 2, 150000);

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

    /// Insertion sort with zero allocations
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

    /// Zero-allocation in-place sort over spans using insertion sort.
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

    /// Get candidate moves near existing stones
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
}
