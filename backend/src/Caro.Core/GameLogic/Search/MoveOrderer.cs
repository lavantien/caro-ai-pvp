using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic.Search;

/// <summary>
/// Orders candidate moves for better alpha-beta pruning efficiency.
/// Priority (optimized for Lazy SMP):
/// 1. Hash Move (TT Move) - UNCONDITIONAL #1 for thread work sharing
/// 2. Emergency Defense - blocks opponent's immediate threats (Open 4/Double 3)
/// 3. Winning Threats - creates own threats (Open 4, Double 3)
/// 4. Killer Moves - caused cutoffs at sibling nodes
/// 5. History/Butterfly Heuristic - general statistical sorting
/// 6. Positional Heuristics - center proximity, nearby stones
/// </summary>
public class MoveOrderer
{
    private const int BoardSize = GameConstants.BoardSize;

    // Move scoring constants (compact scale for within-stage sorting)
    private const int TtMovePriority = 10000;
    private const int EmergencyDefenseScore = 5000;
    private const int KillerMoveScore = 1000;
    private const int NearbyStoneBonus = 5;

    private readonly SearchHeuristics _heuristics;

    public MoveOrderer(SearchHeuristics heuristics)
    {
        _heuristics = heuristics;
    }

    /// <summary>
    /// Score candidates for tie-breaking when minimax scores are equal.
    /// Uses position heuristics similar to OrderMoves but without full sorting.
    /// Higher score = more desirable move.
    /// </summary>
    public int[] ScoreCandidatesForTiebreak(List<(int x, int y)> candidates, Board board, Player player, int depth)
    {
        int count = candidates.Count;
        var scores = new int[count];
        const int butterflySize = BoardSize;

        for (int i = 0; i < count; i++)
        {
            var (x, y) = candidates[i];
            var score = 0;

            // Bounds check - skip invalid coordinates
            if (x < 0 || x >= butterflySize || y < 0 || y >= butterflySize)
            {
                scores[i] = int.MinValue;
                continue;
            }

            // Killer moves get high priority
            if (_heuristics.IsKillerMove(depth, x, y))
                score += KillerMoveScore;

            // Butterfly heuristic
            var butterflyScore = _heuristics.GetButterflyScore(player, x, y);
            score += Math.Min(300, butterflyScore / 100);

            // History heuristic
            var historyScore = _heuristics.GetHistoryScore(player, x, y);
            score += Math.Min(500, historyScore / 10);

            // Tactical pattern scoring
            score += TacticalEvaluator.EvaluateTacticalPattern(board, x, y, player);

            // Prefer center proximity for 16x16 board
            var distanceToCenter = Math.Abs(x - GameConstants.CenterPosition) + Math.Abs(y - GameConstants.CenterPosition);
            score += ((GameConstants.BoardSize - 2) - distanceToCenter) * 10;

            // Prefer moves near existing stones
            var nearby = 0;
            for (int dx = -1; dx <= 1; dx++)
            {
                for (int dy = -1; dy <= 1; dy++)
                {
                    if (dx == 0 && dy == 0) continue;
                    var nx = x + dx;
                    var ny = y + dy;
                    if (nx >= 0 && nx < BoardSize && ny >= 0 && ny < BoardSize)
                    {
                        var cell = board.GetCell(nx, ny);
                        if (cell.Player != Player.None)
                            nearby += NearbyStoneBonus;
                    }
                }
            }
            score += nearby;

            scores[i] = score;
        }

        return scores;
    }

