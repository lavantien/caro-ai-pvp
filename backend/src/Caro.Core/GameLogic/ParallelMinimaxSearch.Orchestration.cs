using System.Collections.Concurrent;
using System.Diagnostics;
using System.Runtime.CompilerServices;
using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic.Search;
using Caro.Core.GameLogic.TimeManagement;
using TMC = Caro.Core.Domain.Configuration.TimeManagementConstants;
using SHC = Caro.Core.Domain.Configuration.SearchHeuristicConstants;
using TC = Caro.Core.Domain.Configuration.TimeConstants;

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

        // SAFETY: Filter candidates to only empty cells to prevent "Cell is already occupied" errors
        candidates.RemoveAll(c => !searchBoard.IsEmpty(c.x, c.y));

        // Apply Open Rule: Red's second move (move #3) must be at least 3 intersections
        // away from the first red stone (5x5 exclusion zone centered on first move)
        if (player == Player.Red && moveNumber == 3)
        {
            candidates.RemoveAll(c => !ParallelThreatAnalyzer.IsValidPerOpenRule(board, c.x, c.y));
        }

        if (candidates.Count == 0)
        {
            // No valid candidates - board is empty or all filtered out
            if (player == Player.Red && moveNumber == 3)
            {
                // Open rule applies - find first valid cell outside exclusion zone
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
            return (center, center); // Center move
        }

        // Use provided time allocation or create default
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
        // CRITICAL FIX: Only filter for MUST-BLOCK threats (immediate wins, open/semi-open fours)
        // Do NOT filter for BrokenFours - let search evaluate offensive vs defensive options
        // This prevents Grandmaster from being forced into purely defensive play
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var criticalThreats = ParallelThreatAnalyzer.GetCriticalThreatMoves(board, opponent, _winDetector);
        if (criticalThreats.Count > 0)
        {
            // Filter candidates to only blocking moves for CRITICAL threats
            var forcingSet = new HashSet<(int x, int y)>(criticalThreats);
            candidates.RemoveAll(c => !forcingSet.Contains((c.x, c.y)));

            // If candidates ended up empty (edge case), fallback to original threat list
            if (candidates.Count == 0)
                candidates = criticalThreats;
        }
        else
        {
            // No critical threats (StraightFour, immediate wins), but check for open threes
            // Open threes (StraightThree) become open fours in ONE move
            // CRITICAL FIX: If opponent has an open three, we MUST block it
            // Filtering to only blocking squares is necessary because:
            // 1. At depth 2-3, search cannot see far enough to recognize the threat
            // 2. Evaluation may score offensive moves higher than blocking moves
            // 3. Open threes lead to open fours which are unblockable (2 winning squares)
            var openThreeBlocks = ParallelThreatAnalyzer.GetOpenThreeBlocks(board, opponent);
            if (openThreeBlocks.Count > 0)
            {
                // FILTER candidates to only blocking squares - this is critical!
                // Prioritization alone doesn't work because search evaluates all moves
                // and may pick a non-blocking move with higher score
                var filteredCandidates = new List<(int x, int y)>(openThreeBlocks.Count);
                foreach (var c in openThreeBlocks)
                    if (searchBoard.IsEmpty(c.x, c.y))
                        filteredCandidates.Add(c);

                // CRITICAL FIX: Only use filtered candidates if they're not empty
                // If all blocking squares are somehow occupied, keep original candidates
                if (filteredCandidates.Count > 0)
                {
                    candidates = filteredCandidates;
                }
            }
        }

        // NPS is learned from actual search performance - no hardcoded targets

        // Multi-threaded Lazy SMP
        var parallelResult = SearchLazySMP(board, player, candidates, alloc);
        return (parallelResult.X, parallelResult.Y);
    }

    /// <summary>
    /// Get best move using parallel search with full statistics reporting
    /// Returns move coordinates along with depth achieved and nodes searched
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

        // SAFETY: Filter candidates to only empty cells to prevent "Cell is already occupied" errors
        // This ensures robustness even if GetCandidateMoves or external callers provide occupied cells
        candidates.RemoveAll(c => !searchBoard.IsEmpty(c.x, c.y));

        // Apply Open Rule: Red's second move (move #3) must be at least 3 intersections
        // away from the first red stone (5x5 exclusion zone centered on first move)
        if (player == Player.Red && moveNumber == 3)
        {
            candidates.RemoveAll(c => !ParallelThreatAnalyzer.IsValidPerOpenRule(board, c.x, c.y));
        }

        if (candidates.Count == 0)
        {
            // Empty board - return center move with depth 1 (not 0, which is misleading)
            // For empty board, center is the only reasonable move
            int center = board.BoardSize / 2;
            return new ParallelSearchResult(center, center, 1, 1, 0, null, 0, 0, 0, 0, 0, 0);
        }

        // Use provided time allocation or create default
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

        // Check for opponent's CRITICAL threats that must be blocked
        // CRITICAL FIX: Only filter for MUST-BLOCK threats (immediate wins, open/semi-open fours)
        // Do NOT filter for BrokenFours - let search evaluate offensive vs defensive options
        // This prevents Grandmaster from being forced into purely defensive play
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var criticalThreats = ParallelThreatAnalyzer.GetCriticalThreatMoves(board, opponent, _winDetector);
        if (criticalThreats.Count > 0)
        {
            // Filter candidates to only blocking moves for CRITICAL threats
            var forcingSet = new HashSet<(int x, int y)>(criticalThreats);
            candidates.RemoveAll(c => !forcingSet.Contains((c.x, c.y)));

            // If candidates ended up empty (edge case), fallback to original threat list
            if (candidates.Count == 0)
                candidates = criticalThreats;
        }
        else
        {
            // No critical threats (StraightFour, immediate wins), but check for open threes
            // Open threes (StraightThree) become open fours in ONE move
            // CRITICAL FIX: If opponent has an open three, we MUST block it
            // Filtering to only blocking squares is necessary because:
            // 1. At depth 2-3, search cannot see far enough to recognize the threat
            // 2. Evaluation may score offensive moves higher than blocking moves
            // 3. Open threes lead to open fours which are unblockable (2 winning squares)
            var openThreeBlocks = ParallelThreatAnalyzer.GetOpenThreeBlocks(board, opponent);
            if (openThreeBlocks.Count > 0)
            {
                // FILTER candidates to only blocking squares - this is critical!
                // Prioritization alone doesn't work because search evaluates all moves
                // and may pick a non-blocking move with higher score
                var filteredCandidates = new List<(int x, int y)>(openThreeBlocks.Count);
                foreach (var c in openThreeBlocks)
                    if (searchBoard.IsEmpty(c.x, c.y))
                        filteredCandidates.Add(c);

                // CRITICAL FIX: Only use filtered candidates if they're not empty
                // If all blocking squares are somehow occupied, keep original candidates
                if (filteredCandidates.Count > 0)
                {
                    candidates = filteredCandidates;
                }
            }
        }

        // PURE TIME-BASED: Always use SearchLazySMP which will internally decide thread count
        return SearchLazySMP(board, player, candidates, alloc, fixedThreadCount);
    }

    /// <summary>
    /// Single-threaded search (fallback for low depths)
    /// Note: TranspositionTable age is incremented by caller
    /// Returns node count via out parameter for accurate reporting
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

    /// <summary>
    /// Lazy SMP: Multiple threads search independently with shared TT
    /// Each thread has slight variation to explore different parts of tree
    /// PURE TIME-BASED: No depth caps - search continues until time runs out.
    /// Thread count is based on processor count, not difficulty.
    /// </summary>
    private ParallelSearchResult SearchLazySMP(
        Board board,
        Player player,
        List<(int x, int y)> candidates,
        TimeAllocation timeAlloc,
        int fixedThreadCount = -1)
    {
        _transpositionTable.IncrementAge();

        // Set up time management with new CancellationTokenSource
        _searchCts?.Cancel(); // Cancel any previous search
        _searchCts = new CancellationTokenSource();

        // Create timer-based time monitor (polls every 10ms)
        // This is more accurate and less taxing than node-count-based checking
        _timeMonitor?.Dispose();
        _timeMonitor = new TimeMonitor(
            hardTimeBoundMs: timeAlloc.HardBoundMs,
            softTimeBoundMs: timeAlloc.SoftBoundMs,
            cts: _searchCts);
        _hardTimeBoundMs = timeAlloc.HardBoundMs;

        // Thread count based on processor count
        // fixedThreadCount = 0 means single-threaded, -1 means use default
        int threadCount = fixedThreadCount >= 0
            ? fixedThreadCount  // 0 = single-threaded, >0 = use that many threads
            : ThreadPoolConfig.GetLazySMPThreadCount();

        // If threadCount is 0 or 1, fall back to single-threaded search
        if (threadCount <= 1)
        {
            _transpositionTable.IncrementAge();

            // Convert Board to SearchBoard once at the boundary
            var singleSearchBoard = new SearchBoard(board);

            // Use time-based single-threaded search (no depth cap)
            var threadData = new ThreadData { ThreadIndex = 0, SearchRadius = MaxSearchRadius };
            var (x, y, score, depth, nodes) = SearchWithIterationTimeAware(
                singleSearchBoard, player, candidates, threadData, timeAlloc, _searchCts.Token);

            // Calculate FMC% for single-threaded search
            double singleFmcPercent = threadData.TotalCutoffs > 0
                ? (threadData.FirstMoveCutoffs * 100.0 / threadData.TotalCutoffs)
                : 0;

            return new ParallelSearchResult(x, y, depth, nodes, 1, null, _hardTimeBoundMs, 0, 0, score, singleFmcPercent, _depthManager.GetEstimatedEbf());
        }

        // Use thread-safe collections with Task-based parallelism
        var results = new ConcurrentBag<(int x, int y, int score, int depth, long nodes, int threadIndex)>();
        var diagnosticsList = new ConcurrentBag<ThreadData>();

        // Create thread-local copies of SearchBoard and candidates for each thread
        var rootSearchBoard = new SearchBoard(board);
        var boardsArray = new SearchBoard[threadCount];
        var candidatesArray = new List<(int x, int y)>[threadCount];
        for (int i = 0; i < threadCount; i++)
        {
            boardsArray[i] = rootSearchBoard.Clone();
            candidatesArray[i] = new List<(int x, int y)>(candidates);
        }

        // Launch parallel searches using Task.Run with LongRunning option for true parallelism
        // This fixes the memory visibility issue with Thread+ConcurrentBag
        var token = _searchCts.Token;
        var tasks = new Task[threadCount];

        for (int i = 0; i < threadCount; i++)
        {
            int threadId = i;

            // Use Task.Factory.StartNew with LongRunning for dedicated threads
            // This ensures true parallelism similar to the original Thread approach
            tasks[i] = Task.Factory.StartNew(() =>
            {
                // CRITICAL FIX: Create threadData OUTSIDE try block so it's available
                // in finally block for diagnostics collection, even when cancelled
                var threadData = new ThreadData
                {
                    ThreadIndex = threadId,
                    SearchRadius = MaxSearchRadius,
                    Random = new Random((int)(Environment.TickCount64 + (long)threadId * 0x9E3779B9L))
                };

                try
                {
                    var result = SearchWithIterationTimeAware(
                        boardsArray[threadId], player, candidatesArray[threadId],
                        threadData, timeAlloc, token);

                    // Add threadIndex to identify master vs helper thread results
                    var (x, y, score, depthAchieved, nodes) = result;
                    results.Add((x, y, score, depthAchieved, nodes, threadId));
                }
                catch (OperationCanceledException)
                {
                    // Expected when time runs out - not an error
                }
                catch (Exception)
                {
                    // Thread exception - search will continue with available results
                }
                finally
                {
                    // CRITICAL FIX: Always collect diagnostics, even when cancelled
                    // This ensures node counts are available for the fallback calculation
                    diagnosticsList.Add(threadData);
                }
            }, CancellationToken.None, TaskCreationOptions.LongRunning, TaskScheduler.Default);
        }

        // Wait for all tasks to complete with proper synchronization
        // Task.WhenAll ensures all results are visible to this thread
        // CRITICAL FIX: Reduced timeout from HardBoundMs+1000 to HardBoundMs+200
        // The old timeout caused 2x time overrun (wait + fallback both used full allocation)
        try
        {
            Task.WaitAll(tasks, (int)(timeAlloc.HardBoundMs + TC.HardBoundBufferMs));
        }
        catch (AggregateException)
        {
            // Some tasks may have thrown - continue with available results
        }

        // CRITICAL FIX: Cancel parallel tasks before checking results
        // Without this, timed-out tasks continue running and cause CPU contention
        // with the fallback search, resulting in extremely slow NPS (300-400 vs 5000+)
        _searchCts?.Cancel();

        // Brief wait for tasks to acknowledge cancellation and release resources
        try
        {
            Task.WaitAll(tasks, 100);
        }
        catch (AggregateException)
        {
            // Tasks may throw on cancellation - ignore
        }

        // INTELLIGENT MERGING: Aggregate results from ALL threads
        // Lazy SMP works best when we consider all thread results, not just master
        // Master thread is more reliable (less cancellation), but helpers can find better moves
        //
        // Selection priority:
        // 1. Depth (deeper is always better)
        // 2. Score (at same depth, higher score wins)
        // 3. Thread reliability (master > helper as tiebreaker)
        //
        // This is the MERGER's job - aggregate intelligently, not authoritarian rejection

        // CRITICAL FIX: Calculate remaining time for fallback
        // Parallel search already used time waiting for tasks
        // Fallback should only use remaining time to avoid 2x time overrun
        long elapsedMs = _timeMonitor?.ElapsedMs ?? 0;
        long remainingHardBoundMs = Math.Max(TC.MinRemainingHardBoundMs, _hardTimeBoundMs - elapsedMs / 2);  // At least 50ms, account for parallel overhead
        var fallbackTimeAlloc = new TimeAllocation
        {
            SoftBoundMs = Math.Max(TC.MinSoftBoundFallbackMs, remainingHardBoundMs / 2),
            HardBoundMs = remainingHardBoundMs,
            OptimalTimeMs = Math.Max(TC.MinSoftBoundFallbackMs, remainingHardBoundMs / 4),
            IsEmergency = timeAlloc.IsEmergency,
            Phase = timeAlloc.Phase
        };

        // Group results by depth, then select best within each depth
        if (results.IsEmpty)
        {
            // CRITICAL FIX: Parallel search failed - fall back to single-threaded search
            // Use remaining time allocation, not original
            if (DebugLogging) Console.WriteLine("[PARALLEL] Falling back to single-threaded (no results)");
            _timeMonitor?.Dispose();
            _timeMonitor = new TimeMonitor(fallbackTimeAlloc.HardBoundMs, _searchCts!);
            _hardTimeBoundMs = fallbackTimeAlloc.HardBoundMs;  // Update hard bound for fallback
            var fallbackSearchBoard = rootSearchBoard.Clone();
            var fallbackThreadData = new ThreadData { ThreadIndex = 0, SearchRadius = MaxSearchRadius };
            var (fx, fy, fscore, fdepth, fnodes) = SearchWithIterationTimeAware(
                fallbackSearchBoard, player, candidates, fallbackThreadData, fallbackTimeAlloc, CancellationToken.None);
            double fmc = fallbackThreadData.TotalCutoffs > 0 ? (fallbackThreadData.FirstMoveCutoffs * 100.0 / fallbackThreadData.TotalCutoffs) : 0;
            return new ParallelSearchResult(fx, fy, fdepth, fnodes, 1, null, _hardTimeBoundMs, 0, 0, fscore, fmc, _depthManager.GetEstimatedEbf());
        }

        var maxDepth = results.Max(r => r.depth);
        if (maxDepth <= 0)
        {
            // CRITICAL FIX: Parallel search returned invalid depth - fall back to single-threaded
            if (DebugLogging) Console.WriteLine("[PARALLEL] Falling back to single-threaded (invalid depth)");
            _timeMonitor?.Dispose();
            _timeMonitor = new TimeMonitor(fallbackTimeAlloc.HardBoundMs, _searchCts!);
            _hardTimeBoundMs = fallbackTimeAlloc.HardBoundMs;  // Update hard bound for fallback
            var fallbackSearchBoard = rootSearchBoard.Clone();
            var fallbackThreadData = new ThreadData { ThreadIndex = 0, SearchRadius = MaxSearchRadius };
            var (fx, fy, fscore, fdepth, fnodes) = SearchWithIterationTimeAware(
                fallbackSearchBoard, player, candidates, fallbackThreadData, fallbackTimeAlloc, CancellationToken.None);
            double fmc = fallbackThreadData.TotalCutoffs > 0 ? (fallbackThreadData.FirstMoveCutoffs * 100.0 / fallbackThreadData.TotalCutoffs) : 0;
            return new ParallelSearchResult(fx, fy, fdepth, fnodes, 1, null, _hardTimeBoundMs, 0, 0, fscore, fmc, _depthManager.GetEstimatedEbf());
        }

        // CRITICAL FIX: Select the best valid result, avoiding int.MinValue scores
        // int.MinValue EXACTLY indicates search failure (cancellation, no moves, etc.)
        // Scores close to int.MinValue (like int.MinValue + 1000) indicate losing positions
        // Strategy: Try maxDepth first, but prefer lower depths with reasonable scores
        // over higher depths with extremely negative scores
        (int x, int y, int score, int depth, long nodes, int threadIndex) bestResult = default;
        bool foundValidResult = false;

        // Score threshold: below this, consider the position "effectively lost"
        // int.MinValue + 1000000 = -2147482648, which is still a valid but terrible score
        // We want to fall back to lower depths if all scores at higher depths are this bad
        const int ReasonableScoreThreshold = (int)SHC.ReasonableScoreThreshold;  // -2147383648

        // First, try to find results at maxDepth with reasonable scores
        var reasonableAtMaxDepth = results
            .Where(r => r.depth == maxDepth && r.score > ReasonableScoreThreshold)
            .ToList();

        if (reasonableAtMaxDepth.Count > 0)
        {
            // Found reasonable results at max depth - pick the best one
            bestResult = reasonableAtMaxDepth
                .OrderByDescending(r => r.score)  // Highest score first
                .ThenBy(r => r.threadIndex == 0 ? 0 : 1)  // Master thread as tiebreaker
                .First();
            foundValidResult = true;
        }
        else
        {
            // All scores at maxDepth are extremely negative or int.MinValue
            // Try lower depths for better results
            for (int tryDepth = maxDepth - 1; tryDepth >= 1 && !foundValidResult; tryDepth--)
            {
                var validAtDepth = results
                    .Where(r => r.depth == tryDepth && r.score != int.MinValue)
                    .ToList();

                if (validAtDepth.Count > 0)
                {
                    // Found valid results at this depth - pick the best one
                    bestResult = validAtDepth
                        .OrderByDescending(r => r.score)
                        .ThenBy(r => r.threadIndex == 0 ? 0 : 1)
                        .First();
                    foundValidResult = true;
                }
            }

            // If still no valid results, use maxDepth results (even if very negative)
            if (!foundValidResult)
            {
                var anyAtMaxDepth = results
                    .Where(r => r.depth == maxDepth && r.score != int.MinValue)
                    .ToList();

                if (anyAtMaxDepth.Count > 0)
                {
                    bestResult = anyAtMaxDepth
                        .OrderByDescending(r => r.score)
                        .First();
                    foundValidResult = true;
                }
            }
        }

        // If no valid results at any depth, use master thread's result as last resort
        if (!foundValidResult)
        {
            // All threads returned int.MinValue - this should be extremely rare
            // Prefer master thread's result as it has the most reliable search
            var masterResult = results.FirstOrDefault(r => r.threadIndex == 0);
            if (!masterResult.Equals(default))
            {
                bestResult = masterResult;
            }
            else
            {
                // Last resort: pick any result
                bestResult = results.First();
            }
        }

        // DEBUG: Log all thread results and selection
        if (DebugLogging)
        {
            Console.WriteLine("[PARALLEL DEBUG] Thread results:");
            foreach (var r in results.OrderByDescending(r => r.depth).ThenBy(r => r.threadIndex))
            {
                Console.WriteLine($"  Thread {r.threadIndex}: move=({r.x},{r.y}), depth={r.depth}, score={r.score}, nodes={r.nodes}");
            }
            Console.WriteLine($"  SELECTED: move=({bestResult.x},{bestResult.y}), depth={bestResult.depth}, score={bestResult.score}, from thread {bestResult.threadIndex}");
        }

        // CRITICAL FIX: Aggregate local node counts from all threads (no Interlocked contention)
        // Each thread counted locally, now sum them up for accurate total
        long totalNodesFinal = results.Sum(r => r.nodes);

        // Build parallel diagnostics string
        var diagBuilder = new System.Text.StringBuilder();

        // Helper thread depths
        var helperResults = results.Where(r => r.threadIndex > 0).ToList();
        if (helperResults.Count > 0)
        {
            var helperDepths = helperResults.Select(r => r.depth).ToList();
            var maxHelperDepth = helperDepths.Max();
            var minHelperDepth = helperDepths.Min();
            var avgHelperDepth = helperDepths.Average();
            diagBuilder.Append($"Helpers: {helperResults.Count} threads, ");
            diagBuilder.Append($"Depths: min={minHelperDepth}, max={maxHelperDepth}, avg={avgHelperDepth:F1}");
        }

        // TT provenance diagnostics
        var masterDiag = diagnosticsList.FirstOrDefault(d => d.ThreadIndex == 0);
        if (masterDiag != null)
        {
            var totalReads = masterDiag.TTReadsFromMaster + masterDiag.TTReadsFromHelpers;
            var masterRate = totalReads > 0 ? (double)masterDiag.TTReadsFromMaster / totalReads * 100 : 0;

            if (diagBuilder.Length > 0)
                diagBuilder.Append("; ");

            diagBuilder.Append($"TT: {masterDiag.TTReadsFromMaster}M/{masterDiag.TTReadsFromHelpers}H reads, ");
            diagBuilder.Append($"{masterRate:F0}% from master");
        }

        string? diagnostics = diagBuilder.Length > 0 ? diagBuilder.ToString() : null;

        // Aggregate TT stats from all threads
        int totalTableHits = diagnosticsList.Sum(d => d.TableHits);
        int totalTableLookups = diagnosticsList.Sum(d => d.TableLookups);

        // Calculate FMC% (First Move Cutoff %) for move ordering quality
        long totalCutoffs = diagnosticsList.Sum(d => d.TotalCutoffs);
        long firstMoveCutoffs = diagnosticsList.Sum(d => d.FirstMoveCutoffs);
        double fmcPercent = totalCutoffs > 0 ? (firstMoveCutoffs * 100.0 / totalCutoffs) : 0;

        // DEFENSIVE: Validate the best move is actually in the candidates list and is empty
        // This catches any bugs where the search might return an invalid move

        // CRITICAL FIX: Handle empty candidates list
        if (candidates.Count == 0)
        {
            // No candidates available - find any empty cell on the board
            int center = board.BoardSize / 2;
            for (int radius = 0; radius < board.BoardSize; radius++)
            {
                for (int dx = -radius; dx <= radius; dx++)
                {
                    for (int dy = -radius; dy <= radius; dy++)
                    {
                        int nx = center + dx;
                        int ny = center + dy;
                        if (nx >= 0 && nx < board.BoardSize && ny >= 0 && ny < board.BoardSize)
                        {
                            if (board.GetCell(nx, ny).IsEmpty)
                            {
                                Console.WriteLine($"[SEARCH ERROR] Empty candidates - using fallback ({nx},{ny})");
                                return new ParallelSearchResult(nx, ny, 1, totalNodesFinal, threadCount, diagnostics, _hardTimeBoundMs, totalTableHits, totalTableLookups, bestResult.score, fmcPercent, _depthManager.GetEstimatedEbf());
                            }
                        }
                    }
                }
            }
            // Board is completely full (shouldn't happen in a real game)
            Console.WriteLine($"[SEARCH ERROR] Board is full - returning center");
            return new ParallelSearchResult(center, center, 1, totalNodesFinal, threadCount, diagnostics, _hardTimeBoundMs, totalTableHits, totalTableLookups, bestResult.score, fmcPercent, _depthManager.GetEstimatedEbf());
        }

        var bestMoveInCandidates = candidates.Any(c => c.x == bestResult.x && c.y == bestResult.y);
        if (!bestMoveInCandidates)
        {
            // Search returned a move not in candidates - this is a bug
            // Fall back to first candidate
            Console.WriteLine($"[SEARCH ERROR] Best move ({bestResult.x},{bestResult.y}) not in candidates list - using first candidate");
            bestResult = (candidates[0].x, candidates[0].y, bestResult.score, bestResult.depth, bestResult.nodes, bestResult.threadIndex);
        }
        else if (!board.GetCell(bestResult.x, bestResult.y).IsEmpty)
        {
            // Search returned an occupied cell - this is a critical bug
            // Find the first empty candidate
            var emptyCandidate = candidates.FirstOrDefault(c => board.GetCell(c.x, c.y).IsEmpty, candidates[0]);
            Console.WriteLine($"[SEARCH ERROR] Best move ({bestResult.x},{bestResult.y}) is occupied - using fallback ({emptyCandidate.x},{emptyCandidate.y})");
            bestResult = (emptyCandidate.x, emptyCandidate.y, bestResult.score, bestResult.depth, bestResult.nodes, bestResult.threadIndex);
        }

        return new ParallelSearchResult(bestResult.x, bestResult.y, bestResult.depth, totalNodesFinal, threadCount, diagnostics, _hardTimeBoundMs, totalTableHits, totalTableLookups, bestResult.score, fmcPercent, _depthManager.GetEstimatedEbf());
    }

    /// <summary>
    /// Iterative deepening search for a single thread with time awareness
    /// Implements move stability detection for early termination
    /// Note: Node counting is done globally via Interlocked in Minimax()
    /// CRITICAL FIX: Only master thread (ThreadIndex=0) can trigger cancellation
    /// Helper threads must NOT cancel as they complete early and would interrupt deeper searches
    ///
    /// PURE TIME-BASED: No depth caps - search continues until time runs out.
    /// Different machines will naturally reach different depths based on their performance.
    /// </summary>
    private (int x, int y, int score, int depth, long nodes) SearchWithIterationTimeAware(
        SearchBoard board,
        Player player,
        List<(int x, int y)> candidates,
        ThreadData threadData,
        TimeAllocation timeAlloc,
        CancellationToken cancellationToken)
    {
        // CRITICAL FIX: Preserve priority moves (blocking squares) at the front
        // The caller may have already prioritized blocking squares for open threes
        // Pre-sorting by evaluation would undo this prioritization
        // Solution: Keep the first few candidates in their original order (they're priority moves)
        // and only sort the rest by static evaluation

        const int PriorityMoveCount = SHC.PriorityMoveCount; // First N candidates are considered "priority" and not re-sorted

        // Filter to empty cells only to prevent PlaceStone from throwing
        var emptyCandidates = candidates
            .Where(c => board.IsEmpty(c.x, c.y))
            .ToList();

        // Separate priority moves (first N) from the rest
        var priorityMoves = emptyCandidates.Take(PriorityMoveCount).ToList();
        var remainingCandidates = emptyCandidates.Skip(PriorityMoveCount).ToList();

        // Sort remaining candidates by static evaluation
        var sortedRemaining = remainingCandidates
            .Select(c =>
            {
                var undo = board.MakeMove(c.x, c.y, player);
                int eval = ParallelNodeEvaluator.Evaluate(board, player);
                board.UnmakeMove(undo);
                return (c, eval);
            })
            .OrderByDescending(x => x.eval)
            .Select(x => x.c)
            .ToList();

        // Combine: priority moves first, then sorted remaining
        var evaluatedCandidates = priorityMoves.Concat(sortedRemaining).ToList();

        // Initialize bestMove with the first candidate (highest priority - may be a blocking square)
        var bestMove = evaluatedCandidates.Count > 0 ? evaluatedCandidates[0] : candidates[0];
        var bestScore = int.MinValue;
        int bestDepth = 1;

        // FIX 1: Track best move from completed depth separately
        // This is preserved even if current iteration aborts
        int lastCompletedDepth = 0;
        (int x, int y) bestMoveFromCompletedDepth = bestMove;
        int stableCount = 0;
        long lastIterationElapsedMs = 0;
        long iterationStartMs = 0;  // Track start time of current iteration
        long nodesAtStart = threadData.LocalNodesSearched;  // Track nodes at iteration start

        bool isMasterThread = threadData.ThreadIndex == 0;

        // PURE TIME-BASED SEARCH
        // Search continues until time runs out
        // LAZY SMP: Per Chessprogramming Wiki, helper threads should search at different
        // depths to exploit nondeterminism. Cheng uses: current depth + (1 for each even helper)
        // This provides:
        // 1. Some threads complete deeper searches (D2) before master completes D1
        // 2. Shared hash table benefits from different search paths
        // 3. Nondeterminism from depth diversity + move ordering + timing
        //
        // Implementation:
        // - Master (ThreadIndex=0): Start at depth 1
        // - Helper odd (ThreadIndex=1,3,...): Start at depth 2
        // - Helper even (ThreadIndex=2,4,...): Start at depth 1
        //
        // This ensures at least some threads attempt D2 even at blitz time controls,
        // which is critical because D2 can see immediate threats that D1 cannot.
        int depthOffset = threadData.ThreadIndex % 2 == 1 ? 1 : 0;
        int currentDepth = 1 + depthOffset;
        const int MaxSearchDepth = SHC.MaxSearchDepth; // Realistic max for Caro - prevents bogus depth inflation from TT hits
        while (true)
        {
            // MAX DEPTH CHECK: Prevent runaway depth values
            // When TT hit rate is high, later iterations can complete very quickly,
            // causing depth to increment thousands of times in milliseconds.
            // Cap at reasonable maximum for Caro (games rarely exceed 100 moves).
            if (currentDepth > MaxSearchDepth)
            {
                break;
            }

            // CRITICAL: Pre-iteration check - Total nodes must scale with depth
            // Real search depth is bounded by: nodes ≈ branching_factor^depth
            // With aggressive pruning, effective branching factor is ~2-3
            // So D20 requires at least 2^20 ≈ 1M nodes, D30 requires 1B nodes, etc.
            // For practical purposes, require: total_nodes >= (depth-5)^2 * 200 for depth > 10
            // D15: 20K nodes, D20: 45K nodes, D30: 125K nodes, D50: 405K nodes
            // IMPORTANT: Only apply for depth > 10 to allow normal search to proceed
            // This catches cases where TT hits allow depth to increment without real search
            //
            // PARALLEL FIX: Each thread searches a portion of total nodes.
            // For N threads, each thread contributes ~total/N nodes.
            // So the per-thread threshold should be total/N, not total.
            // This ensures parallel search can reach the same depth as sequential search.
            if (currentDepth > 10)
            {
                long minimumTotalNodesForDepth = (long)(currentDepth - SHC.DepthEstimationBaseline) * (currentDepth - SHC.DepthEstimationBaseline) * SHC.DepthEstimationMultiplier;
                // Get the thread count used in this search (passed via closure or member)
                int threadCount = _maxThreads > 0 ? _maxThreads : 1;
                // Per-thread minimum is total / threadCount
                long perThreadMinimum = minimumTotalNodesForDepth / threadCount;
                if (threadData.LocalNodesSearched < perThreadMinimum)
                {
                    // Not enough total nodes to justify this depth - stop now
                    break;
                }
            }

            // Record iteration start time BEFORE any work
            iterationStartMs = _timeMonitor?.ElapsedMs ?? 0;

            // TIME BOUND ENFORCEMENT
            // CRITICAL: Always check hard bound, even at D1-D2, to prevent massive time overruns
            // At blitz time controls, D2 can take 2+ seconds which exceeds the 900ms budget
            var elapsedForCheck = _timeMonitor?.ElapsedMs ?? 0;
            long remainingTimeMs = _hardTimeBoundMs - elapsedForCheck;

            // Hard bound check - ALL threads must stop when time is up
            if (elapsedForCheck >= _hardTimeBoundMs)
            {
                if (isMasterThread) _searchCts?.Cancel();
                break;
            }

            // PRE-ITERATION TIME ESTIMATE
            // Estimate if the next iteration can complete in time.
            // Each depth iteration typically takes 2-4x the previous iteration in nodes,
            // but can take 5-10x in time due to deeper search complexity.
            // CRITICAL FIX: Apply for D2+ to prevent time overruns at blitz time controls.
            // D1 is always allowed (completes quickly), but D2+ checks time budget.
            // For D2, estimate based on D1 time * 5 (conservative for time complexity).
            // For D3+, use actual last iteration time * 2.
            if (currentDepth >= 2)
            {
                long estimatedIterationTimeMs;
                if (currentDepth == 2 && lastIterationElapsedMs > 0)
                {
                    // D2 estimate: D1 time * 5 (D2 time is often 5-10x D1 due to deeper complexity)
                    // This is more conservative than the node-based EBF of 2.5
                    estimatedIterationTimeMs = lastIterationElapsedMs * SHC.IterationTimeEstimateAggressive;
                }
                else if (currentDepth > 2 && lastIterationElapsedMs > 0)
                {
                    // D3+ estimate: last iteration * 2
                    estimatedIterationTimeMs = lastIterationElapsedMs * SHC.IterationTimeEstimateNormal;
                }
                else
                {
                    estimatedIterationTimeMs = 0; // No estimate available
                }

                // Only skip if we have an estimate and not enough time
                if (estimatedIterationTimeMs > 0 && remainingTimeMs < estimatedIterationTimeMs)
                {
                    // Not enough time to complete the next iteration - stop now
                    break;
                }
            }

            // Check cancellation
            if (cancellationToken.IsCancellationRequested)
                break;

            // For depth 3+, use soft bound and optimal time checks
            if (currentDepth > 2)
            {
                // SOFT BOUND: Stop early if we're approaching time limit
                if (elapsedForCheck >= _hardTimeBoundMs * SHC.HardTimeCheckRatio)
                {
                    if (isMasterThread) _searchCts?.Cancel();
                    break;
                }

                // PURE TIME-BASED: Check if we should continue based on iteration time
                if (isMasterThread && elapsedForCheck >= timeAlloc.SoftBoundMs)
                {
                    if (lastIterationElapsedMs > remainingTimeMs * SHC.IterationAbortRatio)
                        break;
                }

                // Optimal time check - very stable moves can stop earlier
                if (isMasterThread && elapsedForCheck >= timeAlloc.OptimalTimeMs && stableCount >= 3)
                {
                    if (lastIterationElapsedMs > remainingTimeMs * SHC.IterationCautionRatio)
                        break;
                }
            }

            int alpha = int.MinValue + SHC.AlphaBetaMargin;
            int beta = int.MaxValue - SHC.AlphaBetaMargin;

            if (bestScore > int.MinValue + 2000 && bestScore < int.MaxValue - 2000)
            {
                alpha = Math.Max(int.MinValue + SHC.AlphaBetaMargin, bestScore - SHC.AspirationWindow);
                beta = Math.Min(int.MaxValue - SHC.AlphaBetaMargin, bestScore + SHC.AspirationWindow);
            }

            // Track nodes before this iteration to detect if search actually happened
            long nodesBeforeIteration = threadData.LocalNodesSearched;

            var result = SearchRoot(board, player, currentDepth, candidates, threadData, alpha, beta, cancellationToken);

            var elapsedNow = _timeMonitor?.ElapsedMs ?? 0;
            lastIterationElapsedMs = elapsedNow - iterationStartMs;  // Time for THIS iteration only

            // CRITICAL FIX: Detect if search was aborted (timeout/cancellation)
            // 1. nodesSearchedThisIteration == 0: SearchRoot returned before calling Minimax
            // 2. result.score == int.MinValue: SearchRoot/Minimax returned aborted result
            // In either case, time has run out - break immediately
            long nodesSearchedThisIteration = threadData.LocalNodesSearched - nodesBeforeIteration;

            // Check for actual abort conditions (timeout/cancellation)
            bool searchWasAborted = nodesSearchedThisIteration == 0 || result.score == int.MinValue;
            if (searchWasAborted)
            {
                // No complete search happened - break immediately
                break;
            }

            // Check for TT inflation (very low nodes searched at this depth)
            // REMOVED: The post-iteration check was too aggressive with high TT hit rates
            // We now rely on:
            // 1. Pre-iteration check for depth > 10 (total nodes threshold)
            // 2. Time-based termination
            // 3. searchWasAborted check above for actual aborts

            // CRITICAL FIX: Update bestMove/bestScore BEFORE checking cancellation
            // If we completed the search, we should use the result even if cancellation is requested
            // BUT only update if the score is valid (not int.MinValue from aborted search)
            if (result.score == int.MinValue)
            {
                // Search was aborted - don't update anything, keep previous iteration's result
            }
            else if (result.x == bestMove.Item1 && result.y == bestMove.Item2)
                stableCount++;
            else
            {
                stableCount = 1;
                bestMove = (result.x, result.y);
            }

            // CRITICAL FIX: Only update bestMove/bestDepth when score is valid
            // SearchRoot returns int.MinValue when search was aborted (timeout/cancellation)
            // We should NOT update bestMove with garbage results
            if (result.score > bestScore || bestMove == (-1, -1))
            {
                // Only update bestScore if it's a real search result (not int.MinValue)
                // int.MinValue means the search was aborted and returned first candidate
                if (result.score != int.MinValue)
                {
                    bestScore = result.score;
                    bestMove = (result.x, result.y);
                    bestDepth = currentDepth;

                    // FIX 1: Track best move from completed depth
                    lastCompletedDepth = currentDepth;
                    bestMoveFromCompletedDepth = (result.x, result.y);
                }
                else if (bestMove == (-1, -1))
                {
                    // First iteration with aborted search - use result but keep bestScore at int.MinValue
                    // This is a fallback for the very first search when even D1 times out
                    bestMove = (result.x, result.y);
                    bestDepth = currentDepth;
                }
            }

            // NOW check cancellation - after saving the result
            if (cancellationToken.IsCancellationRequested)
                break;

            if (result.score >= SHC.WinScore)
                break;

            currentDepth++;
        }

        // FIX 1: Return preserved best from completed depth if available
        // This ensures we don't return garbage from aborted iteration
        var finalMove = lastCompletedDepth > 0 ? bestMoveFromCompletedDepth : (bestMove.x, bestMove.y);
        var finalDepth = lastCompletedDepth > 0 ? lastCompletedDepth : bestDepth;

        return (finalMove.x, finalMove.y, bestScore, finalDepth, threadData.LocalNodesSearched);
    }

    /// <summary>
    /// Estimate nodes searched for a given depth and candidate count
    /// This is a rough approximation for statistics reporting
    /// </summary>
    private long EstimateNodes(int depth, int candidateCount)
    {
        // Approximate: candidate_count * (branching_factor^(depth-1))
        // Where branching factor is capped at a reasonable value
        long nodes = candidateCount;
        int branching = Math.Min(candidateCount, 25); // Cap effective branching
        for (int i = 1; i < depth && i < 6; i++) // Limit estimation depth
        {
            nodes *= branching;
        }
        return nodes;
    }

    /// <summary>
    /// Root search with aspiration window
    /// </summary>
    private (int x, int y, int score) SearchRoot(
        SearchBoard board, Player player, int depth, List<(int x, int y)> candidates,
        ThreadData threadData, int alpha, int beta, CancellationToken cancellationToken)
    {
        // Quick cancellation check before starting search at this depth
        // TimeMonitor handles timer-based cancellation automatically
        if (cancellationToken.IsCancellationRequested)
        {
            // CRITICAL FIX: Return int.MinValue to indicate invalid/timeout result
            // Score 0 was being picked by result merging as a "valid" result
            // int.MinValue signals "this search did not complete"
            return (candidates[0].x, candidates[0].y, int.MinValue);
        }

        var bestMove = candidates[0];
        var bestScore = int.MinValue;

        // CRITICAL FIX: Preserve priority moves (blocking squares) at the front
        // The caller may have already prioritized blocking squares for open threes
        // OrderMovesStaged would undo this prioritization
        // Solution: Keep the first few candidates in their original order (they're priority moves)
        // and only reorder the rest
        const int PriorityMoveCount = SHC.PriorityMoveCount; // First N candidates are considered "priority" and not re-ordered

        var priorityMoves = candidates.Take(PriorityMoveCount).ToList();
        var remainingCandidates = candidates.Skip(PriorityMoveCount).ToList();

        // Only reorder the remaining candidates
        var orderedRemaining = OrderMovesStaged(remainingCandidates, depth, board, player, null, threadData);

        // Combine: priority moves first, then ordered remaining
        var orderedMoves = priorityMoves.Concat(orderedRemaining).ToList();

        int moveIndex = 0;
        foreach (var (x, y) in orderedMoves)
        {
            // CRITICAL: Skip non-empty cells to prevent PlaceStone exception
            // This can happen when board is nearly full and candidates include occupied cells
            if (!board.IsEmpty(x, y))
            {
                continue;
            }

            // TimeMonitor handles timer-based cancellation - just check the token
            if (cancellationToken.IsCancellationRequested)
            {
                break;
            }

            var undo = board.MakeMove(x, y, player);
            var score = Minimax(board, depth - 1, alpha, beta, false, player, depth, threadData, cancellationToken);
            board.UnmakeMove(undo);

            // DEBUG: Log score for each move
            if (DebugLogging && threadData.ThreadIndex == 0 && moveIndex < 5)
            {
                Console.WriteLine($"  [SearchRoot] Thread {threadData.ThreadIndex}: move=({x},{y}), score={score}");
            }

            // CRITICAL FIX: Update bestScore/bestMove BEFORE checking cancellation
            // If Minimax completed successfully (score != int.MinValue), we should use the result
            // even if cancellation was requested during the search.
            // Only skip updating if the score is int.MinValue (which means Minimax was cancelled)
            if (score != int.MinValue && score > bestScore)
            {
                bestScore = score;
                bestMove = (x, y);
            }

            // NOW check cancellation - after saving any valid result
            if (cancellationToken.IsCancellationRequested)
            {
                break;
            }

            moveIndex++;

            alpha = Math.Max(alpha, score);
            if (beta <= alpha)
            {
                RecordKillerMove(threadData, depth, x, y);
                break;
            }
        }

        // LAZY SMP TT WRITING: Allow helper threads to write with quality criteria
        // See Minimax() function for detailed explanation
        bool shouldStore = threadData.ThreadIndex == 0 || depth >= 3;
        if (shouldStore)
        {
            _transpositionTable.Store(board.GetHash(), (sbyte)depth, (short)bestScore, (sbyte)bestMove.x, (sbyte)bestMove.y, alpha, beta, (byte)threadData.ThreadIndex, rootDepth: depth);
        }

        return (bestMove.x, bestMove.y, bestScore);
    }
}
