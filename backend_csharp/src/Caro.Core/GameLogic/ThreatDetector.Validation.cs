using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// ThreatDetector partial class - Validation and helper methods.
/// Threat validation, overline checks, and position helpers.
/// </summary>
public partial class ThreatDetector
{
    /// <summary>
    /// Get the cost squares (defense moves) for a threat
    /// </summary>
    public List<(int x, int y)> GetCostSquares(Threat threat, Board board, Player defender)
    {
        var costSquares = new List<(int x, int y)>();
        foreach (var square in threat.GainSquares)
        {
            if (IsValidPosition(square.x, square.y, BitBoard.Size) &&
                board.GetCell(square.x, square.y).IsEmpty)
            {
                costSquares.Add(square);
            }
        }
        return costSquares;
    }

    /// <summary>
    /// Check if a threat move is forcing (requires immediate response)
    /// </summary>
    public bool IsForcingMove(Threat threat, Board board, Player player)
    {
        return threat.Type switch
        {
            ThreatType.StraightFour => true,
            ThreatType.BrokenFour => true,
            ThreatType.StraightThree => true,
            ThreatType.BrokenThree => false,
            _ => false
        };
    }

    /// <summary>
    /// Find all moves that create at least one threat
    /// </summary>
    public List<(int x, int y)> FindThreatMoves(Board board, Player player)
    {
        var threatMoves = new List<(int x, int y)>();

        for (int x = 0; x < BitBoard.Size; x++)
        {
            for (int y = 0; y < BitBoard.Size; y++)
            {
                if (!board.GetCell(x, y).IsEmpty || !IsAdjacentToPlayer(board, x, y, player))
                    continue;

                var testBoard = board.PlaceStone(x, y, player);
                var newThreats = DetectThreats(testBoard, player);

                if (newThreats.Count > 0 && !threatMoves.Contains((x, y)))
                {
                    threatMoves.Add((x, y));
                }
            }
        }

        return threatMoves;
    }

    /// <summary>
    /// Check if a position would create a winning move
    /// </summary>
    public bool IsWinningMove(Board board, int x, int y, Player player)
    {
        // Check if cell is empty before placing stone
        if (!board.GetCell(x, y).IsEmpty)
            return false;

        var testBoard = board.PlaceStone(x, y, player);
        var winResult = _winDetector.CheckWin(testBoard);
        return winResult.HasWinner && winResult.Winner == player;
    }

    private bool IsAdjacentToStoneLine((int x, int y) pos, List<(int x, int y)> stones, int dx, int dy)
    {
        // Check if position is adjacent to the first or last stone in the line direction
        if (stones.Count == 0)
            return false;

        var firstStone = stones[0];
        var lastStone = stones[^1];

        // Check if position is before first stone (in negative direction)
        if (pos.x == firstStone.x - dx && pos.y == firstStone.y - dy)
            return true;

        // Check if position is after last stone (in positive direction)
        if (pos.x == lastStone.x + dx && pos.y == lastStone.y + dy)
            return true;

        return false;
    }

    private bool IsBetweenStones((int x, int y) pos, List<(int x, int y)> stones)
    {
        // Check if the position is between two consecutive stones
        var sortedStones = stones.OrderBy(s => s.x * BitBoard.Size + s.y).ToList();
        for (int i = 0; i < sortedStones.Count - 1; i++)
        {
            var curr = sortedStones[i];
            var next = sortedStones[i + 1];
            int midX = (curr.x + next.x) / 2;
            int midY = (curr.y + next.y) / 2;
            if (pos.x == midX && pos.y == midY)
                return true;
        }
        return false;
    }

    private bool IsAdjacentToStones((int x, int y) pos, List<(int x, int y)> stones)
    {
        foreach (var stone in stones)
        {
            int dist = Math.Abs(pos.x - stone.x) + Math.Abs(pos.y - stone.y);
            if (dist == 1)
                return true;
        }
        return false;
    }

