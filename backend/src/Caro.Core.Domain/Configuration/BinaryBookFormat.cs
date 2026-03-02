using System.Buffers.Binary;
using System.Runtime.CompilerServices;
using System.Security.Cryptography;

namespace Caro.Core.Domain.Configuration;

/// <summary>
/// Binary format for opening book storage (.cobook).
/// Provides ~4x size reduction and ~10x faster load compared to JSON.
///
/// Format specification:
/// - Header (32 bytes): Magic, Version, Flags, EntryCount, TotalMoves, GeneratedAt, MaxDepth
/// - Entries (variable): CanonicalHash, DirectHash, Depth, Player, Symmetry, Flags, Moves
/// - Footer (20 bytes): Magic, EntryCount, TotalMoves, Checksum
///
/// All multi-byte integers are little-endian.
/// Variable-length integers use varint encoding for compactness.
/// </summary>
public static class BinaryBookFormat
{
    // Magic numbers for format identification
    public const uint MagicHeader = 0x4B424F43; // "COBK" in little-endian
    public const uint MagicFooter = 0x45424F43; // "COBE" in little-endian

    // Current format version
    public const ushort CurrentVersion = 1;

    // Header size is fixed at 32 bytes
    public const int HeaderSize = 32;

    // Footer size is fixed at 20 bytes
    public const int FooterSize = 20;

    // Maximum supported board size (19x19 for Caro)
    public const int MaxBoardSize = 19;

    // Maximum moves per position
    public const int MaxMovesPerPosition = 16;

    /// <summary>
    /// Write header to buffer (32 bytes).
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static void WriteHeader(Span<byte> buffer, int entryCount, int totalMoves, long generatedAtTicks, int maxDepth)
    {
        if (buffer.Length < HeaderSize)
            throw new ArgumentException($"Buffer must be at least {HeaderSize} bytes", nameof(buffer));

        BinaryPrimitives.WriteUInt32LittleEndian(buffer, MagicHeader);
        BinaryPrimitives.WriteUInt16LittleEndian(buffer.Slice(4), CurrentVersion);
        BinaryPrimitives.WriteUInt16LittleEndian(buffer.Slice(6), 0); // Flags (reserved)
        BinaryPrimitives.WriteInt32LittleEndian(buffer.Slice(8), entryCount);
        BinaryPrimitives.WriteInt32LittleEndian(buffer.Slice(12), totalMoves);
        BinaryPrimitives.WriteInt64LittleEndian(buffer.Slice(16), generatedAtTicks);
        BinaryPrimitives.WriteInt32LittleEndian(buffer.Slice(24), maxDepth);
        BinaryPrimitives.WriteInt32LittleEndian(buffer.Slice(28), 0); // Reserved
    }

    /// <summary>
    /// Read header from buffer (32 bytes).
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static (int EntryCount, int TotalMoves, long GeneratedAtTicks, int MaxDepth) ReadHeader(ReadOnlySpan<byte> buffer)
    {
        if (buffer.Length < HeaderSize)
            throw new ArgumentException($"Buffer must be at least {HeaderSize} bytes", nameof(buffer));

        var magic = BinaryPrimitives.ReadUInt32LittleEndian(buffer);
        if (magic != MagicHeader)
            throw new InvalidDataException($"Invalid magic header: expected 0x{MagicHeader:X8}, got 0x{magic:X8}");

        var version = BinaryPrimitives.ReadUInt16LittleEndian(buffer.Slice(4));
        if (version > CurrentVersion)
            throw new InvalidDataException($"Unsupported version: {version}");

        var entryCount = BinaryPrimitives.ReadInt32LittleEndian(buffer.Slice(8));
        var totalMoves = BinaryPrimitives.ReadInt32LittleEndian(buffer.Slice(12));
        var generatedAtTicks = BinaryPrimitives.ReadInt64LittleEndian(buffer.Slice(16));
        var maxDepth = BinaryPrimitives.ReadInt32LittleEndian(buffer.Slice(24));

        return (entryCount, totalMoves, generatedAtTicks, maxDepth);
    }

    /// <summary>
    /// Write footer to buffer (20 bytes).
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static void WriteFooter(Span<byte> buffer, int entryCount, int totalMoves, ulong checksum)
    {
        if (buffer.Length < FooterSize)
            throw new ArgumentException($"Buffer must be at least {FooterSize} bytes", nameof(buffer));

        BinaryPrimitives.WriteUInt32LittleEndian(buffer, MagicFooter);
        BinaryPrimitives.WriteInt32LittleEndian(buffer.Slice(4), entryCount);
        BinaryPrimitives.WriteInt32LittleEndian(buffer.Slice(8), totalMoves);
        BinaryPrimitives.WriteUInt64LittleEndian(buffer.Slice(12), checksum);
    }

