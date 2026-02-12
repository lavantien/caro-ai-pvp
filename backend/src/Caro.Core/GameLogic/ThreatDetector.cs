using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Threat detection engine for VCF (Victory by Continuous Four) solver
/// Detects forcing patterns: Straight Four, Broken Four, Straight Three, Broken Three
/// All threats respect Caro rules: no overline (6+), no sandwiched wins (OXXXXXXO)
///
/// Supports both traditional scanning and BitKey-based O(1) pattern lookup.
/// </summary>
public class ThreatDetector
{
    private static readonly (int dx, int dy)[] Directions =
    {
        (1, 0),   // Horizontal
        (0, 1),   // Vertical
        (1, 1),   // Diagonal down-right
        (1, -1)   // Diagonal down-left
    };

    private readonly WinDetector _winDetector = new();

    #region BitKey-based Detection (O(1) pattern lookup)

    /// <summary>
    /// Fast threat detection using BitKey pattern system.
    /// Returns threats detected at a specific position using O(1) pattern lookup.
    /// </summary>
    public List<Threat> DetectThreatsAt(BitKeyBoard bitKeyBoard, int x, int y, Player player)
    {
        var threats = new List<Threat>();
        var (combined, threatCount) = BitKeyPatternTable.GetCombinedPattern(bitKeyBoard, x, y);

        if (combined == Pattern4Evaluator.CaroPattern4.None)
            return threats;

        // Create threat based on pattern type
        var threat = combined switch
        {
            Pattern4Evaluator.CaroPattern4.Exactly5 => CreateFiveThreat(x, y, player),
            Pattern4Evaluator.CaroPattern4.Flex4 => CreateOpenFourThreat(bitKeyBoard, x, y, player),
            Pattern4Evaluator.CaroPattern4.Block4 => CreateClosedFourThreat(bitKeyBoard, x, y, player),
            Pattern4Evaluator.CaroPattern4.Flex3 => CreateOpenThreeThreat(bitKeyBoard, x, y, player),
            Pattern4Evaluator.CaroPattern4.DoubleFlex3 => CreateDoubleThreeThreat(bitKeyBoard, x, y, player),
            Pattern4Evaluator.CaroPattern4.Flex4Flex3 => CreateDoubleThreat(bitKeyBoard, x, y, player),
            _ => null
        };

        if (threat != null)
            threats.Add(threat);

        return threats;
    }

    /// <summary>
    /// Check if a position is a winning move using BitKey O(1) lookup.
    /// </summary>
    public bool IsWinningMoveFast(BitKeyBoard bitKeyBoard, int x, int y, Player player)
    {
        return BitKeyPatternTable.IsWinningMove(bitKeyBoard, x, y, player);
    }

    /// <summary>
    /// Check if a position creates a double threat using BitKey O(1) lookup.
    /// </summary>
    public bool IsDoubleThreatFast(BitKeyBoard bitKeyBoard, int x, int y, Player player)
    {
        return BitKeyPatternTable.IsDoubleThreatMove(bitKeyBoard, x, y, player);
    }

    /// <summary>
    /// Get the pattern classification at a position using BitKey O(1) lookup.
    /// </summary>
    public (Pattern4Evaluator.CaroPattern4 Pattern, int ThreatCount) GetPatternAt(BitKeyBoard bitKeyBoard, int x, int y)
    {
        return BitKeyPatternTable.GetCombinedPattern(bitKeyBoard, x, y);
    }

    /// <summary>
    /// Evaluate a position using BitKey O(1) pattern scoring.
    /// </summary>
    public int EvaluatePosition(BitKeyBoard bitKeyBoard, int x, int y)
    {
        return BitKeyPatternTable.EvaluatePosition(bitKeyBoard, x, y);
    }

    /// <summary>
    /// Find all threat moves using BitKey fast pattern detection.
    /// </summary>
    public List<(int x, int y, int Score)> FindThreatMovesFast(Board board, Player player)
    {
        var threatMoves = new List<(int x, int y, int Score)>();
        var bitKeyBoard = new BitKeyBoard(board);

        for (int x = 0; x < BitBoard.Size; x++)
        {
            for (int y = 0; y < BitBoard.Size; y++)
            {
                if (!board.GetCell(x, y).IsEmpty)
                    continue;

                // Create test board with the move
                var testBoard = bitKeyBoard.Clone();
                testBoard.SetBit(x, y, player);

                var (pattern, threatCount) = BitKeyPatternTable.GetCombinedPattern(testBoard, x, y);

                if (pattern >= Pattern4Evaluator.CaroPattern4.Flex3)
                {
                    int score = BitKeyPatternTable.EvaluatePosition(testBoard, x, y);
                    threatMoves.Add((x, y, score));
                }
            }
        }

        // Sort by score descending
        threatMoves.Sort((a, b) => b.Score.CompareTo(a.Score));
        return threatMoves;
    }

