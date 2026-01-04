using System.Runtime.CompilerServices;
using System.Numerics;

namespace Caro.Core.GameLogic;

/// <summary>
/// BitBoard representation for 15x15 Caro board
/// Layout: 4 ulongs (256 bits total) for 225 cells
///   _bits0: Rows 0-3    (bits 0-59 used)
///   _bits1: Rows 4-7    (bits 0-59 used)
///   _bits2: Rows 8-11   (bits 0-59 used)
///   _bits3: Rows 12-14  (bits 0-44 used)
/// Bit mapping: bit_index = row * 15 + col
/// </summary>
public struct BitBoard
{
    // 15x15 board = 225 cells
    // Each ulong holds 64 bits, so we need 4 ulongs
    // Only first 60 bits of first 3 ulongs are used (4 rows * 15 cols = 60)
    // Only first 45 bits of last ulong is used (3 rows * 15 cols = 45)
    private ulong _bits0;
    private ulong _bits1;
    private ulong _bits2;
    private ulong _bits3;

    /// <summary>
    /// Board size (15x15)
    /// </summary>
    public const int Size = 15;

    /// <summary>
    /// Total number of cells
    /// </summary>
    public const int TotalCells = Size * Size; // 225

    /// <summary>
    /// Create an empty BitBoard
    /// </summary>
    public BitBoard()
    {
        _bits0 = 0;
        _bits1 = 0;
        _bits2 = 0;
        _bits3 = 0;
    }

    /// <summary>
    /// Create BitBoard from existing ulong values
    /// </summary>
    public BitBoard(ulong bits0, ulong bits1, ulong bits2, ulong bits3)
    {
        _bits0 = bits0;
        _bits1 = bits1;
        _bits2 = bits2;
        _bits3 = bits3;
    }

    /// <summary>
    /// Get or set bit at position (x, y)
    /// </summary>
    public bool this[int x, int y]
    {
        get => GetBit(x, y);
        set => SetBit(x, y, value);
    }

    /// <summary>
    /// Get bit at position (x, y)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public bool GetBit(int x, int y)
    {
        int index = y * Size + x;
        int ulongIndex = index / 64;
        int bitIndex = index % 64;

        return ulongIndex switch
        {
            0 => (_bits0 & (1UL << bitIndex)) != 0,
            1 => (_bits1 & (1UL << bitIndex)) != 0,
            2 => (_bits2 & (1UL << bitIndex)) != 0,
            3 => (_bits3 & (1UL << bitIndex)) != 0,
            _ => false
        };
    }

    /// <summary>
    /// Set bit at position (x, y)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public void SetBit(int x, int y, bool value)
    {
        int index = y * Size + x;
        int ulongIndex = index / 64;
        int bitIndex = index % 64;

        ulong mask = 1UL << bitIndex;

        switch (ulongIndex)
        {
            case 0:
                if (value)
                    _bits0 |= mask;
                else
                    _bits0 &= ~mask;
                break;
            case 1:
                if (value)
                    _bits1 |= mask;
                else
                    _bits1 &= ~mask;
                break;
            case 2:
                if (value)
                    _bits2 |= mask;
                else
                    _bits2 &= ~mask;
                break;
            case 3:
                if (value)
                    _bits3 |= mask;
                else
                    _bits3 &= ~mask;
                break;
        }
    }

    /// <summary>
    /// Set bit at position (x, y) to 1
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public void SetBit(int x, int y)
    {
        int index = y * Size + x;
        int ulongIndex = index / 64;
        int bitIndex = index % 64;

        switch (ulongIndex)
        {
            case 0:
                _bits0 |= 1UL << bitIndex;
                break;
            case 1:
                _bits1 |= 1UL << bitIndex;
                break;
            case 2:
                _bits2 |= 1UL << bitIndex;
                break;
            case 3:
                _bits3 |= 1UL << bitIndex;
                break;
        }
    }

    /// <summary>
    /// Clear bit at position (x, y)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public void ClearBit(int x, int y)
    {
        int index = y * Size + x;
        int ulongIndex = index / 64;
        int bitIndex = index % 64;

        switch (ulongIndex)
        {
            case 0:
                _bits0 &= ~(1UL << bitIndex);
                break;
            case 1:
                _bits1 &= ~(1UL << bitIndex);
                break;
            case 2:
                _bits2 &= ~(1UL << bitIndex);
                break;
            case 3:
                _bits3 &= ~(1UL << bitIndex);
                break;
        }
    }