    /// <summary>
    /// Read footer from buffer (20 bytes).
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static (int EntryCount, int TotalMoves, ulong Checksum) ReadFooter(ReadOnlySpan<byte> buffer)
    {
        if (buffer.Length < FooterSize)
            throw new ArgumentException($"Buffer must be at least {FooterSize} bytes", nameof(buffer));

        var magic = BinaryPrimitives.ReadUInt32LittleEndian(buffer);
        if (magic != MagicFooter)
            throw new InvalidDataException($"Invalid magic footer: expected 0x{MagicFooter:X8}, got 0x{magic:X8}");

        var entryCount = BinaryPrimitives.ReadInt32LittleEndian(buffer.Slice(4));
        var totalMoves = BinaryPrimitives.ReadInt32LittleEndian(buffer.Slice(8));
        var checksum = BinaryPrimitives.ReadUInt64LittleEndian(buffer.Slice(12));

        return (entryCount, totalMoves, checksum);
    }

    /// <summary>
    /// Compute XXH64-like checksum of data.
    /// Uses a fast hash algorithm for integrity checking (not cryptographic).
    /// </summary>
    public static ulong ComputeChecksum(ReadOnlySpan<byte> data)
    {
        // FNV-1a variant for fast checksum
        // Not cryptographic but good for integrity detection
        const ulong FnvOffsetBasis = 14695981039346656037;
        const ulong FnvPrime = 1099511628211;

        ulong hash = FnvOffsetBasis;
        foreach (byte b in data)
        {
            hash ^= b;
            hash *= FnvPrime;
        }

        return hash;
    }

    #region Varint Encoding

    /// <summary>
    /// Write unsigned varint (1-10 bytes).
    /// Uses 7 bits per byte, MSB indicates continuation.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int WriteVarUInt64(Span<byte> buffer, ulong value)
    {
        int offset = 0;
        while (value >= 0x80)
        {
            buffer[offset++] = (byte)(value | 0x80);
            value >>= 7;
        }
        buffer[offset++] = (byte)value;
        return offset;
    }

    /// <summary>
    /// Read unsigned varint (1-10 bytes).
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static (ulong Value, int BytesRead) ReadVarUInt64(ReadOnlySpan<byte> buffer)
    {
        ulong result = 0;
        int shift = 0;
        int offset = 0;

        while (offset < buffer.Length)
        {
            byte b = buffer[offset++];
            result |= (ulong)(b & 0x7F) << shift;
            if ((b & 0x80) == 0)
                break;
            shift += 7;
            if (shift >= 70)
                throw new InvalidDataException("Varint too long");
        }

        return (result, offset);
    }

    /// <summary>
    /// Write signed varint using zigzag encoding.
    /// Maps negative numbers to positive: 0->0, -1->1, 1->2, -2->3, 2->4, ...
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int WriteVarInt64(Span<byte> buffer, long value)
    {
        // Zigzag encoding: (n << 1) ^ (n >> 63)
        var zigzag = (ulong)((value << 1) ^ (value >> 63));
        return WriteVarUInt64(buffer, zigzag);
    }

    /// <summary>
    /// Read signed varint using zigzag decoding.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static (long Value, int BytesRead) ReadVarInt64(ReadOnlySpan<byte> buffer)
    {
        var (zigzag, bytesRead) = ReadVarUInt64(buffer);
        // Zigzag decode: (n >> 1) ^ -(n & 1)
        var value = (long)(zigzag >> 1) ^ (-(long)(zigzag & 1));
        return (value, bytesRead);
    }

    /// <summary>
    /// Write int32 as varint (for positive values like counts).
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int WriteVarInt32(Span<byte> buffer, int value)
    {
        if (value < 0)
            throw new ArgumentOutOfRangeException(nameof(value), "Value must be non-negative");
        return WriteVarUInt64(buffer, (ulong)value);
    }

    /// <summary>
    /// Read int32 as varint.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static (int Value, int BytesRead) ReadVarInt32(ReadOnlySpan<byte> buffer)
    {
        var (value, bytesRead) = ReadVarUInt64(buffer);
        if (value > int.MaxValue)
            throw new InvalidDataException($"Varint value {value} exceeds int.MaxValue");
        return ((int)value, bytesRead);
    }

    /// <summary>
    /// Get encoded size of int32 as varint.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int GetVarInt32Size(int value)
    {
        if (value < 0)
            throw new ArgumentOutOfRangeException(nameof(value), "Value must be non-negative");
        return GetVarUInt64Size((ulong)value);
    }

    /// <summary>
    /// Get encoded size of unsigned varint.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int GetVarUInt64Size(ulong value)
    {
        int size = 1;
        while (value >= 0x80)
        {
            size++;
            value >>= 7;
        }
        return size;
    }

    /// <summary>
    /// Get encoded size of signed varint (zigzag).
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int GetVarInt64Size(long value)
    {
        var zigzag = (ulong)((value << 1) ^ (value >> 63));
        return GetVarUInt64Size(zigzag);
    }

    #endregion
}
