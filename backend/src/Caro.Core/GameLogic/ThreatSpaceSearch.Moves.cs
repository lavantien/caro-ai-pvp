using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Move generation methods for ThreatSpaceSearch
/// </summary>
public partial class ThreatSpaceSearch
{
    /// <summary>
    /// Get all threat moves (forcing moves) for a player
    /// </summary>
    public List<(int x, int y)> GetThreatMoves(Board board, Player player)
    {
        var threats = _threatDetector.DetectThreats(board, player);
        var gainSquares = new HashSet<(int x, int y)>();

        // Add all forcing threat gain squares
        foreach (var threat in threats)
        {
            if (_threatDetector.IsForcingMove(threat, board, player))
            {
                foreach (var square in threat.GainSquares)
                {
                    if (board.GetCell(square.x, square.y).IsEmpty)
                    {
                        gainSquares.Add(square);
                    }
                }
            }
        }

        // Also check for immediate winning moves
        for (int x = 0; x < board.BoardSize; x++)
        {
            for (int y = 0; y < board.BoardSize; y++)
            {
                if (board.GetCell(x, y).IsEmpty && _threatDetector.IsWinningMove(board, x, y, player))
                {
                    gainSquares.Add((x, y));
                }
            }
        }

        return gainSquares.ToList();
    }

    /// <summary>
    /// Zero-allocation version: Get threat moves using pre-allocated buffers.
    /// Uses a simple bool array for deduplication instead of HashSet.
    /// </summary>
    /// <param name="board">The board</param>
    /// <param name="player">The player</param>
    /// <param name="buffer">Pre-allocated buffer for results</param>
    /// <param name="seen">Pre-allocated bool[256] for deduplication</param>
    /// <returns>Number of moves written to buffer</returns>
    public int GetThreatMovesZeroAlloc(Board board, Player player, Span<(int x, int y)> buffer, Span<bool> seen)
    {
        // Clear seen array
        seen.Clear();

        int count = 0;
        var threats = _threatDetector.DetectThreats(board, player);

        // Add all forcing threat gain squares
        foreach (var threat in threats)
        {
            if (_threatDetector.IsForcingMove(threat, board, player))
            {
                foreach (var square in threat.GainSquares)
                {
                    if (board.GetCell(square.x, square.y).IsEmpty)
                    {
                        int idx = square.y * 16 + square.x;
                        if (!seen[idx] && count < buffer.Length)
                        {
                            seen[idx] = true;
                            buffer[count++] = square;
                        }
                    }
                }
            }
        }

        // Also check for immediate winning moves
        for (int x = 0; x < board.BoardSize && count < buffer.Length; x++)
        {
            for (int y = 0; y < board.BoardSize && count < buffer.Length; y++)
            {
                int idx = y * 16 + x;
                if (!seen[idx] && board.GetCell(x, y).IsEmpty && _threatDetector.IsWinningMove(board, x, y, player))
                {
                    seen[idx] = true;
                    buffer[count++] = (x, y);
                }
            }
        }

        return count;
    }

    /// <summary>
    /// Get all defense moves for defender against attacker
    /// </summary>
    public List<(int x, int y)> GetDefenseMoves(Board board, Player attacker, Player defender)
    {
        var defenses = new HashSet<(int x, int y)>();

        // Block attacker's threats
        var attackerThreats = _threatDetector.DetectThreats(board, attacker);
        foreach (var threat in attackerThreats)
        {
            if (_threatDetector.IsForcingMove(threat, board, attacker))
            {
                foreach (var square in threat.GainSquares)
                {
                    if (board.GetCell(square.x, square.y).IsEmpty)
                    {
                        defenses.Add(square);
                    }
                }
            }
        }

        // Also consider counter-attacks from defender
        var defenderThreats = _threatDetector.DetectThreats(board, defender);
        foreach (var threat in defenderThreats)
        {
            foreach (var square in threat.GainSquares)
            {
                if (board.GetCell(square.x, square.y).IsEmpty)
                {
                    defenses.Add(square);
                }
            }
        }

        // Limit to most important defenses if too many
        if (defenses.Count > TimeConstants.MaxDefensesPerThreat)
        {
            // Prioritize by threat priority
            var sortedDefenses = defenses.Take(10).ToList();
            return sortedDefenses;
        }

        return defenses.ToList();
    }

    /// <summary>
    /// Zero-allocation version: Get defense moves using pre-allocated buffers.
    /// </summary>
    /// <param name="board">The board</param>
    /// <param name="attacker">The attacking player</param>
    /// <param name="defender">The defending player</param>
    /// <param name="buffer">Pre-allocated buffer for results</param>
    /// <param name="seen">Pre-allocated bool[256] for deduplication</param>
    /// <returns>Number of moves written to buffer</returns>
    public int GetDefenseMovesZeroAlloc(Board board, Player attacker, Player defender, Span<(int x, int y)> buffer, Span<bool> seen)
    {
        // Clear seen array
        seen.Clear();

        int count = 0;
        const int maxMoves = TimeConstants.MaxCandidateMoves;

        // Block attacker's threats
        var attackerThreats = _threatDetector.DetectThreats(board, attacker);
        foreach (var threat in attackerThreats)
        {
            if (_threatDetector.IsForcingMove(threat, board, attacker))
            {
                foreach (var square in threat.GainSquares)
                {
                    if (board.GetCell(square.x, square.y).IsEmpty)
                    {
                        int idx = square.y * 16 + square.x;
                        if (!seen[idx] && count < buffer.Length && count < maxMoves)
                        {
                            seen[idx] = true;
                            buffer[count++] = square;
                        }
                    }
                }
            }
        }

        // Also consider counter-attacks from defender
        if (count < maxMoves)
        {
            var defenderThreats = _threatDetector.DetectThreats(board, defender);
            foreach (var threat in defenderThreats)
            {
                foreach (var square in threat.GainSquares)
                {
                    if (board.GetCell(square.x, square.y).IsEmpty)
                    {
                        int idx = square.y * 16 + square.x;
                        if (!seen[idx] && count < buffer.Length && count < maxMoves)
                        {
                            seen[idx] = true;
                            buffer[count++] = square;
                        }
                    }
                }
            }
        }

        return count;
    }
}
