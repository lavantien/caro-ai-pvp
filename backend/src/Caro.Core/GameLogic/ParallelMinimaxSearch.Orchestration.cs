using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic.Search;
using Caro.Core.GameLogic.TimeManagement;
using SHC = Caro.Core.Domain.Configuration.SearchHeuristicConstants;

namespace Caro.Core.GameLogic;

public sealed partial class ParallelMinimaxSearch
{
    /// <summary>
    /// Get best move using parallel search (Lazy SMP)
    /// </summary>
    public (int x, int y) GetBestMove(
        Board board,
        Player player,
        long? timeRemainingMs = null,
        TimeAllocation? timeAlloc = null,
        int moveNumber = 0)
    {
        if (player == Player.None)
            throw new ArgumentException("Player cannot be None");

        var searchBoard = new SearchBoard(board);
        var candidates = GetCandidateMoves(searchBoard, MaxSearchRadius);

        // SAFETY: Filter candidates to only empty cells
        candidates.RemoveAll(c => !searchBoard.IsEmpty(c.x, c.y));

        // Apply Open Rule: Red's second move (move #3) must be at least 3 intersections
        // away from the first red stone (5x5 exclusion zone centered on first move)
        if (player == Player.Red && moveNumber == 3)
        {
            candidates.RemoveAll(c => !ParallelThreatAnalyzer.IsValidPerOpenRule(board, c.x, c.y));
        }

        if (candidates.Count == 0)
        {
            if (player == Player.Red && moveNumber == 3)
            {
                int boardSize = board.BoardSize;
                for (int x = 0; x < boardSize; x++)
                {
                    for (int y = 0; y < boardSize; y++)
                    {
                        if (board.GetCell(x, y).Player == Player.None && ParallelThreatAnalyzer.IsValidPerOpenRule(board, x, y))
                            return (x, y);
                    }
                }
            }
            int center = board.BoardSize / 2;
            return (center, center);
        }

        var alloc = timeAlloc ?? GetDefaultTimeAllocation(timeRemainingMs);

        // Try VCF first
        {
            var vcfTimeLimit = CalculateVCFTimeLimit(alloc);
            var vcfResult = _vcfSolver.SolveVCF(board, player, vcfTimeLimit, maxDepth: 30);

            if (vcfResult.IsSolved && vcfResult.IsWin && vcfResult.BestMove.HasValue)
            {
                return vcfResult.BestMove.Value;
            }
        }

        // Check for opponent's CRITICAL threats that must be blocked
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var criticalThreats = ParallelThreatAnalyzer.GetCriticalThreatMoves(board, opponent, _winDetector);
        if (criticalThreats.Count > 0)
        {
            var forcingSet = new HashSet<(int x, int y)>(criticalThreats);
            candidates.RemoveAll(c => !forcingSet.Contains((c.x, c.y)));

            if (candidates.Count == 0)
                candidates = criticalThreats;
        }
        else
        {
            // Check for open threes - must block before they become open fours
            var openThreeBlocks = ParallelThreatAnalyzer.GetOpenThreeBlocks(board, opponent);
            if (openThreeBlocks.Count > 0)
            {
                var filteredCandidates = new List<(int x, int y)>(openThreeBlocks.Count);
                foreach (var c in openThreeBlocks)
                    if (searchBoard.IsEmpty(c.x, c.y))
                        filteredCandidates.Add(c);

                if (filteredCandidates.Count > 0)
                {
                    candidates = filteredCandidates;
                }
            }
        }

        var parallelResult = SearchLazySMP(board, player, candidates, alloc);
        return (parallelResult.X, parallelResult.Y);
    }

    /// <summary>
    /// Get best move using parallel search with full statistics reporting
    /// </summary>
    public ParallelSearchResult GetBestMoveWithStats(
        Board board,
        Player player,
        long? timeRemainingMs = null,
        TimeAllocation? timeAlloc = null,
        int moveNumber = 0,
        int fixedThreadCount = -1,
        List<(int x, int y)>? candidates = null)
    {
        if (player == Player.None)
            throw new ArgumentException("Player cannot be None");

        var searchBoard = new SearchBoard(board);
        candidates ??= GetCandidateMoves(searchBoard, MaxSearchRadius);

        // SAFETY: Filter candidates to only empty cells
        candidates.RemoveAll(c => !searchBoard.IsEmpty(c.x, c.y));

        // Apply Open Rule
        if (player == Player.Red && moveNumber == 3)
        {
            candidates.RemoveAll(c => !ParallelThreatAnalyzer.IsValidPerOpenRule(board, c.x, c.y));
        }

        if (candidates.Count == 0)
        {
            int center = board.BoardSize / 2;
            return new ParallelSearchResult(center, center, 1, 1, 0, null, 0, 0, 0, 0, 0, 0);
        }

        var alloc = timeAlloc ?? GetDefaultTimeAllocation(timeRemainingMs);

        // Try VCF first
        {
            var vcfTimeLimit = CalculateVCFTimeLimit(alloc);
            var vcfResult = _vcfSolver.SolveVCF(board, player, vcfTimeLimit, maxDepth: 30);

            if (vcfResult.IsSolved && vcfResult.IsWin && vcfResult.BestMove.HasValue)
            {
                return new ParallelSearchResult(vcfResult.BestMove.Value.x, vcfResult.BestMove.Value.y,
                    vcfResult.DepthAchieved, vcfResult.NodesSearched, 0, null, vcfTimeLimit, 0, 0, SHC.WinScore, 0, 0);
            }
        }

        // Check for opponent's CRITICAL threats
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var criticalThreats = ParallelThreatAnalyzer.GetCriticalThreatMoves(board, opponent, _winDetector);
        if (criticalThreats.Count > 0)
        {
            var forcingSet = new HashSet<(int x, int y)>(criticalThreats);
            candidates.RemoveAll(c => !forcingSet.Contains((c.x, c.y)));

            if (candidates.Count == 0)
                candidates = criticalThreats;
        }
        else
        {
            var openThreeBlocks = ParallelThreatAnalyzer.GetOpenThreeBlocks(board, opponent);
            if (openThreeBlocks.Count > 0)
            {
                var filteredCandidates = new List<(int x, int y)>(openThreeBlocks.Count);
                foreach (var c in openThreeBlocks)
                    if (searchBoard.IsEmpty(c.x, c.y))
                        filteredCandidates.Add(c);

                if (filteredCandidates.Count > 0)
                {
                    candidates = filteredCandidates;
                }
            }
        }

        return SearchLazySMP(board, player, candidates, alloc, fixedThreadCount);
    }

    /// <summary>
    /// Single-threaded search (fallback for low depths)
    /// </summary>
    private (int x, int y, long nodes) SearchSingleThreaded(SearchBoard board, Player player, int depth, List<(int x, int y)> candidates, int searchRadius)
    {
        var threadData = new ThreadData { SearchRadius = searchRadius };
        var cts = new CancellationTokenSource();
        var token = cts.Token;

        var bestMove = candidates[0];
        var bestScore = int.MinValue;

        foreach (var (x, y) in candidates)
        {
            var undo = board.MakeMove(x, y, player);
            var score = Minimax(board, depth - 1, int.MinValue, int.MaxValue, false, player, depth, threadData, token);
            board.UnmakeMove(undo);

            if (score > bestScore)
            {
                bestScore = score;
                bestMove = (x, y);
            }
        }

        return (bestMove.x, bestMove.y, threadData.LocalNodesSearched);
    }
}
