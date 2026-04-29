using System.Runtime.CompilerServices;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Extension methods that add AI technical concerns (BitBoard, hashing) to the pure domain Board.
/// </summary>
public static class BoardExtensions
{
    /// <summary>
    /// Get total cells on the board.
    /// </summary>
    public static int TotalCells(this Board board) => board.BoardSize * board.BoardSize;

    /// <summary>
    /// Get total stones placed on the board.
    /// </summary>
    public static int TotalStones(this Board board) => board.Cells.Count(c => !c.IsEmpty);

    /// <summary>
    /// Get occupied cells as enumerable of (x, y) tuples.
    /// </summary>
    public static IEnumerable<(int x, int y)> GetOccupiedCells(this Board board)
    {
        foreach (var cell in board.Cells)
        {
            if (!cell.IsEmpty)
                yield return (cell.X, cell.Y);
        }
    }

    /// <summary>
    /// Get occupied cells for a specific player as enumerable of (x, y) tuples.
    /// </summary>
    public static IEnumerable<(int x, int y)> GetOccupiedCells(this Board board, Player player)
    {
        foreach (var cell in board.Cells)
        {
            if (cell.Player == player)
                yield return (cell.X, cell.Y);
        }
    }

    /// <summary>
    /// Get the BitBoard representation for Red stones.
    /// PERFORMANCE: O(1) - uses pre-computed bitboards from Board class.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static BitBoard GetRedBitBoard(this Board board)
    {
        var bits = board.GetBitBoardBits(Player.Red);
        return new BitBoard(bits[0], bits[1], bits[2], bits[3]);
    }

    /// <summary>
    /// Get the BitBoard representation for Blue stones.
    /// PERFORMANCE: O(1) - uses pre-computed bitboards from Board class.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static BitBoard GetBlueBitBoard(this Board board)
    {
        var bits = board.GetBitBoardBits(Player.Blue);
        return new BitBoard(bits[0], bits[1], bits[2], bits[3]);
    }

    /// <summary>
    /// Get the BitBoard for a specific player.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static BitBoard GetBitBoard(this Board board, Player player) =>
        player == Player.Red ? board.GetRedBitBoard() : board.GetBlueBitBoard();

    /// <summary>
    /// Get the Zobrist hash of the board position.
    /// PERFORMANCE: O(1) - uses pre-computed hash from Board class.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static ulong GetHash(this Board board)
    {
        return board.GetHash();
    }

}
