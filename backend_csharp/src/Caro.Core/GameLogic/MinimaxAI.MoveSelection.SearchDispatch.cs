using System.Diagnostics;
using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic.Search;
using Caro.Core.GameLogic.TimeManagement;
using Microsoft.Extensions.Logging;
using SHC = Caro.Core.Domain.Configuration.SearchHeuristicConstants;

namespace Caro.Core.GameLogic;

public partial class MinimaxAI
{
    /// <summary>
    /// Try VCF search to find a forced win. Returns the winning move if found, null otherwise.
    /// Also handles emergency mode fallback when VCF doesn't find a win.
    /// </summary>
    private (int x, int y)? TryVCFSearch(Board board, Player player, ref List<(int x, int y)> candidates,
        TimeAllocation timeAlloc, bool hasImmediateThreats, List<(int x, int y)> blockingSquares,
        long? timeRemainingMs)
    {
        var (vcfTimeLimit, vcfMaxDepth) = TimeBudgetCalculator.CalculateVCFTimeLimit(timeAlloc);

        if (timeAlloc.IsEmergency)
        {
            vcfTimeLimit = (int)Math.Min(timeAlloc.HardBoundMs * SHC.SoftBoundRatio, SHC.EmergencyVcfCapMs);
        }

        var vcfResult = _vcfSolver.SolveVCF(board, player, timeLimitMs: vcfTimeLimit, maxDepth: vcfMaxDepth);

        _vcfDepthAchieved = vcfResult.DepthAchieved;
        _vcfNodesSearched = vcfResult.NodesSearched;

        if (vcfResult.IsSolved && vcfResult.IsWin && vcfResult.BestMove.HasValue)
        {
            return vcfResult.BestMove.Value;
        }

        // VCF-FIRST MODE: In emergency mode, if VCF didn't find a win, check opponent threats
        if (timeAlloc.IsEmergency)
        {
            if (hasImmediateThreats && blockingSquares.Count > 0)
            {
                _depthAchieved = 1;
                _nodesSearched = 1;
                return blockingSquares[0];
            }

            var ttMove = GetTranspositionTableMove(board, player, minDepth: 3);
            if (ttMove.HasValue)
            {
                _depthAchieved = 3;
                _nodesSearched = 1;
                return ttMove.Value;
            }

            _depthAchieved = 1;
            _nodesSearched = 1;
            return candidates[0];
        }

        return null;
    }

    /// <summary>
    /// Execute parallel Lazy SMP search and return the best move with final validation.
    /// </summary>
    private (int x, int y) ExecuteParallelSearch(Board board, Player player, List<(int x, int y)> candidates,
        TimeAllocation timeAlloc, SearchOptions options, CancellationToken cancellationToken = default)
    {
        var timeRemainingMs = options.TimeRemainingMs;
        var moveNumber = options.MoveNumber;
        var ponderingEnabled = options.PonderingEnabled;
        var threadCount = options.ThreadCount;
        var parallelSearchEnabled = options.ParallelSearchEnabled;

        int effectiveThreadCount = threadCount ?? ThreadPoolConfig.MaxEngineThreads;
        _lastThreadCount = effectiveThreadCount;
        _tableHits = 0;
        _tableLookups = 0;

        var adjustedTimeAlloc = timeAlloc;

        var parallelResult = _parallelSearch.GetBestMoveWithStats(
            board,
            player,
            timeRemainingMs: timeRemainingMs,
            timeAlloc: adjustedTimeAlloc,
            moveNumber: moveNumber,
            fixedThreadCount: effectiveThreadCount,
            candidates: candidates,
            cancellationToken: cancellationToken);

        // DEFENSIVE: Validate the returned move is actually a valid, empty cell
        var cell = board.GetCell(parallelResult.X, parallelResult.Y);
        if (!cell.IsEmpty)
        {
            Console.WriteLine($"[AI ERROR] Parallel search returned occupied cell ({parallelResult.X},{parallelResult.Y}) at move {moveNumber} - cell player: {cell.Player}");
            var fallbackMove = candidates.FirstOrDefault(c => board.GetCell(c.x, c.y).IsEmpty, candidates[0]);
            parallelResult = new ParallelSearchResult(fallbackMove.x, fallbackMove.y, 1, 1, 0, null, parallelResult.AllocatedTimeMs, 0, 0);
        }

        // Update statistics from parallel search
        _depthAchieved = parallelResult.DepthAchieved;
        _nodesSearched = parallelResult.NodesSearched;
        _lastParallelDiagnostics = parallelResult.ParallelDiagnostics;
        _lastAllocatedTimeMs = parallelResult.AllocatedTimeMs;
        _lastPonderingEnabled = ponderingEnabled;
        _tableHits = parallelResult.TableHits;
        _tableLookups = parallelResult.TableLookups;
        _lastSearchScore = parallelResult.Score;
        _lastFmcPercent = parallelResult.FirstMoveCutoffPercent;
        _lastEbf = parallelResult.EffectiveBranchingFactor;

        // Store PV and board for pondering prediction
        _lastPV = PV.FromSingleMove(parallelResult.X, parallelResult.Y, _depthAchieved, 0);
        _lastBoard = board;

        // Start pondering for opponent's response
        if (ponderingEnabled)
        {
            var opponent = player == Player.Red ? Player.Blue : Player.Red;
            var predictedOpponentMove = _lastPV.GetPredictedOpponentMove();
            var ponderTimeMs = TimeBudgetCalculator.CalculatePonderTime(timeRemainingMs);

            if (ponderTimeMs > 0)
            {
                _ponderer.StartPondering(
                    board,
                    opponent,
                    predictedOpponentMove,
                    player,
                    ponderTimeMs
                );
            }
        }

        return ValidateAndReturnBlockingMove(board, player, (parallelResult.X, parallelResult.Y));
    }

