using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Result of a VCF (Victory by Continuous Four) search
/// </summary>
public enum SearchResult
{
    /// <summary>
    /// Attacker can force a win
    /// </summary>
    Win,

    /// <summary>
    /// Attacker cannot force a win (defender can prevent)
    /// </summary>
    Loss,

    /// <summary>
    /// Search exhausted without conclusive result
    /// </summary>
    Unknown
}

/// <summary>
/// Depth-First Proof Number search for VCF solving
/// Uses proof/disproof numbers to efficiently search threat space
///
/// Algorithm:
/// - Each node has proof (pn) and disproof (dn) numbers
/// - OR nodes (attacker): pn = sum(children.pn), dn = min(children.dn)
/// - AND nodes (defender): pn = min(children.pn), dn = sum(children.dn)
/// - Expand most-proving node (where pn == dn)
/// </summary>
public class DFPNSearch
{
    private readonly ThreatDetector _threatDetector = new();
    private readonly WinDetector _winDetector = new();

    // Infinity value for proof numbers (use large value, not actual infinity)
    private const uint Infinity = 1_000_000;

    /// <summary>
    /// Solve for VCF sequence using df-pn search
    /// </summary>
    /// <param name="board">Current board position</param>
    /// <param name="attacker">Player trying to win (attacker)</param>
    /// <param name="maxDepth">Maximum search depth</param>
    /// <param name="timeLimitMs">Time limit in milliseconds</param>
    /// <returns>Search result and suggested move (if any)</returns>
    public (SearchResult result, (int x, int y)? move) Solve(
        Board board,
        Player attacker,
        int maxDepth = 30,
        int timeLimitMs = 1000)
    {
        var startTime = DateTime.UtcNow;

        // Check if already won
        if (IsWinning(board, attacker))
        {
            return (SearchResult.Win, null);
        }

        // Check if opponent has immediate win (loss for attacker)
        var opponent = GetOpponent(attacker);
        if (IsWinning(board, opponent))
        {
            return (SearchResult.Loss, null);
        }

        // Check for empty board - no VCF possible
        if (IsEmptyBoard(board))
        {
            return (SearchResult.Unknown, null);
        }

        // Check for immediate winning moves
        var immediateWin = FindImmediateWin(board, attacker);
        if (immediateWin.HasValue)
        {
            return (SearchResult.Win, immediateWin);
        }

        var root = CreateNode(board, attacker, true);
        var transpositionTable = new Dictionary<ulong, PNNode>();

        var result = SearchInternal(root, board, attacker, 0, maxDepth, startTime, timeLimitMs, transpositionTable);

        if (result == SearchResult.Win && root.BestMove.HasValue)
        {
            return (SearchResult.Win, root.BestMove);
        }

        return (result, root.BestMove);
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

    /// <summary>
    /// Get proof numbers for a position (for testing)
    /// </summary>
    public (uint proof, uint disproof) GetProofNumbers(Board board, Player player)
    {
        var node = CreateNode(board, player, true);
        return (node.Proof, node.Disproof);
    }

    #region Private Methods

    private SearchResult SearchInternal(
        PNNode node,
        Board board,
        Player attacker,
        int depth,
        int maxDepth,
        DateTime startTime,
        int timeLimitMs,
        Dictionary<ulong, PNNode> transpositionTable)
    {
        while (node.Proof < Infinity && node.Disproof < Infinity && depth < maxDepth)
        {
            // Check time limit
            if ((DateTime.UtcNow - startTime).TotalMilliseconds > timeLimitMs)
            {
                return SearchResult.Unknown;
            }

            // Check for transposition
            ulong hash = board.GetHash();
            if (transpositionTable.TryGetValue(hash, out var cached))
            {
                if (cached.IsSolved)
                {
                    return cached.IsProven ? SearchResult.Win : SearchResult.Loss;
                }
            }

            // Expand most-proving node
            if (node.Children.Count == 0)
            {
                GenerateChildren(node, board, attacker);
                transpositionTable[hash] = node;
            }

            if (node.Children.Count == 0)
            {
                // No moves available - evaluate position
                if (IsWinning(board, attacker))
                {
                    MarkProven(node);
                    return SearchResult.Win;
                }
                MarkDisproven(node);
                return SearchResult.Loss;
            }

            // Select most-proving child
            var mostProving = SelectMostProvingChild(node);
            if (mostProving == null)
            {
                break;
            }

            // Make move and recurse
            bool isAttacker = depth % 2 == 0;
            var nextPlayer = isAttacker ? GetOpponent(attacker) : attacker;
            var nextIsAttacker = !isAttacker;

            var move = mostProving.Move ?? throw new InvalidOperationException("Most proving move is null");
            var newBoard = board.PlaceStone(move.x, move.y, nextPlayer);

            // Check if move creates win
            var winResult = _winDetector.CheckWin(newBoard);
            if (winResult.HasWinner && winResult.Winner == attacker)
            {
                MarkProven(node);
                transpositionTable[hash] = node;
                return SearchResult.Win;
            }

            var childResult = SearchInternal(
                mostProving,
                newBoard,
                attacker,
                depth + 1,
                maxDepth,
                startTime,
                timeLimitMs,
                transpositionTable);

            // Update proof numbers based on child result
            UpdateNodeProofNumbers(node, depth % 2 == 0);
            transpositionTable[hash] = node;

            if (node.Proof == 0)
            {
                MarkProven(node);
                return SearchResult.Win;
            }

            if (node.Disproof == 0)
            {
                MarkDisproven(node);
                return SearchResult.Loss;
            }
        }

        return SearchResult.Unknown;
    }

    private PNNode? SelectMostProvingChild(PNNode node)
    {
        PNNode? selected = null;
        uint minProofDisproof = Infinity;

        foreach (var child in node.Children)
        {
            // For OR node (attacker): select child with min(pn, dn)
            // For AND node (defender): select child with min(pn, dn)
            uint min = Math.Min(child.Proof, child.Disproof);
            if (min < minProofDisproof)
            {
                minProofDisproof = min;
                selected = child;
            }
        }

        return selected;
    }

    private void GenerateChildren(PNNode node, Board board, Player attacker)
    {
        bool isAttackerTurn = node.IsOrNode; // OR node = attacker's turn

        if (isAttackerTurn)
        {
            // Attacker's move: generate threat moves
            var threats = _threatDetector.DetectThreats(board, attacker);
            var gainSquares = new HashSet<(int x, int y)>();

            foreach (var threat in threats)
            {
                if (_threatDetector.IsForcingMove(threat, board, attacker))
                {
                    foreach (var square in threat.GainSquares)
                    {
                        gainSquares.Add(square);
                    }
                }
            }

            // Add immediate winning moves
            for (int x = 0; x < board.BoardSize; x++)
            {
                for (int y = 0; y < board.BoardSize; y++)
                {
                    if (board.GetCell(x, y).IsEmpty)
                    {
                        if (_threatDetector.IsWinningMove(board, x, y, attacker))
                        {
                            gainSquares.Add((x, y));
                        }
                    }
                }
            }

            // Create child nodes for each move
            foreach (var move in gainSquares)
            {
                var child = new PNNode
                {
                    Move = move,
                    IsOrNode = false, // Next is defender (AND node)
                    Proof = 1,
                    Disproof = 1
                };
                node.Children.Add(child);
            }

            // If no threat moves, add some candidate moves
            if (node.Children.Count == 0)
            {
                AddCandidateMoves(node, board, attacker);
            }
        }
        else
        {
            // Defender's move: generate defense moves
            // Find all threats attacker can create and block them
            var attackerThreats = _threatDetector.DetectThreats(board, attacker);
            var costSquares = new HashSet<(int x, int y)>();

            foreach (var threat in attackerThreats)
            {
                if (_threatDetector.IsForcingMove(threat, board, attacker))
                {
                    foreach (var square in threat.GainSquares)
                    {
                        costSquares.Add(square);
                    }
                }
            }

            // Also consider counter-attacks
            var defender = GetOpponent(attacker);
            var defenderThreats = _threatDetector.DetectThreats(board, defender);
            foreach (var threat in defenderThreats)
            {
                foreach (var square in threat.GainSquares)
                {
                    if (board.GetCell(square.x, square.y).IsEmpty)
                    {
                        costSquares.Add(square);
                    }
                }
            }

            // Create child nodes
            foreach (var move in costSquares)
            {
                var child = new PNNode
                {
                    Move = move,
                    IsOrNode = true, // Next is attacker (OR node)
                    Proof = 1,
                    Disproof = 1
                };
                node.Children.Add(child);
            }

            // If no defense moves, add candidates
            if (node.Children.Count == 0)
            {
                AddCandidateMoves(node, board, defender);
            }
        }
    }

    private void AddCandidateMoves(PNNode node, Board board, Player player)
    {
        // Add moves adjacent to existing stones
        var candidates = new HashSet<(int x, int y)>();

        for (int x = 0; x < board.BoardSize; x++)
        {
            for (int y = 0; y < board.BoardSize; y++)
            {
                if (board.GetCell(x, y).IsEmpty)
                {
                    // Check if adjacent to any stone
                    for (int dx = -1; dx <= 1; dx++)
                    {
                        for (int dy = -1; dy <= 1; dy++)
                        {
                            if (dx == 0 && dy == 0) continue;
                            int nx = x + dx, ny = y + dy;
                            if (nx >= 0 && nx < board.BoardSize && ny >= 0 && ny < board.BoardSize)
                            {
                                if (!board.GetCell(nx, ny).IsEmpty)
                                {
                                    candidates.Add((x, y));
                                    goto found;
                                }
                            }
                        }
                    }
                found:;
                }
            }
        }

        // Limit candidates
        int count = 0;
        foreach (var move in candidates)
        {
            if (count++ >= 10) break;
            var child = new PNNode
            {
                Move = move,
                IsOrNode = !node.IsOrNode,
                Proof = 1,
                Disproof = 1
            };
            node.Children.Add(child);
        }
    }

    private void UpdateNodeProofNumbers(PNNode node, bool isOrNode)
    {
        if (node.Children.Count == 0)
        {
            return;
        }

        if (isOrNode)
        {
            // OR node: attacker needs ONE child to win
            // pn = min(children.pn) - pick easiest win
            // dn = sum(children.dn) - defender must block all
            node.Proof = Infinity;
            node.Disproof = 0;

            foreach (var child in node.Children)
            {
                if (child.Proof < node.Proof)
                {
                    node.Proof = child.Proof;
                    node.BestMove = child.Move;
                }
                node.Disproof = Math.Min(Infinity, node.Disproof + child.Disproof);
            }
        }
        else
        {
            // AND node: defender must block ALL threats
            // pn = sum(children.pn) - attacker must break through all defenses
            // dn = min(children.dn) - defender picks easiest defense
            node.Proof = 0;
            node.Disproof = Infinity;

            foreach (var child in node.Children)
            {
                node.Proof = Math.Min(Infinity, node.Proof + child.Proof);
                if (child.Disproof < node.Disproof)
                {
                    node.Disproof = child.Disproof;
                    node.BestMove = child.Move;
                }
            }
        }
    }

    private void MarkProven(PNNode node)
    {
        node.IsProven = true;
        node.IsSolved = true;
        node.Proof = 0;
        node.Disproof = Infinity;
    }

    private void MarkDisproven(PNNode node)
    {
        node.IsProven = false;
        node.IsSolved = true;
        node.Proof = Infinity;
        node.Disproof = 0;
    }

    private PNNode CreateNode(Board board, Player attacker, bool isOrNode)
    {
        return new PNNode
        {
            IsOrNode = isOrNode,
            Proof = 1,
            Disproof = 1,
            IsSolved = false
        };
    }

    private bool IsWinning(Board board, Player player)
    {
        var result = _winDetector.CheckWin(board);
        return result.HasWinner && result.Winner == player;
    }

    private Player GetOpponent(Player player)
    {
        return player == Player.Red ? Player.Blue : Player.Red;
    }

    #endregion

    #region PNNode Class

    private class PNNode
    {
        public uint Proof { get; set; } = 1;        // Proof number
        public uint Disproof { get; set; } = 1;     // Disproof number
        public bool IsOrNode { get; set; }          // True = attacker's turn, False = defender's
        public bool IsSolved { get; set; }          // True if proven/disproven
        public bool IsProven { get; set; }          // True if WIN proven
        public (int x, int y)? Move { get; set; }   // Move that led to this node
        public (int x, int y)? BestMove { get; set; } // Best move from this node
        public List<PNNode> Children { get; set; } = new();
    }

    #endregion
}