    /// <summary>
    /// Toggle bit at position (x, y)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public void ToggleBit(int x, int y)
    {
        int index = y * Size + x;
        int ulongIndex = index / 64;
        int bitIndex = index % 64;

        switch (ulongIndex)
        {
            case 0:
                _bits0 ^= 1UL << bitIndex;
                break;
            case 1:
                _bits1 ^= 1UL << bitIndex;
                break;
            case 2:
                _bits2 ^= 1UL << bitIndex;
                break;
            case 3:
                _bits3 ^= 1UL << bitIndex;
                break;
        }
    }

    /// <summary>
    /// Count total set bits (population count)
    /// Uses hardware POPCNT if available
    /// </summary>
    public int CountBits()
    {
        return BitOperations.PopCount(_bits0) +
               BitOperations.PopCount(_bits1) +
               BitOperations.PopCount(_bits2) +
               BitOperations.PopCount(_bits3);
    }

    /// <summary>
    /// Check if board is empty (no bits set)
    /// </summary>
    public bool IsEmpty => _bits0 == 0 && _bits1 == 0 && _bits2 == 0 && _bits3 == 0;

    /// <summary>
    /// Bitwise OR operation
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static BitBoard operator |(BitBoard left, BitBoard right)
    {
        return new BitBoard(
            left._bits0 | right._bits0,
            left._bits1 | right._bits1,
            left._bits2 | right._bits2,
            left._bits3 | right._bits3
        );
    }

    /// <summary>
    /// Bitwise AND operation
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static BitBoard operator &(BitBoard left, BitBoard right)
    {
        return new BitBoard(
            left._bits0 & right._bits0,
            left._bits1 & right._bits1,
            left._bits2 & right._bits2,
            left._bits3 & right._bits3
        );
    }

    /// <summary>
    /// Bitwise XOR operation
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static BitBoard operator ^(BitBoard left, BitBoard right)
    {
        return new BitBoard(
            left._bits0 ^ right._bits0,
            left._bits1 ^ right._bits1,
            left._bits2 ^ right._bits2,
            left._bits3 ^ right._bits3
        );
    }

    /// <summary>
    /// Bitwise complement operation
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static BitBoard operator ~(BitBoard board)
    {
        // Only mask the used bits (225 bits total)
        // _bits0-2: 60 bits each (4 rows * 15 cols)
        // _bits3: 45 bits (3 rows * 15 cols)
        return new BitBoard(
            ~board._bits0 & 0x0FFFFFFFFFFFFFFFUL,  // 60 bits
            ~board._bits1 & 0x0FFFFFFFFFFFFFFFUL,  // 60 bits
            ~board._bits2 & 0x0FFFFFFFFFFFFFFFUL,  // 60 bits
            ~board._bits3 & 0x000007FFFFFFFFFFUL   // 45 bits
        );
    }

    /// <summary>
    /// Check if two BitBoards are equal
    /// </summary>
    public override bool Equals(object? obj)
    {
        if (obj is not BitBoard other)
            return false;

        return _bits0 == other._bits0 &&
               _bits1 == other._bits1 &&
               _bits2 == other._bits2 &&
               _bits3 == other._bits3;
    }

    /// <summary>
    /// Check equality between two BitBoards
    /// </summary>
    public bool Equals(BitBoard other)
    {
        return _bits0 == other._bits0 &&
               _bits1 == other._bits1 &&
               _bits2 == other._bits2 &&
               _bits3 == other._bits3;
    }

    /// <summary>
    /// Get hash code
    /// </summary>
    public override int GetHashCode()
    {
        return HashCode.Combine(_bits0, _bits1, _bits2, _bits3);
    }

    /// <summary>
    /// Convert to string representation (for debugging)
    /// </summary>
    public override string ToString()
    {
        var sb = new System.Text.StringBuilder();
        sb.AppendLine("BitBoard (15x15):");
        for (int y = 0; y < Size; y++)
        {
            for (int x = 0; x < Size; x++)
            {
                sb.Append(GetBit(x, y) ? '1' : '.');
            }
            sb.AppendLine();
        }
        return sb.ToString();
    }

