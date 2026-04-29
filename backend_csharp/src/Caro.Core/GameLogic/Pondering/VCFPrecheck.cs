using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic.Pondering;

/// <summary>
/// VCF (Victory by Continuous Four) pre-check for pondering
/// Determines if position has potential threats worth pondering
/// Avoids wasting CPU time on quiet positions where pondering has low value
/// </summary>
public sealed class VCFPrecheck
{
    private readonly ThreatDetector _threatDetector = new();

    /// <summary>
    /// Check if position has potential threats worth pondering
    /// Returns true if there are immediate winning threats, forcing sequences, or complex tactical positions
    /// </summary>
    /// <param name="board">Current board state</param>
    /// <param name="player">Player who might be pondering (opponent to move)</param>
    /// <returns>True if pondering is likely to be beneficial</returns>
    public bool HasPotentialThreats(Board board, Player player)
    {
        if (player == Player.None)
            return false;

        // Get all threats for the player
        var threats = _threatDetector.DetectThreats(board, player);

        // Check for immediate winning threats
        foreach (var threat in threats)
        {
            // Straight Four is always a winning threat
            if (threat.Type == ThreatType.StraightFour)
                return true;

            // Broken Four can create double attacks
            if (threat.Type == ThreatType.BrokenFour)
                return true;
        }

        // Check for multiple 3-in-row threats (potential double attack)
        var threeInRowCount = threats.Count(t => t.Type == ThreatType.StraightThree);
        if (threeInRowCount >= 2)
            return true;

        // Check for Straight Three + Broken Four/Three combination
        var anyThree = threats.Any(t => t.Type == ThreatType.StraightThree ||
                                    t.Type == ThreatType.BrokenThree);
        if (anyThree && threeInRowCount >= 1)
            return true;

        // Check board congestion - more stones = more tactical complexity
        var stoneCount = board.GetRedBitBoard().CountBits() +
                        board.GetBlueBitBoard().CountBits();

        // In mid-game with some threats, pondering is valuable
        if (stoneCount > 40 && threats.Count >= 1)
            return true;

        // In late-game (congested), even weak threats are worth pondering
        if (stoneCount > 70 && threats.Count >= 1)
            return true;

        // Check if opponent (other player) has threats we need to respond to
        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var opponentThreats = _threatDetector.DetectThreats(board, opponent);

        // If opponent has forcing threats, we should ponder responses
        foreach (var threat in opponentThreats)
        {
            if (threat.Type == ThreatType.StraightFour ||
                threat.Type == ThreatType.StraightThree)
                return true;
        }

        // No significant threats - skip pondering to save CPU
        return false;
    }

    /// <summary>
    /// Get the urgency level of threats for a position
    /// Higher values indicate more critical positions that benefit from pondering
    /// </summary>
    /// <returns>Urgency level from 0 (no urgency) to 100 (critical)</returns>
    public int GetThreatUrgency(Board board, Player player)
    {
        if (player == Player.None)
            return 0;

        var threats = _threatDetector.DetectThreats(board, player);
        var urgency = 0;

        foreach (var threat in threats)
        {
            urgency += threat.Type switch
            {
                ThreatType.StraightFour => 50,   // Immediate win threat
                ThreatType.BrokenFour => 30,      // Double attack potential
                ThreatType.StraightThree => 15,   // Strong forcing move
                ThreatType.BrokenThree => 5,      // Potential threat
                _ => 0
            };
        }

        // Cap at 100
        return Math.Min(100, urgency);
    }

    /// <summary>
    /// Check if position is in opening phase (few stones on board)
    /// Opening positions benefit less from pondering due to low tactical complexity
    /// </summary>
    public bool IsOpeningPhase(Board board)
    {
        var stoneCount = board.GetRedBitBoard().CountBits() +
                        board.GetBlueBitBoard().CountBits();
        return stoneCount < 10;
    }

    /// <summary>
    /// Check if position is in endgame phase (board nearly full)
    /// Endgame positions benefit most from pondering due to high tactical complexity
    /// </summary>
    public bool IsEndgamePhase(Board board)
    {
        var stoneCount = board.GetRedBitBoard().CountBits() +
                        board.GetBlueBitBoard().CountBits();
        var totalCells = board.BoardSize * board.BoardSize;
        return stoneCount > totalCells * 0.7; // More than 70% full
    }

    /// <summary>
    /// Calculate pondering time multiplier based on position complexity
    /// Returns 0.0 to 2.0 - higher for more complex/tactical positions
    /// </summary>
    public double CalculatePonderTimeMultiplier(Board board, Player player)
    {
        if (player == Player.None)
            return 0.5;

        var urgency = GetThreatUrgency(board, player);

        // Base multiplier from urgency
        var multiplier = 0.5 + (urgency / 100.0) * 1.5;

        // Bonus for endgame positions
        if (IsEndgamePhase(board))
            multiplier *= 1.2;

        // Reduce for opening positions
        if (IsOpeningPhase(board))
            multiplier *= 0.7;

        // Clamp between 0.1 and 2.0
        return Math.Clamp(multiplier, 0.1, 2.0);
    }
}