    /// <summary>
    /// Order moves for an immutable Board using alpha-beta move ordering heuristics.
    /// Zero-allocation implementation using stackalloc and insertion sort.
    /// </summary>
    public List<(int x, int y)> OrderMoves(List<(int x, int y)> candidates, int depth, Board board, Player player, (int x, int y)? ttMove = null)
    {
        int count = candidates.Count;
        if (count <= 1) return candidates;

        Span<int> scores = stackalloc int[count];

        for (int i = 0; i < count; i++)
        {
            var (x, y) = candidates[i];
            var score = 0;

            // PRIORITY #1: Hash Move (TT Move)
            if (ttMove.HasValue && x == ttMove.Value.x && y == ttMove.Value.y)
            {
                score = TtMovePriority;
            }
            else
            {
                // PRIORITY #2: Emergency Defense
                if (TacticalEvaluator.IsEmergencyDefense(board, x, y, player))
                    score += EmergencyDefenseScore;

                // PRIORITY #3: Winning Threats
                score += TacticalEvaluator.EvaluateTacticalPattern(board, x, y, player);

                // PRIORITY #4: Killer Moves
                if (_heuristics.IsKillerMove(depth, x, y))
                    score += KillerMoveScore;

                // PRIORITY #5: History/Butterfly Heuristic
                var butterflyScore = _heuristics.GetButterflyScore(player, x, y);
                score += Math.Min(300, butterflyScore / 100);

                var historyScore = _heuristics.GetHistoryScore(player, x, y);
                score += Math.Min(500, historyScore / 10);

                // PRIORITY #6: Positional Heuristics
                var distanceToCenter = Math.Abs(x - GameConstants.CenterPosition) + Math.Abs(y - GameConstants.CenterPosition);
                score += ((GameConstants.BoardSize - 2) - distanceToCenter) * 10;

                var nearby = 0;
                for (int dx = -1; dx <= 1; dx++)
                {
                    for (int dy = -1; dy <= 1; dy++)
                    {
                        if (dx == 0 && dy == 0) continue;
                        var nx = x + dx;
                        var ny = y + dy;
                        if (nx >= 0 && nx < BoardSize && ny >= 0 && ny < BoardSize)
                        {
                            var cell = board.GetCell(nx, ny);
                            if (cell.Player != Player.None)
                                nearby += NearbyStoneBonus;
                        }
                    }
                }
                score += nearby;
            }

            scores[i] = score;
        }

        // Insertion sort (fast for small arrays, no allocations)
        for (int i = 1; i < count; i++)
        {
            var keyMove = candidates[i];
            var keyScore = scores[i];
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

        return candidates;
    }

    /// <summary>
    /// Order moves for a mutable SearchBoard (high-performance path).
    /// Uses same heuristics as Board version but optimized for SearchBoard.
    /// </summary>
    public List<(int x, int y)> OrderMoves(List<(int x, int y)> candidates, int depth, SearchBoard board, Player player, (int x, int y)? ttMove = null)
    {
        int count = candidates.Count;
        if (count <= 1) return candidates;

        Span<int> scores = stackalloc int[count];

        for (int i = 0; i < count; i++)
        {
            var (x, y) = candidates[i];
            var score = 0;

            // PRIORITY #1: Hash Move (TT Move)
            if (ttMove.HasValue && x == ttMove.Value.x && y == ttMove.Value.y)
            {
                score = TtMovePriority;
            }
            else
            {
                // PRIORITY #2: Emergency Defense
                if (TacticalEvaluator.IsEmergencyDefense(board, x, y, player))
                    score += EmergencyDefenseScore;

                // PRIORITY #3: Winning Threats
                score += TacticalEvaluator.EvaluateTacticalPattern(board, x, y, player);

                // PRIORITY #4: Killer Moves
                if (_heuristics.IsKillerMove(depth, x, y))
                    score += KillerMoveScore;

                // PRIORITY #5: History/Butterfly Heuristic
                var butterflyScore = _heuristics.GetButterflyScore(player, x, y);
                score += Math.Min(300, butterflyScore / 100);

                var historyScore = _heuristics.GetHistoryScore(player, x, y);
                score += Math.Min(500, historyScore / 10);

                // PRIORITY #6: Positional Heuristics
                var distanceToCenter = Math.Abs(x - GameConstants.CenterPosition) + Math.Abs(y - GameConstants.CenterPosition);
                score += ((GameConstants.BoardSize - 2) - distanceToCenter) * 10;

                var nearby = 0;
                for (int dx = -1; dx <= 1; dx++)
                {
                    for (int dy = -1; dy <= 1; dy++)
                    {
                        if (dx == 0 && dy == 0) continue;
                        var nx = x + dx;
                        var ny = y + dy;
                        if (nx >= 0 && nx < BoardSize && ny >= 0 && ny < BoardSize)
                        {
                            if (!board.IsEmpty(nx, ny))
                                nearby += NearbyStoneBonus;
                        }
                    }
                }
                score += nearby;
            }

            scores[i] = score;
        }

        // Insertion sort
        for (int i = 1; i < count; i++)
        {
            var keyMove = candidates[i];
            var keyScore = scores[i];
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

        return candidates;
    }
}
