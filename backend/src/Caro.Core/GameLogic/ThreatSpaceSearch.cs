using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// VCF (Victory by Continuous Four) solver using threat-space search
/// Searches only forcing sequences (threats that require immediate response)
/// Dramatically reduces branching factor from ~200 to 2-10 moves
/// </summary>
public class ThreatSpaceSearch
{
    private readonly ThreatDetector _threatDetector = new();
    private readonly WinDetector _winDetector = new();
    private readonly DFPNSearch _dfpn = new();

    /// <summary>
    /// Result of VCF search
    /// </summary>
    public class VCFResult
    {
        /// <summary>
        /// True if the search found a definitive result (win or loss)
        /// </summary>
        public bool IsSolved { get; init; }

        /// <summary>
        /// True if the attacker can force a win
        /// </summary>
        public bool IsWin { get; init; }

        /// <summary>
        /// Best move found (may be null if no winning sequence)
        /// </summary>
        public (int x, int y)? BestMove { get; init; }

        /// <summary>
        /// Number of nodes searched
        /// </summary>
        public int NodesSearched { get; init; }

        /// <summary>
        /// Search depth achieved
        /// </summary>
        public int DepthAchieved { get; init; }
    }

    /// <summary>
    /// Solve for VCF (Victory by Continuous Four) sequence
    /// </summary>
    /// <param name="board">Current board position</param>
    /// <param name="attacker">Player trying to force win</param>
    /// <param name="timeLimitMs">Time limit in milliseconds</param>
    /// <param name="maxDepth">Maximum search depth</param>
    /// <returns>VCF result with best move if found</returns>
    public VCFResult SolveVCF(
        Board board,
        Player attacker,
        int timeLimitMs = 1000,
        int maxDepth = 30)
    {
        var startTime = DateTime.UtcNow;
        int nodesSearched = 0;

        // Check for immediate win first
        var immediateWin = FindImmediateWin(board, attacker);
        if (immediateWin.HasValue)
        {
            return new VCFResult
            {
                IsSolved = true,
                IsWin = true,
                BestMove = immediateWin,
                NodesSearched = 1,
                DepthAchieved = 1
            };
        }

        // Check for empty board
        if (IsEmptyBoard(board))
        {
            return new VCFResult
            {
                IsSolved = false,
                IsWin = false,
                BestMove = null,
                NodesSearched = 0,
                DepthAchieved = 0
            };
        }

        // Check if opponent has immediate win (loss for attacker)
        var opponent = GetOpponent(attacker);
        var opponentImmediateWin = FindImmediateWin(board, opponent);
        if (opponentImmediateWin.HasValue)
        {
            // If opponent has an immediate win, we cannot execute a VCF
            // because they will ignore our threat and just win
            return new VCFResult
            {
                IsSolved = false,
                IsWin = false,
                BestMove = null,
                NodesSearched = nodesSearched,
                DepthAchieved = 0
            };
        }

        // Generate threat moves for attacker
        var threatMoves = GetThreatMoves(board, attacker);
        if (threatMoves.Count == 0)
        {
            return new VCFResult
            {
                IsSolved = false,
                IsWin = false,
                BestMove = null,
                NodesSearched = 0,
                DepthAchieved = 0
            };
        }

        // Try each threat move and check if any leads to win
        foreach (var move in threatMoves)
        {
            // Check time limit
            if ((DateTime.UtcNow - startTime).TotalMilliseconds > timeLimitMs)
            {
                break;
            }

            var attackBoard = board.PlaceStone(move.x, move.y, attacker);
            nodesSearched++;

            // Check if this move wins
            var winResult = _winDetector.CheckWin(attackBoard);
            if (winResult.HasWinner && winResult.Winner == attacker)
            {
                return new VCFResult
                {
                    IsSolved = true,
                    IsWin = true,
                    BestMove = move,
                    NodesSearched = nodesSearched,
                    DepthAchieved = 1
                };
            }

            // Get defender's possible responses (using attackBoard since it has the attacker's move)
            var defenses = GetDefenseMoves(attackBoard, attacker, opponent);

            bool allDefensesLose = true;
            bool atLeastOneDefense = false;

            foreach (var defense in defenses)
            {
                if ((DateTime.UtcNow - startTime).TotalMilliseconds > timeLimitMs)
                {
                    break;
                }

                atLeastOneDefense = true;

                var defenseBoard = attackBoard.PlaceStone(defense.x, defense.y, opponent);
                nodesSearched++;

                // Recursively check if attacker can still win
                var subResult = SolveVCFRecursive(defenseBoard, attacker, 2, maxDepth, startTime, timeLimitMs, ref nodesSearched);

                if (!subResult.IsWin)
                {
                    // Defender has a response that prevents win
                    allDefensesLose = false;
                    break;
                }
            }

            if (allDefensesLose && atLeastOneDefense)
            {
                // All defender's responses lead to attacker win - VCF found!
                return new VCFResult
                {
                    IsSolved = true,
                    IsWin = true,
                    BestMove = move,
                    NodesSearched = nodesSearched,
                    DepthAchieved = 2
                };
            }
        }

        // No VCF found
        return new VCFResult
        {
            IsSolved = false,
            IsWin = false,
            BestMove = threatMoves.Count > 0 ? threatMoves[0] : null,
            NodesSearched = nodesSearched,
            DepthAchieved = 0
        };
    }

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
        if (defenses.Count > 10)
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
        const int maxMoves = 10; // Limit to prevent excessive branching

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

