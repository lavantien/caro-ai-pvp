using System.Buffers;
using System.Buffers.Binary;
using System.Text;
using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Exports opening book to compact binary format (.cobook).
/// Provides ~4x size reduction and ~10x faster load compared to JSON.
/// </summary>
public sealed class BinaryBookExporter
{
    /// <summary>
    /// Export result containing statistics about the export operation.
    /// </summary>
    public sealed record ExportResult(
        int EntriesExported,
        int TotalMoves,
        int BytesWritten,
        int MaxDepth,
        TimeSpan Duration
    );

    /// <summary>
    /// Export opening book entries to binary format.
    /// </summary>
    /// <param name="entries">Entries to export</param>
    /// <param name="outputPath">Output file path (.cobook)</param>
    /// <returns>Export result with statistics</returns>
    public ExportResult Export(IEnumerable<OpeningBookEntry> entries, string outputPath)
    {
        var startTime = DateTimeOffset.UtcNow;
        var entryList = entries.ToList();

        int totalMoves = 0;
        int maxDepth = 0;

        // First, write all entries to a memory stream
        using var entryStream = new MemoryStream();
        foreach (var entry in entryList)
        {
            WriteEntry(entryStream, entry);
            totalMoves += entry.Moves.Length;
            maxDepth = Math.Max(maxDepth, entry.Depth);
        }

        var entryData = entryStream.ToArray();
        var checksum = BinaryBookFormat.ComputeChecksum(entryData);

        // Now write to file with header + entries + footer
        using var fileStream = new FileStream(outputPath, FileMode.Create, FileAccess.Write, FileShare.None);
        using var writer = new BinaryWriter(fileStream, Encoding.UTF8, leaveOpen: true);

        // Write header
        var headerBuffer = new byte[BinaryBookFormat.HeaderSize];
        BinaryBookFormat.WriteHeader(headerBuffer, entryList.Count, totalMoves, startTime.UtcTicks, maxDepth);
        fileStream.Write(headerBuffer, 0, headerBuffer.Length);

        // Write entries
        fileStream.Write(entryData, 0, entryData.Length);

        // Write footer
        var footerBuffer = new byte[BinaryBookFormat.FooterSize];
        BinaryBookFormat.WriteFooter(footerBuffer, entryList.Count, totalMoves, checksum);
        fileStream.Write(footerBuffer, 0, footerBuffer.Length);

        var duration = DateTimeOffset.UtcNow - startTime;

        return new ExportResult(
            EntriesExported: entryList.Count,
            TotalMoves: totalMoves,
            BytesWritten: (int)fileStream.Length,
            MaxDepth: maxDepth,
            Duration: duration
        );
    }

    private static void WriteEntry(Stream stream, OpeningBookEntry entry)
    {
        using var buffer = new PooledBuffer(1024);
        var span = buffer.Span;
        int offset = 0;

        // Fixed-size fields
        BinaryPrimitives.WriteUInt64LittleEndian(span.Slice(offset), entry.CanonicalHash);
        offset += 8;
        BinaryPrimitives.WriteUInt64LittleEndian(span.Slice(offset), entry.DirectHash);
        offset += 8;

        // Variable-size fields
        offset += BinaryBookFormat.WriteVarInt32(span.Slice(offset), entry.Depth);
        span[offset++] = (byte)entry.Player;
        span[offset++] = (byte)entry.Symmetry;
        span[offset++] = (byte)((entry.IsNearEdge ? 1 : 0) | (entry.Moves.Length > 255 ? 2 : 0));

        // Move count (1 or 2 bytes depending on flag)
        if (entry.Moves.Length > 255)
        {
            BinaryPrimitives.WriteUInt16LittleEndian(span.Slice(offset), (ushort)entry.Moves.Length);
            offset += 2;
        }
        else
        {
            span[offset++] = (byte)entry.Moves.Length;
        }

        // Write moves
        foreach (var move in entry.Moves)
        {
            offset += WriteMove(span.Slice(offset), move);
        }

        stream.Write(span.Slice(0, offset));
    }

    private static int WriteMove(Span<byte> buffer, BookMove move)
    {
        int offset = 0;

        offset += BinaryBookFormat.WriteVarInt32(buffer.Slice(offset), move.RelativeX);
        offset += BinaryBookFormat.WriteVarInt32(buffer.Slice(offset), move.RelativeY);
        buffer[offset++] = (byte)move.WinRate;
        offset += BinaryBookFormat.WriteVarInt32(buffer.Slice(offset), move.DepthAchieved);
        offset += BinaryBookFormat.WriteVarInt64(buffer.Slice(offset), move.NodesSearched);
        offset += BinaryBookFormat.WriteVarInt64(buffer.Slice(offset), move.Score);

        // Pack flags into single byte
        byte flags = (byte)(
            (move.IsForcing ? 1 : 0) |
            (move.IsVerified ? 2 : 0) |
            ((int)move.Source << 2) |
            ((move.Priority & 0x0F) << 4)
        );
        buffer[offset++] = flags;

        // Optional fields (may be zero)
        if (move.ScoreDelta != 0 || move.WinCount != 0 || move.PlayCount != 0)
        {
            offset += BinaryBookFormat.WriteVarInt32(buffer.Slice(offset), move.ScoreDelta + 500); // Shift to make positive
            offset += BinaryBookFormat.WriteVarInt32(buffer.Slice(offset), move.WinCount);
            offset += BinaryBookFormat.WriteVarInt32(buffer.Slice(offset), move.PlayCount);
        }

        return offset;
    }

    private sealed class PooledBuffer : IDisposable
    {
        private readonly byte[] _buffer;
        private bool _disposed;

        public PooledBuffer(int size)
        {
            _buffer = ArrayPool<byte>.Shared.Rent(size);
        }

        public Span<byte> Span => _buffer.AsSpan();

        public void Dispose()
        {
            if (!_disposed)
            {
                ArrayPool<byte>.Shared.Return(_buffer);
                _disposed = true;
            }
        }
    }
}
