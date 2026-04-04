using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Microsoft.Extensions.Logging;
using SHC = Caro.Core.Domain.Configuration.SearchHeuristicConstants;

namespace Caro.Core.GameLogic;

public partial class MinimaxAI
{
    /// <summary>
    /// Result of threat analysis for the current position.
    /// Passed between threat-related extraction methods to avoid recomputation.
    /// </summary>
    private struct ThreatInfo
    {
        public bool HasOpponentThreats;
        public bool HasImmediateThreats;
        public bool HasOpenFour;
        public List<(int x, int y)> BlockingSquares;
        public List<(int x, int y)> PriorityBlockingSquares;
    }

    /// <summary>
    /// Analyze opponent threats and populate threat info for the current position.
    /// Returns threat info for use in subsequent blocking/filtering logic.
    /// </summary>
    private ThreatInfo AnalyzeOpponentThreats(Board board, Player oppPlayer)
    {
        var info = new ThreatInfo
        {
            HasOpponentThreats = false,
            HasImmediateThreats = false,
            HasOpenFour = false,
            BlockingSquares = new(),
            PriorityBlockingSquares = new()
        };

        var threats = _threatDetector.DetectThreats(board, oppPlayer)
            .Where(t => t.Type == ThreatType.StraightFour || t.Type == ThreatType.StraightThree || t.Type == ThreatType.BrokenFour || t.Type == ThreatType.BrokenThree)
            .ToList();

        info.HasOpponentThreats = threats.Count > 0;
        info.HasImmediateThreats = threats.Any(t => t.Type == ThreatType.StraightFour || t.Type == ThreatType.BrokenFour);

        if (!info.HasOpponentThreats)
            return info;

        var straightFourCount = threats.Count(t => t.Type == ThreatType.StraightFour);
        var straightThreeCount = threats.Count(t => t.Type == ThreatType.StraightThree);
        var brokenFourCount = threats.Count(t => t.Type == ThreatType.BrokenFour);
        var brokenThreeCount = threats.Count(t => t.Type == ThreatType.BrokenThree);

        info.BlockingSquares = threats
            .SelectMany(t => t.GainSquares)
            .Where(gs => board.GetCell(gs.x, gs.y).IsEmpty)
            .ToList();

        foreach (var threat in threats.Where(t => t.Type == ThreatType.StraightFour))
        {
            if (threat.GainSquares.Count >= 2)
            {
                info.HasOpenFour = true;
                foreach (var square in threat.GainSquares)
                {
                    if (board.GetCell(square.x, square.y).IsEmpty)
                        info.PriorityBlockingSquares.Add(square);
                }
            }
        }

        if (brokenFourCount > 0)
        {
            info.HasOpenFour = true;
        }

        _logger.LogDebug("[AI DEFENSE] Opponent has {SF} StraightFour, {ST} StraightThree, {BF} BrokenFour, {BT} BrokenThree threat(s), blocking squares: {Blocks}{OpenFour}",
            straightFourCount, straightThreeCount, brokenFourCount, brokenThreeCount,
            string.Join(", ", info.BlockingSquares.Select(g => $"({g.x},{g.y})")),
            info.HasOpenFour ? " [CRITICAL THREAT DETECTED]" : "");

        return info;
    }

