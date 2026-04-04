using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

public partial class DFPNSearch
{
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
            if (count++ >= TimeConstants.MaxDefensesPerThreat) break;
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
}
