using System.Numerics;
using System.Runtime.CompilerServices;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Extension methods for SearchBoard to support integration with existing search infrastructure.
/// </summary>
public static class SearchBoardExtensions
{
    /// <summary>
    /// Get candidate moves within a radius of existing stones.
    /// Optimized for SearchBoard using bitboard operations.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static List<(int x, int y)> GetCandidateMoves(this SearchBoard board, int radius = 2)
    {
        var candidates = new List<(int x, int y)>();
        var occupancy = board.GetOccupancy();
        int size = board.BoardSize;

        // Use bitboard to find occupied cells and expand around them
        for (int x = 0; x < size; x++)
        {
            for (int y = 0; y < size; y++)
            {
                if (board.IsEmpty(x, y))
                {
                    // Check if within radius of any occupied cell
                    if (IsNearOccupied(x, y, occupancy, radius, size))
                    {
                        candidates.Add((x, y));
                    }
                }
            }
        }

        return candidates;
    }

    /// <summary>
    /// Zero-allocation version: Get candidate moves within a radius of existing stones.
    /// Writes to a caller-provided buffer and returns the count.
    /// This is the preferred API for hot paths to avoid GC pressure.
    /// </summary>
    /// <param name="board">The board to search</param>
    /// <param name="buffer">Pre-allocated buffer to write moves to (should be at least 256 for 16x16 board)</param>
    /// <param name="radius">Radius around occupied cells to consider</param>
    /// <returns>Number of candidate moves written to buffer</returns>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int GetCandidateMovesZeroAlloc(this SearchBoard board, Span<(int x, int y)> buffer, int radius = 2)
    {
        var occupancy = board.GetOccupancy();
        int size = board.BoardSize;
        int count = 0;

        // Use bitboard to find occupied cells and expand around them
        for (int x = 0; x < size && count < buffer.Length; x++)
        {
            for (int y = 0; y < size && count < buffer.Length; y++)
            {
                if (board.IsEmpty(x, y))
                {
                    // Check if within radius of any occupied cell
                    if (IsNearOccupied(x, y, occupancy, radius, size))
                    {
                        buffer[count++] = (x, y);
                    }
                }
            }
        }

        return count;
    }

    /// <summary>
    /// Bitwise version: Get candidate moves using bitwise dilation and iteration.
    /// Uses BitOperations.TrailingZeroCount for efficient iteration over set bits.
    /// This is the most efficient version for 16x16 boards.
    /// </summary>
    /// <param name="board">The board to search</param>
    /// <param name="buffer">Pre-allocated buffer (should be at least 256)</param>
    /// <param name="radius">Dilation radius (default 2)</param>
    /// <returns>Number of candidate moves written to buffer</returns>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int GetCandidateMovesBitwise(this SearchBoard board, Span<(int x, int y)> buffer, int radius = 2)
    {
        var occupancy = board.GetOccupancy();
        int size = board.BoardSize;
        int count = 0;

        // Dilate the occupancy bitboard by the radius
        var dilated = DilateBitboard(occupancy, radius, size);

        // Remove the original occupancy (we want empty cells only)
        var candidates = dilated & ~occupancy;

        // Get raw values and iterate over set bits
        var (b0, b1, b2, b3) = candidates.GetRawValues();

        count = ExtractBits(b0, 0, buffer, count);
        count = ExtractBits(b1, 64, buffer, count);
        count = ExtractBits(b2, 128, buffer, count);
        count = ExtractBits(b3, 192, buffer, count);

        return count;
    }

    /// <summary>
    /// Dilate a bitboard by expanding around set bits.
    /// Uses bitwise shift operations for O(1) dilation per direction.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static BitBoard DilateBitboard(BitBoard bits, int radius, int size)
    {
        var result = bits;

        // Dilate in all 8 directions for each radius step
        for (int r = 0; r < radius; r++)
        {
            var h1 = result.ShiftLeft() | result.ShiftRight();
            var v1 = result.ShiftUp() | result.ShiftDown();
            var d1 = result.ShiftUpLeft() | result.ShiftUpRight() | result.ShiftDownLeft() | result.ShiftDownRight();
            result = result | h1 | v1 | d1;
        }

        return result;
    }

