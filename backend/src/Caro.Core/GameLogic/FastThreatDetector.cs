using System.Runtime.CompilerServices;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Fast threat detection using Directional Context LUT
/// Provides O(1) threat queries with 4 table lookups per position
///
/// Key features:
/// - Branchless O(1) threat evaluation
/// - Automatic overline detection (6+ stones)
/// - Automatic sandwiched win detection (OXXXXXO)
/// - Handles the oxxxx_x pattern correctly (not a threat)
/// </summary>
public sealed class FastThreatDetector
{
    private readonly DirectionalThreatLUT _lut;

    public FastThreatDetector()
    {
        _lut = new DirectionalThreatLUT();
    }

    /// <summary>
    /// Get threat score for a position (0-100, or -1 for invalid)
    /// This is O(1) with 4 table lookups (one per direction)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public int GetThreatScore(ulong myStones, ulong oppStones, int x, int y)
    {
        // Check all 4 directions
        int h = _lut.GetThreatScore(LineExtractor.ExtractHorizontalKey(myStones, oppStones, x, y));
        if (h == DirectionalThreatLUT.WinningMove) return DirectionalThreatLUT.WinningMove;
        if (h == -1) return -1; // Invalid move

        int v = _lut.GetThreatScore(LineExtractor.ExtractVerticalKey(myStones, oppStones, x, y));
        if (v == DirectionalThreatLUT.WinningMove) return DirectionalThreatLUT.WinningMove;
        if (v == -1) return -1;

        int dd = _lut.GetThreatScore(LineExtractor.ExtractDiagonalDownKey(myStones, oppStones, x, y));
        if (dd == DirectionalThreatLUT.WinningMove) return DirectionalThreatLUT.WinningMove;
        if (dd == -1) return -1;

        int du = _lut.GetThreatScore(LineExtractor.ExtractDiagonalUpKey(myStones, oppStones, x, y));
        if (du == DirectionalThreatLUT.WinningMove) return DirectionalThreatLUT.WinningMove;
        if (du == -1) return -1;

        // Return maximum threat across all directions
        return Math.Max(Math.Max(h, v), Math.Max(dd, du));
    }

    /// <summary>
    /// Get threat score for a position from Board entity
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public int GetThreatScore(Board board, int x, int y, Player player)
    {
        var (myStones, oppStones) = ExtractBitBoards(board, player);
        return GetThreatScore(myStones, oppStones, x, y);
    }

    /// <summary>
    /// Check if a move would be a winning move
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public bool IsWinningMove(ulong myStones, ulong oppStones, int x, int y)
    {
        return GetThreatScore(myStones, oppStones, x, y) == DirectionalThreatLUT.WinningMove;
    }

    /// <summary>
    /// Check if a move would be a winning move (Board entity version)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public bool IsWinningMove(Board board, int x, int y, Player player)
    {
        var (myStones, oppStones) = ExtractBitBoards(board, player);
        return IsWinningMove(myStones, oppStones, x, y);
    }

    /// <summary>
    /// Check if a move would be invalid (overline/sandwiched win)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public bool IsInvalidMove(ulong myStones, ulong oppStones, int x, int y)
    {
        return GetThreatScore(myStones, oppStones, x, y) == -1;
    }

    /// <summary>
    /// Check if a move would be invalid (Board entity version)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public bool IsInvalidMove(Board board, int x, int y, Player player)
    {
        var (myStones, oppStones) = ExtractBitBoards(board, player);
        return IsInvalidMove(myStones, oppStones, x, y);
    }

    /// <summary>
    /// Get detailed threat info for all 4 directions
    /// Useful for debugging and move ordering
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public ThreatInfo GetThreatInfo(ulong myStones, ulong oppStones, int x, int y)
    {
        var (h, v, dd, du) = LineExtractor.ExtractAllKeys(myStones, oppStones, x, y);

        return new ThreatInfo
        {
            HorizontalScore = _lut.GetThreatScore(h),
            VerticalScore = _lut.GetThreatScore(v),
            DiagonalDownScore = _lut.GetThreatScore(dd),
            DiagonalUpScore = _lut.GetThreatScore(du),
            MaxScore = Math.Max(
                Math.Max(_lut.GetThreatScore(h), _lut.GetThreatScore(v)),
                Math.Max(_lut.GetThreatScore(dd), _lut.GetThreatScore(du))
            ),
            IsWinningMove = _lut.IsWinningMove(h) || _lut.IsWinningMove(v) ||
                           _lut.IsWinningMove(dd) || _lut.IsWinningMove(du),
            IsInvalidMove = _lut.IsInvalidMove(h) || _lut.IsInvalidMove(v) ||
                           _lut.IsInvalidMove(dd) || _lut.IsInvalidMove(du)
        };
    }

    /// <summary>
    /// Get detailed threat info (Board entity version)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public ThreatInfo GetThreatInfo(Board board, int x, int y, Player player)
    {
        var (myStones, oppStones) = ExtractBitBoards(board, player);
        return GetThreatInfo(myStones, oppStones, x, y);
    }

    /// <summary>
    /// Extract bitboards for a player from Board entity
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static (ulong myStones, ulong oppStones) ExtractBitBoards(Board board, Player player)
    {
        ulong myStones = 0;
        ulong oppStones = 0;
        Player opponent = player == Player.Red ? Player.Blue : Player.Red;

        for (int y = 0; y < 15; y++)
        {
            for (int x = 0; x < 15; x++)
            {
                var cell = board.GetCell(x, y);
                int idx = y * 15 + x;
                ulong mask = 1UL << idx;

                if (cell.Player == player)
                    myStones |= mask;
                else if (cell.Player == opponent)
                    oppStones |= mask;
            }
        }

        return (myStones, oppStones);
    }

    /// <summary>
    /// Scan board and find all threats for a player
    /// Useful for move ordering and threat space search
    /// </summary>
    public List<ThreatMove> FindAllThreats(Board board, Player player, byte minThreatLevel = DirectionalThreatLUT.WeakThreat)
    {
        var threats = new List<ThreatMove>();
        var (myStones, oppStones) = ExtractBitBoards(board, player);

        for (int y = 0; y < 15; y++)
        {
            for (int x = 0; x < 15; x++)
            {
                // Skip occupied cells
                int idx = y * 15 + x;
                ulong mask = 1UL << idx;
                if (((myStones | oppStones) & mask) != 0)
                    continue;

                int score = GetThreatScore(myStones, oppStones, x, y);
                if (score >= minThreatLevel)
                {
                    threats.Add(new ThreatMove
                    {
                        X = x,
                        Y = y,
                        ThreatScore = (byte)score,
                        Player = player
                    });
                }
            }
        }

        // Sort by threat score descending
        threats.Sort((a, b) => b.ThreatScore.CompareTo(a.ThreatScore));
        return threats;
    }
}

/// <summary>
/// Detailed threat information for a single position
/// </summary>
public struct ThreatInfo
{
    public int HorizontalScore;
    public int VerticalScore;
    public int DiagonalDownScore;
    public int DiagonalUpScore;
    public int MaxScore;
    public bool IsWinningMove;
    public bool IsInvalidMove;

    public readonly override string ToString()
    {
        return $"H:{HorizontalScore} V:{VerticalScore} DD:{DiagonalDownScore} DU:{DiagonalUpScore} Max:{MaxScore} Win:{IsWinningMove} Invalid:{IsInvalidMove}";
    }
}

/// <summary>
/// Represents a threatening move on the board
/// </summary>
public class ThreatMove
{
    public int X { get; init; }
    public int Y { get; init; }
    public byte ThreatScore { get; init; }
    public Player Player { get; init; }
}
