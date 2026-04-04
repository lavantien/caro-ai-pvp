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
        // Aspiration window: try narrow search first, then wider if needed
        const int aspirationWindow = SHC.AspirationWindow;
        const int maxAspirationAttempts = SHC.MaxAspirationAttempts;

        var bestScore = int.MinValue;
        var bestMove = candidates[0];
        int bestTiebreaker = 0;  // Track tiebreaker score

        // Calculate board hash for transposition table
        var boardHash = _transpositionTable.CalculateHash(board);

        // Initialize SearchBoard from immutable Board for high-performance search
        _searchBoard.CopyFrom(new SearchBoard(board));

        // First, do a quick search at depth-1 to get an estimate (if depth > 2)
        int estimatedScore = 0;
        if (depth > 2)
        {
            // Quick search with wide window to get estimate
            var searchAlpha = int.MinValue;
            var searchBeta = int.MaxValue;

            // Pre-score candidates for tiebreaking (use position heuristics)
            var candidateScores = _moveOrderer.ScoreCandidatesForTiebreak(candidates, board, player, depth);

            int idx = 0;
            foreach (var (x, y) in candidates)
            {
                // CRITICAL: Check time before evaluating each move
                if (_searchStopwatch.ElapsedMilliseconds >= _searchHardBoundMs)
                {
                    _searchStopped = true;
                    return (bestMove.x, bestMove.y, bestScore);
                }

                // Make move on SearchBoard (in-place, zero allocation)
                var undo = _searchBoard.MakeMove(x, y, player);
                var score = MinimaxCore(_searchBoard, depth - 2, searchAlpha, searchBeta, false, player, depth);
                _searchBoard.UnmakeMove(undo);

                // If search was stopped during Minimax, return current best
                if (_searchStopped)
                {
                    return (bestMove.x, bestMove.y, bestScore);
                }

                // Tie-breaking: higher score wins, or equal score with better tiebreaker
                if (score > bestScore || (score == bestScore && candidateScores[idx] > bestTiebreaker))
                {
                    bestScore = score;
                    bestMove = (x, y);
                    bestTiebreaker = candidateScores[idx];
                }

                searchAlpha = Math.Max(searchAlpha, score);
                if (searchBeta <= searchAlpha)
                    break;
                idx++;
            }
            estimatedScore = bestScore;
        }

        // Now search with aspiration window
        var alpha = estimatedScore - aspirationWindow;
        var beta = estimatedScore + aspirationWindow;

        for (int attempt = 0; attempt < maxAspirationAttempts; attempt++)
        {
            // Check transposition table with current window
            _tableLookups++;
            var (found, cachedScore, cachedMove) = _transpositionTable.Lookup(boardHash, depth, alpha, beta);
            if (found && cachedMove.HasValue)
            {
                // CRITICAL: Validate the cached move is actually legal
                // TT entries may be from different positions due to hash collisions or stale data
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
                // If cached move is invalid, fall through to normal search
            }

            // Reset best score for this attempt
            bestScore = int.MinValue;
            bestMove = candidates[0];
            bestTiebreaker = 0;

            // Order moves: Hash > Emergency > Threats > Killers > History > Positional
            var orderedMoves = _moveOrderer.OrderMoves(candidates, depth, board, player, cachedMove);

            // Pre-score ordered moves for tiebreaking
            var orderedTiebreakScores = _moveOrderer.ScoreCandidatesForTiebreak(orderedMoves, board, player, depth);

            var aspirationFailed = false;
            int orderedIdx = 0;
            foreach (var (x, y) in orderedMoves)
            {
                // CRITICAL: Check time before evaluating each move
                // This catches timeout during long candidate loops
                if (_searchStopwatch.ElapsedMilliseconds >= _searchHardBoundMs)
                {
                    _searchStopped = true;
                    return (bestMove.x, bestMove.y, bestScore);  // Return best move found so far
                }

                // Make move on SearchBoard (in-place, zero allocation)
                var undo = _searchBoard.MakeMove(x, y, player);

                // Evaluate using MinimaxCore
                var score = MinimaxCore(_searchBoard, depth - 1, alpha, beta, false, player, depth);

                // Unmake move (restore board state)
                _searchBoard.UnmakeMove(undo);

                // If search was stopped during Minimax, return current best
                if (_searchStopped)
                {
                    return (bestMove.x, bestMove.y, bestScore);
                }

                // Tie-breaking: higher score wins, or equal score with better tiebreaker + small random
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

                    // Prefer better tiebreaker score, or add randomness
                    if (currentTiebreaker + randomBonus > bestTiebreaker)
                    {
                        bestMove = (x, y);
                        bestTiebreaker = currentTiebreaker + randomBonus;
                    }
                }

                alpha = Math.Max(alpha, score);
                if (beta <= alpha)
                {
                    // Beta cutoff - record killer move and history
                    RecordKillerMove(depth, x, y);
                    RecordHistoryMove(player, x, y, depth);
                    break;
                }

                // Check if score exceeds beta (aspiration window too low)
                if (score >= beta)
                {
                    aspirationFailed = true;
                    break;
                }
                orderedIdx++;
            }

            // If aspiration didn't fail, we're done
            if (!aspirationFailed && bestScore > alpha && bestScore < beta)
            {
                // Store result in transposition table
                _transpositionTable.Store(boardHash, depth, bestScore, bestMove, estimatedScore - aspirationWindow, estimatedScore + aspirationWindow);
                return (bestMove.x, bestMove.y, bestScore);
            }

            // Aspiration failed - widen window and try again
            alpha = int.MinValue;
            beta = int.MaxValue;

            // On final attempt, just return the best we found
            if (attempt == maxAspirationAttempts - 1)
            {
                // Store result with wide window
                _transpositionTable.Store(boardHash, depth, bestScore, bestMove, int.MinValue, int.MaxValue);
                return (bestMove.x, bestMove.y, bestScore);
            }
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
        _transpositionTable = new TranspositionTable(newSizeMb);
        _parallelSearch = new ParallelMinimaxSearch(newSizeMb);
    }

    /// <summary>
    /// Quiescence search: extend search in tactical positions to get accurate evaluation
    /// Only considers moves near existing stones (tactical moves)
    /// </summary>
    private int Quiesce(Board board, int alpha, int beta, bool isMaximizing, Player aiPlayer, int rootDepth)
    {
        // Time control: check frequently (every 16 nodes) to avoid timeout
        // Use a different offset to stagger checks between Minimax and Quiesce
        if ((_nodesSearched & 15) == 8)
        {
            var elapsed = _searchStopwatch.ElapsedMilliseconds;
            if (elapsed >= _searchHardBoundMs)
            {
                _searchStopped = true;
                // Return current bound to avoid corrupting alpha-beta
                return isMaximizing ? alpha : beta;
            }
        }

        // Get stand-pat score (static evaluation)
        var standPat = EvaluateBoard(board, aiPlayer);

        // Beta cutoff (stand-pat is good enough for maximizing player)
        if (isMaximizing && standPat >= beta)
            return beta;

        // Alpha cutoff (stand-pat is good enough for minimizing player)
        if (!isMaximizing && standPat <= alpha)
            return alpha;

        // Update bounds for search
        if (isMaximizing)
            alpha = Math.Max(alpha, standPat);
        else
            beta = Math.Min(beta, standPat);

        // Check for terminal states in quiescence
        var winner = CheckWinner(board);
        if (winner != null)
        {
            return winner == aiPlayer ? SHC.WinScore : -SHC.WinScore;
        }

        // Generate tactical moves (only near existing stones)
        var tacticalMoves = CandidateGenerator.GetCandidateMoves(board, SearchConstants.MaxSearchRadius);

        // Limit quiescence search depth to avoid explosion
        const int maxQuiescenceDepth = 4;  // Search up to 4 ply beyond depth 0
        if (rootDepth - 0 > maxQuiescenceDepth)
        {
            return standPat;  // Stop quiescing, return static eval
        }

        // If no tactical moves, return static evaluation
        if (tacticalMoves.Count == 0)
            return standPat;

        var currentPlayer = isMaximizing ? aiPlayer : (aiPlayer == Player.Red ? Player.Blue : Player.Red);

        // Order tactical moves for better pruning
        var orderedMoves = _moveOrderer.OrderMoves(tacticalMoves, rootDepth, board, currentPlayer, null);

        // Search tactical moves (only empty cells)
        if (isMaximizing)
        {
            var maxEval = standPat;
            foreach (var (x, y) in orderedMoves)
            {
                // Skip occupied cells (can happen during quiescence recursion)
                if (!board.GetCell(x, y).IsEmpty)
                    continue;

                var qBoard = board.PlaceStone(x, y, currentPlayer);

                // Recursive quiescence search (depth stays at 0, but we track via rootDepth)
                var eval = Quiesce(qBoard, alpha, beta, false, aiPlayer, rootDepth + 1);

                maxEval = Math.Max(maxEval, eval);
                alpha = Math.Max(alpha, eval);

                if (beta <= alpha)
                    return beta;  // Beta cutoff
            }
            return maxEval;
        }
        else
        {
            var minEval = standPat;
            foreach (var (x, y) in orderedMoves)
            {
                // Skip occupied cells (can happen during quiescence recursion)
                if (!board.GetCell(x, y).IsEmpty)
                    continue;

                var qBoard = board.PlaceStone(x, y, currentPlayer);

                var eval = Quiesce(qBoard, alpha, beta, true, aiPlayer, rootDepth + 1);

                minEval = Math.Min(minEval, eval);
                beta = Math.Min(beta, eval);

                if (beta <= alpha)
                    return alpha;  // Alpha cutoff
            }
            return minEval;
        }
    }

    /// <summary>
    /// Core minimax algorithm using SearchBoard with make/unmake pattern.
    /// High-performance path that avoids Board.PlaceStone allocations.
    /// </summary>
    private int MinimaxCore(SearchBoard board, int depth, int alpha, int beta, bool isMaximizing, Player aiPlayer, int rootDepth)
    {
        // Count this node
        _nodesSearched++;

        // Time control: check periodically (every N nodes) to avoid timeout
        if ((_nodesSearched & (TimeCheckInterval - 1)) == 0)
        {
            var elapsed = _searchStopwatch.ElapsedMilliseconds;
            if (elapsed >= _searchHardBoundMs)
            {
                _searchStopped = true;
                return isMaximizing ? alpha : beta;
            }
        }

        // Check terminal states
        var winner = CheckWinner(board);
        if (winner != null)
        {
            return winner == aiPlayer ? SHC.WinScore : -SHC.WinScore;
        }

        if (depth == 0)
        {
            return QuiesceCore(board, alpha, beta, isMaximizing, aiPlayer, rootDepth);
        }

        // NULL-MOVE PRUNING
        var isNullMoveEligible = (beta - alpha) <= 1;
        if (depth >= SearchConstants.NullMoveMinDepth && isNullMoveEligible && TacticalEvaluator.IsNullMoveSafe(board, isMaximizing ? aiPlayer : (aiPlayer == Player.Red ? Player.Blue : Player.Red)))
        {
            int nullMoveDepth = depth - SearchConstants.NullMoveDepthReduction;
            if (nullMoveDepth > 0)
            {
                int nullMoveScore = MinimaxCore(board, nullMoveDepth, beta - 1, beta, !isMaximizing, aiPlayer, rootDepth);
                if (nullMoveScore >= beta)
                {
                    return beta;
                }
            }
        }

        var candidates = CandidateGenerator.GetCandidateMoves(board, SearchConstants.MaxSearchRadius);
        if (candidates.Count == 0)
        {
            return 0;
        }

        // Transposition table lookup using SearchBoard hash
        var boardHash = board.GetHash();
        _tableLookups++;
        var (found, cachedScore, cachedMove) = _transpositionTable.Lookup(boardHash, depth, alpha, beta);
        if (found)
        {
            _tableHits++;
            return cachedScore;
        }

        var currentPlayer = isMaximizing ? aiPlayer : (aiPlayer == Player.Red ? Player.Blue : Player.Red);

        // Order moves
        var orderedMoves = _moveOrderer.OrderMoves(candidates, rootDepth - depth, board, currentPlayer, cachedMove);

        int score;
        const int lmrFullDepthMoves = SHC.LMRFullDepthMoves;
        const int pvsEnabledDepth = SHC.PVSEnabledDepth;

        if (isMaximizing)
        {
            var maxEval = int.MinValue;
            var moveIndex = 0;

            foreach (var (x, y) in orderedMoves)
            {
                // Make move (mutates board in-place)
                var undo = board.MakeMove(x, y, currentPlayer);

                int eval;
                bool isPvNode = (moveIndex == 0) && (depth >= pvsEnabledDepth);

                if (isPvNode)
                {
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !TacticalEvaluator.IsTacticalPosition(board))
                    {
                        eval = MinimaxCore(board, depth - 2, alpha, beta, false, aiPlayer, rootDepth);
                        if (eval > alpha && eval < beta - 100)
                        {
                            eval = MinimaxCore(board, depth - 1, alpha, beta, false, aiPlayer, rootDepth);
                        }
                    }
                    else
                    {
                        eval = MinimaxCore(board, depth - 1, alpha, beta, false, aiPlayer, rootDepth);
                    }
                }
                else
                {
                    int searchDepth = depth - 1;
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !TacticalEvaluator.IsTacticalPosition(board))
                    {
                        searchDepth = depth - 2;
                    }

                    eval = MinimaxCore(board, searchDepth, alpha, alpha + 1, false, aiPlayer, rootDepth);

                    if (eval > alpha && eval < beta)
                    {
                        if (searchDepth == depth - 2)
                        {
                            eval = MinimaxCore(board, depth - 1, alpha, beta, false, aiPlayer, rootDepth);
                        }
                        else
                        {
                            eval = MinimaxCore(board, depth - 1, alpha, beta, false, aiPlayer, rootDepth);
                        }
                    }
                }

                // Unmake move (restores board state)
                board.UnmakeMove(undo);

                if (eval > maxEval)
                {
                    maxEval = eval;
                }

                alpha = Math.Max(alpha, eval);
                if (beta <= alpha)
                {
                    RecordKillerMove(rootDepth - depth, x, y);
                    RecordHistoryMove(currentPlayer, x, y, depth);
                    break;
                }

                moveIndex++;
            }
            score = maxEval;
        }
        else
        {
            var minEval = int.MaxValue;
            var moveIndex = 0;

            foreach (var (x, y) in orderedMoves)
            {
                // Make move (mutates board in-place)
                var undo = board.MakeMove(x, y, currentPlayer);

                int eval;
                bool isPvNode = (moveIndex == 0) && (depth >= pvsEnabledDepth);

                if (isPvNode)
                {
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !TacticalEvaluator.IsTacticalPosition(board))
                    {
                        eval = MinimaxCore(board, depth - 2, alpha, beta, true, aiPlayer, rootDepth);
                        if (eval < beta && eval > alpha + 100)
                        {
                            eval = MinimaxCore(board, depth - 1, alpha, beta, true, aiPlayer, rootDepth);
                        }
                    }
                    else
                    {
                        eval = MinimaxCore(board, depth - 1, alpha, beta, true, aiPlayer, rootDepth);
                    }
                }
                else
                {
                    int searchDepth = depth - 1;
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !TacticalEvaluator.IsTacticalPosition(board))
                    {
                        searchDepth = depth - 2;
                    }

                    eval = MinimaxCore(board, searchDepth, beta - 1, beta, true, aiPlayer, rootDepth);

                    if (eval < beta && eval > alpha)
                    {
                        if (searchDepth == depth - 2)
                        {
                            eval = MinimaxCore(board, depth - 1, alpha, beta, true, aiPlayer, rootDepth);
                        }
                        else
                        {
                            eval = MinimaxCore(board, depth - 1, alpha, beta, true, aiPlayer, rootDepth);
                        }
                    }
                }

                // Unmake move (restores board state)
                board.UnmakeMove(undo);

                if (eval < minEval)
                {
                    minEval = eval;
                }

                beta = Math.Min(beta, eval);
                if (beta <= alpha)
                {
                    RecordKillerMove(rootDepth - depth, x, y);
                    RecordHistoryMove(currentPlayer, x, y, depth);
                    break;
                }

                moveIndex++;
            }
            score = minEval;
        }

        // Store result in transposition table
        _transpositionTable.Store(boardHash, depth, score, null, alpha, beta);

        return score;
    }

    /// <summary>
    /// Quiescence search using SearchBoard with make/unmake pattern.
    /// High-performance path that avoids Board.PlaceStone allocations.
    /// </summary>
    private int QuiesceCore(SearchBoard board, int alpha, int beta, bool isMaximizing, Player aiPlayer, int rootDepth)
    {
        // Time control
        if ((_nodesSearched & 15) == 8)
        {
            var elapsed = _searchStopwatch.ElapsedMilliseconds;
            if (elapsed >= _searchHardBoundMs)
            {
                _searchStopped = true;
                return isMaximizing ? alpha : beta;
            }
        }

        // Get stand-pat score using SearchBoard evaluator
        var standPat = EvaluateBoard(board, aiPlayer);

        // Beta cutoff
        if (isMaximizing && standPat >= beta)
            return beta;

        // Alpha cutoff
        if (!isMaximizing && standPat <= alpha)
            return alpha;

        // Update bounds
        if (isMaximizing)
            alpha = Math.Max(alpha, standPat);
        else
            beta = Math.Min(beta, standPat);

        // Check for terminal states
        var winner = CheckWinner(board);
        if (winner != null)
        {
            return winner == aiPlayer ? SHC.WinScore : -SHC.WinScore;
        }

        // Generate tactical moves
        var tacticalMoves = CandidateGenerator.GetCandidateMoves(board, SearchConstants.MaxSearchRadius);

        // Limit quiescence depth
        const int maxQuiescenceDepth = 4;
        if (rootDepth > maxQuiescenceDepth)
        {
            return standPat;
        }

        if (tacticalMoves.Count == 0)
            return standPat;

        var currentPlayer = isMaximizing ? aiPlayer : (aiPlayer == Player.Red ? Player.Blue : Player.Red);

        // Order tactical moves
        var orderedMoves = _moveOrderer.OrderMoves(tacticalMoves, rootDepth, board, currentPlayer, null);

        if (isMaximizing)
        {
            var maxEval = standPat;
            foreach (var (x, y) in orderedMoves)
            {
                if (!board.IsEmpty(x, y))
                    continue;

                var undo = board.MakeMove(x, y, currentPlayer);
                var eval = QuiesceCore(board, alpha, beta, false, aiPlayer, rootDepth + 1);
                board.UnmakeMove(undo);

                maxEval = Math.Max(maxEval, eval);
                alpha = Math.Max(alpha, eval);

                if (beta <= alpha)
                    return beta;
            }
            return maxEval;
        }
        else
        {
            var minEval = standPat;
            foreach (var (x, y) in orderedMoves)
            {
                if (!board.IsEmpty(x, y))
                    continue;

                var undo = board.MakeMove(x, y, currentPlayer);
                var eval = QuiesceCore(board, alpha, beta, true, aiPlayer, rootDepth + 1);
                board.UnmakeMove(undo);

                minEval = Math.Min(minEval, eval);
                beta = Math.Min(beta, eval);

                if (beta <= alpha)
                    return alpha;
            }
            return minEval;
        }
    }

    /// <summary>
    /// Minimax algorithm with alpha-beta pruning and transposition table
    /// </summary>
    private int Minimax(Board board, int depth, int alpha, int beta, bool isMaximizing, Player aiPlayer, int rootDepth)
    {
        // Count this node
        _nodesSearched++;

        // Time control: check periodically (every N nodes) to avoid timeout
        if ((_nodesSearched & (TimeCheckInterval - 1)) == 0)
        {
            var elapsed = _searchStopwatch.ElapsedMilliseconds;
            if (elapsed >= _searchHardBoundMs)
            {
                _searchStopped = true;
                // Return current bound to avoid corrupting alpha-beta
                return isMaximizing ? alpha : beta;
            }
        }

        // Check terminal states
        var winner = CheckWinner(board);
        if (winner != null)
        {
            return winner == aiPlayer ? SHC.WinScore : -SHC.WinScore;
        }

        if (depth == 0)
        {
            // Use quiescence search to resolve tactical positions
            return Quiesce(board, alpha, beta, isMaximizing, aiPlayer, rootDepth);
        }

        // NULL-MOVE PRUNING: Skip a move to verify position is already good
        // Only apply in non-PV nodes with sufficient depth and safe position
        var isNullMoveEligible = (beta - alpha) <= 1;  // Not a PV node (narrow window)
        if (depth >= SearchConstants.NullMoveMinDepth && isNullMoveEligible && TacticalEvaluator.IsNullMoveSafe(board, isMaximizing ? aiPlayer : (aiPlayer == Player.Red ? Player.Blue : Player.Red)))
        {
            // Make null move: skip turn, search with reduced depth
            // The reduced depth search is done from opponent's perspective (flipped min/max)
            int nullMoveDepth = depth - SearchConstants.NullMoveDepthReduction;

            if (nullMoveDepth > 0)
            {
                // Search with null move (flip min/max because we skipped a turn)
                int nullMoveScore = Minimax(board, nullMoveDepth, beta - 1, beta, !isMaximizing, aiPlayer, rootDepth);

                // If null move fails high (score >= beta), the position is so good
                // that even giving opponent a free move doesn't help them
                if (nullMoveScore >= beta)
                {
                    // Beta cutoff: position is good enough, skip searching remaining moves
                    return beta;
                }
            }
        }

        var candidates = CandidateGenerator.GetCandidateMoves(board, SearchConstants.MaxSearchRadius);
        if (candidates.Count == 0)
        {
            return 0; // Draw
        }

        // Transposition table lookup
        var boardHash = _transpositionTable.CalculateHash(board);
        _tableLookups++;
        var (found, cachedScore, cachedMove) = _transpositionTable.Lookup(boardHash, depth, alpha, beta);
        if (found)
        {
            _tableHits++;
            return cachedScore;
        }

        var currentPlayer = isMaximizing ? aiPlayer : (aiPlayer == Player.Red ? Player.Blue : Player.Red);

        // IN-TREE VCF CHECK: Check for forcing sequences before move generation
        // VCF runs at all nodes; only time budget limits it (no depth caps)
        // Percentage-based threshold: VCF runs MORE in time scramble (low time remaining)
        var remainingTime = _searchHardBoundMs - _searchStopwatch.ElapsedMilliseconds;
        var initialTime = _inferredInitialTimeMs > 0 ? _inferredInitialTimeMs : _searchHardBoundMs;
        var timeRemainingPercent = (double)remainingTime / initialTime;

        // Time scramble: < 10% time remaining - VCF is critical (find quick wins)
        // Normal time: use 5% of initial time as threshold
        var vcfThresholdMs = timeRemainingPercent < SHC.VcfTimeRemainingThreshold
            ? SHC.VcfMinimumTimeMs
            : initialTime * SHC.VcfInitialTimeFraction;

        if (remainingTime > vcfThresholdMs)
        {
            var vcfResult = _inTreeVCFSolver.CheckNodeVCF(board, currentPlayer, depth, alpha, remainingTime);
            if (vcfResult != null && vcfResult.Type != VCFResultType.NoVCF)
            {
                // VCF found - return immediately with appropriate score
                if (vcfResult.Type == VCFResultType.WinningSequence)
                {
                    _nodesSearched += vcfResult.NodesSearched;
                    return vcfResult.Score;
                }
                // For losing sequences, we could filter candidates, but for now
                // let the normal search handle it with proper alpha-beta bounds
            }
        }

        // Order moves for better pruning (use cached move if available)
        var orderedMoves = _moveOrderer.OrderMoves(candidates, rootDepth - depth, board, currentPlayer, cachedMove);

        int score;
        const int lmrFullDepthMoves = SHC.LMRFullDepthMoves;
        const int pvsEnabledDepth = SHC.PVSEnabledDepth;

        if (isMaximizing)
        {
            var maxEval = int.MinValue;
            var moveIndex = 0;

            foreach (var (x, y) in orderedMoves)
            {
                var newBoard = board.PlaceStone(x, y, currentPlayer);

                int eval;
                bool isPvNode = (moveIndex == 0) && (depth >= pvsEnabledDepth);

                // PRINCIPAL VARIATION SEARCH: first move with full window, rest with null window
                if (isPvNode)
                {
                    // First move: full window search
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !TacticalEvaluator.IsTacticalPosition(board))
                    {
                        // LMR: reduced depth search first
                        eval = Minimax(newBoard, depth - 2, alpha, beta, false, aiPlayer, rootDepth);

                        // If reduced search is promising, re-search at full depth
                        if (eval > alpha && eval < beta - 100)
                        {
                            eval = Minimax(newBoard, depth - 1, alpha, beta, false, aiPlayer, rootDepth);
                        }
                    }
                    else
                    {
                        // Full depth search for early moves or tactical positions
                        eval = Minimax(newBoard, depth - 1, alpha, beta, false, aiPlayer, rootDepth);
                    }
                }
                else
                {
                    // Subsequent moves: try null window search first
                    int searchDepth = depth - 1;

                    // Apply LMR to null window search if applicable
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !TacticalEvaluator.IsTacticalPosition(board))
                    {
                        searchDepth = depth - 2;
                    }

                    // Null window search (alpha, alpha+1)
                    eval = Minimax(newBoard, searchDepth, alpha, alpha + 1, false, aiPlayer, rootDepth);

                    // If null window search beats alpha, re-search with full window
                    if (eval > alpha && eval < beta)
                    {
                        // Re-search with full window to get accurate score
                        if (searchDepth == depth - 2)
                        {
                            // Had used LMR, now search at full depth
                            eval = Minimax(newBoard, depth - 1, alpha, beta, false, aiPlayer, rootDepth);
                        }
                        else
                        {
                            // Re-search with full beta
                            eval = Minimax(newBoard, depth - 1, alpha, beta, false, aiPlayer, rootDepth);
                        }
                    }
                }

                if (eval > maxEval)
                {
                    maxEval = eval;
                }

                alpha = Math.Max(alpha, eval);
                if (beta <= alpha)
                {
                    // Beta cutoff - record killer move and history
                    RecordKillerMove(rootDepth - depth, x, y);
                    RecordHistoryMove(currentPlayer, x, y, depth);
                    break; // Alpha cutoff
                }

                moveIndex++;
            }
            score = maxEval;
        }
        else
        {
            var minEval = int.MaxValue;
            var moveIndex = 0;

            foreach (var (x, y) in orderedMoves)
            {
                var newBoard = board.PlaceStone(x, y, currentPlayer);

                int eval;
                bool isPvNode = (moveIndex == 0) && (depth >= pvsEnabledDepth);

                // PRINCIPAL VARIATION SEARCH: first move with full window, rest with null window
                if (isPvNode)
                {
                    // First move: full window search
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !TacticalEvaluator.IsTacticalPosition(board))
                    {
                        // LMR: reduced depth search first
                        eval = Minimax(newBoard, depth - 2, alpha, beta, true, aiPlayer, rootDepth);

                        // If reduced search is promising, re-search at full depth
                        if (eval < beta && eval > alpha + 100)
                        {
                            eval = Minimax(newBoard, depth - 1, alpha, beta, true, aiPlayer, rootDepth);
                        }
                    }
                    else
                    {
                        // Full depth search for early moves or tactical positions
                        eval = Minimax(newBoard, depth - 1, alpha, beta, true, aiPlayer, rootDepth);
                    }
                }
                else
                {
                    // Subsequent moves: try null window search first
                    int searchDepth = depth - 1;

                    // Apply LMR to null window search if applicable
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !TacticalEvaluator.IsTacticalPosition(board))
                    {
                        searchDepth = depth - 2;
                    }

                    // Null window search (beta-1, beta)
                    eval = Minimax(newBoard, searchDepth, beta - 1, beta, true, aiPlayer, rootDepth);

                    // If null window search is below beta, re-search with full window
                    if (eval < beta && eval > alpha)
                    {
                        // Re-search with full window to get accurate score
                        if (searchDepth == depth - 2)
                        {
                            // Had used LMR, now search at full depth
                            eval = Minimax(newBoard, depth - 1, alpha, beta, true, aiPlayer, rootDepth);
                        }
                        else
                        {
                            // Re-search with full alpha
                            eval = Minimax(newBoard, depth - 1, alpha, beta, true, aiPlayer, rootDepth);
                        }
                    }
                }

                if (eval < minEval)
                {
                    minEval = eval;
                }

                beta = Math.Min(beta, eval);
                if (beta <= alpha)
                {
                    // Alpha cutoff - record killer move and history
                    RecordKillerMove(rootDepth - depth, x, y);
                    RecordHistoryMove(currentPlayer, x, y, depth);
                    break; // Beta cutoff
                }

                moveIndex++;
            }
            score = minEval;
        }

        // Store result in transposition table
        _transpositionTable.Store(boardHash, depth, score, null, alpha, beta);

        return score;
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