    private Threat CreateFiveThreat(int x, int y, Player player)
    {
        return new Threat
        {
            Type = ThreatType.StraightFour,  // Five is the ultimate threat
            Owner = player,
            GainSquares = new List<(int x, int y)> { (x, y) },
            StonePositions = new List<(int x, int y)> { (x, y) },
            Direction = (1, 0)
        };
    }

    private Threat CreateOpenFourThreat(BitKeyBoard board, int x, int y, Player player)
    {
        var gainSquares = FindOpenFourGains(board, x, y, player);
        return new Threat
        {
            Type = ThreatType.StraightFour,
            Owner = player,
            GainSquares = gainSquares,
            StonePositions = new List<(int x, int y)> { (x, y) },
            Direction = (1, 0)  // Direction determined by pattern
        };
    }

    private Threat CreateClosedFourThreat(BitKeyBoard board, int x, int y, Player player)
    {
        var gainSquares = FindClosedFourGains(board, x, y, player);
        return new Threat
        {
            Type = ThreatType.BrokenFour,
            Owner = player,
            GainSquares = gainSquares,
            StonePositions = new List<(int x, int y)> { (x, y) },
            Direction = (1, 0)
        };
    }

    private Threat CreateOpenThreeThreat(BitKeyBoard board, int x, int y, Player player)
    {
        var gainSquares = FindOpenThreeGains(board, x, y, player);
        return new Threat
        {
            Type = ThreatType.StraightThree,
            Owner = player,
            GainSquares = gainSquares,
            StonePositions = new List<(int x, int y)> { (x, y) },
            Direction = (1, 0)
        };
    }

    private Threat CreateDoubleThreeThreat(BitKeyBoard board, int x, int y, Player player)
    {
        return new Threat
        {
            Type = ThreatType.StraightThree,  // Double three is very strong
            Owner = player,
            GainSquares = new List<(int x, int y)> { (x, y) },
            StonePositions = new List<(int x, int y)> { (x, y) },
            Direction = (1, 0)
        };
    }

    private Threat CreateDoubleThreat(BitKeyBoard board, int x, int y, Player player)
    {
        return new Threat
        {
            Type = ThreatType.StraightFour,  // Four + Three is winning
            Owner = player,
            GainSquares = new List<(int x, int y)> { (x, y) },
            StonePositions = new List<(int x, int y)> { (x, y) },
            Direction = (1, 0)
        };
    }

    private List<(int x, int y)> FindOpenFourGains(BitKeyBoard board, int x, int y, Player player)
    {
        var gains = new List<(int x, int y)>();
        // Find the two ends of the open four
        for (int dx = -1; dx <= 1; dx++)
        {
            for (int dy = -1; dy <= 1; dy++)
            {
                if (dx == 0 && dy == 0) continue;
                if (dx != 0 && dy != 0 && dx != dy && dx != -dy) continue;

                // Check both directions
                for (int dir = -1; dir <= 1; dir += 2)
                {
                    int nx = x + dx * dir, ny = y + dy * dir;
                    if (nx >= 0 && nx < 32 && ny >= 0 && ny < 32)
                    {
                        var cellPlayer = board.GetPlayerAt(nx, ny);
                        if (cellPlayer == Player.None)
                            gains.Add((nx, ny));
                    }
                }
            }
        }
        return gains.Distinct().ToList();
    }

    private List<(int x, int y)> FindClosedFourGains(BitKeyBoard board, int x, int y, Player player)
    {
        return FindOpenFourGains(board, x, y, player);
    }

    private List<(int x, int y)> FindOpenThreeGains(BitKeyBoard board, int x, int y, Player player)
    {
        return FindOpenFourGains(board, x, y, player);
    }

    #endregion

