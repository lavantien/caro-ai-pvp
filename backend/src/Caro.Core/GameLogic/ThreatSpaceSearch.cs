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

            board.PlaceStone(move.x, move.y, attacker);
            nodesSearched++;

            // Check if this move wins
            var winResult = _winDetector.CheckWin(board);
            if (winResult.HasWinner && winResult.Winner == attacker)
            {
                board.GetCell(move.x, move.y).Player = Player.None;
                return new VCFResult
                {
                    IsSolved = true,
                    IsWin = true,
                    BestMove = move,
                    NodesSearched = nodesSearched,
                    DepthAchieved = 1
                };
            }

            // Get defender's possible responses
            var defenses = GetDefenseMoves(board, attacker, opponent);

            bool allDefensesLose = true;
            bool atLeastOneDefense = false;

            foreach (var defense in defenses)
            {
                if ((DateTime.UtcNow - startTime).TotalMilliseconds > timeLimitMs)
                {
                    break;
                }

                atLeastOneDefense = true;

                board.PlaceStone(defense.x, defense.y, opponent);
                nodesSearched++;

                // Recursively check if attacker can still win
                var subResult = SolveVCFRecursive(board, attacker, 2, maxDepth, startTime, timeLimitMs, ref nodesSearched);

                board.GetCell(defense.x, defense.y).Player = Player.None;

                if (!subResult.IsWin)
                {
                    // Defender has a response that prevents win
                    allDefensesLose = false;
                    break;
                }
            }

            board.GetCell(move.x, move.y).Player = Player.None;

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
            board.PlaceStone(move.x, move.y, attacker);
            nodesSearched++;

            var defenses = GetDefenseMoves(board, attacker, opponent);

            bool allDefensesLose = true;
            bool atLeastOneDefense = false;

            foreach (var defense in defenses)
            {
                atLeastOneDefense = true;

                board.PlaceStone(defense.x, defense.y, opponent);
                nodesSearched++;

                var subResult = SolveVCFRecursive(board, attacker, depth + 1, maxDepth, startTime, timeLimitMs, ref nodesSearched);

                board.GetCell(defense.x, defense.y).Player = Player.None;

                if (!subResult.IsWin)
                {
                    allDefensesLose = false;
                    break;
                }
            }

            board.GetCell(move.x, move.y).Player = Player.None;

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
