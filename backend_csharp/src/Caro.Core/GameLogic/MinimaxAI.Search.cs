using System.Runtime.CompilerServices;
using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic.Search;
using SHC = Caro.Core.Domain.Configuration.SearchHeuristicConstants;

namespace Caro.Core.GameLogic;

/// <summary>
/// MinimaxAI partial class - Search algorithms.
/// Iterative deepening, minimax, quiescence search, and search helpers.
/// </summary>
public partial class MinimaxAI
{
    private (int x, int y, int score) SearchWithDepth(Board board, Player player, int depth, List<(int x, int y)> candidates)
    {
        var bestScore = int.MinValue;
        var bestMove = candidates[0];
        int bestTiebreaker = 0;

        var boardHash = _transpositionTable.CalculateHash(board);

        _searchBoard.CopyFrom(new SearchBoard(board));

        // Use previous ID iteration score as aspiration estimate (no pre-search)
        int estimatedScore = _lastSearchScore;
        int delta = SHC.AspirationWindow;
        const int maxWidenings = 3;

        // No aspiration for depth 1 (no previous score reliable) or if no prior estimate
        bool useAspiration = depth > 1 && estimatedScore != 0;

        var alpha = useAspiration ? estimatedScore - delta : int.MinValue;
        var beta = useAspiration ? estimatedScore + delta : int.MaxValue;

        for (int widening = 0; widening <= maxWidenings; widening++)
        {
            // TT lookup with current window
            _tableLookups++;
            var (found, cachedScore, cachedMove) = _transpositionTable.Lookup(boardHash, depth, alpha, beta);
            if (found && cachedMove.HasValue)
            {
                var (cx, cy) = cachedMove.Value;
                if (cx >= 0 && cx < BoardSize && cy >= 0 && cy < BoardSize)
                {
                    var cell = board.GetCell(cx, cy);
                    if (cell.IsEmpty)
                    {
                        _tableHits++;
                        return (cx, cy, cachedScore);
                    }
                }
            }

            bestScore = int.MinValue;
            bestMove = candidates[0];
            bestTiebreaker = 0;

            var orderedMoves = _moveOrderer.OrderMoves(candidates, depth, board, player, cachedMove);
            var orderedTiebreakScores = _moveOrderer.ScoreCandidatesForTiebreak(orderedMoves, board, player, depth);

            bool failHigh = false;
            bool failLow = true; // Assume fail-low until a score > alpha
            int orderedIdx = 0;
            foreach (var (x, y) in orderedMoves)
            {
                if (_searchStopwatch.ElapsedMilliseconds >= _searchHardBoundMs)
                {
                    _searchStopped = true;
                    return (bestMove.x, bestMove.y, bestScore);
                }

                var undo = _searchBoard.MakeMove(x, y, player);
                var score = MinimaxCore(_searchBoard, depth - 1, alpha, beta, false, player, depth);
                _searchBoard.UnmakeMove(undo);

                if (_searchStopped)
                    return (bestMove.x, bestMove.y, bestScore);

                if (score > bestScore)
                {
                    bestScore = score;
                    bestMove = (x, y);
                    bestTiebreaker = orderedTiebreakScores[orderedIdx];
                }
                else if (score == bestScore)
                {
                    var currentTiebreaker = orderedTiebreakScores[orderedIdx];
                    var randomBonus = NextRandomInt(SHC.RandomBonusRange);
                    if (currentTiebreaker + randomBonus > bestTiebreaker)
                    {
                        bestMove = (x, y);
                        bestTiebreaker = currentTiebreaker + randomBonus;
                    }
                }

                if (score > alpha)
                {
                    alpha = score;
                    failLow = false;
                }

                if (beta <= alpha)
                {
                    RecordKillerMove(depth, x, y);
                    RecordHistoryMove(player, x, y, depth);
                    break;
                }

                if (score >= beta)
                {
                    failHigh = true;
                    break;
                }
                orderedIdx++;
            }

            // Success: score is within window
            if (!failHigh && !failLow)
            {
                _transpositionTable.Store(boardHash, depth, bestScore, bestMove, alpha, beta);
                return (bestMove.x, bestMove.y, bestScore);
            }

            // Widening failed after max attempts - return best found
            if (widening == maxWidenings)
            {
                _transpositionTable.Store(boardHash, depth, bestScore, bestMove, int.MinValue, int.MaxValue);
                return (bestMove.x, bestMove.y, bestScore);
            }

            // Incremental widening: double delta, widen only the failed bound
            delta *= 2;
            if (failHigh)
                beta = estimatedScore + delta;
            else // failLow
                alpha = estimatedScore - delta;
        }

        return (bestMove.x, bestMove.y, bestScore);
    }

    private void RecordKillerMove(int depth, int x, int y) { _heuristics.RecordKillerMove(depth, x, y); }

    /// <summary>
    /// Record a move that caused a cutoff in the history table
    /// Higher depth = more significant = larger bonus
    /// </summary>
    private void RecordHistoryMove(Player player, int x, int y, int depth) { _heuristics.RecordHistoryMove(player, x, y, depth); }