    /// <summary>
    /// Handle three-threat blocking when opponent has StraightThree or BrokenThree threats.
    /// Returns a blocking move if immediate action is needed, null to continue to search.
    /// </summary>
    private (int x, int y)? HandleThreeThreatBlocking(Board board, Player player, Player oppPlayer,
        ThreatInfo threatInfo)
    {
        // Check if there are three-threats that need handling
        var threats = _threatDetector.DetectThreats(board, oppPlayer)
            .Where(t => t.Type == ThreatType.StraightThree || t.Type == ThreatType.BrokenThree)
            .ToList();

        if (threats.Count == 0)
            return null;

        // First check if we have our own winning threats
        var ourThreats = _threatDetector.DetectThreats(board, player);
        var ourStraightFours = ourThreats.Where(t => t.Type == ThreatType.StraightFour || t.Type == ThreatType.BrokenFour).ToList();

        if (ourStraightFours.Count > 0)
        {
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

        // Collect ALL gain squares from all threats
        var allThreats = _threatDetector.DetectThreats(board, oppPlayer)
            .Where(t => t.Type == ThreatType.StraightFour || t.Type == ThreatType.StraightThree || t.Type == ThreatType.BrokenFour || t.Type == ThreatType.BrokenThree)
            .ToList();

        var allGainSquares = allThreats
            .SelectMany(t => t.GainSquares)
            .Where(gs => board.GetCell(gs.x, gs.y).IsEmpty)
            .Distinct()
            .ToList();

        if (allGainSquares.Count == 0)
            return null;

        // Check if opponent has multiple independent three-threats
        bool hasMultipleIndependentThreats = threats.Count >= 2 &&
            threats.SelectMany(t => t.GainSquares).Distinct().Count() >= 3;

        if (hasMultipleIndependentThreats)
        {
            _logger.LogDebug("[AI DEFENSE] ({Player}) CRITICAL: Opponent has {Count} independent three-threats - blocking is futile, must counter-attack!",
                player, threats.Count);

            var counterAttack = TryCounterAttack(board, player, threats);
            if (counterAttack.HasValue)
                return counterAttack.Value;

            _logger.LogDebug("[AI DEFENSE] ({Player}) No counter-attack found - must block best threat", player);
        }

        // Find the best blocking square
        var bestBlock = ScoreBlockingSquares(board, player, oppPlayer, allGainSquares, out int bestScore);

        // If the best block leaves us in a losing position, try counter-attack
        if (bestScore < -SHC.FourThreatPenalty)
        {
            _logger.LogDebug("[AI DEFENSE] ({Player}) Best block score is {Score} - trying counter-attack instead",
                player, bestScore);

            var desperateCounter = TryDesperateCounterAttack(board, player);
            if (desperateCounter.HasValue)
                return desperateCounter.Value;
        }

        _depthAchieved = 1;
        _nodesSearched = allGainSquares.Count;
        _lastAllocatedTimeMs = 0;
        _moveType = MoveType.ImmediateBlock;
        _logger.LogDebug("[AI DEFENSE] ({Player}) IMMEDIATE three-threat block at ({BX},{BY}) - {Count} gain squares available (score: {Score})",
            player, bestBlock.x, bestBlock.y, allGainSquares.Count, bestScore);
        return ValidateAndReturnBlockingMove(board, player, bestBlock);
    }

    /// <summary>
    /// Score all blocking squares and return the best one.
    /// </summary>
    private (int x, int y) ScoreBlockingSquares(Board board, Player player, Player oppPlayer,
        List<(int x, int y)> gainSquares, out int bestScore)
    {
        var bestBlock = gainSquares[0];
        bestScore = int.MinValue;

        foreach (var block in gainSquares)
        {
            var testBoard = board.PlaceStone(block.x, block.y, player);
            var ourThreatsAfter = _threatDetector.DetectThreats(testBoard, player);
            var theirThreatsAfter = _threatDetector.DetectThreats(testBoard, oppPlayer);

            int theirFourThreats = theirThreatsAfter.Count(t => t.Type == ThreatType.StraightFour || t.Type == ThreatType.BrokenFour);
            int theirThreeThreats = theirThreatsAfter.Count(t => t.Type == ThreatType.StraightThree || t.Type == ThreatType.BrokenThree);
            int ourFourThreats = ourThreatsAfter.Count(t => t.Type == ThreatType.StraightFour || t.Type == ThreatType.BrokenFour);

            int theirWinningSquares = 0;
            for (int wx = 0; wx < BoardSize; wx++)
            {
                for (int wy = 0; wy < BoardSize; wy++)
                {
                    if (testBoard.GetCell(wx, wy).IsEmpty && _threatDetector.IsWinningMove(testBoard, wx, wy, oppPlayer))
                        theirWinningSquares++;
                }
            }

            int multipleThreePenalty = theirThreeThreats >= SHC.MultipleThreeThreshold ? -SHC.MultipleThreePenalty : 0;
            int score = -theirWinningSquares * SHC.WinningSquarePenalty
                - theirFourThreats * SHC.FourThreatPenalty
                - theirThreeThreats * SHC.ThreeThreatPenalty
                + ourFourThreats * SHC.FourThreatBonus
                + multipleThreePenalty;

            int distToCenter = Math.Abs(block.x - SHC.CenterIndex) + Math.Abs(block.y - SHC.CenterIndex);
            score -= distToCenter;

            if (score > bestScore)
            {
                bestScore = score;
                bestBlock = block;
            }
        }

        return bestBlock;
    }

    /// <summary>
    /// Try to find a counter-attack move that creates our own four-threat.
    /// Used when opponent has multiple independent threats.
    /// </summary>
    private (int x, int y)? TryCounterAttack(Board board, Player player, List<Threat> threeThreats)
    {
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
        return null;
    }

    /// <summary>
    /// Try desperate counter-attack when best block still leaves losing position.
    /// Searches for a move that creates a verified winning threat.
    /// </summary>
    private (int x, int y)? TryDesperateCounterAttack(Board board, Player player)
    {
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
        return null;
    }
}
