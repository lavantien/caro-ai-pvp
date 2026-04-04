using Caro.Core.Domain.Configuration;
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
public partial class DFPNSearch
{
    private readonly ThreatDetector _threatDetector = new();
    private readonly WinDetector _winDetector = new();

    // Infinity value for proof numbers (use large value, not actual infinity)
    private const uint Infinity = TimeConstants.DFPNInfinity;

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
        int maxDepth = TimeConstants.DefaultSearchDepth,
        int timeLimitMs = TimeConstants.DefaultTimeLimitMs)
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
