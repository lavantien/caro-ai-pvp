using System.Runtime.CompilerServices;
using System.Numerics;
using Caro.Core.Domain.Configuration;

namespace Caro.Core.GameLogic;

/// <summary>
/// BitBoard representation for 16x16 Caro board
/// Layout: 4 ulongs (256 bits total) for 256 cells
/// Bit mapping: bit_index = y * 16 + x
/// </summary>
public struct BitBoard
{
    // 16x16 board = 256 cells
    // Each ulong holds 64 bits, so we need 4 ulongs
    private ulong _bits0;
    private ulong _bits1;
    private ulong _bits2;
    private ulong _bits3;

    /// <summary>
    /// Board size (16x16)
    /// </summary>
    public const int Size = GameConstants.BoardSize;

    /// <summary>
    /// Total number of cells
    /// </summary>
    public const int TotalCells = GameConstants.TotalCells;

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
    /// Create BitBoard from existing ulong values (4 ulongs for 16x16)
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
        int ulongIndex = index >> 6; // index / 64
        int bitIndex = index & 0x3F; // index % 64

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
        int ulongIndex = index >> 6;
        int bitIndex = index & 0x3F;

        ulong mask = 1UL << bitIndex;

        switch (ulongIndex)
        {
            case 0: if (value) _bits0 |= mask; else _bits0 &= ~mask; break;
            case 1: if (value) _bits1 |= mask; else _bits1 &= ~mask; break;
            case 2: if (value) _bits2 |= mask; else _bits2 &= ~mask; break;
            case 3: if (value) _bits3 |= mask; else _bits3 &= ~mask; break;
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
        }
    }

    /// <summary>
    /// Count total set bits (population count)
    /// </summary>
    public int CountBits()
    {
        return BitOperations.PopCount(_bits0) + BitOperations.PopCount(_bits1) +
               BitOperations.PopCount(_bits2) + BitOperations.PopCount(_bits3);
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
            left._bits0 | right._bits0, left._bits1 | right._bits1,
            left._bits2 | right._bits2, left._bits3 | right._bits3
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
            left._bits2 & right._bits2, left._bits3 & right._bits3
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
            left._bits2 ^ right._bits2, left._bits3 ^ right._bits3
        );
    }

    /// <summary>
    /// Bitwise complement operation
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static BitBoard operator ~(BitBoard board)
    {
        return new BitBoard(
            ~board._bits0, ~board._bits1, ~board._bits2, ~board._bits3
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
               _bits2 == other._bits2 && _bits3 == other._bits3;
    }

    /// <summary>
    /// Check equality between two BitBoards
    /// </summary>
    public bool Equals(BitBoard other)
    {
        return _bits0 == other._bits0 && _bits1 == other._bits1 &&
               _bits2 == other._bits2 && _bits3 == other._bits3;
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
        sb.AppendLine("BitBoard (16x16):");
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
    /// Copy bits from another BitBoard
    /// </summary>
    public void CopyFrom(BitBoard source)
    {
        _bits0 = source._bits0;
        _bits1 = source._bits1;
        _bits2 = source._bits2;
        _bits3 = source._bits3;
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
    /// For 16x16 board, shift by 1 bit with wrap-around handling
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public BitBoard ShiftLeft()
    {
        var result = new BitBoard();
        result._bits0 = (_bits0 >> 1) | ((_bits1 & 1UL) << 63);
        result._bits1 = (_bits1 >> 1) | ((_bits2 & 1UL) << 63);
        result._bits2 = (_bits2 >> 1) | ((_bits3 & 1UL) << 63);
        result._bits3 = _bits3 >> 1;
        return result;
    }

    /// <summary>
    /// Shift all bits right (increase x)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public BitBoard ShiftRight()
    {
        var result = new BitBoard();
        result._bits3 = (_bits3 << 1);
        result._bits2 = (_bits2 << 1) | (_bits3 >> 63);
        result._bits1 = (_bits1 << 1) | (_bits2 >> 63);
        result._bits0 = (_bits0 << 1) | (_bits1 >> 63);
        return result;
    }

    /// <summary>
    /// Shift all bits up (decrease y) - shift by 16 bits (one row)
    /// For 16x16 board, each row is 16 bits, each ulong holds 4 rows
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public BitBoard ShiftUp()
    {
        var result = new BitBoard();
        // Shift up: y decreases, so bits move from higher index to lower index
        const ulong mask16 = 0xFFFF;
        result._bits0 = ((_bits0 >> 16) & 0xFFFFFFFFFFFFUL) | ((_bits1 & mask16) << 48);
        result._bits1 = ((_bits1 >> 16) & 0xFFFFFFFFFFFFUL) | ((_bits2 & mask16) << 48);
        result._bits2 = ((_bits2 >> 16) & 0xFFFFFFFFFFFFUL) | ((_bits3 & mask16) << 48);
        result._bits3 = (_bits3 >> 16); // Top row comes from upper bits of bits3, bottom row empty
        return result;
    }

    /// <summary>
    /// Shift all bits down (increase y) - shift by 16 bits (one row)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public BitBoard ShiftDown()
    {
        var result = new BitBoard();
        // Shift down: y increases, so bits move from lower index to higher index
        result._bits3 = (_bits3 << 16) | ((_bits2 >> 48) & 0xFFFF);
        result._bits2 = (_bits2 << 16) | ((_bits1 >> 48) & 0xFFFF);
        result._bits1 = (_bits1 << 16) | ((_bits0 >> 48) & 0xFFFF);
        result._bits0 = _bits0 << 16; // New top, lower bits come from 0
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
