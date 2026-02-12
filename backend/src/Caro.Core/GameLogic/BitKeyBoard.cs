using System.Numerics;
using System.Runtime.CompilerServices;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// BitKey-based board representation for O(1) pattern lookup.
/// Based on Rapfi's BitKey pattern system - uses 64-bit keys with 2 bits per cell
/// and bit rotation to align patterns around the position being evaluated.
///
/// Each cell is encoded with 2 bits:
/// - 00 = empty
/// - 01 = Red
/// - 10 = Blue
/// - 11 = unused/invalid
/// </summary>
public sealed class BitKeyBoard
{
    private const int BoardSize = 32;
    private const int HalfLineLen = 6;  // Half line length for pattern extraction (12-cell window)

    // Four directional bitkeys (64-bit each)
    // Each array element holds one row/column/diagonal encoded with 2 bits per cell
    private readonly ulong[] _bitKey0;  // Horizontal (32 rows)
    private readonly ulong[] _bitKey1;  // Vertical (32 columns)
    private readonly ulong[] _bitKey2;  // Diagonal (63 diagonals)
    private readonly ulong[] _bitKey3;  // Anti-diagonal (63 diagonals)

    /// <summary>
    /// Initialize an empty BitKeyBoard.
    /// </summary>
    public BitKeyBoard()
    {
        _bitKey0 = new ulong[BoardSize];
        _bitKey1 = new ulong[BoardSize];
        _bitKey2 = new ulong[BoardSize * 2 - 1];
        _bitKey3 = new ulong[BoardSize * 2 - 1];
    }

    /// <summary>
    /// Initialize from an existing Board.
    /// </summary>
    public BitKeyBoard(Board board) : this()
    {
        for (int y = 0; y < BoardSize; y++)
        {
            for (int x = 0; x < BoardSize; x++)
            {
                var cell = board.GetCell(x, y);
                if (cell.Player != Player.None)
                {
                    SetBit(x, y, cell.Player);
                }
            }
        }
    }

    /// <summary>
    /// Initialize from BitBoards.
    /// </summary>
    public BitKeyBoard(BitBoard redBoard, BitBoard blueBoard) : this()
    {
        for (int y = 0; y < BoardSize; y++)
        {
            for (int x = 0; x < BoardSize; x++)
            {
                if (redBoard.GetBit(x, y))
                    SetBit(x, y, Player.Red);
                else if (blueBoard.GetBit(x, y))
                    SetBit(x, y, Player.Blue);
            }
        }
    }

    /// <summary>
    /// Set a stone at position (x, y) for the given player.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public void SetBit(int x, int y, Player player)
    {
        if (x < 0 || x >= BoardSize || y < 0 || y >= BoardSize)
            return;

        // Encode player as 2-bit value
        // Red = 01 (value 1), Blue = 10 (value 2)
        ulong bits = player == Player.Red ? 1UL : 2UL;

        // Update all four directional keys

        // Horizontal (bitKey0[y]): position at 2*x
        int hPos = x * 2;
        _bitKey0[y] |= bits << hPos;

        // Vertical (bitKey1[x]): position at 2*y
        int vPos = y * 2;
        _bitKey1[x] |= bits << vPos;

        // Diagonal (bitKey2[x + y]): position at 2*x
        int dIndex = x + y;
        _bitKey2[dIndex] |= bits << hPos;

        // Anti-diagonal (bitKey3[BoardSize - 1 - x + y]): position at 2*x
        int adIndex = BoardSize - 1 - x + y;
        _bitKey3[adIndex] |= bits << hPos;
    }

    /// <summary>
    /// Clear a stone at position (x, y).
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public void ClearBit(int x, int y)
    {
        if (x < 0 || x >= BoardSize || y < 0 || y >= BoardSize)
            return;

        // Clear 2 bits at position
        ulong mask = 0x3UL;  // 11 in binary

        int hPos = x * 2;
        int vPos = y * 2;
        int dIndex = x + y;
        int adIndex = BoardSize - 1 - x + y;

        _bitKey0[y] &= ~(mask << hPos);
        _bitKey1[x] &= ~(mask << vPos);
        _bitKey2[dIndex] &= ~(mask << hPos);
        _bitKey3[adIndex] &= ~(mask << hPos);
    }

