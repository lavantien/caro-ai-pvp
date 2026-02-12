using Caro.Core.Application.DTOs;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Domain.Interfaces;
using Microsoft.Extensions.Logging;
using ZobristHash = Caro.Core.Domain.ValueObjects.ZobristHash;

namespace Caro.Core.Infrastructure.AI;

/// <summary>
/// Stateless AI search engine
/// All state is injected via AIGameState parameter
/// Enables multiple concurrent games without shared state
/// </summary>
public sealed partial class StatelessSearchEngine
{
    private readonly ILogger<StatelessSearchEngine> _logger;
    private const int WinLength = 5;
    private const int BoardSize = 32;

    public StatelessSearchEngine(ILogger<StatelessSearchEngine> logger)
    {
        _logger = logger;
    }

    /// <summary>
    /// Calculate the best move for the given game state
    /// Stateless - all AI state is passed in via aiState parameter
    /// </summary>
    public (int x, int y, int score, SearchResultStats stats) FindBestMove(
        GameState state,
        AIGameState aiState,
        SearchOptions options,
        CancellationToken cancellationToken = default)
    {
        cancellationToken.ThrowIfCancellationRequested();
        aiState.ResetStatistics();
        var stopwatch = System.Diagnostics.Stopwatch.StartNew();

        int bestX = -1, bestY = -1;
        int bestScore = int.MinValue;

        try
        {
            // Get all empty cells and order them by basic heuristics
            var emptyCells = GetOrderedMoves(state.Board, state.CurrentPlayer, aiState);

            var targetDepth = Math.Min(options.MaxDepth, GetMaxDepthRemaining(state));

            // Iterative deepening
            for (int depth = 1; depth <= targetDepth; depth++)
            {
                cancellationToken.ThrowIfCancellationRequested();

                int currentBestScore = int.MinValue;
                int currentBestX = -1, currentBestY = -1;

                for (int i = 0; i < emptyCells.Length; i++)
                {
                    var (x, y) = emptyCells[i];
                    cancellationToken.ThrowIfCancellationRequested();

                    // Make move on a copy
                    var newBoard = state.Board.PlaceStone(x, y, state.CurrentPlayer);

                    // Check for immediate win
                    if (CheckWin(newBoard, x, y, state.CurrentPlayer))
                    {
                        aiState.LastPV = new[] { new Position(x, y) };
                        StoreTT(newBoard.GetHash(), 100000, depth, new TTMove(x, y), TTFlag.Exact, aiState);
                        return (x, y, 100000 + depth, CreateStats(aiState, stopwatch.ElapsedMilliseconds));
                    }

                    // Alpha-beta search
                    var score = -AlphaBeta(
                        newBoard,
                        depth - 1,
                        int.MinValue + 1,
                        -bestScore,
                        state.CurrentPlayer.Opponent(),
                        aiState,
                        cancellationToken);

                    if (score > currentBestScore)
                    {
                        currentBestScore = score;
                        currentBestX = x;
                        currentBestY = y;
                    }

                    // Update heuristics if this caused a cutoff
                    if (score >= bestScore)
                    {
                        aiState.UpdateHistoryScore(new Position(x, y), depth);
                    }
                }

                bestScore = currentBestScore;
                bestX = currentBestX;
                bestY = currentBestY;
                aiState.MaxDepthReached = depth;

                // Log progress
                _logger.LogDebug("Depth {Depth}: best move ({X},{Y}) score {Score}", depth, bestX, bestY, bestScore);

                // Early exit if we found a winning move
                if (Math.Abs(bestScore) > 90000)
                {
                    break;
                }
            }

            return (bestX, bestY, bestScore, CreateStats(aiState, stopwatch.ElapsedMilliseconds));
        }
        catch (OperationCanceledException)
        {
            // Return best move found so far
            return (bestX, bestY, 0, CreateStats(aiState, stopwatch.ElapsedMilliseconds));
        }
    }

