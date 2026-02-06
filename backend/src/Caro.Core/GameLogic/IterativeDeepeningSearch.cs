using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Iterative deepening search with time budget control.
/// Searches progressively deeper (depth 1, 2, 3...) until time runs out.
/// Always returns the best move from the last completed iteration.
/// </summary>
public sealed class IterativeDeepeningSearch
{
    private readonly Func<Board, Player, int, int, int, bool, Player, int, (int score, int nodes)> _searchFunc;
    private readonly Action<int, long>? _onIterationComplete;
    private readonly System.Diagnostics.Stopwatch _stopwatch = new();

    /// <summary>
    /// Result of iterative deepening search.
    /// </summary>
    public readonly record struct SearchResult(
        int X,
        int Y,
        int DepthAchieved,
        long NodesSearched,
        double ElapsedSeconds,
        int Score
    );

    /// <summary>
    /// Create iterative deepening search.
    ///
    /// searchFunc: (board, player, depth, alpha, beta, nullMove, rootPlayer, currentDepth) => (score, nodes)
    /// </summary>
    public IterativeDeepeningSearch(
        Func<Board, Player, int, int, int, bool, Player, int, (int score, int nodes)> searchFunc,
        Action<int, long>? onIterationComplete = null)
    {
        _searchFunc = searchFunc;
        _onIterationComplete = onIterationComplete;
    }

    /// <summary>
    /// Run iterative deepening search within time budget.
    ///
    /// Returns best move from the deepest completed iteration when time expires.
    /// </summary>
    public SearchResult Search(
        Board board,
        Player player,
        List<(int x, int y)> candidates,
        int minDepth,
        int maxDepth,
        double softBoundSeconds,
        double hardBoundSeconds)
    {
        if (candidates.Count == 0)
            throw new ArgumentException("No candidates to search");

        _stopwatch.Restart();

        // Best result from completed iterations
        (int bestX, int bestY) = candidates[0];
        int bestDepth = minDepth;
        long bestNodes = 0;
        int bestScore = int.MinValue;
        long previousIterationNodes = 1; // Start with 1 to avoid division by zero

        // Order candidates once at the start
        var orderedCandidates = OrderCandidatesByProximity(candidates, board);

        // Iterate deeper until time runs out
        for (int depth = minDepth; depth <= maxDepth; depth++)
        {
            double elapsedSeconds = _stopwatch.Elapsed.TotalSeconds;

            // Check hard bound (must stop)
            if (elapsedSeconds >= hardBoundSeconds)
                break;

            // Check if we have time for this iteration
            // Estimate: previous_nodes * EBF (rough estimate 2.5)
            double estimatedTimeForIteration = elapsedSeconds * 2.5;
            double remainingTime = softBoundSeconds - elapsedSeconds;

            // If we're past soft bound and this iteration would exceed hard bound, stop
            if (elapsedSeconds > softBoundSeconds && elapsedSeconds + estimatedTimeForIteration > hardBoundSeconds)
                break;

            // Search at this depth
            long iterationNodes = 0;
            int iterationBestScore = int.MinValue;
            (int iterX, int iterY) = orderedCandidates[0];

            int alpha = int.MinValue;
            int beta = int.MaxValue;

            // Search each candidate
            foreach (var (x, y) in orderedCandidates)
            {
                // Check time during move loop
                elapsedSeconds = _stopwatch.Elapsed.TotalSeconds;
                if (elapsedSeconds >= hardBoundSeconds)
                    break;

                board.GetCell(x, y).SetPlayerUnsafe(player);

                var (score, nodes) = _searchFunc(board, player, depth - 1, alpha, beta, false, player, depth);

                board.GetCell(x, y).SetPlayerUnsafe(Player.None);

                iterationNodes += nodes;

                if (score > iterationBestScore)
                {
                    iterationBestScore = score;
                    (iterX, iterY) = (x, y);
                }

                alpha = Math.Max(alpha, score);
            }

            // Update best result (this iteration completed)
            bestX = iterX;
            bestY = iterY;
            bestDepth = depth;
            bestNodes = iterationNodes;
            bestScore = iterationBestScore;

            // Report iteration completion
            _onIterationComplete?.Invoke(depth, iterationNodes);

            // Check if we should continue
            elapsedSeconds = _stopwatch.Elapsed.TotalSeconds;

            // Stop if we've used most of soft bound
            if (elapsedSeconds >= softBoundSeconds * 0.9)
                break;

            // Stop if next iteration would likely exceed hard bound
            double estimatedNextTime = elapsedSeconds * (iterationNodes / (double)previousIterationNodes);
            if (elapsedSeconds + estimatedNextTime > hardBoundSeconds)
                break;

            previousIterationNodes = iterationNodes;
        }

        _stopwatch.Stop();

        return new SearchResult(
            bestX, bestY,
            bestDepth,
            bestNodes,
            _stopwatch.Elapsed.TotalSeconds,
            bestScore
        );
    }

    /// <summary>
    /// Order candidates by proximity to center and existing stones.
    /// Simple heuristic for initial move ordering.
    /// </summary>
    private static List<(int x, int y)> OrderCandidatesByProximity(
        List<(int x, int y)> candidates,
        Board board)
    {
        int center = board.BoardSize / 2;

        return candidates
            .OrderBy(c =>
            {
                // Prioritize center
                int distToCenter = Math.Abs(c.x - center) + Math.Abs(c.y - center);

                // Prioritize squares near existing stones
                int distToNearestStone = int.MaxValue;
                for (int i = 0; i < board.BoardSize; i++)
                {
                    for (int j = 0; j < board.BoardSize; j++)
                    {
                        var cell = board.GetCell(i, j);
                        if (!cell.IsEmpty)
                        {
                            int dist = Math.Abs(c.x - i) + Math.Abs(c.y - j);
                            distToNearestStone = Math.Min(distToNearestStone, dist);
                        }
                    }
                }

                return distToCenter * 2 + distToNearestStone;
            })
            .ToList();
    }
}