    private bool IsValidThreat(Threat threat, Board board)
    {
        foreach (var (gx, gy) in threat.GainSquares)
        {
            if (!IsValidPosition(gx, gy, BitBoard.Size))
                return false;
            if (!board.GetCell(gx, gy).IsEmpty)
                return false;
        }

        if (WouldCreateOverline(threat, board))
            return false;

        if (IsSandwichedThreat(threat, board))
            return false;

        return true;
    }

    private bool WouldCreateOverline(Threat threat, Board board)
    {
        // CRITICAL FIX: A threat is only invalid if ALL gain squares create overlines.
        // If at least ONE gain square creates exactly 5 (a valid win), the threat is valid.
        // This handles cases like: XXXX_ X where:
        // - Playing at the gap (after 4 stones) creates 6+ = overline
        // - But playing at the other end creates exactly 5 = win
        bool anyValidWin = false;

        foreach (var (gx, gy) in threat.GainSquares)
        {
            var testBoard = board.PlaceStone(gx, gy, threat.Owner);

            int count = CountInDirection(testBoard, gx, gy, threat.Direction, threat.Owner);

            if (count == 5)
            {
                // This gain square creates exactly 5 = valid win
                anyValidWin = true;
            }
        }

        // If no gain square creates exactly 5, check if all create overlines
        if (!anyValidWin)
        {
            foreach (var (gx, gy) in threat.GainSquares)
            {
                var testBoard = board.PlaceStone(gx, gy, threat.Owner);
                int count = CountInDirection(testBoard, gx, gy, threat.Direction, threat.Owner);
                if (count <= 5)
                {
                    // At least one gain square doesn't create overline, so threat is valid
                    return false;
                }
            }
            return true; // All gain squares create overlines
        }

        return false; // At least one gain square creates a valid win
    }

    private bool IsSandwichedThreat(Threat threat, Board board)
    {
        if (threat.Type != ThreatType.StraightFour || threat.StonePositions.Count < 4)
            return false;

        var (dx, dy) = threat.Direction;
        var first = threat.StonePositions[0];
        var last = threat.StonePositions[^1];

        // Check if both ends are blocked
        bool blockedStart = !IsValidPosition(first.x - dx, first.y - dy, BitBoard.Size) ||
                           (!board.GetCell(first.x - dx, first.y - dy).IsEmpty &&
                            board.GetCell(first.x - dx, first.y - dy).Player != threat.Owner);

        bool blockedEnd = !IsValidPosition(last.x + dx, last.y + dy, BitBoard.Size) ||
                         (!board.GetCell(last.x + dx, last.y + dy).IsEmpty &&
                          board.GetCell(last.x + dx, last.y + dy).Player != threat.Owner);

        return blockedStart && blockedEnd;
    }

    private int CountInDirection(Board board, int startX, int startY, (int dx, int dy) dir, Player player)
    {
        int count = 1;
        var (dx, dy) = dir;

        // Count forward
        int x = startX + dx, y = startY + dy;
        while (IsValidPosition(x, y, BitBoard.Size) && board.GetCell(x, y).Player == player)
        {
            count++;
            x += dx;
            y += dy;
        }

        // Count backward
        x = startX - dx;
        y = startY - dy;
        while (IsValidPosition(x, y, BitBoard.Size) && board.GetCell(x, y).Player == player)
        {
            count++;
            x -= dx;
            y -= dy;
        }

        return count;
    }

    private bool IsAdjacentToPlayer(Board board, int x, int y, Player player)
    {
        for (int dx = -1; dx <= 1; dx++)
        {
            for (int dy = -1; dy <= 1; dy++)
            {
                if (dx == 0 && dy == 0) continue;
                int nx = x + dx, ny = y + dy;
                if (IsValidPosition(nx, ny, BitBoard.Size) && board.GetCell(nx, ny).Player == player)
                    return true;
            }
        }
        return false;
    }

    private bool IsValidPosition(int x, int y, int boardSize)
    {
        return PositionExtensions.InBounds(x, y, boardSize);
    }

    private int CreateThreatKey(Threat threat)
    {
        int hash = (int)threat.Type * 397;
        foreach (var (x, y) in threat.StonePositions)
        {
            hash = hash * 31 + x * 16 + y;
        }
        return hash;
    }
}