    /// <summary>
    /// Alpha-beta minimax search with stateless design
    /// All mutable state is in aiState parameter
    /// </summary>
    private int AlphaBeta(
        Board board,
        int depth,
        int alpha,
        int beta,
        Player player,
        AIGameState aiState,
        CancellationToken cancellationToken)
    {
        aiState.NodesSearched++;

        // Check transposition table
        var ttEntry = aiState.TranspositionTable.Lookup(new ZobristHash(board.GetHash()), depth);
        if (ttEntry != null)
        {
            aiState.TableHits++;
            if (ttEntry.Depth >= depth)
            {
                return ttEntry.Score;
            }
        }
        aiState.TableLookups++;

        // Terminal conditions
        if (depth <= 0)
        {
            return Evaluate(board, player);
        }

        // Check for game over
        if (board.TotalStones() >= board.TotalCells())
        {
            return 0; // Draw
        }

        var moves = GetOrderedMoves(board, player, aiState);
        if (moves.Length == 0)
        {
            return Evaluate(board, player);
        }

        int bestScore = int.MinValue;
        (int bx, int by) = moves[0];
        TTMove bestMove = new TTMove(bx, by);

        foreach (var (x, y) in moves)
        {
            cancellationToken.ThrowIfCancellationRequested();

            var newBoard = board.PlaceStone(x, y, player);

            // Check for win
            if (CheckWin(newBoard, x, y, player))
            {
                int winScore = 100000 + depth; // Prefer faster wins
                StoreTT(board.GetHash(), winScore, depth, new TTMove(x, y), TTFlag.Exact, aiState);
                return winScore;
            }

            var score = -AlphaBeta(newBoard, depth - 1, -beta, -alpha, player.Opponent(), aiState, cancellationToken);

            if (score > bestScore)
            {
                bestScore = score;
                bestMove = new TTMove(x, y);
            }

            if (score > alpha)
            {
                alpha = score;
            }

            if (alpha >= beta)
            {
                // Cutoff - update killer move
                if (depth < aiState.GetKillerMove(depth, 0).X ||
                    aiState.GetKillerMove(depth, 0).X == -1)
                {
                    aiState.SetKillerMove(depth, 0, new Position(x, y));
                }
                return alpha;
            }
        }

        StoreTT(board.GetHash(), bestScore, depth, new TTMove(bestMove.X, bestMove.Y), GetTTType(bestScore, alpha, beta), aiState);
        return bestScore;
    }

    /// <summary>
    /// Get moves ordered by heuristics for better pruning
    /// </summary>
    private static (int x, int y)[] GetOrderedMoves(
        Board board,
        Player player,
        AIGameState aiState)
    {
        var moves = new List<(int x, int y)>();

        // First, get cells near existing stones (better pruning)
        var occupied = new HashSet<Position>();
        var opponent = player.Opponent();

        // Get cells near both players' stones
        foreach (var (x, y) in board.GetOccupiedCells(player))
        {
            for (int dx = -2; dx <= 2; dx++)
            {
                for (int dy = -2; dy <= 2; dy++)
                {
                    if (dx == 0 && dy == 0) continue;
                    var nx = x + dx;
                    var ny = y + dy;
                    var pos = new Position(nx, ny);
                    if (pos.IsValid() && board.IsEmpty(nx, ny))
                    {
                        occupied.Add(pos);
                    }
                }
            }
        }
        foreach (var (x, y) in board.GetOccupiedCells(opponent))
        {
            for (int dx = -2; dx <= 2; dx++)
            {
                for (int dy = -2; dy <= 2; dy++)
                {
                    if (dx == 0 && dy == 0) continue;
                    var nx = x + dx;
                    var ny = y + dy;
                    var pos = new Position(nx, ny);
                    if (pos.IsValid() && board.IsEmpty(nx, ny))
                    {
                        occupied.Add(pos);
                    }
                }
            }
        }

        // If board is nearly empty, consider center positions
        if (occupied.Count == 0)
        {
            moves.Add((9, 9));
            moves.Add((9, 8));
            moves.Add((8, 9));
            moves.Add((9, 10));
            moves.Add((10, 9));
            moves.Add((8, 8));
            moves.Add((10, 10));
            moves.Add((8, 10));
            moves.Add((10, 8));
            return moves.ToArray();
        }

        // Order by heuristic scores
        var scoredMoves = new List<(int x, int y, int score)>();
        foreach (var pos in occupied)
        {
            var historyScore = aiState.GetHistoryScore(pos);
            var butterflyScore = aiState.GetButterflyScore(pos);
            var positionScore = EvaluatePosition(pos, player);
            scoredMoves.Add((pos.X, pos.Y, historyScore + butterflyScore + positionScore));
        }

        scoredMoves.Sort((a, b) => b.score.CompareTo(a.score));

        return scoredMoves.Take(20).Select(m => (m.x, m.y)).ToArray();
    }

    /// <summary>
    /// Static position evaluation for move ordering
    /// Center positions are preferred
    /// </summary>
    private static int EvaluatePosition(Position position, Player player)
    {
        // Prefer center positions
        var centerX = Math.Abs(position.X - 9);
        var centerY = Math.Abs(position.Y - 9);
        return 100 - centerX - centerY;
    }

    /// <summary>
    /// Static board evaluation
    /// </summary>
    private static int Evaluate(Board board, Player player)
    {
        var opponent = player.Opponent();
        int score = 0;

        // Count stones in center area
        for (int x = 7; x <= 11; x++)
        {
            for (int y = 7; y <= 11; y++)
            {
                if (board.GetCell(x, y).Player == player)
                    score += 10;
                else if (board.GetCell(x, y).Player == opponent)
                    score -= 10;
            }
        }

        // Evaluate patterns (simple version)
        score += EvaluatePatterns(board, player) * 100;
        score -= EvaluatePatterns(board, opponent) * 100;

        return score;
    }

