using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Threat detection engine for VCF (Victory by Continuous Four) solver
/// Detects forcing patterns: Straight Four, Broken Four, Straight Three, Broken Three
/// All threats respect Caro rules: no overline (6+), no sandwiched wins (OXXXXXXO)
///
/// Supports both traditional scanning and BitKey-based O(1) pattern lookup.
/// </summary>
public partial class ThreatDetector
{
    private static readonly (int dx, int dy)[] Directions = GameConstants.CardinalDirections;

    private readonly WinDetector _winDetector = new();

    /// <summary>
    /// Detect all threats for the given player on the board
    /// </summary>
    public List<Threat> DetectThreats(Board board, Player player)
    {
        var threats = new List<Threat>();
        var seen = new HashSet<int>();

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

                // Check if there's a stone after this empty (potential gap)
                int nx = x + dx, ny = y + dy;
                if (IsValidPosition(nx, ny, BitBoard.Size) && board.GetCell(nx, ny).Player == player)
                {
                    // There's a stone after this empty - this is a gap
                    // CRITICAL FIX: If we already have 4 consecutive stones (no gaps yet),
                    // stop here to preserve the StraightFour pattern.
                    // Example: XXXX_ X should detect XXXX as StraightFour, not skip it.
                    // But if we have fewer than 4 stones, continue scanning for BrokenFour.
                    if (stones.Count >= 4 && gaps.Count == 0)
                    {
                        // We have 4 consecutive stones - stop to preserve StraightFour
                        // Don't add this as a gap since we're not including stones after
                        break;
                    }
                    // Otherwise, add as a gap and continue scanning
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
