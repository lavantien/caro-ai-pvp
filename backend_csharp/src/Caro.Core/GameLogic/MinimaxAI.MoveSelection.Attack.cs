using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Microsoft.Extensions.Logging;
using SHC = Caro.Core.Domain.Configuration.SearchHeuristicConstants;

namespace Caro.Core.GameLogic;

public partial class MinimaxAI
{
    /// <summary>
    /// PROACTIVE ATTACK: When no opponent threats, create our own threats.
    /// Returns a move if an attack opportunity is found, null otherwise.
    /// </summary>
    private (int x, int y)? TryProactiveAttack(Board board, Player player, List<(int x, int y)> candidates)
    {
        var ourThreats = _threatDetector.DetectThreats(board, player);

        // Priority 1: Create StraightFour/BrokenFour (immediate win)
        var ourFourThreats = ourThreats
            .Where(t => t.Type == ThreatType.StraightFour || t.Type == ThreatType.BrokenFour)
            .ToList();

        if (ourFourThreats.Count > 0)
        {
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
        var ourStraightThrees = ourThreats
            .Where(t => t.Type == ThreatType.StraightThree)
            .ToList();

        if (ourStraightThrees.Count > 0)
        {
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

        return null;
    }
}