    /// <summary>
    /// Get the 64-bit key at position (x, y) for the given direction.
    /// The key is rotated to center the position in the pattern window.
    /// </summary>
    /// <param name="x">X coordinate</param>
    /// <param name="y">Y coordinate</param>
    /// <param name="direction">0=Horizontal, 1=Vertical, 2=Diagonal, 3=Anti-diagonal</param>
    /// <returns>64-bit pattern key centered at the position</returns>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public ulong GetKeyAt(int x, int y, int direction)
    {
        if (x < 0 || x >= BoardSize || y < 0 || y >= BoardSize)
            return 0;

        return direction switch
        {
            0 => BitOperations.RotateRight(_bitKey0[y], 2 * (x - HalfLineLen)),
            1 => BitOperations.RotateRight(_bitKey1[x], 2 * (y - HalfLineLen)),
            2 => BitOperations.RotateRight(_bitKey2[x + y], 2 * (x - HalfLineLen)),
            3 => BitOperations.RotateRight(_bitKey3[BoardSize - 1 - x + y], 2 * (x - HalfLineLen)),
            _ => 0
        };
    }

    /// <summary>
    /// Get all four directional keys at position (x, y).
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public (ulong Horizontal, ulong Vertical, ulong Diagonal, ulong AntiDiagonal) GetAllKeysAt(int x, int y)
    {
        return (
            GetKeyAt(x, y, 0),
            GetKeyAt(x, y, 1),
            GetKeyAt(x, y, 2),
            GetKeyAt(x, y, 3)
        );
    }

    /// <summary>
    /// Get the raw bitkey for a horizontal row (no rotation).
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public ulong GetHorizontalKey(int y) => _bitKey0[y];

    /// <summary>
    /// Get the raw bitkey for a vertical column (no rotation).
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public ulong GetVerticalKey(int x) => _bitKey1[x];

    /// <summary>
    /// Get the raw bitkey for a diagonal (no rotation).
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public ulong GetDiagonalKey(int index) => _bitKey2[index];

    /// <summary>
    /// Get the raw bitkey for an anti-diagonal (no rotation).
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public ulong GetAntiDiagonalKey(int index) => _bitKey3[index];

    /// <summary>
    /// Get the player at position (x, y) from the BitKey encoding.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public Player GetPlayerAt(int x, int y)
    {
        if (x < 0 || x >= BoardSize || y < 0 || y >= BoardSize)
            return Player.None;

        int hPos = x * 2;
        ulong bits = (_bitKey0[y] >> hPos) & 0x3UL;

        return bits switch
        {
            1 => Player.Red,
            2 => Player.Blue,
            _ => Player.None
        };
    }

    /// <summary>
    /// Create a deep copy of this BitKeyBoard.
    /// </summary>
    public BitKeyBoard Clone()
    {
        var clone = new BitKeyBoard();
        Array.Copy(_bitKey0, clone._bitKey0, _bitKey0.Length);
        Array.Copy(_bitKey1, clone._bitKey1, _bitKey1.Length);
        Array.Copy(_bitKey2, clone._bitKey2, _bitKey2.Length);
        Array.Copy(_bitKey3, clone._bitKey3, _bitKey3.Length);
        return clone;
    }

    /// <summary>
    /// Get the combined hash of all bitkeys for position comparison.
    /// </summary>
    public ulong GetHash()
    {
        ulong hash = 0;
        for (int i = 0; i < BoardSize; i++)
        {
            hash ^= _bitKey0[i] << (i % 32);
            hash ^= _bitKey1[i] << ((i + 16) % 32);
        }
        for (int i = 0; i < _bitKey2.Length; i++)
        {
            hash ^= _bitKey2[i] ^ _bitKey3[i];
        }
        return hash;
    }

    /// <summary>
    /// Count total stones on the board.
    /// </summary>
    public int CountStones()
    {
        int count = 0;
        for (int i = 0; i < BoardSize; i++)
        {
            count += CountBitsInKey(_bitKey0[i]);
        }
        return count;
    }

    /// <summary>
    /// Count non-empty cells in a 64-bit key (2 bits per cell).
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static int CountBitsInKey(ulong key)
    {
        // Count how many 2-bit pairs are non-zero
        int count = 0;
        while (key != 0)
        {
            if ((key & 0x3UL) != 0)
                count++;
            key >>= 2;
        }
        return count;
    }

    /// <summary>
    /// Clear all stones from the board.
    /// </summary>
    public void Clear()
    {
        Array.Clear(_bitKey0);
        Array.Clear(_bitKey1);
        Array.Clear(_bitKey2);
        Array.Clear(_bitKey3);
    }

    /// <summary>
    /// Get all positions that have a stone.
    /// </summary>
    public List<(int x, int y)> GetOccupiedPositions()
    {
        var positions = new List<(int x, int y)>();
        for (int y = 0; y < BoardSize; y++)
        {
            for (int x = 0; x < BoardSize; x++)
            {
                if (GetPlayerAt(x, y) != Player.None)
                {
                    positions.Add((x, y));
                }
            }
        }
        return positions;
    }
}