    #region Private Methods

    private VCFResult SolveVCFRecursive(
        Board board,
        Player attacker,
        int depth,
        int maxDepth,
        DateTime startTime,
        int timeLimitMs,
        ref int nodesSearched)
    {
        // Check termination conditions
        if (depth > maxDepth)
        {
            return new VCFResult
            {
                IsSolved = false,
                IsWin = false,
                NodesSearched = nodesSearched,
                DepthAchieved = depth
            };
        }

        if ((DateTime.UtcNow - startTime).TotalMilliseconds > timeLimitMs)
        {
            return new VCFResult
            {
                IsSolved = false,
                IsWin = false,
                NodesSearched = nodesSearched,
                DepthAchieved = depth
            };
        }

        var opponent = GetOpponent(attacker);

        // Check for win
        var winResult = _winDetector.CheckWin(board);
        if (winResult.HasWinner)
        {
            return new VCFResult
            {
                IsSolved = true,
                IsWin = winResult.Winner == attacker,
                NodesSearched = nodesSearched,
                DepthAchieved = depth
            };
        }

        // Get threat moves
        var threatMoves = GetThreatMoves(board, attacker);
        if (threatMoves.Count == 0)
        {
            // No forcing moves - position not resolved
            return new VCFResult
            {
                IsSolved = false,
                IsWin = false,
                NodesSearched = nodesSearched,
                DepthAchieved = depth
            };
        }

        // Try each threat move
        foreach (var move in threatMoves)
        {
            var attackBoard = board.PlaceStone(move.x, move.y, attacker);
            nodesSearched++;

            var defenses = GetDefenseMoves(attackBoard, attacker, opponent);

            bool allDefensesLose = true;
            bool atLeastOneDefense = false;

            foreach (var defense in defenses)
            {
                atLeastOneDefense = true;

                var defenseBoard = attackBoard.PlaceStone(defense.x, defense.y, opponent);
                nodesSearched++;

                var subResult = SolveVCFRecursive(defenseBoard, attacker, depth + 1, maxDepth, startTime, timeLimitMs, ref nodesSearched);

                if (!subResult.IsWin)
                {
                    allDefensesLose = false;
                    break;
                }
            }

            if (allDefensesLose && atLeastOneDefense)
            {
                return new VCFResult
                {
                    IsSolved = true,
                    IsWin = true,
                    BestMove = move,
                    NodesSearched = nodesSearched,
                    DepthAchieved = depth
                };
            }
        }

        return new VCFResult
        {
            IsSolved = false,
            IsWin = false,
            NodesSearched = nodesSearched,
            DepthAchieved = depth
        };
    }

    private (int x, int y)? FindImmediateWin(Board board, Player player)
    {
        for (int x = 0; x < board.BoardSize; x++)
        {
            for (int y = 0; y < board.BoardSize; y++)
            {
                if (board.GetCell(x, y).IsEmpty && _threatDetector.IsWinningMove(board, x, y, player))
                {
                    return (x, y);
                }
            }
        }
        return null;
    }

    private bool IsEmptyBoard(Board board)
    {
        for (int x = 0; x < board.BoardSize; x++)
        {
            for (int y = 0; y < board.BoardSize; y++)
            {
                if (!board.GetCell(x, y).IsEmpty)
                    return false;
            }
        }
        return true;
    }

    private Player GetOpponent(Player player)
    {
        return player == Player.Red ? Player.Blue : Player.Red;
    }

    #endregion
}
