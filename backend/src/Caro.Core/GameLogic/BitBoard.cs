using System.Runtime.CompilerServices;
using System.Numerics;

namespace Caro.Core.GameLogic;

/// <summary>
/// BitBoard representation for 32x32 Caro board
/// Layout: 16 ulongs (1024 bits total) for 1024 cells
/// Bit mapping: bit_index = y * 32 + x
/// </summary>
public struct BitBoard
{
    // 32x32 board = 1024 cells
    // Each ulong holds 64 bits, so we need 16 ulongs
    private ulong _bits0;
    private ulong _bits1;
    private ulong _bits2;
    private ulong _bits3;
    private ulong _bits4;
    private ulong _bits5;
    private ulong _bits6;
    private ulong _bits7;
    private ulong _bits8;
    private ulong _bits9;
    private ulong _bits10;
    private ulong _bits11;
    private ulong _bits12;
    private ulong _bits13;
    private ulong _bits14;
    private ulong _bits15;

    /// <summary>
    /// Board size (32x32)
    /// </summary>
    public const int Size = 32;

    /// <summary>
    /// Total number of cells
    /// </summary>
    public const int TotalCells = Size * Size; // 1024

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
        _bits6 = 0;
        _bits7 = 0;
        _bits8 = 0;
        _bits9 = 0;
        _bits10 = 0;
        _bits11 = 0;
        _bits12 = 0;
        _bits13 = 0;
        _bits14 = 0;
        _bits15 = 0;
    }

    /// <summary>
    /// Create BitBoard from existing ulong values (16 ulongs for 32x32)
    /// </summary>
    public BitBoard(ulong bits0, ulong bits1, ulong bits2, ulong bits3, ulong bits4, ulong bits5,
                    ulong bits6, ulong bits7, ulong bits8, ulong bits9, ulong bits10, ulong bits11,
                    ulong bits12, ulong bits13, ulong bits14, ulong bits15)
    {
        _bits0 = bits0;
        _bits1 = bits1;
        _bits2 = bits2;
        _bits3 = bits3;
        _bits4 = bits4;
        _bits5 = bits5;
        _bits6 = bits6;
        _bits7 = bits7;
        _bits8 = bits8;
        _bits9 = bits9;
        _bits10 = bits10;
        _bits11 = bits11;
        _bits12 = bits12;
        _bits13 = bits13;
        _bits14 = bits14;
        _bits15 = bits15;
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
            6 => (_bits6 & (1UL << bitIndex)) != 0,
            7 => (_bits7 & (1UL << bitIndex)) != 0,
            8 => (_bits8 & (1UL << bitIndex)) != 0,
            9 => (_bits9 & (1UL << bitIndex)) != 0,
            10 => (_bits10 & (1UL << bitIndex)) != 0,
            11 => (_bits11 & (1UL << bitIndex)) != 0,
            12 => (_bits12 & (1UL << bitIndex)) != 0,
            13 => (_bits13 & (1UL << bitIndex)) != 0,
            14 => (_bits14 & (1UL << bitIndex)) != 0,
            15 => (_bits15 & (1UL << bitIndex)) != 0,
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
            case 0: if (value) _bits0 |= mask; else _bits0 &= ~mask; break;
            case 1: if (value) _bits1 |= mask; else _bits1 &= ~mask; break;
            case 2: if (value) _bits2 |= mask; else _bits2 &= ~mask; break;
            case 3: if (value) _bits3 |= mask; else _bits3 &= ~mask; break;
            case 4: if (value) _bits4 |= mask; else _bits4 &= ~mask; break;
            case 5: if (value) _bits5 |= mask; else _bits5 &= ~mask; break;
            case 6: if (value) _bits6 |= mask; else _bits6 &= ~mask; break;
            case 7: if (value) _bits7 |= mask; else _bits7 &= ~mask; break;
            case 8: if (value) _bits8 |= mask; else _bits8 &= ~mask; break;
            case 9: if (value) _bits9 |= mask; else _bits9 &= ~mask; break;
            case 10: if (value) _bits10 |= mask; else _bits10 &= ~mask; break;
            case 11: if (value) _bits11 |= mask; else _bits11 &= ~mask; break;
            case 12: if (value) _bits12 |= mask; else _bits12 &= ~mask; break;
            case 13: if (value) _bits13 |= mask; else _bits13 &= ~mask; break;
            case 14: if (value) _bits14 |= mask; else _bits14 &= ~mask; break;
            case 15: if (value) _bits15 |= mask; else _bits15 &= ~mask; break;
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
            case 0: _bits0 |= 1UL << bitIndex; break;
            case 1: _bits1 |= 1UL << bitIndex; break;
            case 2: _bits2 |= 1UL << bitIndex; break;
            case 3: _bits3 |= 1UL << bitIndex; break;
            case 4: _bits4 |= 1UL << bitIndex; break;
            case 5: _bits5 |= 1UL << bitIndex; break;
            case 6: _bits6 |= 1UL << bitIndex; break;
            case 7: _bits7 |= 1UL << bitIndex; break;
            case 8: _bits8 |= 1UL << bitIndex; break;
            case 9: _bits9 |= 1UL << bitIndex; break;
            case 10: _bits10 |= 1UL << bitIndex; break;
            case 11: _bits11 |= 1UL << bitIndex; break;
            case 12: _bits12 |= 1UL << bitIndex; break;
            case 13: _bits13 |= 1UL << bitIndex; break;
            case 14: _bits14 |= 1UL << bitIndex; break;
            case 15: _bits15 |= 1UL << bitIndex; break;
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
            case 0: _bits0 &= ~(1UL << bitIndex); break;
            case 1: _bits1 &= ~(1UL << bitIndex); break;
            case 2: _bits2 &= ~(1UL << bitIndex); break;
            case 3: _bits3 &= ~(1UL << bitIndex); break;
            case 4: _bits4 &= ~(1UL << bitIndex); break;
            case 5: _bits5 &= ~(1UL << bitIndex); break;
            case 6: _bits6 &= ~(1UL << bitIndex); break;
            case 7: _bits7 &= ~(1UL << bitIndex); break;
            case 8: _bits8 &= ~(1UL << bitIndex); break;
            case 9: _bits9 &= ~(1UL << bitIndex); break;
            case 10: _bits10 &= ~(1UL << bitIndex); break;
            case 11: _bits11 &= ~(1UL << bitIndex); break;
            case 12: _bits12 &= ~(1UL << bitIndex); break;
            case 13: _bits13 &= ~(1UL << bitIndex); break;
            case 14: _bits14 &= ~(1UL << bitIndex); break;
            case 15: _bits15 &= ~(1UL << bitIndex); break;
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
            case 0: _bits0 ^= 1UL << bitIndex; break;
            case 1: _bits1 ^= 1UL << bitIndex; break;
            case 2: _bits2 ^= 1UL << bitIndex; break;
            case 3: _bits3 ^= 1UL << bitIndex; break;
            case 4: _bits4 ^= 1UL << bitIndex; break;
            case 5: _bits5 ^= 1UL << bitIndex; break;
            case 6: _bits6 ^= 1UL << bitIndex; break;
            case 7: _bits7 ^= 1UL << bitIndex; break;
            case 8: _bits8 ^= 1UL << bitIndex; break;
            case 9: _bits9 ^= 1UL << bitIndex; break;
            case 10: _bits10 ^= 1UL << bitIndex; break;
            case 11: _bits11 ^= 1UL << bitIndex; break;
            case 12: _bits12 ^= 1UL << bitIndex; break;
            case 13: _bits13 ^= 1UL << bitIndex; break;
            case 14: _bits14 ^= 1UL << bitIndex; break;
            case 15: _bits15 ^= 1UL << bitIndex; break;
        }
    }

    /// <summary>
    /// Count total set bits (population count)
    /// </summary>
    public int CountBits()
    {
        return BitOperations.PopCount(_bits0) + BitOperations.PopCount(_bits1) +
               BitOperations.PopCount(_bits2) + BitOperations.PopCount(_bits3) +
               BitOperations.PopCount(_bits4) + BitOperations.PopCount(_bits5) +
               BitOperations.PopCount(_bits6) + BitOperations.PopCount(_bits7) +
               BitOperations.PopCount(_bits8) + BitOperations.PopCount(_bits9) +
               BitOperations.PopCount(_bits10) + BitOperations.PopCount(_bits11) +
               BitOperations.PopCount(_bits12) + BitOperations.PopCount(_bits13) +
               BitOperations.PopCount(_bits14) + BitOperations.PopCount(_bits15);
    }

    /// <summary>
    /// Check if board is empty (no bits set)
    /// </summary>
    public bool IsEmpty => _bits0 == 0 && _bits1 == 0 && _bits2 == 0 && _bits3 == 0 &&
                           _bits4 == 0 && _bits5 == 0 && _bits6 == 0 && _bits7 == 0 &&
                           _bits8 == 0 && _bits9 == 0 && _bits10 == 0 && _bits11 == 0 &&
                           _bits12 == 0 && _bits13 == 0 && _bits14 == 0 && _bits15 == 0;

    /// <summary>
    /// Bitwise OR operation
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static BitBoard operator |(BitBoard left, BitBoard right)
    {
        return new BitBoard(
            left._bits0 | right._bits0, left._bits1 | right._bits1,
            left._bits2 | right._bits2, left._bits3 | right._bits3,
            left._bits4 | right._bits4, left._bits5 | right._bits5,
            left._bits6 | right._bits6, left._bits7 | right._bits7,
            left._bits8 | right._bits8, left._bits9 | right._bits9,
            left._bits10 | right._bits10, left._bits11 | right._bits11,
            left._bits12 | right._bits12, left._bits13 | right._bits13,
            left._bits14 | right._bits14, left._bits15 | right._bits15
        );
    }

    /// <summary>
    /// Bitwise AND operation
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static BitBoard operator &(BitBoard left, BitBoard right)
    {
        return new BitBoard(
            left._bits0 & right._bits0, left._bits1 & right._bits1,
            left._bits2 & right._bits2, left._bits3 & right._bits3,
            left._bits4 & right._bits4, left._bits5 & right._bits5,
            left._bits6 & right._bits6, left._bits7 & right._bits7,
            left._bits8 & right._bits8, left._bits9 & right._bits9,
            left._bits10 & right._bits10, left._bits11 & right._bits11,
            left._bits12 & right._bits12, left._bits13 & right._bits13,
            left._bits14 & right._bits14, left._bits15 & right._bits15
        );
    }

    /// <summary>
    /// Bitwise XOR operation
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static BitBoard operator ^(BitBoard left, BitBoard right)
    {
        return new BitBoard(
            left._bits0 ^ right._bits0, left._bits1 ^ right._bits1,
            left._bits2 ^ right._bits2, left._bits3 ^ right._bits3,
            left._bits4 ^ right._bits4, left._bits5 ^ right._bits5,
            left._bits6 ^ right._bits6, left._bits7 ^ right._bits7,
            left._bits8 ^ right._bits8, left._bits9 ^ right._bits9,
            left._bits10 ^ right._bits10, left._bits11 ^ right._bits11,
            left._bits12 ^ right._bits12, left._bits13 ^ right._bits13,
            left._bits14 ^ right._bits14, left._bits15 ^ right._bits15
        );
    }

    /// <summary>
    /// Bitwise complement operation
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static BitBoard operator ~(BitBoard board)
    {
        return new BitBoard(
            ~board._bits0, ~board._bits1, ~board._bits2, ~board._bits3,
            ~board._bits4, ~board._bits5, ~board._bits6, ~board._bits7,
            ~board._bits8, ~board._bits9, ~board._bits10, ~board._bits11,
            ~board._bits12, ~board._bits13, ~board._bits14, ~board._bits15
        );
    }

    /// <summary>
    /// Check if two BitBoards are equal
    /// </summary>
    public override bool Equals(object? obj)
    {
        if (obj is not BitBoard other)
            return false;

        return _bits0 == other._bits0 && _bits1 == other._bits1 &&
               _bits2 == other._bits2 && _bits3 == other._bits3 &&
               _bits4 == other._bits4 && _bits5 == other._bits5 &&
               _bits6 == other._bits6 && _bits7 == other._bits7 &&
               _bits8 == other._bits8 && _bits9 == other._bits9 &&
               _bits10 == other._bits10 && _bits11 == other._bits11 &&
               _bits12 == other._bits12 && _bits13 == other._bits13 &&
               _bits14 == other._bits14 && _bits15 == other._bits15;
    }

    /// <summary>
    /// Check equality between two BitBoards
    /// </summary>
    public bool Equals(BitBoard other)
    {
        return _bits0 == other._bits0 && _bits1 == other._bits1 &&
               _bits2 == other._bits2 && _bits3 == other._bits3 &&
               _bits4 == other._bits4 && _bits5 == other._bits5 &&
               _bits6 == other._bits6 && _bits7 == other._bits7 &&
               _bits8 == other._bits8 && _bits9 == other._bits9 &&
               _bits10 == other._bits10 && _bits11 == other._bits11 &&
               _bits12 == other._bits12 && _bits13 == other._bits13 &&
               _bits14 == other._bits14 && _bits15 == other._bits15;
    }

    /// <summary>
    /// Get hash code
    /// </summary>
    public override int GetHashCode()
    {
        return HashCode.Combine(_bits0, _bits1, _bits2, _bits3,
                                _bits4, _bits5, _bits6, _bits7);
    }

    /// <summary>
    /// Convert to string representation (for debugging)
    /// </summary>
    public override string ToString()
    {
        var sb = new System.Text.StringBuilder();
        sb.AppendLine("BitBoard (32x32):");
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
    public (ulong b0, ulong b1, ulong b2, ulong b3, ulong b4, ulong b5,
            ulong b6, ulong b7, ulong b8, ulong b9, ulong b10, ulong b11,
            ulong b12, ulong b13, ulong b14, ulong b15) GetRawValues() =>
        (_bits0, _bits1, _bits2, _bits3, _bits4, _bits5,
         _bits6, _bits7, _bits8, _bits9, _bits10, _bits11,
         _bits12, _bits13, _bits14, _bits15);

    /// <summary>
    /// Create BitBoard from raw ulong values
    /// </summary>
    public static BitBoard FromRawValues(ulong b0, ulong b1, ulong b2, ulong b3, ulong b4, ulong b5,
                                         ulong b6, ulong b7, ulong b8, ulong b9, ulong b10, ulong b11,
                                         ulong b12, ulong b13, ulong b14, ulong b15) =>
        new BitBoard(b0, b1, b2, b3, b4, b5, b6, b7, b8, b9, b10, b11, b12, b13, b14, b15);

    /// <summary>
    /// Copy bits from another BitBoard
    /// </summary>
    public void CopyFrom(BitBoard source)
    {
        _bits0 = source._bits0; _bits1 = source._bits1;
        _bits2 = source._bits2; _bits3 = source._bits3;
        _bits4 = source._bits4; _bits5 = source._bits5;
        _bits6 = source._bits6; _bits7 = source._bits7;
        _bits8 = source._bits8; _bits9 = source._bits9;
        _bits10 = source._bits10; _bits11 = source._bits11;
        _bits12 = source._bits12; _bits13 = source._bits13;
        _bits14 = source._bits14; _bits15 = source._bits15;
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
    /// For 32x32 board, shift by 1 bit with wrap-around handling
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public BitBoard ShiftLeft()
    {
        var result = new BitBoard();
        result._bits0 = (_bits0 >> 1) | ((_bits1 & 1UL) << 63);
        result._bits1 = (_bits1 >> 1) | ((_bits2 & 1UL) << 63);
        result._bits2 = (_bits2 >> 1) | ((_bits3 & 1UL) << 63);
        result._bits3 = (_bits3 >> 1) | ((_bits4 & 1UL) << 63);
        result._bits4 = (_bits4 >> 1) | ((_bits5 & 1UL) << 63);
        result._bits5 = (_bits5 >> 1) | ((_bits6 & 1UL) << 63);
        result._bits6 = (_bits6 >> 1) | ((_bits7 & 1UL) << 63);
        result._bits7 = (_bits7 >> 1) | ((_bits8 & 1UL) << 63);
        result._bits8 = (_bits8 >> 1) | ((_bits9 & 1UL) << 63);
        result._bits9 = (_bits9 >> 1) | ((_bits10 & 1UL) << 63);
        result._bits10 = (_bits10 >> 1) | ((_bits11 & 1UL) << 63);
        result._bits11 = (_bits11 >> 1) | ((_bits12 & 1UL) << 63);
        result._bits12 = (_bits12 >> 1) | ((_bits13 & 1UL) << 63);
        result._bits13 = (_bits13 >> 1) | ((_bits14 & 1UL) << 63);
        result._bits14 = (_bits14 >> 1) | ((_bits15 & 1UL) << 63);
        result._bits15 = _bits15 >> 1;
        return result;
    }

    /// <summary>
    /// Shift all bits right (increase x)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public BitBoard ShiftRight()
    {
        var result = new BitBoard();
        result._bits15 = (_bits15 << 1);
        result._bits14 = (_bits14 << 1) | (_bits15 >> 63);
        result._bits13 = (_bits13 << 1) | (_bits14 >> 63);
        result._bits12 = (_bits12 << 1) | (_bits13 >> 63);
        result._bits11 = (_bits11 << 1) | (_bits12 >> 63);
        result._bits10 = (_bits10 << 1) | (_bits11 >> 63);
        result._bits9 = (_bits9 << 1) | (_bits10 >> 63);
        result._bits8 = (_bits8 << 1) | (_bits9 >> 63);
        result._bits7 = (_bits7 << 1) | (_bits8 >> 63);
        result._bits6 = (_bits6 << 1) | (_bits7 >> 63);
        result._bits5 = (_bits5 << 1) | (_bits6 >> 63);
        result._bits4 = (_bits4 << 1) | (_bits5 >> 63);
        result._bits3 = (_bits3 << 1) | (_bits4 >> 63);
        result._bits2 = (_bits2 << 1) | (_bits3 >> 63);
        result._bits1 = (_bits1 << 1) | (_bits2 >> 63);
        result._bits0 = (_bits0 << 1) | (_bits1 >> 63);
        return result;
    }

    /// <summary>
    /// Shift all bits up (decrease y) - shift by 32 bits (one row)
    /// For 32x32 board, each row is 32 bits, each ulong holds 2 rows
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public BitBoard ShiftUp()
    {
        var result = new BitBoard();
        // Shift up: y decreases, so bits move from higher index to lower index
        // Upper 32 bits of ulong[n] → Lower 32 bits of ulong[n]
        // Lower 32 bits of ulong[n+1] → Upper 32 bits of ulong[n]
        const ulong mask32 = 0xFFFFFFFF;
        result._bits0 = ((_bits0 >> 32)) | ((_bits1 & mask32) << 32);
        result._bits1 = ((_bits1 >> 32)) | ((_bits2 & mask32) << 32);
        result._bits2 = ((_bits2 >> 32)) | ((_bits3 & mask32) << 32);
        result._bits3 = ((_bits3 >> 32)) | ((_bits4 & mask32) << 32);
        result._bits4 = ((_bits4 >> 32)) | ((_bits5 & mask32) << 32);
        result._bits5 = ((_bits5 >> 32)) | ((_bits6 & mask32) << 32);
        result._bits6 = ((_bits6 >> 32)) | ((_bits7 & mask32) << 32);
        result._bits7 = ((_bits7 >> 32)) | ((_bits8 & mask32) << 32);
        result._bits8 = ((_bits8 >> 32)) | ((_bits9 & mask32) << 32);
        result._bits9 = ((_bits9 >> 32)) | ((_bits10 & mask32) << 32);
        result._bits10 = ((_bits10 >> 32)) | ((_bits11 & mask32) << 32);
        result._bits11 = ((_bits11 >> 32)) | ((_bits12 & mask32) << 32);
        result._bits12 = ((_bits12 >> 32)) | ((_bits13 & mask32) << 32);
        result._bits13 = ((_bits13 >> 32)) | ((_bits14 & mask32) << 32);
        result._bits14 = ((_bits14 >> 32)) | ((_bits15 & mask32) << 32);
        result._bits15 = _bits15 >> 32; // Top row comes from upper32 of bits15, bottom row empty
        return result;
    }

    /// <summary>
    /// Shift all bits down (increase y) - shift by 32 bits (one row)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public BitBoard ShiftDown()
    {
        var result = new BitBoard();
        // Shift down: y increases, so bits move from lower index to higher index
        // Lower 32 bits of ulong[n] → Upper 32 bits of ulong[n]
        // Upper 32 bits of ulong[n-1] → Lower 32 bits of ulong[n]
        const ulong mask32 = 0xFFFFFFFF;
        result._bits15 = (_bits15 << 32) | ((_bits14 >> 32) & mask32);
        result._bits14 = (_bits14 << 32) | ((_bits13 >> 32) & mask32);
        result._bits13 = (_bits13 << 32) | ((_bits12 >> 32) & mask32);
        result._bits12 = (_bits12 << 32) | ((_bits11 >> 32) & mask32);
        result._bits11 = (_bits11 << 32) | ((_bits10 >> 32) & mask32);
        result._bits10 = (_bits10 << 32) | ((_bits9 >> 32) & mask32);
        result._bits9 = (_bits9 << 32) | ((_bits8 >> 32) & mask32);
        result._bits8 = (_bits8 << 32) | ((_bits7 >> 32) & mask32);
        result._bits7 = (_bits7 << 32) | ((_bits6 >> 32) & mask32);
        result._bits6 = (_bits6 << 32) | ((_bits5 >> 32) & mask32);
        result._bits5 = (_bits5 << 32) | ((_bits4 >> 32) & mask32);
        result._bits4 = (_bits4 << 32) | ((_bits3 >> 32) & mask32);
        result._bits3 = (_bits3 << 32) | ((_bits2 >> 32) & mask32);
        result._bits2 = (_bits2 << 32) | ((_bits1 >> 32) & mask32);
        result._bits1 = (_bits1 << 32) | ((_bits0 >> 32) & mask32);
        result._bits0 = _bits0 << 32; // New top, lower32 comes from 0
        return result;
    }

    /// <summary>
    /// Shift diagonally up-left
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public BitBoard ShiftUpLeft() => ShiftUp().ShiftLeft();

    /// <summary>
    /// Shift diagonally up-right
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public BitBoard ShiftUpRight() => ShiftUp().ShiftRight();

    /// <summary>
    /// Shift diagonally down-left
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public BitBoard ShiftDownLeft() => ShiftDown().ShiftLeft();

    /// <summary>
    /// Shift diagonally down-right
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public BitBoard ShiftDownRight() => ShiftDown().ShiftRight();
}