    /// <summary>
    /// Detect all threats for the given player on the board
    /// </summary>
    public List<Threat> DetectThreats(Board board, Player player)
    {
        var threats = new List<Threat>();
        var seen = new HashSet<string>();

        // Scan each cell as a potential starting point
        for (int x = 0; x < BitBoard.Size; x++)
        {
            for (int y = 0; y < BitBoard.Size; y++)
            {
                var cell = board.GetCell(x, y);
                if (cell.IsEmpty || cell.Player != player)
                    continue;

                foreach (var (dx, dy) in Directions)
                {
                    var threat = DetectThreatFromPosition(board, x, y, dx, dy, player);
                    if (threat != null && IsValidThreat(threat, board))
                    {
                        var key = CreateThreatKey(threat);
                        if (seen.Add(key))
                        {
                            threats.Add(threat);
                        }
                    }
                }
            }
        }

        // Sort by priority (descending)
        threats.Sort((a, b) => b.Priority.CompareTo(a.Priority));
        return threats;
    }

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
        var testBoard = board.PlaceStone(x, y, player);
        var winResult = _winDetector.CheckWin(testBoard);
        return winResult.HasWinner && winResult.Winner == player;
    }

    #region Private Methods

    private Threat? DetectThreatFromPosition(Board board, int startX, int startY, int dx, int dy, Player player)
    {
        // Scan the line in both directions to get complete pattern
        var line = ScanLine(board, startX, startY, dx, dy, player);
        if (line.Stones.Count < 3)
            return null;

        // Classify the pattern
        return ClassifyPattern(line, board, player, dx, dy);
    }

    private LineInfo ScanLine(Board board, int startX, int startY, int dx, int dy, Player player)
    {
        var stones = new List<(int x, int y)>();
        var empties = new List<(int x, int y)>();
        var gaps = new List<int>();  // Indices of empties that are gaps between stones

        // First, scan backward to find the start of the line
        int x = startX, y = startY;
        while (IsValidPosition(x - dx, y - dy, BitBoard.Size))
        {
            int bx = x - dx, by = y - dy;
            var cell = board.GetCell(bx, by);
            if (cell.Player == player)
            {
                x = bx;
                y = by;
            }
            else if (cell.IsEmpty)
            {
                // Check if there's a stone before this empty
                int bbx = bx - dx, bby = by - dy;
                if (IsValidPosition(bbx, bby, BitBoard.Size) && board.GetCell(bbx, bby).Player == player)
                {
                    // Empty with player stone on both sides = gap
                    empties.Insert(0, (bx, by));
                    gaps.Add(0);
                    // Adjust all existing gap indices
                    for (int i = 1; i < gaps.Count; i++)
                        gaps[i]++;
                }
                else
                {
                    // Empty at start
                    empties.Insert(0, (bx, by));
                }
                break;
            }
            else
            {
                // Blocked by opponent
                break;
            }
        }

        // Now scan forward from start, collecting all stones and empties
        while (IsValidPosition(x, y, BitBoard.Size))
        {
            var cell = board.GetCell(x, y);
            if (cell.Player == player)
            {
                stones.Add((x, y));
            }
            else if (cell.IsEmpty)
            {
                int emptyIdx = empties.Count;
                empties.Add((x, y));

                // Check if there's a stone after this empty (gap)
                int nx = x + dx, ny = y + dy;
                if (IsValidPosition(nx, ny, BitBoard.Size) && board.GetCell(nx, ny).Player == player)
                {
                    gaps.Add(emptyIdx);
                }
                else
                {
                    // Empty at end - no more stones
                    break;
                }
            }
            else
            {
                // Blocked by opponent
                break;
            }
            x += dx;
            y += dy;
        }

        return new LineInfo
        {
            Stones = stones,
            Empties = empties,
            GapIndices = gaps,
            OpenStart = empties.Count > 0 && IsEmptyInLine(board, empties[0].x - dx, empties[0].y - dy),
            OpenEnd = empties.Count > 0 && IsEmptyInLine(board, empties[^1].x + dx, empties[^1].y + dy),
            Dx = dx,
            Dy = dy
        };
    }

    private bool IsEmptyInLine(Board board, int x, int y)
    {
        return IsValidPosition(x, y, BitBoard.Size) && board.GetCell(x, y).IsEmpty;
    }

    private Threat? ClassifyPattern(LineInfo line, Board board, Player player, int dx, int dy)
    {
        int stoneCount = line.Stones.Count;
        int gapCount = line.GapIndices.Count;

        // Straight Four: XXXX_ (4 consecutive, at least one open end)
        // CRITICAL FIX: Add BOTH forward and backward gain squares
        // For open four (_XXXX or XXXX_), both ends need blocking
        // For semi-open four (OXXXX or XXXXO), only one end needs blocking
        if (stoneCount == 4 && gapCount == 0)
        {
            var gainSquares = new List<(int x, int y)>();

            // Add forward direction (after the last stone)
            var lastStone = line.Stones[^1];
            int gainX = lastStone.x + dx, gainY = lastStone.y + dy;
            if (IsValidPosition(gainX, gainY, BitBoard.Size) &&
                board.GetCell(gainX, gainY).IsEmpty)
            {
                gainSquares.Add((gainX, gainY));
            }

            // CRITICAL FIX: Also add backward direction (before the first stone)
            // This ensures both blocking squares are detected for an open four
            var firstStone = line.Stones[0];
            int backX = firstStone.x - dx, backY = firstStone.y - dy;
            if (IsValidPosition(backX, backY, BitBoard.Size) &&
                board.GetCell(backX, backY).IsEmpty)
            {
                gainSquares.Add((backX, backY));
            }

            return new Threat
            {
                Type = ThreatType.StraightFour,
                Owner = player,
                GainSquares = gainSquares,
                StonePositions = line.Stones,
                Direction = (dx, dy)
            };
        }

        // Broken Four: XXX_X (4 stones with 1 gap)
        if (stoneCount == 4 && gapCount == 1)
        {
            var gainSquares = new List<(int x, int y)>();

            // Add gap square (fills the gap to make 5)
            foreach (var gapIdx in line.GapIndices)
            {
                if (gapIdx < line.Empties.Count)
                {
                    gainSquares.Add(line.Empties[gapIdx]);
                }
            }

            // Also add open ends if they complete to 5
            foreach (var empty in line.Empties)
            {
                if (IsAdjacentToStoneLine(empty, line.Stones, dx, dy) && !gainSquares.Contains(empty))
                {
                    gainSquares.Add(empty);
                }
            }

            gainSquares = gainSquares.Distinct().ToList();

            return new Threat
            {
                Type = ThreatType.BrokenFour,
                Owner = player,
                GainSquares = gainSquares,
                StonePositions = line.Stones,
                Direction = (dx, dy)
            };
        }

        // Straight Three: XXX__ (3 consecutive, both ends open)
        if (stoneCount == 3 && gapCount == 0 && line.Empties.Count >= 2)
        {
            var gainSquares = new List<(int x, int y)>();

            foreach (var empty in line.Empties)
            {
                if (IsAdjacentToStoneLine(empty, line.Stones, dx, dy))
                {
                    gainSquares.Add(empty);
                }
            }

            return new Threat
            {
                Type = ThreatType.StraightThree,
                Owner = player,
                GainSquares = gainSquares,
                StonePositions = line.Stones,
                Direction = (dx, dy)
            };
        }

        // Broken Three: XX_X_ (3 stones with 1 gap)
        if (stoneCount == 3 && gapCount == 1)
        {
            var gainSquares = new List<(int x, int y)>();

            foreach (var gapIdx in line.GapIndices)
            {
                if (gapIdx < line.Empties.Count)
                {
                    gainSquares.Add(line.Empties[gapIdx]);
                }
            }

            return new Threat
            {
                Type = ThreatType.BrokenThree,
                Owner = player,
                GainSquares = gainSquares,
                StonePositions = line.Stones,
                Direction = (dx, dy)
            };
        }

        return null;
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
        foreach (var (gx, gy) in threat.GainSquares)
        {
            var testBoard = board.PlaceStone(gx, gy, threat.Owner);

            int count = CountInDirection(testBoard, gx, gy, threat.Direction, threat.Owner);

            if (count > 5)
                return true;
        }
        return false;
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
        return x >= 0 && x < boardSize && y >= 0 && y < boardSize;
    }

    private string CreateThreatKey(Threat threat)
    {
        return $"{threat.Type}:{string.Join(",", threat.StonePositions.Select(p => $"{p.x},{p.y}"))}";
    }

    #endregion

    private class LineInfo
    {
        public List<(int x, int y)> Stones { get; set; } = new();
        public List<(int x, int y)> Empties { get; set; } = new();
        public List<int> GapIndices { get; set; } = new();
        public bool OpenStart { get; set; }
        public bool OpenEnd { get; set; }
        public int Dx { get; set; }
        public int Dy { get; set; }
    }
}
