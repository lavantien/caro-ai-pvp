using System.Numerics;
using System.Runtime.CompilerServices;
using Caro.Core.Domain.Configuration;

namespace Caro.Core.Domain.ValueObjects;

/// <summary>
/// Immutable BitBoard representation for 32x32 Caro board.
/// Uses 16 ulongs (1024 bits total) for 1024 cells.
/// Bit mapping: bit_index = y * 32 + x
/// </summary>
public readonly record struct BitBoard
{
    private readonly ulong _bits0;
    private readonly ulong _bits1;
    private readonly ulong _bits2;
    private readonly ulong _bits3;
    private readonly ulong _bits4;
    private readonly ulong _bits5;
    private readonly ulong _bits6;
    private readonly ulong _bits7;
    private readonly ulong _bits8;
    private readonly ulong _bits9;
    private readonly ulong _bits10;
    private readonly ulong _bits11;
    private readonly ulong _bits12;
    private readonly ulong _bits13;
    private readonly ulong _bits14;
    private readonly ulong _bits15;

    /// <summary>
    /// Board size (32x32)
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
        _bits0 = 0; _bits1 = 0; _bits2 = 0; _bits3 = 0;
        _bits4 = 0; _bits5 = 0; _bits6 = 0; _bits7 = 0;
        _bits8 = 0; _bits9 = 0; _bits10 = 0; _bits11 = 0;
        _bits12 = 0; _bits13 = 0; _bits14 = 0; _bits15 = 0;
    }

    /// <summary>
    /// Static empty BitBoard instance
    /// </summary>
    public static readonly BitBoard Empty = new();

    /// <summary>
    /// Create BitBoard from existing ulong values (16 ulongs for 32x32)
    /// </summary>
    public BitBoard(ulong bits0, ulong bits1, ulong bits2, ulong bits3, ulong bits4, ulong bits5,
                    ulong bits6, ulong bits7, ulong bits8, ulong bits9, ulong bits10, ulong bits11,
                    ulong bits12, ulong bits13, ulong bits14, ulong bits15)
    {
        _bits0 = bits0; _bits1 = bits1; _bits2 = bits2; _bits3 = bits3;
        _bits4 = bits4; _bits5 = bits5; _bits6 = bits6; _bits7 = bits7;
        _bits8 = bits8; _bits9 = bits9; _bits10 = bits10; _bits11 = bits11;
        _bits12 = bits12; _bits13 = bits13; _bits14 = bits14; _bits15 = bits15;
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
    /// Return a new BitBoard with the bit at (x, y) set to the given value
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public readonly BitBoard WithBit(int x, int y, bool value)
    {
        int index = y * Size + x;
        int ulongIndex = index >> 6;
        int bitIndex = index & 0x3F;

        ulong mask = 1UL << bitIndex;
        ulong newBit = value ? mask : 0UL;

        return ulongIndex switch
        {
            0 => new BitBoard(_bits0 & ~mask | newBit, _bits1, _bits2, _bits3, _bits4, _bits5,
                              _bits6, _bits7, _bits8, _bits9, _bits10, _bits11, _bits12, _bits13, _bits14, _bits15),
            1 => new BitBoard(_bits0, _bits1 & ~mask | newBit, _bits2, _bits3, _bits4, _bits5,
                              _bits6, _bits7, _bits8, _bits9, _bits10, _bits11, _bits12, _bits13, _bits14, _bits15),
            2 => new BitBoard(_bits0, _bits1, _bits2 & ~mask | newBit, _bits3, _bits4, _bits5,
                              _bits6, _bits7, _bits8, _bits9, _bits10, _bits11, _bits12, _bits13, _bits14, _bits15),
            3 => new BitBoard(_bits0, _bits1, _bits2, _bits3 & ~mask | newBit, _bits4, _bits5,
                              _bits6, _bits7, _bits8, _bits9, _bits10, _bits11, _bits12, _bits13, _bits14, _bits15),
            4 => new BitBoard(_bits0, _bits1, _bits2, _bits3, _bits4 & ~mask | newBit, _bits5,
                              _bits6, _bits7, _bits8, _bits9, _bits10, _bits11, _bits12, _bits13, _bits14, _bits15),
            5 => new BitBoard(_bits0, _bits1, _bits2, _bits3, _bits4, _bits5 & ~mask | newBit,
                              _bits6, _bits7, _bits8, _bits9, _bits10, _bits11, _bits12, _bits13, _bits14, _bits15),
            6 => new BitBoard(_bits0, _bits1, _bits2, _bits3, _bits4, _bits5,
                              _bits6 & ~mask | newBit, _bits7, _bits8, _bits9, _bits10, _bits11, _bits12, _bits13, _bits14, _bits15),
            7 => new BitBoard(_bits0, _bits1, _bits2, _bits3, _bits4, _bits5,
                              _bits6, _bits7 & ~mask | newBit, _bits8, _bits9, _bits10, _bits11, _bits12, _bits13, _bits14, _bits15),
            8 => new BitBoard(_bits0, _bits1, _bits2, _bits3, _bits4, _bits5,
                              _bits6, _bits7, _bits8 & ~mask | newBit, _bits9, _bits10, _bits11, _bits12, _bits13, _bits14, _bits15),
            9 => new BitBoard(_bits0, _bits1, _bits2, _bits3, _bits4, _bits5,
                              _bits6, _bits7, _bits8, _bits9 & ~mask | newBit, _bits10, _bits11, _bits12, _bits13, _bits14, _bits15),
            10 => new BitBoard(_bits0, _bits1, _bits2, _bits3, _bits4, _bits5,
                               _bits6, _bits7, _bits8, _bits9, _bits10 & ~mask | newBit, _bits11, _bits12, _bits13, _bits14, _bits15),
            11 => new BitBoard(_bits0, _bits1, _bits2, _bits3, _bits4, _bits5,
                               _bits6, _bits7, _bits8, _bits9, _bits10, _bits11 & ~mask | newBit, _bits12, _bits13, _bits14, _bits15),
            12 => new BitBoard(_bits0, _bits1, _bits2, _bits3, _bits4, _bits5,
                               _bits6, _bits7, _bits8, _bits9, _bits10, _bits11, _bits12 & ~mask | newBit, _bits13, _bits14, _bits15),
            13 => new BitBoard(_bits0, _bits1, _bits2, _bits3, _bits4, _bits5,
                               _bits6, _bits7, _bits8, _bits9, _bits10, _bits11, _bits12, _bits13 & ~mask | newBit, _bits14, _bits15),
            14 => new BitBoard(_bits0, _bits1, _bits2, _bits3, _bits4, _bits5,
                               _bits6, _bits7, _bits8, _bits9, _bits10, _bits11, _bits12, _bits13, _bits14 & ~mask | newBit, _bits15),
            15 => new BitBoard(_bits0, _bits1, _bits2, _bits3, _bits4, _bits5,
                               _bits6, _bits7, _bits8, _bits9, _bits10, _bits11, _bits12, _bits13, _bits14, _bits15 & ~mask | newBit),
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
    public readonly bool IsEmpty => _bits0 == 0 && _bits1 == 0 && _bits2 == 0 && _bits3 == 0 &&
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
    /// Get raw ulong values (for serialization/hashing)
    /// </summary>
    public readonly (ulong b0, ulong b1, ulong b2, ulong b3, ulong b4, ulong b5,
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
            _bits0 >> 1, (_bits0 & 1UL) << 63 | _bits1 >> 1,
            (_bits1 & 1UL) << 63 | _bits2 >> 1, (_bits2 & 1UL) << 63 | _bits3 >> 1,
            (_bits3 & 1UL) << 63 | _bits4 >> 1, (_bits4 & 1UL) << 63 | _bits5 >> 1,
            (_bits5 & 1UL) << 63 | _bits6 >> 1, (_bits6 & 1UL) << 63 | _bits7 >> 1,
            (_bits7 & 1UL) << 63 | _bits8 >> 1, (_bits8 & 1UL) << 63 | _bits9 >> 1,
            (_bits9 & 1UL) << 63 | _bits10 >> 1, (_bits10 & 1UL) << 63 | _bits11 >> 1,
            (_bits11 & 1UL) << 63 | _bits12 >> 1, (_bits12 & 1UL) << 63 | _bits13 >> 1,
            (_bits13 & 1UL) << 63 | _bits14 >> 1, (_bits14 & 1UL) << 63 | _bits15 >> 1
        );
    }

    /// <summary>
    /// Shift all bits right (increase x)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public readonly BitBoard ShiftRight()
    {
        return new BitBoard(
            _bits1 >> 63 << 63 | _bits0 << 1, _bits2 >> 63 << 63 | _bits1 << 1,
            _bits3 >> 63 << 63 | _bits2 << 1, _bits4 >> 63 << 63 | _bits3 << 1,
            _bits5 >> 63 << 63 | _bits4 << 1, _bits6 >> 63 << 63 | _bits5 << 1,
            _bits7 >> 63 << 63 | _bits6 << 1, _bits8 >> 63 << 63 | _bits7 << 1,
            _bits9 >> 63 << 63 | _bits8 << 1, _bits10 >> 63 << 63 | _bits9 << 1,
            _bits11 >> 63 << 63 | _bits10 << 1, _bits12 >> 63 << 63 | _bits11 << 1,
            _bits13 >> 63 << 63 | _bits12 << 1, _bits14 >> 63 << 63 | _bits13 << 1,
            _bits15 >> 63 << 63 | _bits14 << 1, _bits15 << 1
        );
    }

    /// <summary>
    /// Shift all bits up (decrease y) - shift by 32 bits (one row for 32x32)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public readonly BitBoard ShiftUp()
    {
        return new BitBoard(
            _bits2, _bits3, _bits4, _bits5, _bits6, _bits7,
            _bits8, _bits9, _bits10, _bits11, _bits12, _bits13,
            _bits14, _bits15, 0, 0
        );
    }

    /// <summary>
    /// Shift all bits down (increase y) - shift by 32 bits (one row for 32x32)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public readonly BitBoard ShiftDown()
    {
        return new BitBoard(
            0, 0, _bits0, _bits1, _bits2, _bits3,
            _bits4, _bits5, _bits6, _bits7, _bits8, _bits9,
            _bits10, _bits11, _bits12, _bits13
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
}
