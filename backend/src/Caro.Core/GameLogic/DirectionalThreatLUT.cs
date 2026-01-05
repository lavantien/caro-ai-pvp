using System.Runtime.CompilerServices;

namespace Caro.Core.GameLogic;

/// <summary>
/// Directional Context Look-Up Table for O(1) threat detection
/// Precomputes all valid 9-cell line segments for Caro rules (exactly 5 wins)
///
/// Key advantages:
/// - O(1) lookup with no branching
/// - Handles overline (6+) automatically
/// - Handles sandwiched wins (OXXXXXO) automatically
/// - Fits entirely in CPU L2 cache
///
/// Based on tournament-winning Gomoku AI research by Tomek Czajka (CodeCup 2020)
/// </summary>
public sealed class DirectionalThreatLUT
{
    // 262,144 entries × 1 byte = 256 KB
    private readonly byte[] _threatTable;

    // Threat intensity values (stored in table)
    public const byte
        NoThreat = 0,
        WeakThreat = 10,      // Open 2
        MediumThreat = 25,    // Broken 3
        StrongThreat = 50,    // Open 3
        VeryStrongThreat = 75,// Semi-open 4
        WinningMove = 100,    // Open 4 or Exact 5
        InvalidWin = 255;     // Suicide/Overline - prune this branch

    /// <summary>
    /// Gets the table size (for testing)
    /// </summary>
    public int TableSize => _threatTable.Length;

    public DirectionalThreatLUT()
    {
        _threatTable = new byte[1 << 18]; // 2^18 entries
        InitializeTable();
    }

    /// <summary>
    /// Precompute all threat values at startup
    /// This runs once and creates the complete lookup table
    /// </summary>
    private void InitializeTable()
    {
        // Iterate through all possible 18-bit keys
        for (int key = 0; key < (1 << 18); key++)
        {
            _threatTable[key] = ComputeThreatValue(key);
        }
    }

    /// <summary>
    /// Compute threat value for a single 9-cell context
    /// </summary>
    private static byte ComputeThreatValue(int key)
    {
        // Decode 18-bit key into 9 cells (2 bits each)
        // Cell encoding: 00=Empty, 01=MyStone, 10=OpponentStone, 11=Wall
        Span<CellType> cells = stackalloc CellType[9];
        for (int i = 0; i < 9; i++)
        {
            int twoBits = (key >> (i * 2)) & 0b11;
            cells[i] = (CellType)twoBits;
        }

        // Center cell (index 4) is the candidate move - we're evaluating placing a stone here
        // The board state has center empty, and we check "what if I play here?"

        // First, check if center is occupied or wall - not a valid move
        if (cells[4] != CellType.Empty)
            return NoThreat; // Can't play here anyway

        // Count consecutive stones from center (excluding center itself which is empty)
        int countLeft = 0;
        int countRight = 0;

        // Count to the left (indices 3, 2, 1, 0)
        for (int i = 3; i >= 0; i--)
        {
            if (cells[i] == CellType.MyStone)
                countLeft++;
            else
                break;
        }

        // Count to the right (indices 5, 6, 7, 8)
        for (int i = 5; i < 9; i++)
        {
            if (cells[i] == CellType.MyStone)
                countRight++;
            else
                break;
        }

        int totalStonesIfPlayed = countLeft + countRight + 1; // +1 for center (our move)

        // Check for overline (6+ is NOT a win in Caro)
        if (totalStonesIfPlayed > 5)
            return InvalidWin;

        // Check for sandwiched 5-in-row (OXXXXXO pattern)
        // If we create exact 5, check if both ends are blocked by opponent
        if (totalStonesIfPlayed == 5)
        {
            // Check left side for opponent stone
            int leftBoundary = 4 - countLeft - 1;
            bool leftBlocked = leftBoundary >= 0 && cells[leftBoundary] == CellType.OpponentStone;

            // Check right side for opponent stone
            int rightBoundary = 4 + countRight + 1;
            bool rightBlocked = rightBoundary < 9 && cells[rightBoundary] == CellType.OpponentStone;

            if (leftBlocked && rightBlocked)
                return InvalidWin; // Sandwiched 5-in-row is not a win

            return WinningMove;
        }

        // 4 stones - check if we can complete to 5
        if (totalStonesIfPlayed == 4)
        {
            // Count ways to complete to 5
            int waysToComplete = 0;

            // Check left side
            int leftExtendPos = 4 - countLeft - 1;
            if (leftExtendPos >= 0 && cells[leftExtendPos] == CellType.Empty)
            {
                // Check if placing there would create 6 (overline)
                int leftOfExtend = leftExtendPos - 1;
                bool wouldOverline = leftOfExtend >= 0 && cells[leftOfExtend] == CellType.MyStone;
                if (!wouldOverline)
                    waysToComplete++;
            }

            // Check right side
            int rightExtendPos = 4 + countRight + 1;
            if (rightExtendPos < 9 && cells[rightExtendPos] == CellType.Empty)
            {
                // Check if placing there would create 6 (overline)
                int rightOfExtend = rightExtendPos + 1;
                bool wouldOverline = rightOfExtend < 9 && cells[rightOfExtend] == CellType.MyStone;
                if (!wouldOverline)
                    waysToComplete++;
            }

            return waysToComplete switch
            {
                2 => WinningMove,       // Open four (two ways to complete)
                1 => VeryStrongThreat,  // Semi-open four (one way to complete)
                0 => NoThreat,
                _ => NoThreat
            };
        }

        // 3 stones - check for three-in-a-row threats
        if (totalStonesIfPlayed == 3)
        {
            int waysToComplete = CountWaysToComplete(cells, countLeft, countRight, needed: 2);
            return waysToComplete switch
            {
                >= 4 => VeryStrongThreat,  // Open three with many completions
                >= 2 => StrongThreat,      // Open three
                1 => MediumThreat,         // Broken three
                0 => NoThreat,
                _ => NoThreat
            };
        }

        // 2 stones
        if (totalStonesIfPlayed == 2)
        {
            int waysToComplete = CountWaysToComplete(cells, countLeft, countRight, needed: 3);
            return waysToComplete >= 2 ? WeakThreat : NoThreat;
        }

        return NoThreat;
    }

