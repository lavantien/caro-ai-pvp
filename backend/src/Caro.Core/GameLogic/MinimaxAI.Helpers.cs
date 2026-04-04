using System.Runtime.CompilerServices;
using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic.TimeManagement;
using Caro.Core.GameLogic.Search;
using Microsoft.Extensions.Logging;
using SHC = Caro.Core.Domain.Configuration.SearchHeuristicConstants;

namespace Caro.Core.GameLogic;

public partial class MinimaxAI
{
    /// <summary>
    /// SAFEGUARD: Final validation that the returned move blocks opponent's winning threats.
    /// This catches any edge cases where the blocking logic might be bypassed.
    /// </summary>
    private (int x, int y) ValidateAndReturnBlockingMove(Board board, Player player, (int x, int y) proposedMove)
    {
        var oppPlayer = player == Player.Red ? Player.Blue : Player.Red;
        var opponentWinningSquares = new List<(int x, int y)>();

        // CRITICAL: Validate that proposedMove is an empty square
        // If the proposed move is on an occupied square, we MUST find a valid move
        bool proposedMoveIsEmpty = board.GetCell(proposedMove.x, proposedMove.y).Player == Player.None;
        if (!proposedMoveIsEmpty)
        {
            _logger.LogWarning("[AI SAFEGUARD] CRITICAL: Proposed move ({X},{Y}) is occupied! Finding valid move...", proposedMove.x, proposedMove.y);
        }

        // Re-scan the full board for opponent winning moves (immediate 5-in-a-row)
        for (int fx = 0; fx < BoardSize; fx++)
        {
            for (int fy = 0; fy < BoardSize; fy++)
            {
                if (board.GetCell(fx, fy).Player == Player.None)
                {
                    if (_threatDetector.IsWinningMove(board, fx, fy, oppPlayer))
                    {
                        opponentWinningSquares.Add((fx, fy));
                    }
                }
            }
        }

        // CRITICAL: Also check for open fours (StraightFour with 2 winning squares)
        // An open four means opponent wins next move regardless of which square we block
        // We must detect these and block them before they're created
        var opponentThreats = _threatDetector.DetectThreats(board, oppPlayer);
        foreach (var threat in opponentThreats)
        {
            if (threat.Type == ThreatType.StraightFour || threat.Type == ThreatType.BrokenFour)
            {
                // Add all gain squares from open fours to the blocking list
                foreach (var gs in threat.GainSquares)
                {
                    if (board.GetCell(gs.x, gs.y).IsEmpty && !opponentWinningSquares.Contains(gs))
                    {
                        opponentWinningSquares.Add(gs);
                    }
                }
            }
        }

        if (opponentWinningSquares.Count == 0)
        {
            // No opponent threats - return the proposed move IF it's empty
            if (proposedMoveIsEmpty)
            {
                return proposedMove;
            }
            // Proposed move is occupied - find any empty square
            return QuickWinChecker.FindAnyEmptySquare(board, proposedMove);
        }

        // If proposed move is occupied, we MUST find a blocking move - skip the validation
        if (!proposedMoveIsEmpty)
        {
            // Skip to the blocking logic below
            goto FindBlockingMove;
        }

        // Check if our proposed move blocks all threats
        var testBoard = board.PlaceStone(proposedMove.x, proposedMove.y, player);
        bool blocksAllThreats = true;

        // Check remaining winning squares
        foreach (var (wx, wy) in opponentWinningSquares)
        {
            if (wx == proposedMove.x && wy == proposedMove.y)
                continue; // This square is now occupied
            if (_threatDetector.IsWinningMove(testBoard, wx, wy, oppPlayer))
            {
                blocksAllThreats = false;
                break;
            }
        }

        // CRITICAL: Also check if opponent still has open fours after our move
        // An open four means opponent wins next move regardless of single block
        if (blocksAllThreats)
        {
            var remainingThreats = _threatDetector.DetectThreats(testBoard, oppPlayer);
            foreach (var threat in remainingThreats)
            {
                if (threat.Type == ThreatType.StraightFour || threat.Type == ThreatType.BrokenFour)
                {
                    // Opponent still has an open four - our block doesn't work
                    blocksAllThreats = false;
                    _logger.LogDebug("[AI SAFEGUARD] Block at ({X},{Y}) leaves opponent with {ThreatType}", proposedMove.x, proposedMove.y, threat.Type);
                    break;
                }
            }
        }

        if (blocksAllThreats)
        {
            // Proposed move is valid - it blocks all threats
            return proposedMove;
        }

    FindBlockingMove:

        // Our move doesn't block all threats - find one that does
        foreach (var (bx, by) in opponentWinningSquares)
        {
            var blockTestBoard = board.PlaceStone(bx, by, player);
            bool thisBlockWorks = true;

            // Check remaining winning squares
            foreach (var (wx, wy) in opponentWinningSquares)
            {
                if (wx == bx && wy == by)
                    continue;
                if (_threatDetector.IsWinningMove(blockTestBoard, wx, wy, oppPlayer))
                {
                    thisBlockWorks = false;
                    break;
                }
            }

            // CRITICAL: Also check if opponent still has open fours after this block
            if (thisBlockWorks)
            {
                var remainingThreats = _threatDetector.DetectThreats(blockTestBoard, oppPlayer);
                foreach (var threat in remainingThreats)
                {
                    if (threat.Type == ThreatType.StraightFour || threat.Type == ThreatType.BrokenFour)
                    {
                        // Opponent still has an open four - this block doesn't work
                        thisBlockWorks = false;
                        break;
                    }
                }
            }

            if (thisBlockWorks)
            {
                _logger.LogDebug("[AI SAFEGUARD] Forcing block at ({BX},{BY}) instead of ({X},{Y})", bx, by, proposedMove.x, proposedMove.y);
                _moveType = MoveType.ImmediateBlock;
                return (bx, by);
            }
        }

        // No single block works - opponent has multiple independent winning threats
        // Try to find a counter-attack move that creates our own winning threat
        // If we can force opponent to block, we gain a tempo and might survive
        var ourWinningMove = QuickWinChecker.FindOurWinningMove(board, player, _threatDetector);
        if (ourWinningMove.HasValue)
        {
            _logger.LogDebug("[AI SAFEGUARD] No single block works - counter-attacking with winning move at ({WX},{WY})", ourWinningMove.Value.x, ourWinningMove.Value.y);
            _moveType = MoveType.ImmediateWin;
            return ourWinningMove.Value;
        }

        // CRITICAL FIX: Find the block that minimizes remaining winning squares
        // For an open four with 2 winning squares, blocking one reduces winning squares to 1
        // This gives us a chance if opponent makes a mistake, or time to create our own threat
        var bestDelayingBlock = opponentWinningSquares[0];
        int minRemainingWinningSquares = int.MaxValue;

        foreach (var (bx, by) in opponentWinningSquares)
        {
            var blockTestBoard = board.PlaceStone(bx, by, player);
            int remainingWinningSquares = 0;

            // Count remaining winning squares after this block
            for (int wx = 0; wx < BoardSize; wx++)
            {
                for (int wy = 0; wy < BoardSize; wy++)
                {
                    if (blockTestBoard.GetCell(wx, wy).IsEmpty && _threatDetector.IsWinningMove(blockTestBoard, wx, wy, oppPlayer))
                        remainingWinningSquares++;
                }
            }

            if (remainingWinningSquares < minRemainingWinningSquares)
            {
                minRemainingWinningSquares = remainingWinningSquares;
                bestDelayingBlock = (bx, by);
            }
        }

        _logger.LogDebug("[AI SAFEGUARD] No single block works - best delaying block at ({BX},{BY}) leaves {Count} winning squares",
            bestDelayingBlock.x, bestDelayingBlock.y, minRemainingWinningSquares);
        _moveType = MoveType.ImmediateBlock;
        return bestDelayingBlock;
    }