    /// <summary>
    /// Get raw ulong values (for serialization/hashing)
    /// </summary>
    public (ulong b0, ulong b1, ulong b2, ulong b3) GetRawValues() =>
        (_bits0, _bits1, _bits2, _bits3);

    /// <summary>
    /// Create BitBoard from raw ulong values
    /// </summary>
    public static BitBoard FromRawValues(ulong b0, ulong b1, ulong b2, ulong b3) =>
        new BitBoard(b0, b1, b2, b3);

    /// <summary>
    /// Get all set positions as a list
    /// </summary>
    public List<(int x, int y)> GetSetPositions()
    {
        var positions = new List<(int, int)>();
        for (int y = 0; y < Size; y++)
        {
            for (int x = 0; x < Size; x++)
            {
                if (GetBit(x, y))
                    positions.Add((x, y));
            }
        }
        return positions;
    }

    /// <summary>
    /// Shift all bits in a direction
    /// Useful for pattern matching and threat detection
    /// Optimized with bit-level operations instead of loops
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public BitBoard ShiftLeft()
    {
        // Horizontal shift left: bits move to lower x values
        // x=0 bits are removed (can't shift left from x=0)
        // index = y * 15 + x, calculate bitIndex = index % 64
        // y=0: 0->bit0, y=1: 15->bit15, y=2: 30->bit30, y=3: 45->bit45, y=4: 60->bit60 (all in _bits0)
        // y=5: 75->bit11, y=6: 90->bit26, y=7: 105->bit41, y=8: 120->bit56 (all in _bits1)
        // y=9: 135->bit7, y=10: 150->bit22, y=11: 165->bit37, y=12: 180->bit52 (all in _bits2)
        // y=13: 195->bit3, y=14: 210->bit18 (both in _bits3)

        var result = new BitBoard();

        // _bits0: clear bits 0,15,30,45,60 then shift right
        ulong bits0Cleared = _bits0 & ~(1UL << 0) & ~(1UL << 15) & ~(1UL << 30) & ~(1UL << 45) & ~(1UL << 60);
        result._bits0 = bits0Cleared >> 1;

        // _bits1: clear bits 11,26,41,56 then shift right
        ulong bits1Cleared = _bits1 & ~(1UL << 11) & ~(1UL << 26) & ~(1UL << 41) & ~(1UL << 56);
        result._bits1 = bits1Cleared >> 1;

        // _bits2: clear bits 7,22,37,52 then shift right
        ulong bits2Cleared = _bits2 & ~(1UL << 7) & ~(1UL << 22) & ~(1UL << 37) & ~(1UL << 52);
        result._bits2 = bits2Cleared >> 1;

        // _bits3: clear bits 3,18 then shift right
        ulong bits3Cleared = _bits3 & ~(1UL << 3) & ~(1UL << 18);
        result._bits3 = bits3Cleared >> 1;

        return result;
    }

    /// <summary>
    /// Shift all bits right
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public BitBoard ShiftRight()
    {
        // Horizontal shift right: bits move to higher x values
        // x=14 bits are removed (can't shift right from x=14)
        // index = y * 15 + 14, calculate bitIndex = index % 64
        // y=0: 14->bit14, y=1: 29->bit29, y=2: 44->bit44, y=3: 59->bit59 (all in _bits0)
        // y=4: 74->bit10, y=5: 89->bit25, y=6: 104->bit40, y=7: 119->bit55 (all in _bits1)
        // y=8: 134->bit6, y=9: 149->bit21, y=10: 164->bit36, y=11: 179->bit51 (all in _bits2)
        // y=12: 194->bit2, y=13: 209->bit17, y=14: 224->bit32 (all in _bits3)

        var result = new BitBoard();

        // _bits0: clear bits 14,29,44,59 then shift left
        ulong bits0Cleared = _bits0 & ~(1UL << 14) & ~(1UL << 29) & ~(1UL << 44) & ~(1UL << 59);
        result._bits0 = (bits0Cleared << 1) & 0x0FFFFFFFFFFFFFFFUL;

        // _bits1: clear bits 10,25,40,55 then shift left
        ulong bits1Cleared = _bits1 & ~(1UL << 10) & ~(1UL << 25) & ~(1UL << 40) & ~(1UL << 55);
        result._bits1 = (bits1Cleared << 1) & 0x0FFFFFFFFFFFFFFFUL;

        // _bits2: clear bits 6,21,36,51 then shift left
        ulong bits2Cleared = _bits2 & ~(1UL << 6) & ~(1UL << 21) & ~(1UL << 36) & ~(1UL << 51);
        result._bits2 = (bits2Cleared << 1) & 0x0FFFFFFFFFFFFFFFUL;

        // _bits3: clear bits 2,17,32 then shift left
        ulong bits3Cleared = _bits3 & ~(1UL << 2) & ~(1UL << 17) & ~(1UL << 32);
        result._bits3 = (bits3Cleared << 1) & 0x000007FFFFFFFFFFUL;

        return result;
    }