    /// <summary>
    /// Count ways to complete a line to reach 5 stones
    /// </summary>
    private static int CountWaysToComplete(Span<CellType> cells, int countLeft, int countRight, int needed)
    {
        int ways = 0;
        int leftBoundary = 4 - countLeft;
        int rightBoundary = 4 + countRight;

        // Check completions on the left
        for (int i = leftBoundary - 1; i >= Math.Max(0, leftBoundary - needed); i--)
        {
            if (cells[i] == CellType.Empty)
            {
                // Check if we can complete through this path without creating overline
                bool canComplete = true;
                int stonesWouldAdd = 0;

                for (int j = i; j < leftBoundary; j++)
                {
                    if (cells[j] == CellType.Empty)
                        stonesWouldAdd++;
                    else if (cells[j] == CellType.OpponentStone)
                    {
                        canComplete = false;
                        break;
                    }
                }

                // Check for overline if we complete this way
                if (canComplete)
                {
                    int totalIfComplete = countLeft + countRight + 1 + stonesWouldAdd;
                    if (totalIfComplete <= 5 && !WouldCreateOverline(cells, i, leftBoundary, countLeft, countRight))
                        ways++;
                    else
                        break;
                }
                else
                    break;
            }
            else if (cells[i] == CellType.OpponentStone)
                break;
        }

        // Check completions on the right
        for (int i = rightBoundary + 1; i <= Math.Min(8, rightBoundary + needed); i++)
        {
            if (cells[i] == CellType.Empty)
            {
                bool canComplete = true;
                int stonesWouldAdd = 0;

                for (int j = rightBoundary + 1; j <= i; j++)
                {
                    if (cells[j] == CellType.Empty)
                        stonesWouldAdd++;
                    else if (cells[j] == CellType.OpponentStone)
                    {
                        canComplete = false;
                        break;
                    }
                }

                if (canComplete)
                {
                    int totalIfComplete = countLeft + countRight + 1 + stonesWouldAdd;
                    if (totalIfComplete <= 5 && !WouldCreateOverline(cells, rightBoundary + 1, i, countLeft, countRight))
                        ways++;
                    else
                        break;
                }
                else
                    break;
            }
            else if (cells[i] == CellType.OpponentStone)
                break;
        }

        return ways;
    }

    /// <summary>
    /// Check if completing through the given range would create overline (6+ stones)
    /// </summary>
    private static bool WouldCreateOverline(Span<CellType> cells, int fromPos, int toPos, int countLeft, int countRight)
    {
        // Check if there are stones immediately outside the completion range
        int checkLeft = fromPos - 1;
        if (checkLeft >= 0 && cells[checkLeft] == CellType.MyStone)
            return true;

        int checkRight = toPos + 1;
        if (checkRight < 9 && cells[checkRight] == CellType.MyStone)
            return true;

        return false;
    }

    /// <summary>
    /// Runtime check: O(1) lookup with no branching
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public byte GetThreatValue(int key)
    {
        return _threatTable[key & 0x3FFFF]; // Mask to 18 bits
    }

    /// <summary>
    /// Check if a position is a winning move (for quick termination)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public bool IsWinningMove(int key)
    {
        return _threatTable[key & 0x3FFFF] == WinningMove;
    }

    /// <summary>
    /// Check if a move is invalid (would create overline)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public bool IsInvalidMove(int key)
    {
        return _threatTable[key & 0x3FFFF] == InvalidWin;
    }

    /// <summary>
    /// Get threat score for a key (higher = more threatening)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public int GetThreatScore(int key)
    {
        byte value = _threatTable[key & 0x3FFFF];
        return value == InvalidWin ? -1 : value;
    }

    /// <summary>
    /// Cell type encoding for the 9-cell context
    /// </summary>
    private enum CellType : byte
    {
        Empty = 0b00,
        MyStone = 0b01,
        OpponentStone = 0b10,
        Wall = 0b11
    }
}