    /// <summary>
    /// Calculate appropriate search depth based on time allocation.
    ///
    /// PURE TIME-BASED DEPTH:
    /// - Search runs until time expires via iterative deepening
    /// - NO NPS estimation (unreliable across different machines)
    /// - NO artificial depth floors or reductions
    /// - Higher difficulties get more time allocation, naturally reaching deeper
    /// - Different machines reach different depths based on hardware capability
    /// </summary>
    private int CalculateDepthForTime(int baseDepth, TimeAllocation timeAlloc, long? timeRemainingMs, int candidateCount)
    {
        // Infer initial time from the move number and remaining time (for emergency detection)
        if (timeRemainingMs.HasValue && timeAlloc.Phase == GamePhase.Opening)
        {
            if (_inferredInitialTimeMs < 0)
            {
                _inferredInitialTimeMs = timeRemainingMs.Value;
            }
            else if (Math.Abs(timeRemainingMs.Value - _inferredInitialTimeMs) > _inferredInitialTimeMs * 0.3)
            {
                _inferredInitialTimeMs = timeRemainingMs.Value;
            }
        }

        // Emergency mode: minimum depth to avoid timeout
        if (timeAlloc.IsEmergency)
        {
            return 1;
        }

        // Return high max depth and let iterative deepening stop when time runs out
        // The search naturally completes as many depths as possible within the time budget
        // Different machines will reach different depths - this is expected and correct
        return SHC.MaxAIMaxDepth;
    }

    /// <summary>
    // REMOVED: GetCriticalDefenseLevel - no longer used, threats handled by evaluation function

