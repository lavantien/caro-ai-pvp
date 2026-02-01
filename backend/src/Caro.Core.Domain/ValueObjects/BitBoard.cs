using System.Numerics;
using System.Runtime.CompilerServices;

namespace Caro.Core.Domain.ValueObjects;

/// <summary>
/// Immutable BitBoard representation for 19x19 Caro board.
/// Uses 6 ulongs (384 bits total) for 361 cells.
/// Bit mapping: bit_index = y * 19 + x
/// </summary>
public readonly record struct BitBoard
{
    private readonly ulong _bits0;
    private readonly ulong _bits1;
    private readonly ulong _bits2;
    private readonly ulong _bits3;
    private readonly ulong _bits4;
    private readonly ulong _bits5;

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
    /// Static empty BitBoard instance
    /// </summary>
    public static readonly BitBoard Empty = new();

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
    /// Private constructor for efficient WithXxx methods
    /// </summary>
    private BitBoard(
        ulong bits0, ulong bits1, ulong bits2, ulong bits3, ulong bits4, ulong bits5,
        byte _) // Dummy parameter to distinguish from public constructor
    {
        _bits0 = bits0;
        _bits1 = bits1;
        _bits2 = bits2;
        _bits3 = bits3;
        _bits4 = bits4;
        _bits5 = bits5;
    }

    /// <summary>
    /// Get bit at position (x, y)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public readonly bool GetBit(int x, int y)
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
    /// Return a new BitBoard with the bit at (x, y) set to the given value
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public BitBoard WithBit(int x, int y, bool value)
    {
        int index = y * Size + x;
        int ulongIndex = index >> 6;
        int bitIndex = index & 0x3F;

        ulong mask = 1UL << bitIndex;

        return ulongIndex switch
        {
            0 => new BitBoard(
                value ? _bits0 | mask : _bits0 & ~mask,
                _bits1, _bits2, _bits3, _bits4, _bits5, 0),
            1 => new BitBoard(
                _bits0,
                value ? _bits1 | mask : _bits1 & ~mask,
                _bits2, _bits3, _bits4, _bits5, 0),
            2 => new BitBoard(
                _bits0, _bits1,
                value ? _bits2 | mask : _bits2 & ~mask,
                _bits3, _bits4, _bits5, 0),
            3 => new BitBoard(
                _bits0, _bits1, _bits2,
                value ? _bits3 | mask : _bits3 & ~mask,
                _bits4, _bits5, 0),
            4 => new BitBoard(
                _bits0, _bits1, _bits2, _bits3,
                value ? _bits4 | mask : _bits4 & ~mask,
                _bits5, 0),
            5 => new BitBoard(
                _bits0, _bits1, _bits2, _bits3, _bits4,
                value ? _bits5 | mask : _bits5 & ~mask,
                0),
            _ => this
        };
    }

    /// <summary>
    /// Return a new BitBoard with the bit at (x, y) set to 1
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public readonly BitBoard SetBit(int x, int y) => WithBit(x, y, true);

    /// <summary>
    /// Return a new BitBoard with the bit at (x, y) cleared
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public readonly BitBoard ClearBit(int x, int y) => WithBit(x, y, false);

    /// <summary>
    /// Count total set bits (population count)
    /// </summary>
    public readonly int CountBits()
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
    public readonly bool IsEmpty => _bits0 == 0 && _bits1 == 0 && _bits2 == 0 &&
                                    _bits3 == 0 && _bits4 == 0 && _bits5 == 0;

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
    /// Get raw ulong values (for serialization/hashing)
    /// </summary>
    public readonly (ulong b0, ulong b1, ulong b2, ulong b3, ulong b4, ulong b5) GetRawValues() =>
        (_bits0, _bits1, _bits2, _bits3, _bits4, _bits5);

    /// <summary>
    /// Create BitBoard from raw ulong values
    /// </summary>
    public static BitBoard FromRawValues(ulong b0, ulong b1, ulong b2, ulong b3, ulong b4, ulong b5) =>
        new BitBoard(b0, b1, b2, b3, b4, b5);

    /// <summary>
    /// Get all set positions as a list
    /// </summary>
    public readonly List<(int x, int y)> GetSetPositions()
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
    public readonly BitBoard ShiftLeft()
    {
        return new BitBoard(
            (_bits0 >> 1) & 0x7FFFFFFFFFFFFFFFUL,
            ((_bits0 & 1UL) << 63) | (_bits1 >> 1),
            ((_bits1 & 1UL) << 63) | (_bits2 >> 1),
            ((_bits2 & 1UL) << 63) | (_bits3 >> 1),
            ((_bits3 & 1UL) << 63) | (_bits4 >> 1),
            ((_bits4 & 1UL) << 63) | (_bits5 >> 1)
        );
    }

    /// <summary>
    /// Shift all bits right (increase x)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public readonly BitBoard ShiftRight()
    {
        return new BitBoard(
            ((_bits1 >> 63) << 63) | ((_bits0 << 1) & 0xFFFFFFFFFFFFFFFFUL),
            ((_bits2 >> 63) << 63) | ((_bits1 << 1) & 0xFFFFFFFFFFFFFFFFUL),
            ((_bits3 >> 63) << 63) | ((_bits2 << 1) & 0xFFFFFFFFFFFFFFFFUL),
            ((_bits4 >> 63) << 63) | ((_bits3 << 1) & 0xFFFFFFFFFFFFFFFFUL),
            ((_bits5 >> 40) << 63) | ((_bits4 << 1) & 0xFFFFFFFFFFFFFFFFUL),
            (_bits5 << 1) & 0x000001FFFFFFFFFFUL
        );
    }

    /// <summary>
    /// Shift all bits up (decrease y)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public readonly BitBoard ShiftUp()
    {
        return new BitBoard(
            (_bits0 >> 19) | ((_bits1 & 0x7FFFFUL) << 45),
            ((_bits1 >> 19) | ((_bits2 & 0x7FFFFUL) << 45)) & 0xFFFFFFFFFFFFFFFFUL,
            ((_bits2 >> 19) | ((_bits3 & 0x7FFFFUL) << 45)) & 0xFFFFFFFFFFFFFFFFUL,
            ((_bits3 >> 19) | ((_bits4 & 0x7FFFFUL) << 45)) & 0xFFFFFFFFFFFFFFFFUL,
            ((_bits4 >> 19) | ((_bits5 & 0x7FFFFUL) << 45)) & 0xFFFFFFFFFFFFFFFFUL,
            (_bits5 >> 19) & 0x000001FFFFFFFFFFUL
        );
    }

    /// <summary>
    /// Shift all bits down (increase y)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public readonly BitBoard ShiftDown()
    {
        return new BitBoard(
            ((_bits0 << 19) & 0xFFFFFFFFFFFFFFFFUL) | ((_bits1 >> 45) & 0x7FFFFUL),
            ((_bits1 << 19) & 0xFFFFFFFFFFFFFFFFUL) | ((_bits2 >> 45) & 0x7FFFFUL),
            ((_bits2 << 19) & 0xFFFFFFFFFFFFFFFFUL) | ((_bits3 >> 45) & 0x7FFFFUL),
            ((_bits3 << 19) & 0xFFFFFFFFFFFFFFFFUL) | ((_bits4 >> 45) & 0x7FFFFUL),
            ((_bits4 << 19) & 0xFFFFFFFFFFFFFFFFUL) | ((_bits5 >> 45) & 0x7FFFFUL),
            (_bits5 << 19) & 0x000001FFFFFFFFFFUL
        );
    }

    /// <summary>
    /// Shift diagonally up-left
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public readonly BitBoard ShiftUpLeft() => ShiftUp().ShiftLeft();

    /// <summary>
    /// Shift diagonally up-right
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public readonly BitBoard ShiftUpRight() => ShiftUp().ShiftRight();

    /// <summary>
    /// Shift diagonally down-left
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public readonly BitBoard ShiftDownLeft() => ShiftDown().ShiftLeft();

    /// <summary>
    /// Shift diagonally down-right
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public readonly BitBoard ShiftDownRight() => ShiftDown().ShiftRight();

    /// <summary>
    /// Convert to string representation (for debugging)
    /// </summary>
    public override readonly string ToString()
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
}
