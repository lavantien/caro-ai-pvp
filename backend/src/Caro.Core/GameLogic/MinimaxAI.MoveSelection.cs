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
    /// Dynamically adjusts search depth based on remaining time
    /// </summary>
    public (int x, int y) GetBestMove(Board board, Player player, long? timeRemainingMs, bool ponderingEnabled = true, bool parallelSearchEnabled = true)
    {
        return GetBestMove(board, player, new SearchOptions
        {
            TimeRemainingMs = timeRemainingMs,
            PonderingEnabled = ponderingEnabled,
            ParallelSearchEnabled = parallelSearchEnabled,
        });
    }

    /// <summary>
    /// Get the best move for the AI player with full search configuration.

    public (int x, int y) GetBestMove(Board board, Player player, SearchOptions options)
    {
        if (player == Player.None)
            throw new ArgumentException("Player cannot be None");

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
        _lastThreadCount = threadCount ?? ThreadPoolConfig.GetLazySMPThreadCount();
        _lastParallelDiagnostics = null;

        // Apply Open Rule: Red's second move (move #3) must be at least 3 intersections away from first red stone
        // Rule: |x - firstX| >= 3 OR |y - firstY| >= 3 (outside 5x5 zone centered on first move)
        if (player == Player.Red && moveNumber == 3)
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
                        if (player == Player.Red && moveNumber == 3)
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
        // On ponder hit, the ponder search is already running with the correct position.
        // We should wait for it to complete (up to our time budget) and use the result.
        // On ponder miss, we fall through to normal search.
        if (ponderingEnabled && _ponderer.IsPondering && _lastPV.IsEmpty == false)
        {
            var lastOppMove = QuickWinChecker.GetLastOpponentMove(board, player);
            if (lastOppMove.HasValue)
            {
                // Check if opponent played the predicted move
                var (ponderState, _) = _ponderer.HandleOpponentMove(lastOppMove.Value.x, lastOppMove.Value.y);

                if (ponderState == PonderState.PonderHit)
                {
                    // PONDER HIT - opponent played expected move!
                    // The ponder search was running during opponent's turn (free precomputation).
                    // CRITICAL FIX: Still check for immediate wins and threats before using ponder result.
                    // The ponder search might not have prioritized tactical moves correctly.

                    // First, check if we have an immediate winning move
                    // DESIGN: All difficulties use same engine logic - strength comes from threads + time only
                    foreach (var (cx, cy) in candidates)
                    {
                        if (_threatDetector.IsWinningMove(board, cx, cy, player))
                        {
                            _ponderer.StopPondering();
                            _depthAchieved = 1;
                            _nodesSearched = 1;
                            _lastAllocatedTimeMs = 0;
                            _moveType = MoveType.ImmediateWin;
                            return (cx, cy);
                        }
                    }

                    // Second, check if opponent has an immediate winning threat we must block
                    // DESIGN: All difficulties use same engine logic - strength comes from threads + time only
                    var ponderOppPlayer = player == Player.Red ? Player.Blue : Player.Red;
                    var ponderOpponentWinningSquares = new List<(int x, int y)>();
                    for (int x = 0; x < BoardSize; x++)
                    {
                        for (int y = 0; y < BoardSize; y++)
                        {
                            if (board.GetCell(x, y).Player == Player.None)
                            {
                                if (_threatDetector.IsWinningMove(board, x, y, ponderOppPlayer))
                                {
                                    ponderOpponentWinningSquares.Add((x, y));
                                }
                            }
                        }
                    }

                    // If there are immediate threats, must block - don't use ponder result
                    if (ponderOpponentWinningSquares.Count > 0)
                    {
                        // Fall through to normal blocking logic
                        // Don't use ponder result when immediate blocking is needed
                    }
                    else
                    {
                        // No immediate threats - safe to use ponder result
                        var ponderResult = _ponderer.GetPonderHitResult();

                        if (ponderResult.BestMove.HasValue && ponderResult.Depth > 0)
                        {
                            var ponderMove = ponderResult.BestMove.Value;
                            // Validate the ponder move is still valid on current board
                            if (board.GetCell(ponderMove.x, ponderMove.y).IsEmpty)
                            {
                                _depthAchieved = ponderResult.Depth;
                                _nodesSearched = ponderResult.NodesSearched;
                                _lastAllocatedTimeMs = ponderResult.TimeSpentMs;
                                _moveType = MoveType.Normal;
                                //Console.WriteLine($"[PONDER HIT] Used ponder result: depth={ponderResult.Depth}, nodes={ponderResult.NodesSearched:N0}, time={ponderResult.TimeSpentMs}ms");
                                return ponderMove;
                            }
                        }
                    }
                }
                // Ponder miss - fall through to normal search
                // The ponder search was already stopped by HandleOpponentMove on miss
            }
        }

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

        bool hasOpponentThreats = false;
        bool hasImmediateThreats = false;  // Only StraightFour and BrokenFour - require immediate response
        bool hasOpenFour = false;
        List<(int x, int y)> blockingSquares = new();
        List<(int x, int y)> priorityBlockingSquares = new();

        {
            // CRITICAL DEFENSE: Check for opponent threats BEFORE any early returns
            // This ensures we don't skip blocking in emergency mode
            // Note: oppPlayer is already defined above
            // CRITICAL FIX: Include BrokenThree threats - they become BrokenFour in one move!
            var threats = _threatDetector.DetectThreats(board, oppPlayer)
                .Where(t => t.Type == ThreatType.StraightFour || t.Type == ThreatType.StraightThree || t.Type == ThreatType.BrokenFour || t.Type == ThreatType.BrokenThree)
                .ToList();

            hasOpponentThreats = threats.Count > 0;

            // CRITICAL FIX: Only filter candidates for IMMEDIATE threats (StraightFour, BrokenFour)
            // StraightThree and BrokenThree are developing threats that don't require immediate response
            // The evaluation function will handle them through normal search
            hasImmediateThreats = threats.Any(t => t.Type == ThreatType.StraightFour || t.Type == ThreatType.BrokenFour);

            if (hasOpponentThreats)
            {
                var straightFourCount = threats.Count(t => t.Type == ThreatType.StraightFour);
                var straightThreeCount = threats.Count(t => t.Type == ThreatType.StraightThree);
                var brokenFourCount = threats.Count(t => t.Type == ThreatType.BrokenFour);
                var brokenThreeCount = threats.Count(t => t.Type == ThreatType.BrokenThree);

                blockingSquares = threats
                    .SelectMany(t => t.GainSquares)
                    .Where(gs => board.GetCell(gs.x, gs.y).IsEmpty)
                    .ToList();

                // Check for open four (StraightFour with exactly 2 blocking squares)
                // This is a critical threat that requires special handling
                foreach (var threat in threats.Where(t => t.Type == ThreatType.StraightFour))
                {
                    if (threat.GainSquares.Count >= 2)
                    {
                        hasOpenFour = true;
                        // For open fours, prioritize blocking squares that also prevent other threats
                        foreach (var square in threat.GainSquares)
                        {
                            if (board.GetCell(square.x, square.y).IsEmpty)
                                priorityBlockingSquares.Add(square);
                        }
                    }
                }

                // CRITICAL FIX: BrokenFour also indicates critical threat (double attack potential)
                if (brokenFourCount > 0)
                {
                    hasOpenFour = true;  // Treat as critically as open four
                }

                // CRITICAL FIX: StraightThree and BrokenThree should be blocked, BUT only if we don't have
                // our own winning threats. If we can win immediately, that's better than blocking.
                // CRITICAL: A StraightThree becomes a StraightFour in ONE move, NOT two moves!
                // We must block three-threats BEFORE they become unstoppable open fours.
                // A BrokenThree becomes a StraightFour in 1 move if the gap is filled!
                // FIX: Handle three-threats even when there ARE four-threats, because:
                // 1. Three-threats in different directions can become additional four-threats
                // 2. Blocking a three-threat gain square might also block a four-threat
                // 3. We need to find the BEST block that addresses ALL threats
                if ((straightThreeCount > 0 || brokenThreeCount > 0))
                {
                    // First check if we have our own winning threats
                    var ourThreats = _threatDetector.DetectThreats(board, player);
                    var ourStraightFours = ourThreats.Where(t => t.Type == ThreatType.StraightFour || t.Type == ThreatType.BrokenFour).ToList();

                    // If we have a winning threat (open four), play it instead of blocking
                    // CRITICAL: Only counter-attack if we have a GUARANTEED win (StraightFour/BrokenFour)
                    // OR if we have multiple StraightThrees (double threat - opponent can't block both!)
                    if (ourStraightFours.Count > 0)
                    {
                        // We have an open four - find and verify our winning move
                        foreach (var threat in ourStraightFours)
                        {
                            foreach (var gs in threat.GainSquares)
                            {
                                if (board.GetCell(gs.x, gs.y).IsEmpty && _threatDetector.IsWinningMove(board, gs.x, gs.y, player))
                                {
                                    _depthAchieved = 1;
                                    _nodesSearched = 1;
                                    _lastAllocatedTimeMs = 0;
                                    _moveType = MoveType.ImmediateWin;
                                    _logger.LogDebug("[AI DEFENSE] ({Player}) COUNTER-ATTACK with verified winning move at ({WX},{WY}) instead of blocking",
                                        player, gs.x, gs.y);
                                    return gs;
                                }
                            }
                        }
                    }

                    // DESIGN PRINCIPLE: Per ENGINE_FEATURES.md, threat blocks are added to candidate list,
                    // not returned immediately. Search evaluates offensive vs defensive options together.
                    // This maintains strategic initiative instead of reactive blocking.

                    // CRITICAL: Collect ALL gain squares from both three-threats AND four-threats
                    // When both exist, we need to find a block that addresses ALL threats
                    var threeThreats = threats
                        .Where(t => t.Type == ThreatType.StraightThree || t.Type == ThreatType.BrokenThree)
                        .ToList();

                    var fourThreats = threats
                        .Where(t => t.Type == ThreatType.StraightFour || t.Type == ThreatType.BrokenFour)
                        .ToList();

                    var allGainSquares = threats  // Include ALL threats, not just three-threats
                        .SelectMany(t => t.GainSquares)
                        .Where(gs => board.GetCell(gs.x, gs.y).IsEmpty)
                        .Distinct()
                        .ToList();

                    if (allGainSquares.Count > 0)
                    {
                        // Immediately block three-threats
                        // A StraightThree becomes an open four in ONE move. We must block NOW.
                        // Returning immediately bypasses search, guaranteeing the block.
                        {
                            // CRITICAL: Check if opponent has multiple independent three-threats
                            // This is a "double threat" situation - blocking one leaves the other
                            // which becomes a four-threat next turn. We MUST counter-attack.
                            var distinctThreeThreats = threeThreats
                                .Where(t => t.Type == ThreatType.StraightThree || t.Type == ThreatType.BrokenThree)
                                .GroupBy(t => t.Direction)  // Group by direction to find parallel threats
                                .Count(g => g.Any());

                            bool hasMultipleIndependentThreats = threeThreats.Count >= 2 &&
                                threeThreats.SelectMany(t => t.GainSquares).Distinct().Count() >= 3;

                            // If opponent has 2+ independent threats, blocking is futile
                            // We must create our own winning threat to counter
                            if (hasMultipleIndependentThreats)
                            {
                                _logger.LogDebug("[AI DEFENSE] ({Player}) CRITICAL: Opponent has {Count} independent three-threats - blocking is futile, must counter-attack!",
                                    player, threeThreats.Count);

                                // Try to find a move that creates our own winning threat
                                for (int x = 0; x < BoardSize; x++)
                                {
                                    for (int y = 0; y < BoardSize; y++)
                                    {
                                        if (!board.GetCell(x, y).IsEmpty) continue;

                                        var testBoard = board.PlaceStone(x, y, player);
                                        var ourNewThreats = _threatDetector.DetectThreats(testBoard, player);
                                        var ourNewFourThreats = ourNewThreats.Where(t =>
                                            t.Type == ThreatType.StraightFour || t.Type == ThreatType.BrokenFour).ToList();

                                        // If we can create a four-threat (open four), that forces opponent to block
                                        // This changes the dynamic - they have to respond to us
                                        if (ourNewFourThreats.Count > 0)
                                        {
                                            // Check if this also blocks one of their threats (bonus!)
                                            bool alsoBlocks = threeThreats.Any(t => t.GainSquares.Contains((x, y)));

                                            _depthAchieved = 1;
                                            _nodesSearched = (x + 1) * BoardSize + y + 1;
                                            _lastAllocatedTimeMs = 0;
                                            _moveType = alsoBlocks ? MoveType.ImmediateBlock : MoveType.CounterAttack;
                                            _logger.LogDebug("[AI DEFENSE] ({Player}) COUNTER-ATTACK at ({X},{Y}) creates {Count} four-threat(s){AlsoBlocks}",
                                                player, x, y, ourNewFourThreats.Count, alsoBlocks ? " and blocks!" : "");
                                            return ValidateAndReturnBlockingMove(board, player, (x, y));
                                        }
                                    }
                                }

                                // No counter-attack available - fall through to best blocking strategy
                                _logger.LogDebug("[AI DEFENSE] ({Player}) No counter-attack found - must block best threat",
                                    player);
                            }

                            // Find the best blocking square - prioritize eliminating immediate threats
                            var bestBlock = allGainSquares.First();
                            int bestScore = int.MinValue;

                            foreach (var block in allGainSquares)
                            {
                                var testBoard = board.PlaceStone(block.x, block.y, player);
                                var ourThreatsAfter = _threatDetector.DetectThreats(testBoard, player);
                                var theirThreatsAfter = _threatDetector.DetectThreats(testBoard, oppPlayer);

                                // Count threats by type for weighted scoring
                                int theirFourThreats = theirThreatsAfter.Count(t => t.Type == ThreatType.StraightFour || t.Type == ThreatType.BrokenFour);
                                int theirThreeThreats = theirThreatsAfter.Count(t => t.Type == ThreatType.StraightThree || t.Type == ThreatType.BrokenThree);
                                int ourFourThreats = ourThreatsAfter.Count(t => t.Type == ThreatType.StraightFour || t.Type == ThreatType.BrokenFour);

                                // CRITICAL: Check for immediate winning squares after this block
                                int theirWinningSquares = 0;
                                for (int wx = 0; wx < BoardSize; wx++)
                                {
                                    for (int wy = 0; wy < BoardSize; wy++)
                                    {
                                        if (testBoard.GetCell(wx, wy).IsEmpty && _threatDetector.IsWinningMove(testBoard, wx, wy, oppPlayer))
                                            theirWinningSquares++;
                                    }
                                }

                                // Score: heavily penalize blocks that leave immediate threats
                                // -10000 per winning square (CRITICAL - must block these!)
                                // -5000 per four-threat (URGENT - becomes winning next move)
                                // -500 per three-threat (important but not immediate)
                                // +8000 per our four-threat (STRONG COUNTER-ATTACK - forces opponent to respond!)
                                // -2000 BONUS penalty for multiple three-threats (can lead to double threat)
                                // CRITICAL: Counter-attacking is often better than just blocking!
                                int multipleThreePenalty = theirThreeThreats >= SHC.MultipleThreeThreshold ? -SHC.MultipleThreePenalty : 0;
                                int score = -theirWinningSquares * SHC.WinningSquarePenalty - theirFourThreats * SHC.FourThreatPenalty - theirThreeThreats * SHC.ThreeThreatPenalty + ourFourThreats * SHC.FourThreatBonus + multipleThreePenalty;

                                // Prefer central blocks as tiebreaker
                                int distToCenter = Math.Abs(block.x - SHC.CenterIndex) + Math.Abs(block.y - SHC.CenterIndex);
                                score -= distToCenter;

                                if (score > bestScore)
                                {
                                    bestScore = score;
                                    bestBlock = block;
                                }
                            }

                            // CRITICAL: If the best block still leaves us in a losing position,
                            // try counter-attacking instead - creating our own winning threat
                            // This is especially important when opponent has multiple developing threats
                            if (bestScore < -SHC.FourThreatPenalty) // Very negative = opponent still has winning squares
                            {
                                _logger.LogDebug("[AI DEFENSE] ({Player}) Best block score is {Score} - trying counter-attack instead",
                                    player, bestScore);

                                for (int x = 0; x < BoardSize; x++)
                                {
                                    for (int y = 0; y < BoardSize; y++)
                                    {
                                        if (!board.GetCell(x, y).IsEmpty) continue;

                                        var testBoard = board.PlaceStone(x, y, player);
                                        var ourNewThreats = _threatDetector.DetectThreats(testBoard, player);
                                        var ourNewFourThreats = ourNewThreats.Where(t =>
                                            t.Type == ThreatType.StraightFour || t.Type == ThreatType.BrokenFour).ToList();

                                        // If we can create a four-threat that is also a winning move, take it!
                                        if (ourNewFourThreats.Count > 0)
                                        {
                                            foreach (var threat in ourNewFourThreats)
                                            {
                                                foreach (var gs in threat.GainSquares)
                                                {
                                                    if (testBoard.GetCell(gs.x, gs.y).IsEmpty &&
                                                        _threatDetector.IsWinningMove(testBoard, gs.x, gs.y, player))
                                                    {
                                                        // This counter-attack creates a verified winning position!
                                                        _depthAchieved = 1;
                                                        _nodesSearched = (x + 1) * BoardSize + y + 1;
                                                        _lastAllocatedTimeMs = 0;
                                                        _moveType = MoveType.CounterAttack;
                                                        _logger.LogDebug("[AI DEFENSE] ({Player}) DESPERATE COUNTER-ATTACK at ({X},{Y}) creates verified winning threat",
                                                            player, x, y);
                                                        return ValidateAndReturnBlockingMove(board, player, (x, y));
                                                    }
                                                }
                                            }
                                        }
                                    }
                                }
                            }

                            _depthAchieved = 1;
                            _nodesSearched = allGainSquares.Count;
                            _lastAllocatedTimeMs = 0;
                            _moveType = MoveType.ImmediateBlock;
                            _logger.LogDebug("[AI DEFENSE] ({Player}) IMMEDIATE three-threat block at ({BX},{BY}) - {Count} gain squares available (score: {Score})",
                                player, bestBlock.x, bestBlock.y, allGainSquares.Count, bestScore);
                            return ValidateAndReturnBlockingMove(board, player, bestBlock);
                        }
                    }
                }

                _logger.LogDebug("[AI DEFENSE] ({Player}) Opponent has {StraightFourCount} StraightFour, {StraightThreeCount} StraightThree, {BrokenFourCount} BrokenFour, {BrokenThreeCount} BrokenThree threat(s), blocking squares: {BlockingSquares}{OpenFourSuffix}",
                    player, straightFourCount, straightThreeCount, brokenFourCount, brokenThreeCount,
                    string.Join(", ", blockingSquares.Select(g => $"({g.x},{g.y})")),
                    hasOpenFour ? " [CRITICAL THREAT DETECTED]" : "");
            }
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
        // This is critical for winning - we must attack, not just defend
        if (!hasOpponentThreats)
        {
            var ourThreats = _threatDetector.DetectThreats(board, player);

            // Priority 1: Create StraightFour/BrokenFour (immediate win)
            var ourFourThreats = ourThreats
                .Where(t => t.Type == ThreatType.StraightFour || t.Type == ThreatType.BrokenFour)
                .ToList();

            if (ourFourThreats.Count > 0)
            {
                // CRITICAL FIX: Verify the gain square actually wins using IsWinningMove
                // A StraightFour/BrokenFour threat means we have 4 stones, but we must verify
                // the gain square completes 5 in a row (not blocked, no overline, etc.)
                foreach (var threat in ourFourThreats)
                {
                    foreach (var gs in threat.GainSquares)
                    {
                        if (board.GetCell(gs.x, gs.y).IsEmpty && _threatDetector.IsWinningMove(board, gs.x, gs.y, player))
                        {
                            _depthAchieved = 1;
                            _nodesSearched = ourFourThreats.Count;
                            _lastAllocatedTimeMs = 0;
                            _moveType = MoveType.ImmediateWin;
                            _logger.LogDebug("[AI ATTACK] ({Player}) Playing verified winning move at ({WX},{WY})",
                                player, gs.x, gs.y);
                            return gs;
                        }
                    }
                }
            }

            // Priority 2: Extend existing StraightThree to create open four threat
            // SAFEGUARD: Validate move doesn't miss opponent winning squares
            var ourStraightThrees = ourThreats
                .Where(t => t.Type == ThreatType.StraightThree)
                .ToList();

            if (ourStraightThrees.Count > 0)
            {
                // Find the best StraightThree to extend (most open ends)
                var bestThree = ourStraightThrees
                    .OrderByDescending(t => t.GainSquares.Count(gs => board.GetCell(gs.x, gs.y).IsEmpty))
                    .First();

                var extendSquare = bestThree.GainSquares
                    .FirstOrDefault(gs => board.GetCell(gs.x, gs.y).IsEmpty);

                if (extendSquare != default)
                {
                    _depthAchieved = 1;
                    _nodesSearched = ourStraightThrees.Count;
                    _lastAllocatedTimeMs = 0;
                    _moveType = MoveType.ThreatCreation;
                    _logger.LogDebug("[AI ATTACK] ({Player}) Extending StraightThree at ({EX},{EY}) to create open four",
                        player, extendSquare.x, extendSquare.y);
                    return ValidateAndReturnBlockingMove(board, player, extendSquare);
                }
            }

            // Priority 3: Create new StraightThree by finding moves that create threats
            foreach (var candidate in candidates.Take(20))
            {
                if (!board.GetCell(candidate.x, candidate.y).IsEmpty)
                    continue;

                var testBoard = board.PlaceStone(candidate.x, candidate.y, player);
                var newThreats = _threatDetector.DetectThreats(testBoard, player);

                // Prioritize moves that create StraightThree
                if (newThreats.Any(t => t.Type == ThreatType.StraightThree))
                {
                    _depthAchieved = 1;
                    _nodesSearched = 20;
                    _lastAllocatedTimeMs = 0;
                    _moveType = MoveType.ThreatCreation;
                    _logger.LogDebug("[AI ATTACK] ({Player}) Creating StraightThree at ({TX},{TY})",
                        player, candidate.x, candidate.y);
                    return ValidateAndReturnBlockingMove(board, player, candidate);
                }
            }
        }

        // CRITICAL DEFENSE: Filter candidates to blocking/winning moves when opponent has IMMEDIATE threats
        // IMMEDIATE threats: StraightFour, BrokenFour (must be blocked now or lose)
        // DEVELOPING threats: StraightThree (can wait, evaluation will handle it)
        // Store original candidates in case filtering produces empty list
        var originalCandidates = candidates.ToList();
        if (hasOpponentThreats && hasImmediateThreats)
        {
            // CRITICAL FIX: For open fours, reserve minimum time to respond properly
            // An open four is a game-ending threat that requires proper calculation
            if (hasOpenFour)
            {
                const long minCriticalResponseTimeMs = 3000;  // Minimum 3 seconds for critical responses

                if (timeAlloc.SoftBoundMs < minCriticalResponseTimeMs)
                {
                    _logger.LogDebug("[AI DEFENSE] ({Player}) CRITICAL: Open four detected - reserving minimum time ({MinCriticalResponseTimeMs}ms)",
                        player, minCriticalResponseTimeMs);
                    timeAlloc = new TimeAllocation
                    {
                        SoftBoundMs = Math.Max(minCriticalResponseTimeMs, timeAlloc.SoftBoundMs),
                        HardBoundMs = Math.Max(minCriticalResponseTimeMs * 13 / 10, timeAlloc.HardBoundMs),
                        OptimalTimeMs = Math.Max(minCriticalResponseTimeMs * 8 / 10, timeAlloc.OptimalTimeMs),
                        IsEmergency = false,
                        Phase = timeAlloc.Phase,
                        ComplexityMultiplier = timeAlloc.ComplexityMultiplier
                    };
                }
            }

            // FIX: Include our winning moves AND developing threats in candidate list
            // When opponent has threats, we should consider:
            // 1. Blocking their threats (blocking squares)
            // 2. Our immediate wins (StraightFour, BrokenFour)
            // 3. Our developing threats (StraightThree) - these can become winning threats
            var ourThreats = _threatDetector.DetectThreats(board, player);

            // Immediate winning squares
            var ourWinningSquares = ourThreats
                .Where(t => t.Type == ThreatType.StraightFour || t.Type == ThreatType.BrokenFour)
                .SelectMany(t => t.GainSquares)
                .Where(gs => board.GetCell(gs.x, gs.y).IsEmpty)
                .ToList();

            // Developing threat squares (StraightThree) - build our own threats
            var ourDevelopingSquares = ourThreats
                .Where(t => t.Type == ThreatType.StraightThree)
                .SelectMany(t => t.GainSquares)
                .Where(gs => board.GetCell(gs.x, gs.y).IsEmpty)
                .ToList();

            var blockingSet = new HashSet<(int x, int y)>(blockingSquares);
            var winningSet = new HashSet<(int x, int y)>(ourWinningSquares);
            var developingSet = new HashSet<(int x, int y)>(ourDevelopingSquares);

            // Include blocking squares, winning moves, AND developing moves
            // FIX: Only filter candidates when there are IMMEDIATE threats (StraightFour, BrokenFour)
            // For developing threats (StraightThree only), skip filtering and let search decide
            var filteredCandidates = candidates
                .Where(c => blockingSet.Contains(c) || winningSet.Contains(c) || developingSet.Contains(c))
                .ToList();

            if (filteredCandidates.Count > 0)
            {
                // Prioritize: winning > blocking > developing
                filteredCandidates = filteredCandidates
                    .OrderByDescending(c => winningSet.Contains(c) ? 2 : (blockingSet.Contains(c) ? 1 : 0))
                    .ToList();
                candidates = filteredCandidates;
                _logger.LogDebug("[AI DEFENSE] ({Player}) Filtered to {CandidateCount} move(s) ({WinningCount} winning, {BlockingCount} blocking, {DevelopingCount} developing)",
                    player, candidates.Count, winningSet.Count, blockingSet.Count, developingSet.Count);
            }
            else
            {
                // Fallback: use blocking, winning, and developing squares directly as candidates
                candidates = blockingSquares.Concat(ourWinningSquares).Concat(ourDevelopingSquares).Distinct().ToList();
                _logger.LogDebug("[AI DEFENSE] ({Player}) Using blocking/winning/developing squares directly as candidates",
                    player);
            }

            // CRITICAL FIX FOR GRANDMASTER: Immediately return best blocking move for four-threats
            // This bypasses search to guarantee we block correctly
            if (candidates.Count > 0)
            {
                // First check if we have an immediate winning move
                foreach (var winSquare in ourWinningSquares)
                {
                    if (_threatDetector.IsWinningMove(board, winSquare.x, winSquare.y, player))
                    {
                        _depthAchieved = 1;
                        _nodesSearched = 1;
                        _lastAllocatedTimeMs = 0;
                        _moveType = MoveType.ImmediateWin;
                        _logger.LogDebug("[AI DEFENSE] ({Player}) COUNTER-ATTACK with verified winning move at ({WX},{WY})",
                            player, winSquare.x, winSquare.y);
                        return winSquare;
                    }
                }

                // Find the best blocking square using the same scoring as three-threat blocking
                var bestBlock = candidates.First();
                int bestScore = int.MinValue;

                foreach (var block in candidates)
                {
                    if (!board.GetCell(block.x, block.y).IsEmpty)
                        continue;

                    var testBoard = board.PlaceStone(block.x, block.y, player);
                    var ourThreatsAfter = _threatDetector.DetectThreats(testBoard, player);
                    var theirThreatsAfter = _threatDetector.DetectThreats(testBoard, oppPlayer);

                    // Count threats by type for weighted scoring
                    int theirFourThreats = theirThreatsAfter.Count(t => t.Type == ThreatType.StraightFour || t.Type == ThreatType.BrokenFour);
                    int theirThreeThreats = theirThreatsAfter.Count(t => t.Type == ThreatType.StraightThree || t.Type == ThreatType.BrokenThree);
                    int ourFourThreats = ourThreatsAfter.Count(t => t.Type == ThreatType.StraightFour || t.Type == ThreatType.BrokenFour);

                    // CRITICAL: Check for immediate winning squares after this block
                    int theirWinningSquares = 0;
                    for (int wx = 0; wx < BoardSize; wx++)
                    {
                        for (int wy = 0; wy < BoardSize; wy++)
                        {
                            if (testBoard.GetCell(wx, wy).IsEmpty && _threatDetector.IsWinningMove(testBoard, wx, wy, oppPlayer))
                                theirWinningSquares++;
                        }
                    }

                    // Score: heavily penalize blocks that leave immediate threats
                    // Counter-attack is valuable even when blocking four-threats!
                    // +8000 for four-threat counter-attack
                    int score = -theirWinningSquares * SHC.WinningSquarePenalty - theirFourThreats * SHC.FourThreatPenalty - theirThreeThreats * SHC.ThreeThreatPenalty + ourFourThreats * SHC.FourThreatBonus;

                    // Prefer central blocks as tiebreaker
                    int distToCenter = Math.Abs(block.x - SHC.CenterIndex) + Math.Abs(block.y - SHC.CenterIndex);
                    score -= distToCenter;

                    if (score > bestScore)
                    {
                        bestScore = score;
                        bestBlock = block;
                    }
                }

                // Only return immediately if the best block leaves no winning squares
                if (bestScore >= -4000)  // No winning squares left (-10000 each)
                {
                    _depthAchieved = 1;
                    _nodesSearched = candidates.Count;
                    _lastAllocatedTimeMs = 0;
                    _moveType = MoveType.ImmediateBlock;
                    _logger.LogDebug("[AI DEFENSE] ({Player}) IMMEDIATE four-threat block at ({BX},{BY}) - score {Score}",
                        player, bestBlock.x, bestBlock.y, bestScore);
                    return ValidateAndReturnBlockingMove(board, player, bestBlock);
                }

                // CRITICAL: If the best block still leaves us in a losing position,
                // try counter-attacking instead - creating our own winning threat
                if (bestScore < -SHC.FourThreatPenalty)
                {
                    _logger.LogDebug("[AI DEFENSE] ({Player}) Four-threat best block score is {Score} - trying counter-attack",
                        player, bestScore);

                    for (int x = 0; x < BoardSize; x++)
                    {
                        for (int y = 0; y < BoardSize; y++)
                        {
                            if (!board.GetCell(x, y).IsEmpty) continue;

                            var testBoard = board.PlaceStone(x, y, player);
                            var ourNewThreats = _threatDetector.DetectThreats(testBoard, player);
                            var ourNewFourThreats = ourNewThreats.Where(t =>
                                t.Type == ThreatType.StraightFour || t.Type == ThreatType.BrokenFour).ToList();

                            if (ourNewFourThreats.Count > 0)
                            {
                                foreach (var threat in ourNewFourThreats)
                                {
                                    foreach (var gs in threat.GainSquares)
                                    {
                                        if (testBoard.GetCell(gs.x, gs.y).IsEmpty &&
                                            _threatDetector.IsWinningMove(testBoard, gs.x, gs.y, player))
                                        {
                                            _depthAchieved = 1;
                                            _nodesSearched = (x + 1) * BoardSize + y + 1;
                                            _lastAllocatedTimeMs = 0;
                                            _moveType = MoveType.CounterAttack;
                                            _logger.LogDebug("[AI DEFENSE] ({Player}) DESPERATE COUNTER-ATTACK at ({X},{Y}) creates verified winning threat",
                                                player, x, y);
                                            return ValidateAndReturnBlockingMove(board, player, (x, y));
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            }

            // CRITICAL FIX: If threat filtering produced empty candidates, restore original candidates
            // This can happen when threat detection finds threats but gain squares are already occupied
            // or when our threat detection finds no counter-threats
            if (candidates.Count == 0)
            {
                _logger.LogDebug("[AI DEFENSE] ({Player}) Threat filtering produced empty candidates - restoring original {OriginalCount} candidates",
                    player, originalCandidates.Count);
                candidates = originalCandidates.ToList();
            }

            // CRITICAL FIX: For open fours (StraightFour with 2+ blocking squares),
            // we're in a lost position if we can't win immediately. Log this for debugging.
            if (hasOpenFour)
            {
                _logger.LogDebug("[AI DEFENSE] ({Player}) WARNING: Open four detected - opponent can win in 2 moves",
                    player);

                // Check if we have counter-threats
                var counterThreats = _threatDetector.DetectThreats(board, player);
                var ourStraightFours = counterThreats.Count(t => t.Type == ThreatType.StraightFour);
                var ourStraightThrees = counterThreats.Count(t => t.Type == ThreatType.StraightThree);

                if (ourStraightFours > 0)
                {
                    _logger.LogDebug("[AI DEFENSE] ({Player}) We have {OurStraightFours} StraightFour threat(s) - counter-attack instead of just blocking",
                        player, ourStraightFours);
                }
                else if (ourStraightThrees > 1)
                {
                    _logger.LogDebug("[AI DEFENSE] ({Player}) We have {OurStraightThrees} StraightThree threat(s) - creating counter-play",
                        player, ourStraightThrees);
                }
            }
        }
        // VCF Defense was causing Grandmaster+ to play too reactively, blocking opponent threats
        // instead of developing its own position. The evaluation function's defense
        // multiplier (2.2x for opponent threats) should be sufficient for defense.
        // Grandmaster's advantage comes from offensive VCF, not defensive VCF detection.

        // Try VCF (Victory by Continuous Four) search
        // VCF finds forced win sequences through continuous four threats.
        // VCF always enabled
        {
            var (vcfTimeLimit, vcfMaxDepth) = TimeBudgetCalculator.CalculateVCFTimeLimit(timeAlloc);

            // VCF-FIRST MODE: In emergency, use up to 80% of hard bound for VCF
            // CRITICAL: Even in emergency, VCF time scales with difficulty!
            // This prevents emergency mode from making all AIs equal
            if (timeAlloc.IsEmergency)
            {
                vcfTimeLimit = (int)Math.Min(timeAlloc.HardBoundMs * SHC.SoftBoundRatio, SHC.EmergencyVcfCapMs);
            }

            var vcfResult = _vcfSolver.SolveVCF(board, player, timeLimitMs: vcfTimeLimit, maxDepth: vcfMaxDepth);

            // Capture VCF statistics even if not a winning sequence
            _vcfDepthAchieved = vcfResult.DepthAchieved;
            _vcfNodesSearched = vcfResult.NodesSearched;

            if (vcfResult.IsSolved && vcfResult.IsWin && vcfResult.BestMove.HasValue)
            {
                // VCF found a forced win sequence - use it immediately
                return vcfResult.BestMove.Value;
            }

            // VCF-FIRST MODE: In emergency mode, if VCF didn't find a win, check opponent threats
            // CRITICAL: Don't skip blocking even in emergency mode - but only for IMMEDIATE threats
            if (timeAlloc.IsEmergency)
            {
                // If opponent has IMMEDIATE threats (StraightFour, BrokenFour), MUST block
                // For developing threats (StraightThree), let search decide
                if (hasImmediateThreats && blockingSquares.Count > 0)
                {
                    // Return a blocking square immediately
                    _depthAchieved = 1;
                    _nodesSearched = 1;
                    return blockingSquares[0];
                }

                // No opponent threats - safe to use TT move
                var ttMove = GetTranspositionTableMove(board, player, minDepth: 3);
                if (ttMove.HasValue)
                {
                    _depthAchieved = 3;
                    _nodesSearched = 1;
                    return ttMove.Value;
                }

                // Last resort: return the first candidate (usually the center or near existing stones)
                _depthAchieved = 1;
                _nodesSearched = 1;
                return candidates[0];
            }
        }

        // NPS is learned from actual search performance - no hardcoded targets

        // PARALLEL SEARCH: Use Lazy SMP when enabled
        if (parallelSearchEnabled)
        {
            int effectiveThreadCount = threadCount ?? ThreadPoolConfig.GetLazySMPThreadCount();
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
                candidates: candidates);

            // DEFENSIVE: Validate the returned move is actually a valid, empty cell
            // NOTE: Move validation against candidates is already done in SearchLazySMP
            // GetBestMoveWithStats may filter candidates for blocking moves, so checking
            // against the original candidates list here would be a false positive
            var cell = board.GetCell(parallelResult.X, parallelResult.Y);
            if (!cell.IsEmpty)
            {
                Console.WriteLine($"[AI ERROR] Parallel search returned occupied cell ({parallelResult.X},{parallelResult.Y}) at move {moveNumber} - cell player: {cell.Player}");
                // Fall back to first empty candidate
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

            //Console.WriteLine($"[AI PARALLEL] Move: ({parallelResult.X}, {parallelResult.Y}), Depth: {_depthAchieved}, Nodes: {_nodesSearched:N0}, Threads: {parallelResult.ThreadCount}");

            // CRITICAL SAFEGUARD for parallel search path
            return ValidateAndReturnBlockingMove(board, player, (parallelResult.X, parallelResult.Y));
        }

        // TIME-BUDGET-BASED SEARCH: No hardcoded depths, scales with machine capability
        // Faster machines reach deeper depths naturally, slower machines stop earlier

        // Track thread count for diagnostics (even if using sequential search)
        _lastThreadCount = ThreadPoolConfig.GetLazySMPThreadCount();
        _lastParallelDiagnostics = null;
        _lastPonderingEnabled = ponderingEnabled;

        // NPS is learned from actual search performance - no hardcoded targets

        long adjustedSoftBoundMs = Math.Max(50, timeAlloc.SoftBoundMs);
        long adjustedHardBoundMs = Math.Max(adjustedSoftBoundMs, timeAlloc.HardBoundMs);

        (int x, int y) bestMove;
        int depthAchieved;
        long nodesSearched;

        // Initialize transposition table for this search
        _transpositionTable.IncrementAge();
        _tableHits = 0;
        _tableLookups = 0;

        // Initialize search statistics
        _nodesSearched = 0;
        _depthAchieved = 0;
        _vcfNodesSearched = 0;
        _vcfDepthAchieved = 0;
        _searchStopwatch.Restart();

        // Initialize time control for search timeout
        _searchHardBoundMs = adjustedHardBoundMs;
        _lastAllocatedTimeMs = adjustedHardBoundMs;
        _searchStopped = false;

        // ITERATIVE DEEPENING: Search depth 1, 2, 3... until time runs out
        // PURE TIME-BASED: No depth target - different machines reach different depths naturally
        // Always return best move from deepest completed iteration
        bestMove = candidates[0];
        int currentDepth = 1;

        // SAFEGUARD: Absolute max depth to prevent runaway values from TT bugs
        const int AbsoluteMaxDepth = SearchConstants.AbsoluteMaxDepth;
        const long MinNodesForValidIteration = 10;

        while (true)
        {
            if (currentDepth > AbsoluteMaxDepth)
            {
                break;
            }

            // Pre-iteration check: Total nodes must scale with depth
            // Prevents depth inflation from TT cache hits without real search
            // MUST match the same formula used in ParallelMinimaxSearch.SearchWithIterationTimeAware
            if (currentDepth > 10)
            {
                long minimumTotalNodesForDepth = (long)(currentDepth - 5) * (currentDepth - 5) * 200;
                if (_nodesSearched < minimumTotalNodesForDepth)
                {
                    // Not enough total nodes to justify this depth - stop now
                    break;
                }
            }

            // Max depth limit from "go depth N"
            if (maxDepth.HasValue && currentDepth > maxDepth.Value)
            {
                break;
            }

            // Max nodes limit from "go nodes N"
            if (maxNodes.HasValue && _nodesSearched >= maxNodes.Value)
            {
                break;
            }

            // Check time bounds using TimeAllocation
            var elapsed = _searchStopwatch.ElapsedMilliseconds;

            // Hard bound check - must stop
            if (elapsed >= _searchHardBoundMs)
            {
                break;
            }

            // Soft bound check with time multiplier applied
            // Lower difficulties hit soft bound earlier due to multiplier
            if (elapsed >= adjustedSoftBoundMs)
            {
                // Check if we should continue for one more iteration
                // Only continue if we have significant time left and next iteration won't exceed hard bound
                double remainingSeconds = (_searchHardBoundMs - elapsed) / 1000.0;
                double estimatedNextTime = elapsed / 1000.0 * SHC.EffectiveBranchingFactorEstimate;
                if (remainingSeconds < estimatedNextTime * SHC.SoftBoundRatio)
                {
                    break;
                }
            }

            // Reset stopped flag for this depth
            _searchStopped = false;

            // Track nodes and time before this iteration to detect TT cache hits
            long nodesBeforeIteration = _nodesSearched;
            long ticksBeforeIteration = _searchStopwatch.ElapsedTicks;

            var result = SearchWithDepth(board, player, currentDepth, candidates);
            long nodesSearchedThisIteration = _nodesSearched - nodesBeforeIteration;
            // Use ticks for high-resolution timing (ms has ~15ms resolution on Windows)
            long ticksThisIteration = _searchStopwatch.ElapsedTicks - ticksBeforeIteration;
            long timeThisIterationMs = (long)(ticksThisIteration * 1000.0 / System.Diagnostics.Stopwatch.Frequency);

            if (result.x != -1)
            {
                bestMove = (result.x, result.y);
                _lastSearchScore = result.score;

                // Only update depth if this was a real search (not just TT cache hit)
                // TT hits return instantly with 0-1 nodes, which shouldn't count as "depth achieved"
                if (nodesSearchedThisIteration >= MinNodesForValidIteration)
                {
                    _depthAchieved = currentDepth; // Track deepest completed search
                }
            }

            // If search was stopped due to timeout, don't continue to next depth
            if (_searchStopped)
            {
                break;
            }

            // Only increment depth if meaningful search occurred
            // This prevents depth inflation from TT cache hits
            if (nodesSearchedThisIteration >= MinNodesForValidIteration)
            {
                currentDepth++;
            }
            else
            {
                // TT cache hit or instant return - no point searching deeper with cached results
                // Break to prevent depth inflation
                break;
            }
        }

        _searchStopwatch.Stop();
        depthAchieved = _depthAchieved;
        nodesSearched = _nodesSearched;

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
                    player,  // Pondering for us (next to move after opponent)
                    ponderTimeMs
                );
            }
        }

        // Publish search stats to channel
        PublishSearchStats(player, StatsType.MainSearch, _searchStopwatch.ElapsedMilliseconds);

        // CRITICAL SAFEGUARD: Final validation that the move blocks opponent's winning threats
        return ValidateAndReturnBlockingMove(board, player, bestMove);
    }
}
