using System.Runtime.CompilerServices;
using System.Diagnostics;
using System.Threading.Channels;
using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic.TimeManagement;
using Caro.Core.GameLogic.Pondering;
using Caro.Core.GameLogic.Search;
using Microsoft.Extensions.Logging;
using SHC = Caro.Core.Domain.Configuration.SearchHeuristicConstants;

namespace Caro.Core.GameLogic;

public partial class MinimaxAI
{
    /// <summary>
    /// Get the best move for the AI player with time awareness
    /// Convenience overload that creates SearchOptions internally
    /// </summary>
    public (int x, int y) GetBestMove(Board board, Player player, long? timeRemainingMs, bool ponderingEnabled = true, bool parallelSearchEnabled = true)
    {
        return GetBestMove(board, player, new SearchOptions
        {
            TimeRemainingMs = timeRemainingMs,
            PonderingEnabled = ponderingEnabled,
            ParallelSearchEnabled = parallelSearchEnabled,
        }, CancellationToken.None);
    }

    /// <summary>
    /// Get the best move for the AI player with full search configuration.

    public (int x, int y) GetBestMove(Board board, Player player, SearchOptions options, CancellationToken cancellationToken)
    {
        if (player == Player.None)
            throw new ArgumentException("Player cannot be None");

        // Defensive: stop any lingering pondering from previous search.
        // Critical for mixed-difficulty games: L5 player may have started pondering,
        // then L1 player calls GetBestMove. Without this, pondering continues
        // in the background, polluting TT and VCF cache.
        _ponderer.StopPondering();

        var timeRemainingMs = options.TimeRemainingMs;
        var moveNumber = options.MoveNumber;
        var ponderingEnabled = options.PonderingEnabled;
        var parallelSearchEnabled = options.ParallelSearchEnabled;
        var incrementSeconds = options.IncrementSeconds;
        var threadCount = options.ThreadCount;
        var maxDepth = options.MaxDepth;
        var maxNodes = options.MaxNodes;
        var maxTimeMs = options.MaxTimeMs;

        var candidates = CandidateGenerator.GetCandidateMoves(board, SearchConstants.MaxSearchRadius);

        // Initialize search statistics BEFORE any early returns
        // This ensures stats are clean even for instant moves (error rate, critical defense, VCF, etc.)
        _nodesSearched = 0;
        _depthAchieved = 0;
        _vcfNodesSearched = 0;
        _vcfDepthAchieved = 0;
        _moveType = MoveType.Normal;  // Default, will be overridden by early exits
        _searchStopwatch.Restart();

        // Reset thread count and parallel diagnostics
        _lastThreadCount = threadCount ?? ThreadPoolConfig.MaxEngineThreads;
        _lastParallelDiagnostics = null;

        // Apply Open Rule: Red's second move (move #3) must be at least 3 intersections away from first red stone
        // Rule: |x - firstX| >= 3 OR |y - firstY| >= 3 (outside 5x5 zone centered on first move)
        // MoveNumber is the count of stones already on the board, so Red's 2nd move = MoveNumber 2
        if (player == Player.Red && moveNumber == 2)
        {
            // Find first red stone
            (int firstX, int firstY)? firstRed = null;
            for (int bx = 0; bx < board.BoardSize; bx++)
            {
                for (int by = 0; by < board.BoardSize; by++)
                {
                    if (board.GetCell(bx, by).Player == Player.Red)
                    {
                        firstRed = (bx, by);
                        break;
                    }
                }
                if (firstRed.HasValue)
                    break;
            }

            if (firstRed.HasValue)
            {
                var fx = firstRed.Value.firstX;
                var fy = firstRed.Value.firstY;
                candidates = candidates.Where(c =>
                {
                    int dx = System.Math.Abs(c.x - fx);
                    int dy = System.Math.Abs(c.y - fy);
                    return dx >= SHC.OpenRuleDistance || dy >= SHC.OpenRuleDistance;
                }).ToList();
            }
        }

        if (candidates.Count == 0)
        {
            // No valid candidates - board is empty or all filtered out
            // Play first available cell that satisfies open rule (if applicable)
            for (int x = 0; x < board.BoardSize; x++)
            {
                for (int y = 0; y < board.BoardSize; y++)
                {
                    if (board.GetCell(x, y).Player == Player.None)
                    {
                        // For move #3, check open rule
                        if (player == Player.Red && moveNumber == 2)
                        {
                            // Find first red stone and check distance
                            (int firstX, int firstY)? firstRed = null;
                            for (int fx = 0; fx < board.BoardSize; fx++)
                            {
                                for (int fy = 0; fy < board.BoardSize; fy++)
                                {
                                    if (board.GetCell(fx, fy).Player == Player.Red)
                                    {
                                        firstRed = (fx, fy);
                                        break;
                                    }
                                }
                                if (firstRed.HasValue)
                                    break;
                            }

                            if (firstRed.HasValue)
                            {
                                int dx = System.Math.Abs(x - firstRed.Value.firstX);
                                int dy = System.Math.Abs(y - firstRed.Value.firstY);
                                if (dx >= SHC.OpenRuleDistance || dy >= SHC.OpenRuleDistance)
                                    return (x, y);
                            }
                            else
                            {
                                // No red stone yet (shouldn't happen on move #3), play anywhere
                                return (x, y);
                            }
                        }
                        else
                        {
                            return (x, y);
                        }
                    }
                }
            }
            // Board is full - no valid moves. Return (-1, -1) as sentinel.
            return (-1, -1);
        }

        // PONDER HIT HANDLING
        var ponderHit = TryPonderHit(board, player, candidates, ponderingEnabled);
        if (ponderHit.HasValue)
            return ponderHit.Value;

        // CRITICAL OPTIMIZATION: Check for immediate winning moves BEFORE any expensive operations
        // This ensures we never waste time searching when a win is available in one move
        // DESIGN: All difficulties use same engine logic - strength comes from threads + time only
        foreach (var (cx, cy) in candidates)
        {
            if (_threatDetector.IsWinningMove(board, cx, cy, player))
            {
                _depthAchieved = 1;
                _nodesSearched = 1;
                _lastAllocatedTimeMs = 0;
                _moveType = MoveType.ImmediateWin;
                return (cx, cy);
            }
        }

        // CRITICAL DEFENSE: Check for opponent's immediate winning moves
        // Must scan full board since blocking square may be far from existing stones
        // This is O(n²) but necessary to prevent instant losses
        // DESIGN: All difficulties use same engine logic - strength comes from threads + time only
        var oppPlayer = player == Player.Red ? Player.Blue : Player.Red;
        var opponentWinningSquares = new List<(int x, int y)>();

        for (int x = 0; x < BoardSize; x++)
        {
            for (int y = 0; y < BoardSize; y++)
            {
                if (board.GetCell(x, y).Player == Player.None)
                {
                    if (_threatDetector.IsWinningMove(board, x, y, oppPlayer))
                    {
                        opponentWinningSquares.Add((x, y));
                    }
                }
            }
        }

        // CRITICAL: Also check for opponent's OPEN FOURS (StraightFour)
        // An open four is 4-in-a-row with an open end - opponent wins next turn if not blocked
        // This is NOT caught by IsWinningMove since it's not yet 5-in-a-row
        // DESIGN: All difficulties use same engine logic - strength comes from threads + time only
        if (opponentWinningSquares.Count == 0)
        {
            var opponentThreats = _threatDetector.DetectThreats(board, oppPlayer);
            foreach (var threat in opponentThreats)
            {
                if (threat.Type == ThreatType.StraightFour || threat.Type == ThreatType.BrokenFour)
                {
                    // Add all gain squares (the squares that complete the 5-in-a-row)
                    foreach (var gain in threat.GainSquares)
                    {
                        if (board.GetCell(gain.x, gain.y).IsEmpty && !opponentWinningSquares.Contains(gain))
                        {
                            opponentWinningSquares.Add(gain);
                        }
                    }
                }
            }
        }

        // If opponent has immediate winning moves, we must respond
        // DESIGN PRINCIPLE: Per ENGINE_FEATURES.md, threat blocks are added to candidate list,
        // not returned immediately. Search evaluates offensive vs defensive options together.
        // The only early returns are for VERIFIED immediate wins.
        if (opponentWinningSquares.Count > 0)
        {
            // First check if we have our own winning move - always best to win immediately
            foreach (var (cx, cy) in candidates)
            {
                if (_threatDetector.IsWinningMove(board, cx, cy, player))
                {
                    _depthAchieved = 1;
                    _nodesSearched = opponentWinningSquares.Count + 1;
                    _lastAllocatedTimeMs = 0;
                    _moveType = MoveType.ImmediateWin;
                    return (cx, cy);
                }
            }

            // Add all blocking squares to candidates with highest priority
            // Search will evaluate which is best (considering our own threats, position, etc.)
            foreach (var block in opponentWinningSquares)
            {
                if (!candidates.Contains(block))
                {
                    candidates.Insert(0, block);
                }
            }

            // Filter to ONLY blocking moves - when opponent has winning threats, we MUST block
            // The search will find the best blocking move
            candidates = candidates.Where(c => opponentWinningSquares.Contains(c)).ToList();
            _logger.LogDebug("[AI DEFENSE] Filtering to {Count} blocking move(s) for search evaluation",
                candidates.Count);
            // Fall through to normal search with filtered candidates
        }

        // PROACTIVE DEFENSE: Check for opponent's open threes (3 in a row with both ends open)
        // An open three becomes an open four on the next move, which has 2 winning squares.
        // We should block open threes BEFORE they become open fours.
        // This is critical for Caro rules where sandwiched wins are blocked.
        // CHANGED: Don't immediately block - instead add to blocking candidates for search evaluation
        // This allows the AI to consider whether its own threats might be more urgent
        var openThreeBlocks = QuickWinChecker.FindOpenThreeBlocks(board, oppPlayer);

        // If there are open threes but NO immediate winning threats, add them to candidates
        // This ensures the search considers blocking open threes
        if (openThreeBlocks.Count > 0)
        {
            // Add open three blocks to candidates if not already present
            foreach (var block in openThreeBlocks)
            {
                if (!candidates.Contains(block))
                {
                    candidates.Insert(0, block); // Insert at beginning for high priority
                }
            }
        }

        // Calculate time allocation for chess-clock time control
        // Infer initial time and increment from the remaining time
        // This works for any time control: 3+2, 7+5, 15+10, etc.
        TimeAllocation timeAlloc;
        // Handle maxTimeMs (from "go movetime N") - use as absolute time budget
        if (maxTimeMs.HasValue)
        {
            timeRemainingMs = maxTimeMs.Value;
            timeAlloc = new TimeAllocation
            {
                SoftBoundMs = (long)(maxTimeMs.Value * SHC.SoftBoundRatio),
                HardBoundMs = maxTimeMs.Value,
                OptimalTimeMs = (long)(maxTimeMs.Value * SHC.OptimalBoundRatio)
            };
        }
        // CRITICAL FIX: For long time budgets, use direct time allocation without AdaptiveTimeManager
        // The adaptive manager under-allocates for long time budgets
        else if (timeRemainingMs.HasValue)
        {
            // Infer initial time from first few moves
            var inferredInitialMs = _inferredInitialTimeMs > 0 ? _inferredInitialTimeMs : timeRemainingMs.Value;
            var initialTimeSeconds = (int)(inferredInitialMs / 1000);

            // Use explicit increment if provided, otherwise estimate
            int effectiveIncrementSeconds;
            if (incrementSeconds.HasValue)
            {
                effectiveIncrementSeconds = incrementSeconds.Value;
            }
            else if (initialTimeSeconds <= 120)
            {
                // Short time control - assume sudden death (no increment)
                effectiveIncrementSeconds = 0;
            }
            else
            {
                // Longer time controls - estimate increment based on common ratios
                effectiveIncrementSeconds = Math.Max(SHC.MinEffectiveIncrementSeconds, (int)Math.Round(initialTimeSeconds / SHC.InitialTimeToIncrementDivisor));
            }

            // Use AdaptiveTimeManager with PID-like controller for better time management
            // Automatically adjusts to any time control without hardcoded multipliers
            timeAlloc = _adaptiveTimeManager.CalculateMoveTime(
                timeRemainingMs.Value,
                moveNumber,
                candidates.Count,
                board,
                player,
                initialTimeSeconds,
                effectiveIncrementSeconds
            );
        }
        else
        {
            timeAlloc = TimeBudgetCalculator.GetDefaultTimeAllocation();
        }

        // Apply difficulty time fraction to PID controller output.
        // Using 'with' on readonly struct preserves all other fields.
        var timeFraction = options.TimeFraction;
        if (timeFraction < 1.0)
        {
            timeAlloc = timeAlloc with
            {
                SoftBoundMs = Math.Max(1, (long)(timeAlloc.SoftBoundMs * timeFraction)),
                HardBoundMs = Math.Max(1, (long)(timeAlloc.HardBoundMs * timeFraction)),
                OptimalTimeMs = Math.Max(1, (long)(timeAlloc.OptimalTimeMs * timeFraction)),
            };
        }

        // Analyze opponent threats
        var threatInfo = AnalyzeOpponentThreats(board, oppPlayer);
        bool hasOpponentThreats = threatInfo.HasOpponentThreats;
        bool hasImmediateThreats = threatInfo.HasImmediateThreats;
        bool hasOpenFour = threatInfo.HasOpenFour;
        List<(int x, int y)> blockingSquares = threatInfo.BlockingSquares;
        List<(int x, int y)> priorityBlockingSquares = threatInfo.PriorityBlockingSquares;

        // Handle three-threat blocking (StraightThree/BrokenThree)
        if (hasOpponentThreats)
        {
            var threeBlock = HandleThreeThreatBlocking(board, player, oppPlayer, threatInfo);
            if (threeBlock.HasValue)
                return threeBlock.Value;
        }

        // Emergency mode - use TT move if available
        // BUT: If opponent has threats, blocking takes priority
        if (timeAlloc.IsEmergency && !hasOpponentThreats)
        {
            var ttMove = GetTranspositionTableMove(board, player, minDepth: 5);
            if (ttMove.HasValue)
            {
                //Console.WriteLine("[AI] Emergency mode: Using TT move at D5+");
                _depthAchieved = 5;
                _nodesSearched = 1;
                return ttMove.Value;
            }
        }

        // PROACTIVE ATTACK: When no opponent threats, create our own threats
        if (!hasOpponentThreats)
        {
            var attackMove = TryProactiveAttack(board, player, candidates);
            if (attackMove.HasValue)
                return attackMove.Value;
        }

        // CRITICAL DEFENSE: Filter candidates when opponent has immediate threats
        if (hasOpponentThreats)
        {
            var defenseMove = FilterCandidatesForCriticalDefense(board, player, oppPlayer,
                ref candidates, ref timeAlloc, hasImmediateThreats, hasOpenFour, blockingSquares);
            if (defenseMove.HasValue)
                return defenseMove.Value;
        }

        // Try VCF (Victory by Continuous Four) search
        // Guarded by UseVCF: low difficulty levels skip pre-search VCF to save time.
        // In-tree VCF during parallel search is lightweight and stays enabled.
        if (options.UseVCF)
        {
            var vcfMove = TryVCFSearch(board, player, ref candidates, timeAlloc, hasImmediateThreats, blockingSquares, timeRemainingMs);
            if (vcfMove.HasValue)
                return vcfMove.Value;
        }

        // PARALLEL SEARCH: Use Lazy SMP when enabled
        if (parallelSearchEnabled)
        {
            return ExecuteParallelSearch(board, player, candidates, timeAlloc, options, cancellationToken);
        }

        // Sequential search fallback
        return ExecuteSequentialSearch(board, player, candidates, timeAlloc, options, cancellationToken);
    }
}
