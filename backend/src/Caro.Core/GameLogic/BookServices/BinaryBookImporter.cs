using System.Buffers.Binary;
using System.Text;
using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Imports opening book from compact binary format (.cobook).
/// </summary>
public sealed class BinaryBookImporter
{
    /// <summary>
    /// Import result containing loaded entries and statistics.
    /// </summary>
    public sealed record ImportResult(
        List<OpeningBookEntry> Entries,
        int TotalMoves,
        int MaxDepth,
        TimeSpan Duration
    );

    /// <summary>
    /// Validation result for binary book files.
    /// </summary>
    public sealed record BinaryBookValidationResult(
        bool IsValid,
        int EntryCount,
        int TotalMoves,
        string? ErrorMessage
    );

    /// <summary>
    /// Import opening book entries from binary format.
    /// </summary>
    /// <param name="inputPath">Input file path (.cobook)</param>
    /// <param name="verifyChecksum">Whether to verify checksum during import</param>
    /// <returns>Import result with loaded entries</returns>
    public ImportResult Import(string inputPath, bool verifyChecksum = false)
    {
        var startTime = DateTimeOffset.UtcNow;

        using var stream = new FileStream(inputPath, FileMode.Open, FileAccess.Read, FileShare.Read);

        // Read header
        var headerBuffer = new byte[BinaryBookFormat.HeaderSize];
        stream.ReadExactly(headerBuffer, 0, headerBuffer.Length);
        var (entryCount, totalMoves, _, maxDepth) = BinaryBookFormat.ReadHeader(headerBuffer);

        // Read all data for optional checksum verification
        var dataLength = (int)(stream.Length - BinaryBookFormat.HeaderSize - BinaryBookFormat.FooterSize);
        var dataBuffer = new byte[dataLength];
        stream.Position = BinaryBookFormat.HeaderSize;
        stream.ReadExactly(dataBuffer, 0, dataBuffer.Length);

        // Read footer
        var footerBuffer = new byte[BinaryBookFormat.FooterSize];
        stream.ReadExactly(footerBuffer, 0, footerBuffer.Length);
        var (footerEntryCount, footerTotalMoves, checksum) = BinaryBookFormat.ReadFooter(footerBuffer);

        if (verifyChecksum)
        {
            var computedChecksum = BinaryBookFormat.ComputeChecksum(dataBuffer);
            if (computedChecksum != checksum)
            {
                throw new InvalidDataException(
                    $"Checksum mismatch: expected 0x{checksum:X16}, got 0x{computedChecksum:X16}");
            }
        }

        // Parse entries
        var entries = new List<OpeningBookEntry>(entryCount);
        var span = dataBuffer.AsSpan();
        int offset = 0;

        for (int i = 0; i < entryCount; i++)
        {
            var (entry, bytesRead) = ReadEntry(span.Slice(offset));
            entries.Add(entry);
            offset += bytesRead;
        }

        var duration = DateTimeOffset.UtcNow - startTime;

        return new ImportResult(
            Entries: entries,
            TotalMoves: totalMoves,
            MaxDepth: maxDepth,
            Duration: duration
        );
    }

    /// <summary>
    /// Validate a binary book file without fully loading it.
    /// </summary>
    /// <param name="inputPath">Input file path (.cobook)</param>
    /// <returns>Validation result</returns>
    public BinaryBookValidationResult Validate(string inputPath)
    {
        try
        {
            if (!File.Exists(inputPath))
            {
                return new BinaryBookValidationResult(false, 0, 0, "File not found");
            }

            using var stream = new FileStream(inputPath, FileMode.Open, FileAccess.Read, FileShare.Read);

            if (stream.Length < BinaryBookFormat.HeaderSize + BinaryBookFormat.FooterSize)
            {
                return new BinaryBookValidationResult(false, 0, 0, "File too small");
            }

            // Read header
            var headerBuffer = new byte[BinaryBookFormat.HeaderSize];
            stream.ReadExactly(headerBuffer, 0, headerBuffer.Length);
            var (entryCount, totalMoves, _, _) = BinaryBookFormat.ReadHeader(headerBuffer);

            return new BinaryBookValidationResult(true, entryCount, totalMoves, null);
        }
        catch (InvalidDataException ex)
        {
            return new BinaryBookValidationResult(false, 0, 0, ex.Message);
        }
        catch (Exception ex)
        {
            return new BinaryBookValidationResult(false, 0, 0, ex.Message);
        }
    }

