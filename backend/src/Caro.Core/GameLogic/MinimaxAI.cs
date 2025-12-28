using Caro.Core.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// AI opponent using Minimax algorithm with alpha-beta pruning and advanced optimizations
/// Optimizations: Transposition Table, Killer Heuristic, History Heuristic, Improved Move Ordering, Iterative Deepening
/// </summary>
public class MinimaxAI
{
    private readonly BoardEvaluator _evaluator = new();
    private readonly Random _random = new();
    private readonly TranspositionTable _transpositionTable = new();

    // Search radius around existing stones (optimization)
    private const int SearchRadius = 2;

    // Killer heuristic: track best moves at each depth
    private const int MaxKillerMoves = 2;
    private readonly (int x, int y)[,] _killerMoves = new (int x, int y)[20, MaxKillerMoves]; // Max depth 20

    // History heuristic: track moves that cause cutoffs across all depths
    // Two tables: one for Red, one for Blue (each move can be good for different players)
    private readonly int[,] _historyRed = new int[15, 15];   // History scores for Red moves
    private readonly int[,] _historyBlue = new int[15, 15];  // History scores for Blue moves

    // Track transposition table hits for debugging
    private int _tableHits;
    private int _tableLookups;

    /// <summary>
    /// Get the best move for the AI player
    /// </summary>
    public (int x, int y) GetBestMove(Board board, Player player, AIDifficulty difficulty)
    {
        return GetBestMove(board, player, difficulty, timeRemainingMs: null);
    }

    /// <summary>
    /// Get the best move for the AI player with time awareness
    /// Dynamically adjusts search depth based on remaining time
    /// </summary>
    public (int x, int y) GetBestMove(Board board, Player player, AIDifficulty difficulty, long? timeRemainingMs)
    {
        if (player == Player.None)
            throw new ArgumentException("Player cannot be None");

        var baseDepth = (int)difficulty;
        var candidates = GetCandidateMoves(board);

        if (candidates.Count == 0)
        {
            // Board is empty, play center
            return (7, 7);
        }

        // For Easy difficulty, sometimes add randomness
        if (difficulty == AIDifficulty.Easy && _random.Next(100) < 20)
        {
            // 20% chance to play random valid move
            var randomIndex = _random.Next(candidates.Count);
            return candidates[randomIndex];
        }

        // Adjust depth based on time remaining
        var adjustedDepth = CalculateDepthForTime(baseDepth, timeRemainingMs, candidates.Count);

        // Initialize transposition table for this search
        _transpositionTable.IncrementAge();
        _tableHits = 0;
        _tableLookups = 0;

        // Debug: Print depth usage for Medium, Hard, Expert, and Master to investigate balance
        if (difficulty == AIDifficulty.Medium || difficulty == AIDifficulty.Hard || difficulty == AIDifficulty.Expert || difficulty == AIDifficulty.Master)
        {
            Console.WriteLine($"[AI DEBUG] {difficulty}: baseDepth={baseDepth}, adjustedDepth={adjustedDepth}, candidates={candidates.Count}, timeRemaining={(timeRemainingMs.HasValue ? $"{timeRemainingMs.Value/1000.0:F1}s" : "N/A")}");
        }

        // Iterative deepening with time awareness
        var bestMove = candidates[0];
        var currentDepth = Math.Min(2, adjustedDepth); // Start with depth 2
        var searchStopwatch = System.Diagnostics.Stopwatch.StartNew();

        while (currentDepth <= adjustedDepth)
        {
            // Check if we're running low on time during search
            if (timeRemainingMs.HasValue)
            {
                var timeUsed = searchStopwatch.ElapsedMilliseconds;
                var timeLeft = timeRemainingMs.Value - timeUsed;
                if (timeLeft < 2000) // Less than 2 seconds left, stop searching
                {
                    break;
                }
            }

            var result = SearchWithDepth(board, player, currentDepth, candidates);
            if (result.x != -1)
            {
                bestMove = (result.x, result.y);
            }
            currentDepth++;
        }

        // Print transposition table statistics for debugging
        if (difficulty == AIDifficulty.Hard || difficulty == AIDifficulty.Expert || difficulty == AIDifficulty.Master)
        {
            double hitRate = _tableLookups > 0 ? (double)_tableHits / _tableLookups * 100 : 0;
            Console.WriteLine($"[AI TT] Hits: {_tableHits}/{_tableLookups} ({hitRate:F1}%)");
            var (used, usage) = _transpositionTable.GetStats();
            Console.WriteLine($"[AI TT] Table usage: {used} entries ({usage:F2}%)");
        }

        return bestMove;
    }