    /// <summary>
    /// Extract bit positions from a ulong and write to buffer as (x, y) coordinates.
    /// Uses BitOperations.TrailingZeroCount for efficient iteration.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static int ExtractBits(ulong bits, int offset, Span<(int x, int y)> buffer, int count)
    {
        while (bits != 0 && count < buffer.Length)
        {
            int bitIndex = BitOperations.TrailingZeroCount(bits);
            int cellIndex = offset + bitIndex;
            buffer[count++] = (cellIndex % 16, cellIndex / 16);
            bits &= bits - 1; // Clear lowest set bit
        }
        return count;
    }

    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static bool IsNearOccupied(int x, int y, BitBoard occupancy, int radius, int size)
    {
        int minX = Math.Max(0, x - radius);
        int maxX = Math.Min(size - 1, x + radius);
        int minY = Math.Max(0, y - radius);
        int maxY = Math.Min(size - 1, y + radius);

        for (int ox = minX; ox <= maxX; ox++)
        {
            for (int oy = minY; oy <= maxY; oy++)
            {
                if (occupancy.GetBit(ox, oy))
                {
                    return true;
                }
            }
        }
        return false;
    }

    /// <summary>
    /// Check if the current player has won.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static bool HasWin(this SearchBoard board, Player player)
    {
        var bits = board.GetBitBoard(player);
        return CheckFiveInRow(bits);
    }

    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static bool CheckFiveInRow(BitBoard bits)
    {
        // Check horizontal
        var h1 = bits;
        var h2 = h1.ShiftRight();
        var h3 = h2.ShiftRight();
        var h4 = h3.ShiftRight();
        if ((h1 & h2 & h3 & h4).IsEmpty == false)
            return true;

        // Check vertical
        var v1 = bits;
        var v2 = v1.ShiftDown();
        var v3 = v2.ShiftDown();
        var v4 = v3.ShiftDown();
        if ((v1 & v2 & v3 & v4).IsEmpty == false)
            return true;

        // Check diagonal \
        var d1 = bits;
        var d2 = d1.ShiftDownRight();
        var d3 = d2.ShiftDownRight();
        var d4 = d3.ShiftDownRight();
        if ((d1 & d2 & d3 & d4).IsEmpty == false)
            return true;

        // Check diagonal /
        var a1 = bits;
        var a2 = a1.ShiftDownLeft();
        var a3 = a2.ShiftDownLeft();
        var a4 = a3.ShiftDownLeft();
        if ((a1 & a2 & a3 & a4).IsEmpty == false)
            return true;

        return false;
    }

    /// <summary>
    /// Check if placing at (x, y) would win for the player.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static bool IsWinningMove(this SearchBoard board, int x, int y, Player player)
    {
        var undo = board.MakeMove(x, y, player);
        bool wins = board.HasWin(player);
        board.UnmakeMove(undo);
        return wins;
    }

    /// <summary>
    /// Count stones for a player.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int StoneCount(this SearchBoard board, Player player)
    {
        if (player == Player.None)
            return 0;
        return board.GetBitBoard(player).CountBits();
    }

    /// <summary>
    /// Get all positions occupied by a player.
    /// </summary>
    public static IEnumerable<(int x, int y)> GetOccupiedPositions(this SearchBoard board, Player player)
    {
        var bits = board.GetBitBoard(player);
        int size = board.BoardSize;

        for (int x = 0; x < size; x++)
        {
            for (int y = 0; y < size; y++)
            {
                if (bits.GetBit(x, y))
                {
                    yield return (x, y);
                }
            }
        }
    }
}
