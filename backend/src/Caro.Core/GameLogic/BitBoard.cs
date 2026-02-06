using System.Runtime.CompilerServices;
using System.Numerics;

namespace Caro.Core.GameLogic;

/// <summary>
/// BitBoard representation for 19x19 Caro board
/// Layout: 6 ulongs (384 bits total) for 361 cells
/// Bit mapping: bit_index = y * 19 + x
/// </summary>
public struct BitBoard
{
    // 19x19 board = 361 cells
    // Each ulong holds 64 bits, so we need 6 ulongs
    // Last ulong (_bits5) uses only 41 bits (361 - 320 = 41)
    private ulong _bits0;
    private ulong _bits1;
    private ulong _bits2;
    private ulong _bits3;
    private ulong _bits4;
    private ulong _bits5;

    /// <summary>
    /// Board size (19x19)
    /// </summary>
    public const int Size = 19;

    /// <summary>
    /// Total number of cells
    /// </summary>
    public const int TotalCells = Size * Size; // 361

    /// <summary>
    /// Create an empty BitBoard
    /// </summary>
    public BitBoard()
    {
        _bits0 = 0;
        _bits1 = 0;
        _bits2 = 0;
        _bits3 = 0;
        _bits4 = 0;
        _bits5 = 0;
    }

    /// <summary>
    /// Create BitBoard from existing ulong values
    /// </summary>
    public BitBoard(ulong bits0, ulong bits1, ulong bits2, ulong bits3, ulong bits4, ulong bits5)
    {
        _bits0 = bits0;
        _bits1 = bits1;
        _bits2 = bits2;
        _bits3 = bits3;
        _bits4 = bits4;
        _bits5 = bits5;
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
        int ulongIndex = index >> 6; // index / 64
        int bitIndex = index & 0x3F; // index % 64

        return ulongIndex switch
        {
            0 => (_bits0 & (1UL << bitIndex)) != 0,
            1 => (_bits1 & (1UL << bitIndex)) != 0,
            2 => (_bits2 & (1UL << bitIndex)) != 0,
            3 => (_bits3 & (1UL << bitIndex)) != 0,
            4 => (_bits4 & (1UL << bitIndex)) != 0,
            5 => (_bits5 & (1UL << bitIndex)) != 0,
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
        int ulongIndex = index >> 6;
        int bitIndex = index & 0x3F;

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
            case 4:
                if (value)
                    _bits4 |= mask;
                else
                    _bits4 &= ~mask;
                break;
            case 5:
                if (value)
                    _bits5 |= mask;
                else
                    _bits5 &= ~mask;
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
        int ulongIndex = index >> 6;
        int bitIndex = index & 0x3F;

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
            case 4:
                _bits4 |= 1UL << bitIndex;
                break;
            case 5:
                _bits5 |= 1UL << bitIndex;
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
        int ulongIndex = index >> 6;
        int bitIndex = index & 0x3F;

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
            case 4:
                _bits4 &= ~(1UL << bitIndex);
                break;
            case 5:
                _bits5 &= ~(1UL << bitIndex);
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
        int ulongIndex = index >> 6;
        int bitIndex = index & 0x3F;

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
            case 4:
                _bits4 ^= 1UL << bitIndex;
                break;
            case 5:
                _bits5 ^= 1UL << bitIndex;
                break;
        }
    }

    /// <summary>
    /// Count total set bits (population count)
    /// </summary>
    public int CountBits()
    {
        return BitOperations.PopCount(_bits0) +
               BitOperations.PopCount(_bits1) +
               BitOperations.PopCount(_bits2) +
               BitOperations.PopCount(_bits3) +
               BitOperations.PopCount(_bits4) +
               BitOperations.PopCount(_bits5);
    }