    private static (OpeningBookEntry Entry, int BytesRead) ReadEntry(ReadOnlySpan<byte> buffer)
    {
        int offset = 0;

        // Fixed-size fields
        var canonicalHash = BinaryPrimitives.ReadUInt64LittleEndian(buffer.Slice(offset));
        offset += 8;
        var directHash = BinaryPrimitives.ReadUInt64LittleEndian(buffer.Slice(offset));
        offset += 8;

        // Variable-size fields
        var (depth, depthBytes) = BinaryBookFormat.ReadVarInt32(buffer.Slice(offset));
        offset += depthBytes;

        var player = (Player)buffer[offset++];
        var symmetry = (SymmetryType)buffer[offset++];

        var flags = buffer[offset++];
        var isNearEdge = (flags & 1) != 0;
        var hasLargeMoveCount = (flags & 2) != 0;

        int moveCount;
        if (hasLargeMoveCount)
        {
            moveCount = BinaryPrimitives.ReadUInt16LittleEndian(buffer.Slice(offset));
            offset += 2;
        }
        else
        {
            moveCount = buffer[offset++];
        }

        // Read moves
        var moves = new BookMove[moveCount];
        for (int i = 0; i < moveCount; i++)
        {
            var (move, moveBytes) = ReadMove(buffer.Slice(offset));
            moves[i] = move;
            offset += moveBytes;
        }

        var entry = new OpeningBookEntry
        {
            CanonicalHash = canonicalHash,
            DirectHash = directHash,
            Depth = depth,
            Player = player,
            Symmetry = symmetry,
            IsNearEdge = isNearEdge,
            Moves = moves
        };

        return (entry, offset);
    }

    private static (BookMove Move, int BytesRead) ReadMove(ReadOnlySpan<byte> buffer)
    {
        int offset = 0;

        var (relativeX, xBytes) = BinaryBookFormat.ReadVarInt32(buffer.Slice(offset));
        offset += xBytes;

        var (relativeY, yBytes) = BinaryBookFormat.ReadVarInt32(buffer.Slice(offset));
        offset += yBytes;

        var winRate = buffer[offset++];

        var (depthAchieved, depthBytes) = BinaryBookFormat.ReadVarInt32(buffer.Slice(offset));
        offset += depthBytes;

        var (nodesSearched, nodesBytes) = BinaryBookFormat.ReadVarInt64(buffer.Slice(offset));
        offset += nodesBytes;

        var (score, scoreBytes) = BinaryBookFormat.ReadVarInt64(buffer.Slice(offset));
        offset += scoreBytes;

        var flags = buffer[offset++];
        var isForcing = (flags & 1) != 0;
        var isVerified = (flags & 2) != 0;
        var source = (MoveSource)((flags >> 2) & 0x03);
        var priority = (flags >> 4) & 0x0F;

        // Check if optional fields are present by trying to read more data
        int scoreDelta = 0;
        int winCount = 0;
        int playCount = 0;

        if (offset < buffer.Length)
        {
            var (delta, deltaBytes) = BinaryBookFormat.ReadVarInt32(buffer.Slice(offset));
            scoreDelta = delta - 500; // Shift back
            offset += deltaBytes;

            var (wins, winsBytes) = BinaryBookFormat.ReadVarInt32(buffer.Slice(offset));
            winCount = wins;
            offset += winsBytes;

            var (plays, playsBytes) = BinaryBookFormat.ReadVarInt32(buffer.Slice(offset));
            playCount = plays;
            offset += playsBytes;
        }

        var move = new BookMove
        {
            RelativeX = relativeX,
            RelativeY = relativeY,
            WinRate = winRate,
            DepthAchieved = depthAchieved,
            NodesSearched = nodesSearched,
            Score = (int)score,
            IsForcing = isForcing,
            IsVerified = isVerified,
            Source = source,
            Priority = priority,
            ScoreDelta = scoreDelta,
            WinCount = winCount,
            PlayCount = playCount
        };

        return (move, offset);
    }
}