    /// <summary>
    /// Execute sequential iterative-deepening search with time management.
    /// </summary>
    private (int x, int y) ExecuteSequentialSearch(Board board, Player player, List<(int x, int y)> candidates,
        TimeAllocation timeAlloc, SearchOptions options, CancellationToken cancellationToken = default)
    {
        var timeRemainingMs = options.TimeRemainingMs;
        var ponderingEnabled = options.PonderingEnabled;
        var maxDepth = options.MaxDepth;
        var maxNodes = options.MaxNodes;

        _lastThreadCount = options.ThreadCount ?? ThreadPoolConfig.MaxEngineThreads;
        _lastParallelDiagnostics = null;
        _lastPonderingEnabled = ponderingEnabled;

        long adjustedSoftBoundMs = Math.Max(50, timeAlloc.SoftBoundMs);
        long adjustedHardBoundMs = Math.Max(adjustedSoftBoundMs, timeAlloc.HardBoundMs);

        (int x, int y) bestMove;

        _transpositionTable.IncrementAge();
        _tableHits = 0;
        _tableLookups = 0;

        _nodesSearched = 0;
        _depthAchieved = 0;
        _vcfNodesSearched = 0;
        _vcfDepthAchieved = 0;
        _searchStopwatch.Restart();

        _searchHardBoundMs = adjustedHardBoundMs;
        _lastAllocatedTimeMs = adjustedHardBoundMs;
        _searchStopped = false;

        bestMove = candidates[0];
        int currentDepth = 1;

        const int AbsoluteMaxDepth = SearchConstants.AbsoluteMaxDepth;
        const long MinNodesForValidIteration = 10;

        while (true)
        {
            if (currentDepth > AbsoluteMaxDepth)
                break;

            if (currentDepth > 10)
            {
                long minimumTotalNodesForDepth = (long)(currentDepth - 5) * (currentDepth - 5) * 200;
                if (_nodesSearched < minimumTotalNodesForDepth)
                    break;
            }

            if (maxDepth.HasValue && currentDepth > maxDepth.Value)
                break;

            if (maxNodes.HasValue && _nodesSearched >= maxNodes.Value)
                break;

            var elapsed = _searchStopwatch.ElapsedMilliseconds;

            if (elapsed >= _searchHardBoundMs)
                break;

            if (elapsed >= adjustedSoftBoundMs)
            {
                double remainingSeconds = (_searchHardBoundMs - elapsed) / 1000.0;
                double estimatedNextTime = elapsed / 1000.0 * SHC.EffectiveBranchingFactorEstimate;
                if (remainingSeconds < estimatedNextTime * SHC.SoftBoundRatio)
                    break;
            }

            _searchStopped = false;

            long nodesBeforeIteration = _nodesSearched;
            long ticksBeforeIteration = _searchStopwatch.ElapsedTicks;

            var result = SearchWithDepth(board, player, currentDepth, candidates);
            long nodesSearchedThisIteration = _nodesSearched - nodesBeforeIteration;
            long ticksThisIteration = _searchStopwatch.ElapsedTicks - ticksBeforeIteration;

            if (result.x != -1)
            {
                bestMove = (result.x, result.y);
                _lastSearchScore = result.score;

                if (nodesSearchedThisIteration >= MinNodesForValidIteration)
                {
                    _depthAchieved = currentDepth;
                }
            }

            if (_searchStopped)
                break;

            if (nodesSearchedThisIteration >= MinNodesForValidIteration)
            {
                currentDepth++;
            }
            else
            {
                break;
            }
        }

        _searchStopwatch.Stop();

        // Report time used to adaptive time manager for feedback loop
        if (timeRemainingMs.HasValue)
        {
            var actualTimeMs = _searchStopwatch.ElapsedMilliseconds;
            bool timedOut = actualTimeMs >= timeAlloc.HardBoundMs;
            _adaptiveTimeManager.ReportTimeUsed(actualTimeMs, timeAlloc.SoftBoundMs, timedOut);
        }

        // Store PV for pondering
        _lastPV = PV.FromSingleMove(bestMove.x, bestMove.y, _depthAchieved, 0);
        _lastBoard = board;
        _lastPlayer = player;

        // Start pondering for opponent's response
        if (ponderingEnabled)
        {
            var opponent = player == Player.Red ? Player.Blue : Player.Red;
            var predictedOpponentMove = _lastPV.GetPredictedOpponentMove();
            var ponderTimeMs = TimeBudgetCalculator.CalculatePonderTime(timeRemainingMs);

            if (ponderTimeMs > 0)
            {
                _ponderer.StartPondering(
                    board,
                    opponent,
                    predictedOpponentMove,
                    player,
                    ponderTimeMs
                );
            }
        }

        PublishSearchStats(player, StatsType.MainSearch, _searchStopwatch.ElapsedMilliseconds);

        return ValidateAndReturnBlockingMove(board, player, bestMove);
    }
}