    /// <summary>
    /// Get the history score for a move
    /// </summary>
    private int GetHistoryScore(Player player, int x, int y) => _heuristics.GetHistoryScore(player, x, y);

    /// <summary>
    /// Clear history tables (call at start of new game)
    /// </summary>
    public void ClearHistory()
    {
        _heuristics.ClearHistory();
    }

    /// <summary>
    /// Clear search state for new position while preserving transposition table.
    /// Clears: history tables, killer moves, pondering state.
    /// Preserves: transposition table entries (memoization), adaptive time state.
    /// </summary>
    public void ClearSearchState()
    {
        ClearHistory();
        ResetPondering();

        // Clear killer moves (position-specific move ordering)
        _heuristics.ClearKillers();

        // Note: Transposition table is NOT cleared - this preserves memoization
        // TT entries will be aged out naturally via the depth-age replacement strategy
    }

    /// <summary>
    /// Clear all AI state between games to prevent cross-contamination
    /// This is critical when AI of different difficulties play in sequence
    /// </summary>
    public void ClearAllState()
    {
        ClearHistory();
        _transpositionTable.Clear();
        _parallelSearch.Clear();  // Also clear parallel search's TT
        ResetPondering();

        // Reset adaptive time manager state
        _adaptiveTimeManager.Reset();

        // Reset inferred initial time for adaptive thresholds
        // -1 means "unknown, will infer from first move"
        _inferredInitialTimeMs = -1;

        // Clear killer moves
        _heuristics.ClearKillers();

        // Reset statistics
        _nodesSearched = 0;
        _depthAchieved = 0;
        _vcfNodesSearched = 0;
        _vcfDepthAchieved = 0;
        _tableHits = 0;
        _tableLookups = 0;

        // Clear parallel search state
        _parallelSearch.Clear();

        // Reset PV prediction state for pondering
        _lastPV = PV.Empty;
        _lastBoard = null;
    }

    /// <summary>
    /// Clear the transposition table to prevent position leakage between games.
    /// Use this for self-play scenarios where you want to reset search state
    /// without clearing all AI configuration.
    /// </summary>
    public void ClearTranspositionTable()
    {
        _transpositionTable.Clear();
        _parallelSearch.Clear();  // Also clear parallel search's TT
    }

    /// <summary>
    /// Resize transposition table. Clears and rebuilds both main and parallel search TTs.
    /// </summary>
    public void ResizeTranspositionTable(int newSizeMb)
    {
        var oldParallel = _parallelSearch;
        _transpositionTable = new TranspositionTable(newSizeMb);
        _parallelSearch = new ParallelMinimaxSearch(newSizeMb);
        oldParallel?.Dispose();
    }

    /// <summary>
    /// Check if there's a winner using SearchBoard (high-performance path).
    /// Uses bitboard-based 5-in-a-row detection.
    /// </summary>
    private Player? CheckWinner(SearchBoard board)
    {
        if (board.HasWin(Player.Red))
            return Player.Red;
        if (board.HasWin(Player.Blue))
            return Player.Blue;
        return null;
    }

    /// <summary>
    /// Check if there's a winner on the board using WinDetector
    /// This ensures Caro rules are enforced: exact 5-in-a-row, no sandwiched wins, no overlines
    /// </summary>
    private Player? CheckWinner(Board board)
    {
        var result = _winDetector.CheckWin(board);
        return result.HasWinner ? result.Winner : null;
    }

    /// <summary>
    /// Calculate LMR reduction based on move index and depth
    /// More aggressive reduction for later moves at higher depths
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private int CalculateLMRReduction(int depth, int moveIndex, bool isCriticalMove)
    {
        // Critical moves (threats/blocks) get no reduction
        if (isCriticalMove) return 0;

        // Base reduction: floor(depth/3) + floor(moveIndex/3)
        int reduction = depth / 3 + moveIndex / 3;

        // Cap reduction at depth-2 (always search at least 2 ply)
        return Math.Min(reduction, depth - 2);
    }

    /// <summary>
    /// ProbCut: Probabilistic cutoff for deep searches
    /// Try a shallow search first; if it shows clear cutoff, skip deep search
    /// </summary>
    private bool TryProbCut(Board board, int depth, int alpha, int beta, bool isMaximizing, Player aiPlayer, int rootDepth)
    {
        // Only use ProbCut at depth 5+ when we have a narrow window
        if (depth < 5 || (beta - alpha) > 100) return false;

        // Try shallow search at reduced depth
        int probCutDepth = depth / 2;
        int probCutBeta = beta + SHC.ProbCutMargin;

        var score = Minimax(board, probCutDepth, probCutBeta - 1, probCutBeta, isMaximizing, aiPlayer, rootDepth);

        // If shallow search already exceeds beta, we're likely to cutoff
        return score >= probCutBeta;
    }
}