    /// <summary>
    /// Check if board is empty (no bits set)
    /// </summary>
    public bool IsEmpty => _bits0 == 0 && _bits1 == 0 && _bits2 == 0 && _bits3 == 0 && _bits4 == 0 && _bits5 == 0;

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
            left._bits3 | right._bits3,
            left._bits4 | right._bits4,
            left._bits5 | right._bits5
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
            left._bits3 & right._bits3,
            left._bits4 & right._bits4,
            left._bits5 & right._bits5
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
            left._bits3 ^ right._bits3,
            left._bits4 ^ right._bits4,
            left._bits5 ^ right._bits5
        );
    }

    /// <summary>
    /// Bitwise complement operation
    /// Only masks the used bits (361 bits total, last ulong uses 41 bits)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static BitBoard operator ~(BitBoard board)
    {
        // _bits0-4: all 64 bits used
        // _bits5: only 41 bits used (361 - 320)
        return new BitBoard(
            ~board._bits0,
            ~board._bits1,
            ~board._bits2,
            ~board._bits3,
            ~board._bits4,
            ~board._bits5 & 0x000001FFFFFFFFFFUL  // 41 bits
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
               _bits3 == other._bits3 &&
               _bits4 == other._bits4 &&
               _bits5 == other._bits5;
    }

    /// <summary>
    /// Check equality between two BitBoards
    /// </summary>
    public bool Equals(BitBoard other)
    {
        return _bits0 == other._bits0 &&
               _bits1 == other._bits1 &&
               _bits2 == other._bits2 &&
               _bits3 == other._bits3 &&
               _bits4 == other._bits4 &&
               _bits5 == other._bits5;
    }

    /// <summary>
    /// Get hash code
    /// </summary>
    public override int GetHashCode()
    {
        return HashCode.Combine(_bits0, _bits1, _bits2, _bits3, _bits4, _bits5);
    }

    /// <summary>
    /// Convert to string representation (for debugging)
    /// </summary>
    public override string ToString()
    {
        var sb = new System.Text.StringBuilder();
        sb.AppendLine("BitBoard (19x19):");
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
    public (ulong b0, ulong b1, ulong b2, ulong b3, ulong b4, ulong b5) GetRawValues() =>
        (_bits0, _bits1, _bits2, _bits3, _bits4, _bits5);

    /// <summary>
    /// Create BitBoard from raw ulong values
    /// </summary>
    public static BitBoard FromRawValues(ulong b0, ulong b1, ulong b2, ulong b3, ulong b4, ulong b5) =>
        new BitBoard(b0, b1, b2, b3, b4, b5);

    /// <summary>
    /// Copy bits from another BitBoard
    /// </summary>
    public void CopyFrom(BitBoard source)
    {
        _bits0 = source._bits0;
        _bits1 = source._bits1;
        _bits2 = source._bits2;
        _bits3 = source._bits3;
        _bits4 = source._bits4;
        _bits5 = source._bits5;
    }

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
    /// Shift all bits left (decrease x)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public BitBoard ShiftLeft()
    {
        var result = new BitBoard();
        result._bits0 = (_bits0 >> 1) & 0x7FFFFFFFFFFFFFFFUL; // Clear bit 63
        result._bits1 = ((_bits0 & 1UL) << 63) | (_bits1 >> 1);
        result._bits2 = ((_bits1 & 1UL) << 63) | (_bits2 >> 1);
        result._bits3 = ((_bits2 & 1UL) << 63) | (_bits3 >> 1);
        result._bits4 = ((_bits3 & 1UL) << 63) | (_bits4 >> 1);
        result._bits5 = ((_bits4 & 1UL) << 63) | (_bits5 >> 1);
        return result;
    }

    /// <summary>
    /// Shift all bits right (increase x)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public BitBoard ShiftRight()
    {
        var result = new BitBoard();
        result._bits5 = (_bits5 << 1) & 0x000001FFFFFFFFFFUL; // Only 41 bits
        result._bits4 = ((_bits5 >> 40) << 63) | ((_bits4 << 1) & 0xFFFFFFFFFFFFFFFFUL);
        result._bits3 = ((_bits4 >> 63) << 63) | ((_bits3 << 1) & 0xFFFFFFFFFFFFFFFFUL);
        result._bits2 = ((_bits3 >> 63) << 63) | ((_bits2 << 1) & 0xFFFFFFFFFFFFFFFFUL);
        result._bits1 = ((_bits2 >> 63) << 63) | ((_bits1 << 1) & 0xFFFFFFFFFFFFFFFFUL);
        result._bits0 = ((_bits1 >> 63) << 63) | ((_bits0 << 1) & 0xFFFFFFFFFFFFFFFFUL);
        return result;
    }

    /// <summary>
    /// Shift all bits up (decrease y)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public BitBoard ShiftUp()
    {
        // Each row has 19 bits
        // Shifting up means moving bits from higher rows to lower rows
        var result = new BitBoard();

        // _bits0: gets bits shifted from _bits0 (rows above) and from _bits1
        result._bits0 = (_bits0 >> 19) | ((_bits1 & 0x7FFFFUL) << 45);
        result._bits1 = ((_bits1 >> 19) | ((_bits2 & 0x7FFFFUL) << 45)) & 0xFFFFFFFFFFFFFFFFUL;
        result._bits2 = ((_bits2 >> 19) | ((_bits3 & 0x7FFFFUL) << 45)) & 0xFFFFFFFFFFFFFFFFUL;
        result._bits3 = ((_bits3 >> 19) | ((_bits4 & 0x7FFFFUL) << 45)) & 0xFFFFFFFFFFFFFFFFUL;
        result._bits4 = ((_bits4 >> 19) | ((_bits5 & 0x7FFFFUL) << 45)) & 0xFFFFFFFFFFFFFFFFUL;
        result._bits5 = (_bits5 >> 19) & 0x000001FFFFFFFFFFUL;

        return result;
    }

    /// <summary>
    /// Shift all bits down (increase y)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public BitBoard ShiftDown()
    {
        // Each row has 19 bits
        // Shifting down means moving bits from lower rows to higher rows
        var result = new BitBoard();

        result._bits0 = ((_bits0 << 19) & 0xFFFFFFFFFFFFFFFFUL) | ((_bits1 >> 45) & 0x7FFFFUL);
        result._bits1 = ((_bits1 << 19) & 0xFFFFFFFFFFFFFFFFUL) | ((_bits2 >> 45) & 0x7FFFFUL);
        result._bits2 = ((_bits2 << 19) & 0xFFFFFFFFFFFFFFFFUL) | ((_bits3 >> 45) & 0x7FFFFUL);
        result._bits3 = ((_bits3 << 19) & 0xFFFFFFFFFFFFFFFFUL) | ((_bits4 >> 45) & 0x7FFFFUL);
        result._bits4 = ((_bits4 << 19) & 0xFFFFFFFFFFFFFFFFUL) | ((_bits5 >> 45) & 0x7FFFFUL);
        result._bits5 = ((_bits5 << 19) & 0x000001FFFFFFFFFFUL);

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