    /// <summary>
    /// Get a move from the transposition table if available at sufficient depth
    /// Used for emergency mode when time is very low
    /// </summary>
    private (int x, int y)? GetTranspositionTableMove(Board board, Player player, int minDepth)
    {
        var boardHash = _transpositionTable.CalculateHash(board);

        // Try to get the best move from TT with a wide search
        _tableLookups++;
        var (found, cachedScore, cachedMove) = _transpositionTable.Lookup(
            boardHash, minDepth, int.MinValue, int.MaxValue);

        if (found && cachedMove.HasValue)
        {
            // Verify the move is valid
            var (x, y) = cachedMove.Value;
            if (x >= 0 && x < BoardSize && y >= 0 && y < BoardSize)
            {
                var cell = board.GetCell(x, y);
                if (cell.IsEmpty)
                {
                    _tableHits++;
                    return cachedMove;
                }
            }
        }

        return null;
    }

    /// <summary>
    /// Check if opponent has an immediate winning move that must be blocked
    /// This is a critical defensive check that runs before any search
    /// Returns the blocking position if found, null otherwise
    // REMOVED: FindCriticalDefense - no longer used, threats handled by evaluation function

    /// <summary>
    /// Check if opponent can VCF (Victory by Continuous Four) and find blocking move
    /// This is essential for Grandmaster+ to prevent losing to VCF attacks
    ///
    /// OPTIMIZED: Uses fast threat detection + immediate defensive move selection
    /// - VCF check time scales with remaining time budget
    /// - Quick check: if opponent has no threats, skip VCF check entirely
    /// - Single VCF check (not nested) to detect opponent threats
    /// - Return first valid defensive move without re-checking VCF for each one
    /// - Skip VCF defense in emergency mode (prioritize speed over accuracy)
    /// </summary>
    private (int x, int y)? FindVCFDefense(Board board, Player player, TimeAllocation timeAlloc)
    {
        // Skip VCF defense in emergency mode - we need to move quickly
        // In emergency, the VCF-first mode already handles defensive prioritization
        if (timeAlloc.IsEmergency)
        {
            return null;
        }

        var opponent = player == Player.Red ? Player.Blue : Player.Red;

        // Quick check: if opponent has very few threats, no need for VCF defense
        // This is a fast check that avoids expensive VCF search in non-tactical positions
        var opponentThreats = _vcfSolver.GetThreatMoves(board, opponent);
        if (opponentThreats.Count < 2)
        {
            // Opponent has less than 2 threat moves - not a VCF danger
            return null;
        }

        // Use scaled VCF time based on difficulty for defensive checking
        // Higher difficulties get more time to find defensive moves
        var (vcfCheckTime, vcfMaxDepth) = TimeBudgetCalculator.CalculateVCFTimeLimit(timeAlloc);

        // For defensive VCF, use 50% of the offensive VCF time (we need to be efficient)
        vcfCheckTime = vcfCheckTime / 2;

        var opponentVCFResult = _vcfSolver.SolveVCF(board, opponent, timeLimitMs: vcfCheckTime, maxDepth: vcfMaxDepth);

        // If opponent can VCF, we need to find a defensive move
        if (opponentVCFResult.IsSolved && opponentVCFResult.IsWin)
        {
            // Get defensive moves - these are moves that block opponent's threats
            var defenses = _vcfSolver.GetDefenseMoves(board, opponent, player);

            if (defenses.Count > 0)
            {
                // OPTIMIZATION: Return first valid defensive move without re-checking
                // The old implementation did a nested VCF check for each defense move,
                // which was O(defenses × VCF_time) = 10 × 500ms = 5+ seconds overhead
                // The new approach is O(1) = just validate and return first valid move

                foreach (var defense in defenses)
                {
                    // Validate move is on board and empty
                    if (defense.x >= 0 && defense.x < board.BoardSize &&
                        defense.y >= 0 && defense.y < board.BoardSize &&
                        board.GetCell(defense.x, defense.y).IsEmpty)
                    {
                        return defense;
                    }
                }

                // Fallback: use first defensive move even if not currently empty
                // (shouldn't happen, but handle gracefully)
                var fallback = defenses[0];
                if (fallback.x >= 0 && fallback.x < board.BoardSize &&
                    fallback.y >= 0 && fallback.y < board.BoardSize)
                {
                    return fallback;
                }
            }
        }

        // CRITICAL FIX: Check for opponent immediate win even when VCF returns IsSolved=false
        // The VCF solver returns IsSolved=false when opponent has an immediate one-move win
        // We must detect this and defend against it by scanning all empty squares
        for (int x = 0; x < board.BoardSize; x++)
        {
            for (int y = 0; y < board.BoardSize; y++)
            {
                if (board.GetCell(x, y).IsEmpty)
                {
                    var testBoard = board.PlaceStone(x, y, opponent);
                    var winResult = _winDetector.CheckWin(testBoard);

                    if (winResult.HasWinner && winResult.Winner == opponent)
                    {
                        _logger.LogDebug("[AI DEFENSE] ({Player}) Opponent has immediate win at ({X}, {Y}) - blocking!",
                            player, x, y);
                        return (x, y);
                    }
                }
            }
        }

        return null;
    }

}
