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
    /// Filter and score candidates when opponent has immediate threats (StraightFour, BrokenFour).
    /// Modifies candidates in place. Returns a move if immediate action is needed, null to continue to search.
    /// </summary>
    private (int x, int y)? FilterCandidatesForCriticalDefense(
        Board board, Player player, Player oppPlayer,
        ref List<(int x, int y)> candidates, ref TimeAllocation timeAlloc,
        bool hasImmediateThreats, bool hasOpenFour,
        List<(int x, int y)> blockingSquares)
    {
        if (!hasImmediateThreats)
            return null;

        var originalCandidates = candidates.ToList();

        // For open fours, reserve minimum time to respond properly
        if (hasOpenFour)
        {
            const long minCriticalResponseTimeMs = 3000;

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

        // Include our winning moves AND developing threats in candidate list
        var ourThreats = _threatDetector.DetectThreats(board, player);

        var ourWinningSquares = ourThreats
            .Where(t => t.Type == ThreatType.StraightFour || t.Type == ThreatType.BrokenFour)
            .SelectMany(t => t.GainSquares)
            .Where(gs => board.GetCell(gs.x, gs.y).IsEmpty)
            .ToList();

        var ourDevelopingSquares = ourThreats
            .Where(t => t.Type == ThreatType.StraightThree)
            .SelectMany(t => t.GainSquares)
            .Where(gs => board.GetCell(gs.x, gs.y).IsEmpty)
            .ToList();

        var blockingSet = new HashSet<(int x, int y)>(blockingSquares);
        var winningSet = new HashSet<(int x, int y)>(ourWinningSquares);
        var developingSet = new HashSet<(int x, int y)>(ourDevelopingSquares);

        var filteredCandidates = candidates
            .Where(c => blockingSet.Contains(c) || winningSet.Contains(c) || developingSet.Contains(c))
            .ToList();

        if (filteredCandidates.Count > 0)
        {
            filteredCandidates = filteredCandidates
                .OrderByDescending(c => winningSet.Contains(c) ? 2 : (blockingSet.Contains(c) ? 1 : 0))
                .ToList();
            candidates = filteredCandidates;
            _logger.LogDebug("[AI DEFENSE] ({Player}) Filtered to {CandidateCount} move(s) ({WinningCount} winning, {BlockingCount} blocking, {DevelopingCount} developing)",
                player, candidates.Count, winningSet.Count, blockingSet.Count, developingSet.Count);
        }
        else
        {
            candidates = blockingSquares.Concat(ourWinningSquares).Concat(ourDevelopingSquares).Distinct().ToList();
            _logger.LogDebug("[AI DEFENSE] ({Player}) Using blocking/winning/developing squares directly as candidates",
                player);
        }

        // Immediately return best blocking move for four-threats
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

            // Find the best blocking square
            var bestBlock = ScoreBlockingSquares(board, player, oppPlayer, candidates, out int bestScore);

            if (bestScore >= -4000)
            {
                _depthAchieved = 1;
                _nodesSearched = candidates.Count;
                _lastAllocatedTimeMs = 0;
                _moveType = MoveType.ImmediateBlock;
                _logger.LogDebug("[AI DEFENSE] ({Player}) IMMEDIATE four-threat block at ({BX},{BY}) - score {Score}",
                    player, bestBlock.x, bestBlock.y, bestScore);
                return ValidateAndReturnBlockingMove(board, player, bestBlock);
            }

            if (bestScore < -SHC.FourThreatPenalty)
            {
                _logger.LogDebug("[AI DEFENSE] ({Player}) Four-threat best block score is {Score} - trying counter-attack",
                    player, bestScore);

                var desperateCounter = TryDesperateCounterAttack(board, player);
                if (desperateCounter.HasValue)
                    return desperateCounter.Value;
            }
        }

        // If threat filtering produced empty candidates, restore original
        if (candidates.Count == 0)
        {
            _logger.LogDebug("[AI DEFENSE] ({Player}) Threat filtering produced empty candidates - restoring original {OriginalCount} candidates",
                player, originalCandidates.Count);
            candidates = originalCandidates.ToList();
        }

        // Log open four warning
        if (hasOpenFour)
        {
            _logger.LogDebug("[AI DEFENSE] ({Player}) WARNING: Open four detected - opponent can win in 2 moves", player);

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

        return null;
    }
}
