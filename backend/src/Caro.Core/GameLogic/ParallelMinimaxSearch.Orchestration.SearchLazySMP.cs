using System.Collections.Concurrent;
using System.Diagnostics;
using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic.Search;
using Caro.Core.GameLogic.TimeManagement;
using SHC = Caro.Core.Domain.Configuration.SearchHeuristicConstants;
using TC = Caro.Core.Domain.Configuration.TimeConstants;

namespace Caro.Core.GameLogic;

public sealed partial class ParallelMinimaxSearch
{
    /// Lazy SMP: Multiple threads search independently with shared TT, pure time-based.
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
        const int ReasonableScoreThreshold = (int)SHC.ReasonableScoreThreshold;  // -2147383648

        // DEPTH-ADVANTAGE OVERRIDE: If a helper thread reached depth N+2 or higher
        // vs the master thread's depth N, prefer the helper's deeper result.
        // Deeper search is more reliable even if score differs.
        var masterResult = results.FirstOrDefault(r => r.threadIndex == 0);
        if (masterResult.depth > 0)
        {
            const int DepthAdvantageThreshold = 2;
            var deeperHelper = results
                .Where(r => r.threadIndex != 0
                    && r.depth >= masterResult.depth + DepthAdvantageThreshold
                    && r.score != int.MinValue
                    && r.score < SHC.WinScore * 2)
                .OrderByDescending(r => r.depth)
                .ThenByDescending(r => r.score)
                .FirstOrDefault();

            if (deeperHelper.depth > 0)
            {
                if (DebugLogging)
                {
                    Console.WriteLine($"[PARALLEL] Depth-advantage override: helper thread {deeperHelper.threadIndex} " +
                        $"reached depth {deeperHelper.depth} vs master depth {masterResult.depth}");
                }
                bestResult = deeperHelper;
                foundValidResult = true;
            }
        }

        // First, try to find results at maxDepth with reasonable scores
        // Reject int.MaxValue leaks (uninitialized minimizing nodes) and int.MinValue (search failures)
        var reasonableAtMaxDepth = results
            .Where(r => r.depth == maxDepth
                && r.score > ReasonableScoreThreshold
                && r.score < SHC.WinScore * 2)
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
}
