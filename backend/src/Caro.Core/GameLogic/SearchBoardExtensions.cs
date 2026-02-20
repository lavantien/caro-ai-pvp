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