    /// <summary>
    /// Shift all bits up (decrease y)
    /// Optimized for 15x15 board with 15-bit rows
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public BitBoard ShiftUp()
    {
        // Vertical shift up: each row moves to previous row position
        // Row 0 bits disappear, Row 1 -> Row 0, etc.

        var result = new BitBoard();

        // _bits0: rows 0-3 (60 bits used)
        // After shift up: row 1->0, 2->1, 3->2, row 3 comes from _bits1 row 4
        result._bits0 = (_bits0 >> 15) |  // Shift rows 1-3 to 0-2
                       ((_bits1 & 0x7FFFUL) << 45);  // Row 4 (bits 0-14 of _bits1) -> row 3

        // _bits1: rows 4-7 (60 bits used)
        result._bits1 = (_bits1 >> 15) |  // Shift rows 5-7 to 4-6
                       ((_bits2 & 0x7FFFUL) << 45);  // Row 8 -> row 7

        // _bits2: rows 8-11 (60 bits used)
        result._bits2 = (_bits2 >> 15) |  // Shift rows 9-11 to 8-10
                       ((_bits3 & 0x7FFFUL) << 45);  // Row 12 -> row 11

        // _bits3: rows 12-14 (45 bits used)
        result._bits3 = (_bits3 >> 15);  // Shift rows 13-14 to 12-13

        return result;
    }

    /// <summary>
    /// Shift all bits down (increase y)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public BitBoard ShiftDown()
    {
        // Vertical shift down: each row moves to next row position

        var result = new BitBoard();

        // _bits0: rows 0-3
        // Row 0 disappears, rows 1-3 shift down, row 3 gets row 4 from _bits1
        result._bits0 = ((_bits0 & 0x0FFFFFFFFFFFFFFFUL) << 15) |  // Shift rows 0-2 to 1-3
                       ((_bits1 >> 45) & 0x7FFFUL);  // Row 4 -> row 0

        // _bits1: rows 4-7
        result._bits1 = ((_bits1 & 0x0FFFFFFFFFFFFFFFUL) << 15) |
                       ((_bits2 >> 45) & 0x7FFFUL);

        // _bits2: rows 8-11
        result._bits2 = ((_bits2 & 0x0FFFFFFFFFFFFFFFUL) << 15) |
                       ((_bits3 >> 45) & 0x7FFFUL);

        // _bits3: rows 12-14
        result._bits3 = ((_bits3 & 0x000003FFFFFFFFFFUL) << 15);

        return result;
    }

    /// <summary>
    /// Shift diagonally up-left
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public BitBoard ShiftUpLeft()
    {
        return ShiftUp().ShiftLeft();
    }

    /// <summary>
    /// Shift diagonally up-right
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public BitBoard ShiftUpRight()
    {
        return ShiftUp().ShiftRight();
    }

    /// <summary>
    /// Shift diagonally down-left
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public BitBoard ShiftDownLeft()
    {
        return ShiftDown().ShiftLeft();
    }

    /// <summary>
    /// Shift diagonally down-right
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public BitBoard ShiftDownRight()
    {
        return ShiftDown().ShiftRight();
    }
}