    /// <summary>
    /// Evaluate threat patterns on the board
    /// </summary>
    private static int EvaluatePatterns(Board board, Player player)
    {
        int score = 0;
        var bitBoard = board.GetBitBoard(player);

        // Check for sequences in all directions
        foreach (var (x, y) in board.GetOccupiedCells(player))
        {
            // Horizontal
            score += CountLine(bitBoard, x, y, 1, 0, player);
            // Vertical
            score += CountLine(bitBoard, x, y, 0, 1, player);
            // Diagonal
            score += CountLine(bitBoard, x, y, 1, 1, player);
            score += CountLine(bitBoard, x, y, 1, -1, player);
        }

        return score;
    }

    /// <summary>
    /// Count consecutive stones in a direction
    /// </summary>
    private static int CountLine(BitBoard bitBoard, int startX, int startY, int dx, int dy, Player player)
    {
        int count = 1;

        // Count forward
        int x = startX + dx, y = startY + dy;
        while (x >= 0 && x < BoardSize && y >= 0 && y < BoardSize && bitBoard.GetBit(x, y))
        {
            count++;
            x += dx;
            y += dy;
        }

        // Count backward
        x = startX - dx;
        y = startY - dy;
        while (x >= 0 && x < BoardSize && y >= 0 && y < BoardSize && bitBoard.GetBit(x, y))
        {
            count++;
            x -= dx;
            y -= dy;
        }

        // Exponential scoring for longer lines
        return count switch
        {
            2 => 1,
            3 => 10,
            4 => 100,
            _ => 0
        };
    }

    /// <summary>
    /// Check if placing a stone at (x,y) wins the game
    /// </summary>
    private static bool CheckWin(Board board, int x, int y, Player player)
    {
        var directions = new[] { (1, 0), (0, 1), (1, 1), (1, -1) };

        foreach (var (dx, dy) in directions)
        {
            int count = 1;

            // Count forward
            int nx = x + dx, ny = y + dy;
            while (nx >= 0 && nx < BoardSize && ny >= 0 && ny < BoardSize && board.GetCell(nx, ny).Player == player)
            {
                count++;
                nx += dx;
                ny += dy;
            }

            // Count backward
            nx = x - dx;
            ny = y - dy;
            while (nx >= 0 && nx < BoardSize && ny >= 0 && ny < BoardSize && board.GetCell(nx, ny).Player == player)
            {
                count++;
                nx -= dx;
                ny -= dy;
            }

            if (count >= WinLength)
                return true;
        }

        return false;
    }

    /// <summary>
    /// Get appropriate search depth based on moves remaining
    /// </summary>
    private static int GetMaxDepthRemaining(GameState state)
    {
        var remaining = state.Board.TotalCells() - state.Board.TotalStones();

        return remaining switch
        {
            <= 10 => 20,  // Endgame - deep search
            <= 30 => 15,  // Middlegame
            <= 100 => 10, // Early-midgame
            _ => 6         // Opening - shallow search
        };
    }

    /// <summary>
    /// Store result in transposition table
    /// </summary>
    private static void StoreTT(ulong hash, int score, int depth, TTMove bestMove, TTFlag flag, AIGameState aiState)
    {
        aiState.TranspositionTable.Store(new ZobristHash(hash), depth, score, bestMove, flag, aiState.Age);
    }

    /// <summary>
    /// Determine TT entry type from search bounds
    /// </summary>
    private static TTFlag GetTTType(int score, int alpha, int beta)
    {
        if (score <= alpha) return TTFlag.UpperBound;
        if (score >= beta) return TTFlag.LowerBound;
        return TTFlag.Exact;
    }

    /// <summary>
    /// Create search result statistics
    /// </summary>
    private static SearchResultStats CreateStats(AIGameState aiState, long elapsedMs)
    {
        return new SearchResultStats
        {
            NodesSearched = aiState.NodesSearched,
            DepthReached = aiState.MaxDepthReached,
            TableHits = aiState.TableHits,
            TableLookups = aiState.TableLookups,
            HitRate = aiState.TableLookups > 0 ? (double)aiState.TableHits / aiState.TableLookups : 0,
            ElapsedMs = elapsedMs,
            NodesPerSecond = elapsedMs > 0 ? (double)aiState.NodesSearched * 1000 / elapsedMs : 0
        };
    }
}

/// <summary>
/// Search options
/// </summary>
public sealed record SearchOptions
{
    public int MaxDepth { get; init; } = 15;
    public long TimeLimitMs { get; init; } = 5000;
    public bool UseIterativeDeepening { get; init; } = true;
    public bool UseParallelSearch { get; init; } = false;
}

/// <summary>
/// Search result statistics
/// </summary>
public sealed record SearchResultStats
{
    public long NodesSearched { get; init; }
    public int DepthReached { get; init; }
    public int TableHits { get; init; }
    public int TableLookups { get; init; }
    public double HitRate { get; init; }
    public long ElapsedMs { get; init; }
    public double NodesPerSecond { get; init; }
}