    /// <summary>
    /// Calculate appropriate search depth based on time remaining and position complexity
    /// Accounts for increment system (2s per move) - more aggressive time management
    /// </summary>
    private int CalculateDepthForTime(int baseDepth, long? timeRemainingMs, int candidateCount)
    {
        if (!timeRemainingMs.HasValue)
        {
            return baseDepth; // No time limit, use full depth
        }

        var timeLeft = timeRemainingMs.Value;

        // With 3+2 time control, be more aggressive about time management
        // Average game length ~25 moves, so ~2s net loss per move

        // Emergency mode: less than 30 seconds
        if (timeLeft < 30000)
        {
            return Math.Max(1, baseDepth - 2); // Significant reduction
        }

        // Low time: less than 60 seconds (20 moves remaining)
        if (timeLeft < 60000)
        {
            return Math.Max(2, baseDepth - 1); // Moderate reduction
        }

        // Moderate time: less than 90 seconds (30 moves remaining)
        if (timeLeft < 90000)
        {
            if (candidateCount > 15) // Complex position
            {
                return Math.Max(2, baseDepth - 1);
            }
            return baseDepth;
        }

        // Plenty of time (>90s): use full depth
        return baseDepth;
    }

    private (int x, int y) SearchWithDepth(Board board, Player player, int depth, List<(int x, int y)> candidates)
    {
        // Aspiration window: try narrow search first, then wider if needed
        const int aspirationWindow = 50;  // Initial window size
        const int maxAspirationAttempts = 3;  // Max re-searches with wider windows

        var bestScore = int.MinValue;
        var bestMove = candidates[0];

        // Calculate board hash for transposition table
        var boardHash = _transpositionTable.CalculateHash(board);

        // First, do a quick search at depth-1 to get an estimate (if depth > 2)
        int estimatedScore = 0;
        if (depth > 2)
        {
            // Quick search with wide window to get estimate
            var searchAlpha = int.MinValue;
            var searchBeta = int.MaxValue;
            foreach (var (x, y) in candidates)
            {
                board.PlaceStone(x, y, player);
                var score = Minimax(board, depth - 2, searchAlpha, searchBeta, false, player, depth);
                board.GetCell(x, y).Player = Player.None;

                if (score > bestScore)
                {
                    bestScore = score;
                    bestMove = (x, y);
                }

                searchAlpha = Math.Max(searchAlpha, score);
                if (searchBeta <= searchAlpha)
                    break;
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
            if (found)
            {
                _tableHits++;
                if (cachedMove.HasValue)
                    return cachedMove.Value;
            }

            // Reset best score for this attempt
            bestScore = int.MinValue;
            bestMove = candidates[0];

            // Order moves: try killer moves first, then by proximity to center, then cached move
            var orderedMoves = OrderMoves(candidates, depth, board, player, cachedMove);

            var aspirationFailed = false;
            foreach (var (x, y) in orderedMoves)
            {
                // Make move
                board.PlaceStone(x, y, player);

                // Evaluate using minimax
                var score = Minimax(board, depth - 1, alpha, beta, false, player, depth);

                // Undo move
                board.GetCell(x, y).Player = Player.None;

                if (score > bestScore)
                {
                    bestScore = score;
                    bestMove = (x, y);
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
            }

            // If aspiration didn't fail, we're done
            if (!aspirationFailed && bestScore > alpha && bestScore < beta)
            {
                // Store result in transposition table
                _transpositionTable.Store(boardHash, depth, bestScore, bestMove, estimatedScore - aspirationWindow, estimatedScore + aspirationWindow);
                return bestMove;
            }

            // Aspiration failed - widen window and try again
            alpha = int.MinValue;
            beta = int.MaxValue;

            // On final attempt, just return the best we found
            if (attempt == maxAspirationAttempts - 1)
            {
                // Store result with wide window
                _transpositionTable.Store(boardHash, depth, bestScore, bestMove, int.MinValue, int.MaxValue);
                return bestMove;
            }
        }

        return bestMove;
    }

    /// <summary>
    /// Order moves for better alpha-beta pruning
    /// Priority: TT cached move > Killer moves > History heuristic > Tactical patterns > Position heuristics
    /// </summary>
    private List<(int x, int y)> OrderMoves(List<(int x, int y)> candidates, int depth, Board board, Player player, (int x, int y)? cachedMove = null)
    {
        var scored = candidates.Select(move =>
        {
            var score = 0;

            // Cached move from transposition table gets highest priority
            if (cachedMove.HasValue && move.x == cachedMove.Value.x && move.y == cachedMove.Value.y)
            {
                score += 2000;
            }

            // Killer moves get high priority
            for (int i = 0; i < MaxKillerMoves; i++)
            {
                if (_killerMoves[depth, i].x == move.x && _killerMoves[depth, i].y == move.y)
                {
                    score += 1000;
                    break;
                }
            }

            // History heuristic: moves that caused cutoffs get priority
            // Scale down history score to fit in our scoring system
            var historyScore = GetHistoryScore(player, move.x, move.y);
            score += Math.Min(500, historyScore / 10); // Cap at 500 to avoid dominating

            // ENHANCED: Tactical pattern scoring
            score += EvaluateTacticalPattern(board, move.x, move.y, player);

            // Prefer center (7,7)
            var distanceToCenter = Math.Abs(move.x - 7) + Math.Abs(move.y - 7);
            score += (14 - distanceToCenter) * 10;

            // Prefer moves near existing stones
            var nearby = 0;
            for (int dx = -1; dx <= 1; dx++)
            {
                for (int dy = -1; dy <= 1; dy++)
                {
                    if (dx == 0 && dy == 0) continue;
                    var nx = move.x + dx;
                    var ny = move.y + dy;
                    if (nx >= 0 && nx < 15 && ny >= 0 && ny < 15)
                    {
                        var cell = board.GetCell(nx, ny);
                        if (cell.Player != Player.None)
                            nearby += 5;
                    }
                }
            }
            score += nearby;

            return (move, score);
        })
        .OrderByDescending(m => m.score)
        .Select(m => m.move)
        .ToList();

        return scored;
    }

    /// <summary>
    /// Evaluate tactical importance of a move by detecting patterns
    /// Returns high scores for winning moves, threats, and blocks
    /// </summary>
    private int EvaluateTacticalPattern(Board board, int x, int y, Player player)
    {
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var score = 0;

        // Temporarily place the stone to evaluate
        board.PlaceStone(x, y, player);

        // Check all 4 directions for patterns
        var directions = new[] { (1, 0), (0, 1), (1, 1), (1, -1) };

        foreach (var (dx, dy) in directions)
        {
            // Count consecutive stones in both directions
            var count = 1; // Include the placed stone
            var openEnds = 0;

            // Check positive direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x + dx * i;
                var ny = y + dy * i;
                if (nx < 0 || nx >= 15 || ny < 0 || ny >= 15) break;

                var cell = board.GetCell(nx, ny);
                if (cell.Player == player)
                {
                    count++;
                }
                else if (cell.Player == Player.None)
                {
                    openEnds++;
                    break;
                }
                else
                {
                    break; // Blocked by opponent
                }
            }

            // Check negative direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x - dx * i;
                var ny = y - dy * i;
                if (nx < 0 || nx >= 15 || ny < 0 || ny >= 15) break;

                var cell = board.GetCell(nx, ny);
                if (cell.Player == player)
                {
                    count++;
                }
                else if (cell.Player == Player.None)
                {
                    openEnds++;
                    break;
                }
                else
                {
                    break; // Blocked by opponent
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

        // Remove the temporary stone
        board.GetCell(x, y).Player = Player.None;

        // Check blocking value (how much this blocks opponent)
        board.PlaceStone(x, y, opponent);

        foreach (var (dx, dy) in directions)
        {
            var count = 1;
            var openEnds = 0;

            // Positive direction
            for (int i = 1; i <= 4; i++)
            {
                var nx = x + dx * i;
                var ny = y + dy * i;
                if (nx < 0 || nx >= 15 || ny < 0 || ny >= 15) break;

                var cell = board.GetCell(nx, ny);
                if (cell.Player == opponent)
                {
                    count++;
                }
                else if (cell.Player == Player.None)
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
                if (nx < 0 || nx >= 15 || ny < 0 || ny >= 15) break;

                var cell = board.GetCell(nx, ny);
                if (cell.Player == opponent)
                {
                    count++;
                }
                else if (cell.Player == Player.None)
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

        // Remove the temporary stone
        board.GetCell(x, y).Player = Player.None;

        return score;
    }

    private void RecordKillerMove(int depth, int x, int y)
    {
        // Shift existing killer moves
        for (int i = MaxKillerMoves - 1; i > 0; i--)
        {
            _killerMoves[depth, i] = _killerMoves[depth, i - 1];
        }
        _killerMoves[depth, 0] = (x, y);
    }

    /// <summary>
    /// Record a move that caused a cutoff in the history table
    /// Higher depth = more significant = larger bonus
    /// </summary>
    private void RecordHistoryMove(Player player, int x, int y, int depth)
    {
        // Depth-based bonus: deeper cutoffs are more significant
        var bonus = depth * depth;

        if (player == Player.Red)
        {
            _historyRed[x, y] += bonus;
        }
        else
        {
            _historyBlue[x, y] += bonus;
        }
    }

    /// <summary>
    /// Get the history score for a move
    /// </summary>
    private int GetHistoryScore(Player player, int x, int y)
    {
        return player == Player.Red ? _historyRed[x, y] : _historyBlue[x, y];
    }

    /// <summary>
    /// Clear history tables (call at start of new game)
    /// </summary>
    public void ClearHistory()
    {
        Array.Clear(_historyRed, 0, _historyRed.Length);
        Array.Clear(_historyBlue, 0, _historyBlue.Length);
    }

    /// <summary>
    /// Quiescence search: extend search in tactical positions to get accurate evaluation
    /// Only considers moves near existing stones (tactical moves)
    /// </summary>
    private int Quiesce(Board board, int alpha, int beta, bool isMaximizing, Player aiPlayer, int rootDepth)
    {
        // Get stand-pat score (static evaluation)
        var standPat = _evaluator.Evaluate(board, aiPlayer);

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
            return winner == aiPlayer ? 100000 : -100000;
        }

        // Generate tactical moves (only near existing stones)
        var tacticalMoves = GetCandidateMoves(board);

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
        var orderedMoves = OrderMoves(tacticalMoves, rootDepth, board, currentPlayer, null);

        // Search tactical moves
        if (isMaximizing)
        {
            var maxEval = standPat;
            foreach (var (x, y) in orderedMoves)
            {
                board.PlaceStone(x, y, currentPlayer);

                // Recursive quiescence search (depth stays at 0, but we track via rootDepth)
                var eval = Quiesce(board, alpha, beta, false, aiPlayer, rootDepth + 1);

                board.GetCell(x, y).Player = Player.None;

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
                board.PlaceStone(x, y, currentPlayer);

                var eval = Quiesce(board, alpha, beta, true, aiPlayer, rootDepth + 1);

                board.GetCell(x, y).Player = Player.None;

                minEval = Math.Min(minEval, eval);
                beta = Math.Min(beta, eval);

                if (beta <= alpha)
                    return alpha;  // Alpha cutoff
            }
            return minEval;
        }
    }

    /// <summary>
    /// Check if position is tactical (has threats) - should not use reduced depth
    /// Tactical positions have: 3+ in a row, or multiple threats nearby
    /// </summary>
    private bool IsTacticalPosition(Board board)
    {
        // Check for 3+ in a row
        for (int x = 0; x < 15; x++)
        {
            for (int y = 0; y < 15; y++)
            {
                var cell = board.GetCell(x, y);
                if (cell.Player == Player.None)
                    continue;

                // Check horizontal
                var count = 1;
                for (int dy = 1; dy <= 4 && y + dy < 15; dy++)
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
                for (int dx = 1; dx <= 4 && x + dx < 15; dx++)
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
                for (int i = 1; i <= 4 && x + i < 15 && y + i < 15; i++)
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
                for (int i = 1; i <= 4 && x + i < 15 && y - i >= 0; i++)
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
    /// Minimax algorithm with alpha-beta pruning and transposition table
    /// </summary>
    private int Minimax(Board board, int depth, int alpha, int beta, bool isMaximizing, Player aiPlayer, int rootDepth)
    {
        // Check terminal states
        var winner = CheckWinner(board);
        if (winner != null)
        {
            return winner == aiPlayer ? 100000 : -100000;
        }

        if (depth == 0)
        {
            // Use quiescence search to resolve tactical positions
            return Quiesce(board, alpha, beta, isMaximizing, aiPlayer, rootDepth);
        }

        var candidates = GetCandidateMoves(board);
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

        // Order moves for better pruning (use cached move if available)
        var orderedMoves = OrderMoves(candidates, rootDepth - depth, board, currentPlayer, cachedMove);

        int score;
        const int lmrFullDepthMoves = 4;  // First 4 moves at full depth
        const int pvsEnabledDepth = 2;  // Enable PVS at depth 2+

        if (isMaximizing)
        {
            var maxEval = int.MinValue;
            var moveIndex = 0;
            (int x, int y)? bestMove = null;

            foreach (var (x, y) in orderedMoves)
            {
                board.PlaceStone(x, y, currentPlayer);

                int eval;
                bool isPvNode = (moveIndex == 0) && (depth >= pvsEnabledDepth);

                // PRINCIPAL VARIATION SEARCH: first move with full window, rest with null window
                if (isPvNode)
                {
                    // First move: full window search
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !IsTacticalPosition(board))
                    {
                        // LMR: reduced depth search first
                        eval = Minimax(board, depth - 2, alpha, beta, false, aiPlayer, rootDepth);

                        // If reduced search is promising, re-search at full depth
                        if (eval > alpha && eval < beta - 100)
                        {
                            eval = Minimax(board, depth - 1, alpha, beta, false, aiPlayer, rootDepth);
                        }
                    }
                    else
                    {
                        // Full depth search for early moves or tactical positions
                        eval = Minimax(board, depth - 1, alpha, beta, false, aiPlayer, rootDepth);
                    }
                }
                else
                {
                    // Subsequent moves: try null window search first
                    int searchDepth = depth - 1;

                    // Apply LMR to null window search if applicable
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !IsTacticalPosition(board))
                    {
                        searchDepth = depth - 2;
                    }

                    // Null window search (alpha, alpha+1)
                    eval = Minimax(board, searchDepth, alpha, alpha + 1, false, aiPlayer, rootDepth);

                    // If null window search beats alpha, re-search with full window
                    if (eval > alpha && eval < beta)
                    {
                        // Re-search with full window to get accurate score
                        if (searchDepth == depth - 2)
                        {
                            // Had used LMR, now search at full depth
                            eval = Minimax(board, depth - 1, alpha, beta, false, aiPlayer, rootDepth);
                        }
                        else
                        {
                            // Re-search with full beta
                            eval = Minimax(board, depth - 1, alpha, beta, false, aiPlayer, rootDepth);
                        }
                    }
                }

                board.GetCell(x, y).Player = Player.None;

                if (eval > maxEval)
                {
                    maxEval = eval;
                    bestMove = (x, y);
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
            (int x, int y)? bestMove = null;

            foreach (var (x, y) in orderedMoves)
            {
                board.PlaceStone(x, y, currentPlayer);

                int eval;
                bool isPvNode = (moveIndex == 0) && (depth >= pvsEnabledDepth);

                // PRINCIPAL VARIATION SEARCH: first move with full window, rest with null window
                if (isPvNode)
                {
                    // First move: full window search
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !IsTacticalPosition(board))
                    {
                        // LMR: reduced depth search first
                        eval = Minimax(board, depth - 2, alpha, beta, true, aiPlayer, rootDepth);

                        // If reduced search is promising, re-search at full depth
                        if (eval < beta && eval > alpha + 100)
                        {
                            eval = Minimax(board, depth - 1, alpha, beta, true, aiPlayer, rootDepth);
                        }
                    }
                    else
                    {
                        // Full depth search for early moves or tactical positions
                        eval = Minimax(board, depth - 1, alpha, beta, true, aiPlayer, rootDepth);
                    }
                }
                else
                {
                    // Subsequent moves: try null window search first
                    int searchDepth = depth - 1;

                    // Apply LMR to null window search if applicable
                    if (depth >= 3 && moveIndex >= lmrFullDepthMoves && !IsTacticalPosition(board))
                    {
                        searchDepth = depth - 2;
                    }

                    // Null window search (beta-1, beta)
                    eval = Minimax(board, searchDepth, beta - 1, beta, true, aiPlayer, rootDepth);

                    // If null window search is below beta, re-search with full window
                    if (eval < beta && eval > alpha)
                    {
                        // Re-search with full window to get accurate score
                        if (searchDepth == depth - 2)
                        {
                            // Had used LMR, now search at full depth
                            eval = Minimax(board, depth - 1, alpha, beta, true, aiPlayer, rootDepth);
                        }
                        else
                        {
                            // Re-search with full alpha
                            eval = Minimax(board, depth - 1, alpha, beta, true, aiPlayer, rootDepth);
                        }
                    }
                }

                board.GetCell(x, y).Player = Player.None;

                if (eval < minEval)
                {
                    minEval = eval;
                    bestMove = (x, y);
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
    /// Get candidate moves (empty cells near existing stones)
    /// </summary>
    private List<(int x, int y)> GetCandidateMoves(Board board)
    {
        var candidates = new List<(int x, int y)>();
        var considered = new bool[15, 15];

        for (int x = 0; x < 15; x++)
        {
            for (int y = 0; y < 15; y++)
            {
                var cell = board.GetCell(x, y);
                if (cell.Player != Player.None)
                {
                    // Check neighboring cells
                    for (int dx = -SearchRadius; dx <= SearchRadius; dx++)
                    {
                        for (int dy = -SearchRadius; dy <= SearchRadius; dy++)
                        {
                            var nx = x + dx;
                            var ny = y + dy;

                            if (nx >= 0 && nx < 15 && ny >= 0 && ny < 15 && !considered[nx, ny])
                            {
                                if (board.GetCell(nx, ny).Player == Player.None)
                                {
                                    candidates.Add((nx, ny));
                                    considered[nx, ny] = true;
                                }
                            }
                        }
                    }
                }
            }
        }

        // If no candidates (empty board), return center
        if (candidates.Count == 0)
        {
            return new List<(int x, int y)> { (7, 7) };
        }

        return candidates;
    }

    /// <summary>
    /// Check if there's a winner on the board
    /// </summary>
    private Player? CheckWinner(Board board)
    {
        // Check all possible 5-in-row sequences
        for (int x = 0; x < 15; x++)
        {
            for (int y = 0; y < 15; y++)
            {
                var cell = board.GetCell(x, y);
                if (cell.Player == Player.None)
                    continue;

                // Check all 4 directions
                var directions = new[] { (1, 0), (0, 1), (1, 1), (1, -1) };
                foreach (var (dx, dy) in directions)
                {
                    if (CheckDirection(board, x, y, dx, dy, cell.Player))
                    {
                        return cell.Player;
                    }
                }
            }
        }

        return null;
    }

    /// <summary>
    /// Check if there's a winning sequence in a direction
    /// </summary>
    private bool CheckDirection(Board board, int startX, int startY, int dx, int dy, Player player)
    {
        for (int i = 1; i < 5; i++)
        {
            var x = startX + i * dx;
            var y = startY + i * dy;

            if (x < 0 || x >= 15 || y < 0 || y >= 15)
                return false;

            if (board.GetCell(x, y).Player != player)
                return false;
        }

        return true;
    }
}
